package coordinator

import (
	"testing"
	"time"

	controlinternal "github.com/ezra-sullivan/flx/internal/control"
	linkinternal "github.com/ezra-sullivan/flx/internal/link"
	"github.com/ezra-sullivan/flx/internal/metrics"
	resourceinternal "github.com/ezra-sullivan/flx/internal/resource"
)

func TestTickScalesUpDownstreamBeforeShrinkingUpstream(t *testing.T) {
	coord := New(Policy{})
	upstream := controlinternal.NewController(2)
	downstream := controlinternal.NewController(1)

	coord.RegisterControllableStage("download", upstream, StageBudget{MinWorkers: 1, MaxWorkers: 2})
	coord.RegisterControllableStage("resize", downstream, StageBudget{MinWorkers: 1, MaxWorkers: 3})
	coord.ObserveStage(metrics.StageMetrics{StageName: "download", ActiveWorkers: 1})
	coord.ObserveStage(metrics.StageMetrics{StageName: "resize", ActiveWorkers: 1})
	coord.ObserveLink(linkinternal.Metrics{FromStage: "download", ToStage: "resize", Backlog: 2})

	decision := coord.Tick()

	if got := upstream.Workers(); got != 2 {
		t.Fatalf("expected upstream workers to stay at 2, got %d", got)
	}
	if got := downstream.Workers(); got != 2 {
		t.Fatalf("expected downstream workers to scale to 2, got %d", got)
	}
	if len(decision.Stages) != 1 {
		t.Fatalf("expected one stage decision, got %#v", decision.Stages)
	}
	if got := decision.Stages[0]; got.StageName != "resize" || got.Reason != "incoming_link_backlog" {
		t.Fatalf("unexpected stage decision: %#v", got)
	}
}

func TestTickShrinksIdleStageWithinBudget(t *testing.T) {
	coord := New(Policy{ScaleDownStep: 2})
	stage := controlinternal.NewController(4)

	coord.RegisterControllableStage("resize", stage, StageBudget{MinWorkers: 1, MaxWorkers: 5})
	coord.ObserveStage(metrics.StageMetrics{StageName: "resize", ActiveWorkers: 1, Backlog: 0})

	decision := coord.Tick()

	if got := stage.Workers(); got != 2 {
		t.Fatalf("expected workers to shrink to 2, got %d", got)
	}
	if len(decision.Stages) != 1 {
		t.Fatalf("expected one stage decision, got %#v", decision.Stages)
	}
	if got := decision.Stages[0]; got.Reason != "idle_shrink" || got.FromWorkers != 4 || got.ToWorkers != 2 {
		t.Fatalf("unexpected stage decision: %#v", got)
	}
}

func TestTickResourcePressureBrakesScaleUp(t *testing.T) {
	coord := New(Policy{}, resourceinternal.ObserverFunc(func() resourceinternal.Snapshot {
		return resourceinternal.Snapshot{
			Samples: []resourceinternal.PressureSample{{
				Name:  "memory",
				Level: resourceinternal.PressureLevelWarning,
			}},
		}
	}))
	stage := controlinternal.NewController(1)

	coord.RegisterControllableStage("resize", stage, StageBudget{MinWorkers: 1, MaxWorkers: 3})
	coord.ObserveStage(metrics.StageMetrics{StageName: "resize", ActiveWorkers: 1, Backlog: 2})

	decision := coord.Tick()

	if got := stage.Workers(); got != 1 {
		t.Fatalf("expected workers to stay at 1 under resource brake, got %d", got)
	}
	if len(decision.Stages) != 0 {
		t.Fatalf("expected no stage decision under resource brake, got %#v", decision.Stages)
	}
}

func TestTickCriticalResourcePressureShrinksStage(t *testing.T) {
	coord := New(Policy{}, resourceinternal.ObserverFunc(func() resourceinternal.Snapshot {
		return resourceinternal.Snapshot{
			Samples: []resourceinternal.PressureSample{{
				Name:  "memory",
				Level: resourceinternal.PressureLevelCritical,
			}},
		}
	}))
	stage := controlinternal.NewController(4)

	coord.RegisterControllableStage("resize", stage, StageBudget{MinWorkers: 1, MaxWorkers: 5})
	coord.ObserveStage(metrics.StageMetrics{StageName: "resize", ActiveWorkers: 4, Backlog: 0})

	decision := coord.Tick()

	if got := stage.Workers(); got != 3 {
		t.Fatalf("expected workers to shrink to 3, got %d", got)
	}
	if len(decision.Stages) != 1 {
		t.Fatalf("expected one stage decision, got %#v", decision.Stages)
	}
	if got := decision.Stages[0]; got.Reason != "resource_pressure" || got.FromWorkers != 4 || got.ToWorkers != 3 {
		t.Fatalf("unexpected stage decision: %#v", got)
	}
}

