package coordinator

import (
	"time"

	coordinatorinternal "github.com/ezra-sullivan/flx/internal/coordinator"
	linkinternal "github.com/ezra-sullivan/flx/internal/link"
	"github.com/ezra-sullivan/flx/internal/metrics"
	resourceinternal "github.com/ezra-sullivan/flx/internal/resource"
	"github.com/ezra-sullivan/flx/pipeline/observe"
)

// PipelineCoordinatorPolicy configures the coordinator tick strategy.
type PipelineCoordinatorPolicy struct {
	ScaleUpStep         int
	ScaleDownStep       int
	ScaleUpCooldown     time.Duration
	ScaleDownCooldown   time.Duration
	ScaleUpHysteresis   int
	ScaleDownHysteresis int
}

// PipelineCoordinator aggregates pipeline snapshots for one pipeline run and
// exposes a stable read-only view for coordinator decisions. It retains the
// last observed state for the lifetime of the instance, so callers should
// create a fresh coordinator for each new run.
type PipelineCoordinator struct {
	policy           PipelineCoordinatorPolicy
	decisionObserver func(observe.PipelineDecision)
	coord            *coordinatorinternal.Coordinator
}

// NewPipelineCoordinator returns a new pipeline coordinator for one pipeline
// run. The current snapshot model keeps one outbound link view per from-stage,
// so fan-out topologies are not yet represented as distinct links.
func NewPipelineCoordinator(policy PipelineCoordinatorPolicy, opts ...CoordinatorOption) *PipelineCoordinator {
	options := coordinatorOptions{}
	for _, opt := range opts {
		opt(&options)
	}

	return &PipelineCoordinator{
		policy:           policy,
		decisionObserver: options.decisionObserver,
		coord: coordinatorinternal.New(coordinatorinternal.Policy{
			ScaleUpStep:         policy.ScaleUpStep,
			ScaleDownStep:       policy.ScaleDownStep,
			ScaleUpCooldown:     policy.ScaleUpCooldown,
			ScaleDownCooldown:   policy.ScaleDownCooldown,
			ScaleUpHysteresis:   policy.ScaleUpHysteresis,
			ScaleDownHysteresis: policy.ScaleDownHysteresis,
		}, options.resourceObservers...),
	}
}

// Snapshot returns the latest known pipeline snapshot.
func (c *PipelineCoordinator) Snapshot() observe.PipelineSnapshot {
	if c == nil || c.coord == nil {
		return observe.PipelineSnapshot{}
	}

	snapshot := c.coord.Snapshot()

	return observe.PipelineSnapshot{
		Stages:    mapStageMetrics(snapshot.Stages),
		Links:     mapLinkMetrics(snapshot.Links),
		Resources: mapResourceSnapshot(snapshot.Resources),
	}
}

// Tick runs one coordinator policy pass and returns the worker adjustments it
// applied.
func (c *PipelineCoordinator) Tick() observe.PipelineDecision {
	if c == nil || c.coord == nil {
		return observe.PipelineDecision{}
	}

	decision := mapPipelineDecision(c.coord.Tick())
	if c.decisionObserver != nil {
		c.decisionObserver(decision)
	}

	return decision
}

func (c *PipelineCoordinator) internalCoordinator() *coordinatorinternal.Coordinator {
	if c == nil {
		return nil
	}

	return c.coord
}

func mapStageMetrics(input []metrics.StageMetrics) []observe.StageMetrics {
	out := make([]observe.StageMetrics, 0, len(input))
	for _, snapshot := range input {
		out = append(out, observe.StageMetrics{
			StageName:     snapshot.StageName,
			InputCount:    snapshot.InputCount,
			OutputCount:   snapshot.OutputCount,
			ErrorCount:    snapshot.ErrorCount,
			ActiveWorkers: snapshot.ActiveWorkers,
			IdleWorkers:   snapshot.IdleWorkers,
			Backlog:       snapshot.Backlog,
		})
	}

	return out
}

func mapLinkMetrics(input []linkinternal.Metrics) []observe.LinkMetrics {
	out := make([]observe.LinkMetrics, 0, len(input))
	for _, snapshot := range input {
		out = append(out, observe.LinkMetrics{
			FromStage:     snapshot.FromStage,
			ToStage:       snapshot.ToStage,
			SentCount:     snapshot.SentCount,
			ReceivedCount: snapshot.ReceivedCount,
			Backlog:       snapshot.Backlog,
			Capacity:      snapshot.Capacity,
		})
	}

	return out
}

func mapPipelineDecision(input coordinatorinternal.Decision) observe.PipelineDecision {
	out := observe.PipelineDecision{
		Stages: make([]observe.StageDecision, 0, len(input.Stages)),
	}
	for _, decision := range input.Stages {
		out.Stages = append(out.Stages, observe.StageDecision{
			StageName:    decision.StageName,
			FromWorkers:  decision.FromWorkers,
			ToWorkers:    decision.ToWorkers,
			Reason:       decision.Reason,
			StageBacklog: decision.StageBacklog,
			LinkBacklog:  decision.LinkBacklog,
		})
	}

	return out
}

func mapResourceSnapshot(input resourceinternal.Snapshot) observe.ResourceSnapshot {
	out := observe.ResourceSnapshot{
		Samples: make([]observe.PressureSample, 0, len(input.Samples)),
	}
	for _, sample := range input.Samples {
		out.Samples = append(out.Samples, observe.PressureSample{
			Name:  sample.Name,
			Level: mapObservePressureLevel(sample.Level),
			Value: sample.Value,
			Limit: sample.Limit,
		})
	}

	return out
}

func mapObservePressureLevel(level resourceinternal.PressureLevel) observe.PressureLevel {
	switch level {
	case resourceinternal.PressureLevelCritical:
		return observe.PressureLevelCritical
	case resourceinternal.PressureLevelWarning:
		return observe.PressureLevelWarning
	default:
		return observe.PressureLevelOK
	}
}
