package coordinator

import (
	"slices"
	"sync"
	"time"

	controlinternal "github.com/ezra-sullivan/flx/internal/control"
	linkinternal "github.com/ezra-sullivan/flx/internal/link"
	"github.com/ezra-sullivan/flx/internal/metrics"
	resourceinternal "github.com/ezra-sullivan/flx/internal/resource"
)

// Coordinator aggregates the latest stage and link snapshots for one pipeline.
// It is safe for concurrent use.
type Coordinator struct {
	mu     sync.RWMutex
	tickMu sync.Mutex

	registry  stageRegistry
	policy    Policy
	stages    map[string]metrics.StageMetrics
	links     map[string]linkinternal.Metrics
	controls  map[string]controllableStage
	state     map[string]stagePolicyState
	resources []resourceinternal.Observer
	now       func() time.Time
}

// New returns an empty pipeline coordinator with one immutable policy.
func New(policy Policy, resourceObservers ...resourceinternal.Observer) *Coordinator {
	observers := make([]resourceinternal.Observer, 0, len(resourceObservers))
	for _, observer := range resourceObservers {
		if observer == nil {
			continue
		}
		observers = append(observers, observer)
	}

	return &Coordinator{
		policy:    policy.normalized(),
		stages:    make(map[string]metrics.StageMetrics),
		links:     make(map[string]linkinternal.Metrics),
		controls:  make(map[string]controllableStage),
		state:     make(map[string]stagePolicyState),
		resources: observers,
		now:       time.Now,
	}
}

// ResolveStageName returns a stable stage identity, generating one when name
// is empty.
func (c *Coordinator) ResolveStageName(name string) string {
	if c == nil {
		return name
	}

	return c.registry.Resolve(name)
}

// ObserveStage stores the latest snapshot for one stage.
func (c *Coordinator) ObserveStage(snapshot metrics.StageMetrics) {
	if c == nil {
		return
	}

	stageName := c.ResolveStageName(snapshot.StageName)
	snapshot.StageName = stageName

	c.mu.Lock()
	c.stages[stageName] = snapshot
	c.mu.Unlock()
}

// ObserveLink stores the latest snapshot for one outbound link.
func (c *Coordinator) ObserveLink(snapshot linkinternal.Metrics) {
	if c == nil {
		return
	}

	fromStage := c.ResolveStageName(snapshot.FromStage)
	snapshot.FromStage = fromStage

	c.mu.Lock()
	c.links[fromStage] = snapshot
	c.mu.Unlock()
}

// RegisterControllableStage records the controller and budget for one stage.
func (c *Coordinator) RegisterControllableStage(stageName string, controller *controlinternal.Controller, budget StageBudget) {
	if c == nil || controller == nil {
		return
	}

	stageName = c.ResolveStageName(stageName)
	budget = budget.normalize(controller.Workers())

	c.mu.Lock()
	c.controls[stageName] = controllableStage{
		controller: controller,
		budget:     budget,
	}
	if _, ok := c.state[stageName]; !ok {
		c.state[stageName] = stagePolicyState{}
	}
	c.mu.Unlock()
}

// Snapshot returns a stable copy of the latest known pipeline metrics.
func (c *Coordinator) Snapshot() Snapshot {
	if c == nil {
		return Snapshot{}
	}

	c.mu.RLock()
	stages := make([]metrics.StageMetrics, 0, len(c.stages))
	for _, snapshot := range c.stages {
		stages = append(stages, snapshot)
	}

	links := make([]linkinternal.Metrics, 0, len(c.links))
	for _, snapshot := range c.links {
		links = append(links, snapshot)
	}
	resourceObservers := append([]resourceinternal.Observer(nil), c.resources...)
	c.mu.RUnlock()

	resources := resourceinternal.ObserveAll(resourceObservers)

	slices.SortFunc(stages, func(a, b metrics.StageMetrics) int {
		switch {
		case a.StageName < b.StageName:
			return -1
		case a.StageName > b.StageName:
			return 1
		default:
			return 0
		}
	})

	slices.SortFunc(links, func(a, b linkinternal.Metrics) int {
		switch {
		case a.FromStage < b.FromStage:
			return -1
		case a.FromStage > b.FromStage:
			return 1
		case a.ToStage < b.ToStage:
			return -1
		case a.ToStage > b.ToStage:
			return 1
		default:
			return 0
		}
	})

	return Snapshot{
		Stages:    stages,
		Links:     links,
		Resources: resources,
	}
}