func TestTickScaleUpCooldownDelaysRepeatedExpansion(t *testing.T) {
	now := time.Unix(0, 0)
	coord := New(Policy{ScaleUpCooldown: time.Second})
	coord.now = func() time.Time { return now }
	stage := controlinternal.NewController(1)

	coord.RegisterControllableStage("resize", stage, StageBudget{MinWorkers: 1, MaxWorkers: 4})
	coord.ObserveStage(metrics.StageMetrics{StageName: "resize", ActiveWorkers: 1, Backlog: 1})

	first := coord.Tick()
	if got := stage.Workers(); got != 2 {
		t.Fatalf("expected first expansion to 2 workers, got %d", got)
	}
	if len(first.Stages) != 1 || first.Stages[0].Reason != "stage_backlog" {
		t.Fatalf("unexpected first decision: %#v", first.Stages)
	}

	now = now.Add(500 * time.Millisecond)
	coord.ObserveStage(metrics.StageMetrics{StageName: "resize", ActiveWorkers: 2, Backlog: 1})

	second := coord.Tick()
	if got := stage.Workers(); got != 2 {
		t.Fatalf("expected cooldown to hold workers at 2, got %d", got)
	}
	if len(second.Stages) != 0 {
		t.Fatalf("expected no decision during cooldown, got %#v", second.Stages)
	}

	now = now.Add(600 * time.Millisecond)
	coord.ObserveStage(metrics.StageMetrics{StageName: "resize", ActiveWorkers: 2, Backlog: 1})

	third := coord.Tick()
	if got := stage.Workers(); got != 3 {
		t.Fatalf("expected cooldown expiry to allow expansion to 3, got %d", got)
	}
	if len(third.Stages) != 1 || third.Stages[0].ToWorkers != 3 {
		t.Fatalf("unexpected third decision: %#v", third.Stages)
	}
}

func TestTickScaleUpHysteresisRequiresRepeatedSignalAndResetsOnClear(t *testing.T) {
	coord := New(Policy{ScaleUpHysteresis: 2})
	stage := controlinternal.NewController(1)

	coord.RegisterControllableStage("resize", stage, StageBudget{MinWorkers: 1, MaxWorkers: 4})
	coord.ObserveStage(metrics.StageMetrics{StageName: "resize", ActiveWorkers: 1, Backlog: 1})

	first := coord.Tick()
	if got := stage.Workers(); got != 1 {
		t.Fatalf("expected first signal to only arm hysteresis, got %d workers", got)
	}
	if len(first.Stages) != 0 {
		t.Fatalf("expected no decision on first hysteresis tick, got %#v", first.Stages)
	}

	coord.ObserveStage(metrics.StageMetrics{StageName: "resize", ActiveWorkers: 1, Backlog: 0})
	second := coord.Tick()
	if got := stage.Workers(); got != 1 {
		t.Fatalf("expected cleared signal to keep workers at 1, got %d", got)
	}
	if len(second.Stages) != 0 {
		t.Fatalf("expected no decision after signal clear, got %#v", second.Stages)
	}

	coord.ObserveStage(metrics.StageMetrics{StageName: "resize", ActiveWorkers: 1, Backlog: 1})
	third := coord.Tick()
	if got := stage.Workers(); got != 1 {
		t.Fatalf("expected hysteresis to restart from 1 tick, got %d", got)
	}
	if len(third.Stages) != 0 {
		t.Fatalf("expected no decision on restarted hysteresis tick, got %#v", third.Stages)
	}

	coord.ObserveStage(metrics.StageMetrics{StageName: "resize", ActiveWorkers: 1, Backlog: 1})
	fourth := coord.Tick()
	if got := stage.Workers(); got != 2 {
		t.Fatalf("expected second consecutive signal to expand to 2, got %d", got)
	}
	if len(fourth.Stages) != 1 || fourth.Stages[0].Reason != "stage_backlog" {
		t.Fatalf("unexpected hysteresis decision: %#v", fourth.Stages)
	}
}

