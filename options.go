package flx

import "errors"

const (
	defaultWorkers        = 16
	minWorkers            = 1
	unlimitedWorkerBuffer = 4096
)

var (
	// ErrNilController reports that a dynamic-worker option received a nil
	// concurrency controller.
	ErrNilController = errors.New("flx: nil concurrency controller")
	// ErrInterruptibleWorkersRequireContextTransform reports that forced dynamic
	// workers were requested for a transform that does not accept a context.
	ErrInterruptibleWorkersRequireContextTransform = errors.New("flx: WithInterruptibleWorkers/WithForcedDynamicWorkers requires MapContext/FlatMapContext")
)

// opOptions stores execution settings for one transform or terminal operation.
type opOptions struct {
	workers       int
	unlimited     bool
	controller    *ConcurrencyController
	interruptible bool
	errorStrategy ErrorStrategy
}

// Option mutates the execution settings for one transform or terminal call.
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

// WithDynamicWorkers enables graceful dynamic resizing for the current
// operation. Shrinking does not interrupt workers that already hold a slot.
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

// WithForcedDynamicWorkers enables forced dynamic resizing for the current
// operation. Shrinking cancels excess workers via context and only works with
// MapContext or FlatMapContext variants.
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

// WithInterruptibleWorkers is a compatibility alias for
// WithForcedDynamicWorkers.
func WithInterruptibleWorkers(controller *ConcurrencyController) Option {
	return WithForcedDynamicWorkers(controller)
}

// WithErrorStrategy configures how worker panics and errors are handled for the
// current operation.
func WithErrorStrategy(strategy ErrorStrategy) Option {
	return func(opts *opOptions) {
		mustValidateErrorStrategy(strategy)
		opts.errorStrategy = strategy
	}
}

// newOptions returns the default operation settings.
func newOptions() *opOptions {
	return &opOptions{
		workers:       defaultWorkers,
		errorStrategy: ErrorStrategyFailFast,
	}
}

// buildOptions applies opts in order on top of the default settings.
func buildOptions(opts ...Option) *opOptions {
	options := newOptions()
	for _, opt := range opts {
		opt(options)
	}

	return options
}
