package flx

import "errors"

const (
	defaultWorkers        = 16
	minWorkers            = 1
	unlimitedWorkerBuffer = 4096
)

var (
	ErrNilController                               = errors.New("flx: nil concurrency controller")
	ErrInterruptibleWorkersRequireContextTransform = errors.New("flx: WithInterruptibleWorkers/WithForcedDynamicWorkers requires MapContext/FlatMapContext")
)

type opOptions struct {
	workers       int
	unlimited     bool
	controller    *ConcurrencyController
	interruptible bool
	errorStrategy ErrorStrategy
}

type Option func(*opOptions)

// WithWorkers limits the current operation to a fixed number of workers.
func WithWorkers(workers int) Option {
	return func(opts *opOptions) {
		opts.workers = max(workers, minWorkers)
		opts.unlimited = false
		opts.controller = nil
		opts.interruptible = false
	}
}

// WithUnlimitedWorkers spawns one worker per item for the current operation.
func WithUnlimitedWorkers() Option {
	return func(opts *opOptions) {
		opts.unlimited = true
		opts.controller = nil
		opts.interruptible = false
	}
}

// WithDynamicWorkers enables graceful dynamic resizing for the current operation.
// Shrinking does not interrupt workers that already hold a slot.
func WithDynamicWorkers(controller *ConcurrencyController) Option {
	return func(opts *opOptions) {
		if controller == nil {
			panic(ErrNilController)
		}

		opts.unlimited = false
		opts.controller = controller
		opts.interruptible = false
	}
}

// WithForcedDynamicWorkers enables forced dynamic resizing for the current operation.
// Shrinking cancels excess workers via context and only works with MapContext/FlatMapContext.
func WithForcedDynamicWorkers(controller *ConcurrencyController) Option {
	return func(opts *opOptions) {
		if controller == nil {
			panic(ErrNilController)
		}

		opts.unlimited = false
		opts.controller = controller
		opts.interruptible = true
	}
}

// WithInterruptibleWorkers is kept as a compatibility alias for WithForcedDynamicWorkers.
func WithInterruptibleWorkers(controller *ConcurrencyController) Option {
	return WithForcedDynamicWorkers(controller)
}

// WithErrorStrategy configures how worker panic/error is handled for the current operation.
func WithErrorStrategy(strategy ErrorStrategy) Option {
	return func(opts *opOptions) {
		mustValidateErrorStrategy(strategy)
		opts.errorStrategy = strategy
	}
}

func newOptions() *opOptions {
	return &opOptions{
		workers:       defaultWorkers,
		errorStrategy: ErrorStrategyFailFast,
	}
}

func buildOptions(opts ...Option) *opOptions {
	options := newOptions()
	for _, opt := range opts {
		opt(options)
	}

	return options
}
