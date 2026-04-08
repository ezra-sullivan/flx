package coordinator

import (
	"github.com/ezra-sullivan/flx/internal/config"
	coordinatorinternal "github.com/ezra-sullivan/flx/internal/coordinator"
	resourceinternal "github.com/ezra-sullivan/flx/internal/resource"
	"github.com/ezra-sullivan/flx/pipeline/control"
	"github.com/ezra-sullivan/flx/pipeline/observe"
)

type coordinatorOptions struct {
	decisionObserver  func(observe.PipelineDecision)
	resourceObservers []resourceinternal.Observer
}

// CoordinatorOption mutates one coordinator configuration.
type CoordinatorOption func(*coordinatorOptions)

// StageBudget constrains one coordinator-managed dynamic stage to a worker
// budget window.
type StageBudget struct {
	MinWorkers int
	MaxWorkers int
}

// WithStageName labels the current transform or terminal operation as one
// named stage for observation and future coordinator policies.
func WithStageName(name string) control.Option {
	return config.WithStageName(name)
}

// WithCoordinator attaches one pipeline coordinator to the current operation.
func WithCoordinator(coordinator *PipelineCoordinator) control.Option {
	if coordinator == nil {
		return config.WithCoordinator(nil)
	}

	return config.WithCoordinator(coordinator.internalCoordinator())
}

// WithStageBudget constrains one coordinator-managed dynamic stage to a worker
// budget window.
func WithStageBudget(budget StageBudget) control.Option {
	return config.WithStageBudget(coordinatorinternal.StageBudget{
		MinWorkers: budget.MinWorkers,
		MaxWorkers: budget.MaxWorkers,
	})
}

// WithDecisionObserver registers a callback for future coordinator decisions.
func WithDecisionObserver(observer func(observe.PipelineDecision)) CoordinatorOption {
	return func(opts *coordinatorOptions) {
		opts.decisionObserver = observer
	}
}

// WithResourceObserver registers one resource observer that the coordinator
// polls during Tick.
func WithResourceObserver(observer observe.ResourceObserver) CoordinatorOption {
	return func(opts *coordinatorOptions) {
		if observer == nil {
			return
		}

		opts.resourceObservers = append(opts.resourceObservers, resourceinternal.ObserverFunc(func() resourceinternal.Snapshot {
			snapshot := observer.ObserveResource()
			samples := make([]resourceinternal.PressureSample, 0, len(snapshot.Samples))
			for _, sample := range snapshot.Samples {
				samples = append(samples, resourceinternal.PressureSample{
					Name:  sample.Name,
					Level: mapPressureLevel(sample.Level),
					Value: sample.Value,
					Limit: sample.Limit,
				})
			}

			return resourceinternal.Snapshot{Samples: samples}
		}))
	}
}

func mapPressureLevel(level observe.PressureLevel) resourceinternal.PressureLevel {
	switch level {
	case observe.PressureLevelCritical:
		return resourceinternal.PressureLevelCritical
	case observe.PressureLevelWarning:
		return resourceinternal.PressureLevelWarning
	default:
		return resourceinternal.PressureLevelOK
	}
}
