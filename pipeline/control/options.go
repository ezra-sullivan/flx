package control

import "github.com/ezra-sullivan/flx/internal/config"

// Option mutates the execution settings for one transform or terminal call.
type Option = config.Option

// WithWorkers limits the current operation to a fixed number of workers.
func WithWorkers(workers int) Option {
	return config.WithWorkers(workers)
}

// WithUnlimitedWorkers spawns one worker per item for the current operation.
func WithUnlimitedWorkers() Option {
	return config.WithUnlimitedWorkers()
}

// WithDynamicWorkers enables graceful dynamic resizing for the current
// operation. Shrinking does not interrupt workers that already hold a slot.
func WithDynamicWorkers(controller *ConcurrencyController) Option {
	return config.WithDynamicWorkers(controller.internalController())
}

// WithForcedDynamicWorkers enables forced dynamic resizing for the current
// operation. Shrinking cancels excess workers via context and only works with
// MapContext or FlatMapContext variants.
func WithForcedDynamicWorkers(controller *ConcurrencyController) Option {
	return config.WithForcedDynamicWorkers(controller.internalController())
}

// WithInterruptibleWorkers is a compatibility alias for
// WithForcedDynamicWorkers.
//
// Deprecated: use WithForcedDynamicWorkers.
func WithInterruptibleWorkers(controller *ConcurrencyController) Option {
	return WithForcedDynamicWorkers(controller)
}

// WithErrorStrategy configures how worker panics and errors are handled for
// the current operation.
func WithErrorStrategy(strategy ErrorStrategy) Option {
	return config.WithErrorStrategy(config.ErrorStrategy(strategy))
}
