package main

import (
	"runtime"
	"time"

	pipelinecoordinator "github.com/ezra-sullivan/flx/pipeline/coordinator"
	"github.com/ezra-sullivan/flx/pipeline/observe"
)

const (
	exampleHeapSoftLimitBytes = 256 << 20
	exampleHeapWarningRatio   = 0.80
	exampleHeapCriticalRatio  = 0.95
)

func newExamplePipelineCoordinator() *pipelinecoordinator.PipelineCoordinator {
	return pipelinecoordinator.NewPipelineCoordinator(
		pipelinecoordinator.PipelineCoordinatorPolicy{
			ScaleUpStep:         1,
			ScaleDownStep:       1,
			ScaleUpCooldown:     250 * time.Millisecond,
			ScaleDownCooldown:   450 * time.Millisecond,
			ScaleUpHysteresis:   1,
			ScaleDownHysteresis: 2,
		},
		pipelinecoordinator.WithDecisionObserver(logCoordinatorDecision),
		pipelinecoordinator.WithResourceObserver(newHeapPressureObserver(exampleHeapSoftLimitBytes)),
	)
}

func newHeapPressureObserver(softLimitBytes uint64) observe.ResourceObserver {
	return observe.ResourceObserverFunc(func() observe.ResourceSnapshot {
		if softLimitBytes == 0 {
			return observe.ResourceSnapshot{}
		}

		var stats runtime.MemStats
		runtime.ReadMemStats(&stats)

		usage := float64(stats.Alloc)
		limit := float64(softLimitBytes)
		level := observe.PressureLevelOK
		ratio := usage / limit

		switch {
		case ratio >= exampleHeapCriticalRatio:
			level = observe.PressureLevelCritical
		case ratio >= exampleHeapWarningRatio:
			level = observe.PressureLevelWarning
		}

		return observe.ResourceSnapshot{
			Samples: []observe.PressureSample{{
				Name:  "heap",
				Level: level,
				Value: usage,
				Limit: limit,
			}},
		}
	})
}
