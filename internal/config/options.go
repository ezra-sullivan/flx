package config

import (
	"errors"

	"github.com/ezra-sullivan/flx/internal/control"
)

const (
	DefaultWorkers        = 16
	MinWorkers            = 1
	UnlimitedWorkerBuffer = 4096
)

var (
	// ErrNilController reports that a dynamic-worker option received a nil
	// concurrency controller.
	ErrNilController = errors.New("flx: nil concurrency controller")
	// ErrInterruptibleWorkersRequireContextTransform reports that forced dynamic
	// workers were requested for a transform that does not accept a context.
	ErrInterruptibleWorkersRequireContextTransform = errors.New("flx: WithInterruptibleWorkers/WithForcedDynamicWorkers requires MapContext/FlatMapContext")
)

// Options stores execution settings for one transform or terminal operation.
type Options struct {
	workers       int
	unlimited     bool
	controller    *control.Controller
	interruptible bool
	errorStrategy ErrorStrategy
}

// Option mutates the execution settings for one transform or terminal call.
type Option func(*Options)

// WithWorkers limits the current operation to a fixed number of workers.
func WithWorkers(workers int) Option {
	return func(opts *Options) {
		opts.workers = max(workers, MinWorkers)
		opts.unlimited = false
		opts.controller = nil
		opts.interruptible = false
	}
}

// WithUnlimitedWorkers spawns one worker per item for the current operation.
func WithUnlimitedWorkers() Option {
	return func(opts *Options) {
		opts.unlimited = true
		opts.controller = nil
		opts.interruptible = false
	}
}

// WithDynamicWorkers enables graceful dynamic resizing for the current
// operation. Shrinking does not interrupt workers that already hold a slot.
func WithDynamicWorkers(controller *control.Controller) Option {
	return func(opts *Options) {
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
func WithForcedDynamicWorkers(controller *control.Controller) Option {
	return func(opts *Options) {
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
func WithInterruptibleWorkers(controller *control.Controller) Option {
	return WithForcedDynamicWorkers(controller)
}

// WithErrorStrategy configures how worker panics and errors are handled for the
// current operation.
func WithErrorStrategy(strategy ErrorStrategy) Option {
	return func(opts *Options) {
		MustValidateErrorStrategy(strategy)
		opts.errorStrategy = strategy
	}
}

// NewOptions returns the default operation settings.
func NewOptions() *Options {
	return &Options{
		workers:       DefaultWorkers,
		errorStrategy: ErrorStrategyFailFast,
	}
}

// BuildOptions applies opts in order on top of the default settings.
func BuildOptions(opts ...Option) *Options {
	options := NewOptions()
	for _, opt := range opts {
		opt(options)
	}

	return options
}

// Workers returns the configured fixed worker count.
func (o *Options) Workers() int {
	if o == nil {
		return DefaultWorkers
	}

	return o.workers
}

// Unlimited reports whether the operation should spawn one worker per item.
func (o *Options) Unlimited() bool {
	return o != nil && o.unlimited
}

// Controller returns the dynamic concurrency controller, if configured.
func (o *Options) Controller() *control.Controller {
	if o == nil {
		return nil
	}

	return o.controller
}

// Interruptible reports whether dynamic worker shrink should cancel workers.
func (o *Options) Interruptible() bool {
	return o != nil && o.interruptible
}

// ErrorStrategy returns the worker failure strategy for the operation.
func (o *Options) ErrorStrategy() ErrorStrategy {
	if o == nil {
		return ErrorStrategyFailFast
	}

	return o.errorStrategy
}
