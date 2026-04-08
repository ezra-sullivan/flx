package coordinator

import (
	"time"

	controlinternal "github.com/ezra-sullivan/flx/internal/control"
	resourceinternal "github.com/ezra-sullivan/flx/internal/resource"
)

// Policy configures the read/decide loop for one pipeline coordinator.
type Policy struct {
	ScaleUpStep         int
	ScaleDownStep       int
	ScaleUpCooldown     time.Duration
	ScaleDownCooldown   time.Duration
	ScaleUpHysteresis   int
	ScaleDownHysteresis int
}

// StageBudget constrains one controllable stage to a worker window.
type StageBudget struct {
	MinWorkers int
	MaxWorkers int
}

// StageDecision captures one worker adjustment for a stage.
type StageDecision struct {
	StageName    string
	FromWorkers  int
	ToWorkers    int
	Reason       string
	StageBacklog int
	LinkBacklog  int
}

// Decision captures one coordinator tick result.
type Decision struct {
	Stages []StageDecision
}

type controllableStage struct {
	controller *controlinternal.Controller
	budget     StageBudget
}

func (p Policy) normalized() Policy {
	if p.ScaleUpStep < 1 {
		p.ScaleUpStep = 1
	}
	if p.ScaleDownStep < 1 {
		p.ScaleDownStep = 1
	}
	if p.ScaleUpCooldown < 0 {
		p.ScaleUpCooldown = 0
	}
	if p.ScaleDownCooldown < 0 {
		p.ScaleDownCooldown = 0
	}
	if p.ScaleUpHysteresis < 1 {
		p.ScaleUpHysteresis = 1
	}
	if p.ScaleDownHysteresis < 1 {
		p.ScaleDownHysteresis = 1
	}

	return p
}

func (p Policy) brakesScaleUp(level resourceinternal.PressureLevel) bool {
	return level >= resourceinternal.PressureLevelWarning
}

func (p Policy) forcesShrink(level resourceinternal.PressureLevel) bool {
	return level >= resourceinternal.PressureLevelCritical
}

func (p Policy) allowsScaleUp(state stagePolicyState, now time.Time) bool {
	return state.upSignalTicks >= p.ScaleUpHysteresis && cooldownElapsed(state.lastScaleUpAt, p.ScaleUpCooldown, now)
}

func (p Policy) allowsScaleDown(state stagePolicyState, now time.Time) bool {
	return state.downSignalTicks >= p.ScaleDownHysteresis && cooldownElapsed(state.lastScaleDownAt, p.ScaleDownCooldown, now)
}

func cooldownElapsed(last time.Time, cooldown time.Duration, now time.Time) bool {
	if cooldown <= 0 || last.IsZero() {
		return true
	}

	return !now.Before(last.Add(cooldown))
}

func (b StageBudget) normalize(currentWorkers int) StageBudget {
	if currentWorkers < 1 {
		currentWorkers = 1
	}

	if b == (StageBudget{}) {
		return StageBudget{
			MinWorkers: currentWorkers,
			MaxWorkers: currentWorkers,
		}
	}

	if b.MinWorkers < 1 {
		b.MinWorkers = 1
	}
	if b.MaxWorkers < 1 {
		b.MaxWorkers = currentWorkers
	}
	if b.MaxWorkers < b.MinWorkers {
		b.MaxWorkers = b.MinWorkers
	}

	return b
}