// Tick applies one policy pass and returns the worker adjustments it made.
func (c *Coordinator) Tick() Decision {
	if c == nil {
		return Decision{}
	}
	c.tickMu.Lock()
	defer c.tickMu.Unlock()

	type controlView struct {
		controller *controlinternal.Controller
		budget     StageBudget
		state      stagePolicyState
	}

	c.mu.RLock()
	policy := c.policy
	stages := make(map[string]metrics.StageMetrics, len(c.stages))
	for name, snapshot := range c.stages {
		stages[name] = snapshot
	}
	links := make([]linkinternal.Metrics, 0, len(c.links))
	for _, snapshot := range c.links {
		links = append(links, snapshot)
	}
	controls := make(map[string]controlView, len(c.controls))
	for name, stage := range c.controls {
		controls[name] = controlView{
			controller: stage.controller,
			budget:     stage.budget,
			state:      c.state[name],
		}
	}
	resourceObservers := append([]resourceinternal.Observer(nil), c.resources...)
	now := time.Now
	if c.now != nil {
		now = c.now
	}
	c.mu.RUnlock()

	resourceLevel := resourceinternal.MaxPressureLevel(resourceinternal.ObserveAll(resourceObservers))
	tickAt := now()

	incomingBacklog := make(map[string]int, len(links))
	outgoingBacklog := make(map[string]int, len(links))
	for _, snapshot := range links {
		outgoingBacklog[snapshot.FromStage] += snapshot.Backlog
		if snapshot.ToStage == "" {
			continue
		}
		incomingBacklog[snapshot.ToStage] += snapshot.Backlog
	}

	stageNames := mapsKeys(controls)
	slices.Sort(stageNames)
	decisions := make([]StageDecision, 0, len(stageNames))
	updatedState := make(map[string]stagePolicyState, len(stageNames))

	for _, stageName := range stageNames {
		controlView := controls[stageName]
		controller := controlView.controller
		if controller == nil {
			continue
		}
		policyState := controlView.state

		currentWorkers := controller.Workers()
		budget := controlView.budget.normalize(currentWorkers)
		stageSnapshot, hasStageSnapshot := stages[stageName]
		stageBacklog := 0
		activeWorkers := 0
		if hasStageSnapshot {
			stageBacklog = stageSnapshot.Backlog
			activeWorkers = stageSnapshot.ActiveWorkers
		}
		linkBacklog := incomingBacklog[stageName]
		downstreamBacklog := outgoingBacklog[stageName]

		switch {
		case currentWorkers < budget.MinWorkers:
			targetWorkers := budget.MinWorkers
			controller.SetWorkers(targetWorkers)
			policyState = policyState.recordScale(signalUp, tickAt)
			updatedState[stageName] = policyState
			decisions = append(decisions, StageDecision{
				StageName:    stageName,
				FromWorkers:  currentWorkers,
				ToWorkers:    targetWorkers,
				Reason:       "budget_min",
				StageBacklog: stageBacklog,
				LinkBacklog:  linkBacklog,
			})
			continue
		}

		signal := stageSignal{}
		switch {
		case hasStageSnapshot && policy.forcesShrink(resourceLevel) && stageBacklog == 0 && linkBacklog == 0 && downstreamBacklog == 0 && currentWorkers > budget.MinWorkers:
			signal = stageSignal{direction: signalDown, reason: "resource_pressure"}
		case !policy.brakesScaleUp(resourceLevel) && linkBacklog > 0:
			signal = stageSignal{direction: signalUp, reason: "incoming_link_backlog"}
		case !policy.brakesScaleUp(resourceLevel) && stageBacklog > 0:
			signal = stageSignal{direction: signalUp, reason: "stage_backlog"}
		case hasStageSnapshot && stageBacklog == 0 && linkBacklog == 0 && downstreamBacklog == 0 && activeWorkers < currentWorkers && currentWorkers > budget.MinWorkers:
			signal = stageSignal{direction: signalDown, reason: "idle_shrink"}
		}

		policyState = policyState.observe(signal.direction)
		updatedState[stageName] = policyState

		if signal.direction == signalNone {
			continue
		}

		switch signal.direction {
		case signalUp:
			if !policy.allowsScaleUp(policyState, tickAt) {
				continue
			}
		case signalDown:
			if !policy.allowsScaleDown(policyState, tickAt) {
				continue
			}
		}

		targetWorkers := currentWorkers
		switch signal.direction {
		case signalUp:
			targetWorkers = min(currentWorkers+policy.ScaleUpStep, budget.MaxWorkers)
		case signalDown:
			targetWorkers = max(currentWorkers-policy.ScaleDownStep, budget.MinWorkers)
		}
		if targetWorkers == currentWorkers {
			continue
		}

		controller.SetWorkers(targetWorkers)
		policyState = policyState.recordScale(signal.direction, tickAt)
		updatedState[stageName] = policyState
		decisions = append(decisions, StageDecision{
			StageName:    stageName,
			FromWorkers:  currentWorkers,
			ToWorkers:    targetWorkers,
			Reason:       signal.reason,
			StageBacklog: stageBacklog,
			LinkBacklog:  linkBacklog,
		})
	}

	c.mu.Lock()
	for stageName, state := range updatedState {
		c.state[stageName] = state
	}
	c.mu.Unlock()

	return Decision{Stages: decisions}
}

func mapsKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}

	return keys
}