func TestTickScaleDownCooldownDelaysRepeatedShrink(t *testing.T) {
	now := time.Unix(0, 0)
	coord := New(Policy{ScaleDownCooldown: time.Second})
	coord.now = func() time.Time { return now }
	stage := controlinternal.NewController(4)

	coord.RegisterControllableStage("resize", stage, StageBudget{MinWorkers: 1, MaxWorkers: 5})
	coord.ObserveStage(metrics.StageMetrics{StageName: "resize", ActiveWorkers: 1, Backlog: 0})

	first := coord.Tick()
	if got := stage.Workers(); got != 3 {
		t.Fatalf("expected first shrink to 3 workers, got %d", got)
	}
	if len(first.Stages) != 1 || first.Stages[0].Reason != "idle_shrink" {
		t.Fatalf("unexpected first shrink decision: %#v", first.Stages)
	}

	now = now.Add(500 * time.Millisecond)
	coord.ObserveStage(metrics.StageMetrics{StageName: "resize", ActiveWorkers: 1, Backlog: 0})

	second := coord.Tick()
	if got := stage.Workers(); got != 3 {
		t.Fatalf("expected cooldown to hold workers at 3, got %d", got)
	}
	if len(second.Stages) != 0 {
		t.Fatalf("expected no shrink decision during cooldown, got %#v", second.Stages)
	}

	now = now.Add(600 * time.Millisecond)
	coord.ObserveStage(metrics.StageMetrics{StageName: "resize", ActiveWorkers: 1, Backlog: 0})

	third := coord.Tick()
	if got := stage.Workers(); got != 2 {
		t.Fatalf("expected cooldown expiry to allow shrink to 2, got %d", got)
	}
	if len(third.Stages) != 1 || third.Stages[0].ToWorkers != 2 {
		t.Fatalf("unexpected third shrink decision: %#v", third.Stages)
	}
}

func TestTickScaleDownHysteresisRequiresRepeatedIdleSignal(t *testing.T) {
	coord := New(Policy{ScaleDownHysteresis: 2})
	stage := controlinternal.NewController(3)

	coord.RegisterControllableStage("resize", stage, StageBudget{MinWorkers: 1, MaxWorkers: 5})
	coord.ObserveStage(metrics.StageMetrics{StageName: "resize", ActiveWorkers: 1, Backlog: 0})

	first := coord.Tick()
	if got := stage.Workers(); got != 3 {
		t.Fatalf("expected first idle tick to only arm hysteresis, got %d", got)
	}
	if len(first.Stages) != 0 {
		t.Fatalf("expected no shrink on first hysteresis tick, got %#v", first.Stages)
	}

	coord.ObserveStage(metrics.StageMetrics{StageName: "resize", ActiveWorkers: 3, Backlog: 0})
	second := coord.Tick()
	if got := stage.Workers(); got != 3 {
		t.Fatalf("expected active workload to reset shrink hysteresis, got %d", got)
	}
	if len(second.Stages) != 0 {
		t.Fatalf("expected no decision on reset tick, got %#v", second.Stages)
	}

	coord.ObserveStage(metrics.StageMetrics{StageName: "resize", ActiveWorkers: 1, Backlog: 0})
	third := coord.Tick()
	if got := stage.Workers(); got != 3 {
		t.Fatalf("expected restarted idle hysteresis to wait, got %d", got)
	}
	if len(third.Stages) != 0 {
		t.Fatalf("expected no decision on restarted idle tick, got %#v", third.Stages)
	}

	coord.ObserveStage(metrics.StageMetrics{StageName: "resize", ActiveWorkers: 1, Backlog: 0})
	fourth := coord.Tick()
	if got := stage.Workers(); got != 2 {
		t.Fatalf("expected second consecutive idle tick to shrink to 2, got %d", got)
	}
	if len(fourth.Stages) != 1 || fourth.Stages[0].Reason != "idle_shrink" {
		t.Fatalf("unexpected shrink hysteresis decision: %#v", fourth.Stages)
	}
}

func TestTickBudgetMinBypassesCooldownAndHysteresis(t *testing.T) {
	now := time.Unix(0, 0)
	coord := New(Policy{
		ScaleUpCooldown:   time.Hour,
		ScaleUpHysteresis: 99,
	})
	coord.now = func() time.Time { return now }
	stage := controlinternal.NewController(1)

	coord.RegisterControllableStage("resize", stage, StageBudget{MinWorkers: 3, MaxWorkers: 5})

	decision := coord.Tick()
	if got := stage.Workers(); got != 3 {
		t.Fatalf("expected budget min to force workers to 3, got %d", got)
	}
	if len(decision.Stages) != 1 || decision.Stages[0].Reason != "budget_min" {
		t.Fatalf("unexpected budget min decision: %#v", decision.Stages)
	}
}
