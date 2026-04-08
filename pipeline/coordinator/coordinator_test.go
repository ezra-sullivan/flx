package coordinator

import (
	"slices"
	"testing"

	"github.com/ezra-sullivan/flx/internal/config"
	controlinternal "github.com/ezra-sullivan/flx/internal/control"
	coordinatorinternal "github.com/ezra-sullivan/flx/internal/coordinator"
	linkinternal "github.com/ezra-sullivan/flx/internal/link"
	"github.com/ezra-sullivan/flx/internal/metrics"
	"github.com/ezra-sullivan/flx/pipeline/observe"
)

func TestWithStageNameConfiguresStageIdentity(t *testing.T) {
	opts := config.BuildOptions(WithStageName("download"))

	if got := opts.StageName(); got != "download" {
		t.Fatalf("unexpected stage name: got %q want %q", got, "download")
	}
}

func TestSnapshotStartsEmpty(t *testing.T) {
	coord := NewPipelineCoordinator(PipelineCoordinatorPolicy{})
	got := coord.Snapshot()

	if len(got.Stages) != 0 || len(got.Links) != 0 || len(got.Resources.Samples) != 0 {
		t.Fatalf("expected empty snapshot, got %#v", got)
	}
}

func TestWithDecisionObserverBuildsCoordinator(t *testing.T) {
	var called bool
	coord := NewPipelineCoordinator(
		PipelineCoordinatorPolicy{},
		WithDecisionObserver(func(observe.PipelineDecision) {
			called = true
		}),
	)

	if coord == nil {
		t.Fatal("expected coordinator")
	}
	if coord.decisionObserver == nil {
		t.Fatal("expected decision observer to be configured")
	}

	coord.decisionObserver(observe.PipelineDecision{
		Stages: []observe.StageDecision{{StageName: "download"}},
	})
	if !called {
		t.Fatal("expected decision observer to be invoked")
	}
}

func TestWithResourceObserverBrakesScaleUp(t *testing.T) {
	coord := NewPipelineCoordinator(
		PipelineCoordinatorPolicy{},
		WithResourceObserver(observe.ResourceObserverFunc(func() observe.ResourceSnapshot {
			return observe.ResourceSnapshot{
				Samples: []observe.PressureSample{{
					Name:  "memory",
					Level: observe.PressureLevelWarning,
					Value: 90,
					Limit: 100,
				}},
			}
		})),
	)

	internal := coord.internalCoordinator()
	controller := controlinternal.NewController(1)
	internal.RegisterControllableStage("resize", controller, coordinatorinternal.StageBudget{
		MinWorkers: 1,
		MaxWorkers: 3,
	})
	internal.ObserveStage(metrics.StageMetrics{StageName: "resize", Backlog: 2, ActiveWorkers: 1})

	decision := coord.Tick()

	if got := controller.Workers(); got != 1 {
		t.Fatalf("expected resource observer to brake scale up, got %d workers", got)
	}
	if len(decision.Stages) != 0 {
		t.Fatalf("expected no decision while resource brake is active, got %#v", decision.Stages)
	}
}

func TestWithCoordinatorConfiguresCoordinator(t *testing.T) {
	coord := NewPipelineCoordinator(PipelineCoordinatorPolicy{})
	opts := config.BuildOptions(WithCoordinator(coord))

	if opts.Coordinator() == nil {
		t.Fatal("expected pipeline coordinator to be configured")
	}
}

func TestWithStageBudgetConfiguresBudget(t *testing.T) {
	opts := config.BuildOptions(WithStageBudget(StageBudget{
		MinWorkers: 1,
		MaxWorkers: 4,
	}))

	got := opts.StageBudget()
	if got.MinWorkers != 1 || got.MaxWorkers != 4 {
		t.Fatalf("unexpected stage budget: %#v", got)
	}
}

func TestSnapshotReturnsSortedCopies(t *testing.T) {
	coord := NewPipelineCoordinator(
		PipelineCoordinatorPolicy{},
		WithResourceObserver(observe.ResourceObserverFunc(func() observe.ResourceSnapshot {
			return observe.ResourceSnapshot{
				Samples: []observe.PressureSample{{
					Name:  "memory",
					Level: observe.PressureLevelWarning,
					Value: 80,
					Limit: 100,
				}},
			}
		})),
	)

	internal := coord.internalCoordinator()
	internal.ObserveStage(configureStage("resize", 4))
	internal.ObserveStage(configureStage("download", 3))
	internal.ObserveLink(configureLink("resize", "", 4))
	internal.ObserveLink(configureLink("download", "resize", 3))

	got := coord.Snapshot()

	if !slices.Equal(got.Stages, []observe.StageMetrics{
		{StageName: "download", InputCount: 3},
		{StageName: "resize", InputCount: 4},
	}) {
		t.Fatalf("unexpected stages: %#v", got.Stages)
	}

	if !slices.Equal(got.Links, []observe.LinkMetrics{
		{FromStage: "download", ToStage: "resize", SentCount: 3},
		{FromStage: "resize", ToStage: "", SentCount: 4},
	}) {
		t.Fatalf("unexpected links: %#v", got.Links)
	}

	if !slices.Equal(got.Resources.Samples, []observe.PressureSample{{
		Name:  "memory",
		Level: observe.PressureLevelWarning,
		Value: 80,
		Limit: 100,
	}}) {
		t.Fatalf("unexpected resources: %#v", got.Resources.Samples)
	}
}

func TestTickMapsInternalDecisionAndInvokesObserver(t *testing.T) {
	var gotDecision observe.PipelineDecision
	coord := NewPipelineCoordinator(
		PipelineCoordinatorPolicy{},
		WithDecisionObserver(func(decision observe.PipelineDecision) {
			gotDecision = decision
		}),
	)

	internal := coord.internalCoordinator()
	controller := controlinternal.NewController(1)
	internal.RegisterControllableStage("resize", controller, coordinatorinternal.StageBudget{
		MinWorkers: 1,
		MaxWorkers: 3,
	})
	internal.ObserveStage(metrics.StageMetrics{StageName: "resize", Backlog: 1, ActiveWorkers: 1})

	decision := coord.Tick()

	want := observe.PipelineDecision{
		Stages: []observe.StageDecision{{
			StageName:    "resize",
			FromWorkers:  1,
			ToWorkers:    2,
			Reason:       "stage_backlog",
			StageBacklog: 1,
		}},
	}
	if !slices.Equal(decision.Stages, want.Stages) {
		t.Fatalf("unexpected decision: got %#v want %#v", decision, want)
	}
	if !slices.Equal(gotDecision.Stages, want.Stages) {
		t.Fatalf("unexpected observed decision: got %#v want %#v", gotDecision, want)
	}
}

func configureStage(stageName string, inputCount int64) metrics.StageMetrics {
	return metrics.StageMetrics{
		StageName:  stageName,
		InputCount: inputCount,
	}
}

func configureLink(fromStage, toStage string, sentCount int64) linkinternal.Metrics {
	return linkinternal.Metrics{
		FromStage: fromStage,
		ToStage:   toStage,
		SentCount: sentCount,
	}
}
