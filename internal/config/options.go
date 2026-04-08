package config

import (
	"errors"

	"github.com/ezra-sullivan/flx/internal/control"
	coordinatorinternal "github.com/ezra-sullivan/flx/internal/coordinator"
	"github.com/ezra-sullivan/flx/internal/link"
	"github.com/ezra-sullivan/flx/internal/metrics"
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
	workers              int
	unlimited            bool
	controller           *control.Controller
	interruptible        bool
	errorStrategy        ErrorStrategy
	coordinator          *coordinatorinternal.Coordinator
	stageName            string
	resolvedStageName    string
	stageBudget          coordinatorinternal.StageBudget
	linkMetricsObserver  link.MetricsObserver
	stageMetricsObserver metrics.StageMetricsObserver
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

// WithStageMetricsObserver registers the callback that receives stage metrics.
func WithStageMetricsObserver(observer metrics.StageMetricsObserver) Option {
	return func(opts *Options) {
		opts.stageMetricsObserver = observer
	}
}

// WithLinkMetricsObserver registers the callback that receives link metrics.
func WithLinkMetricsObserver(observer link.MetricsObserver) Option {
	return func(opts *Options) {
		opts.linkMetricsObserver = observer
	}
}

// WithStageName labels the current transform or terminal operation as one
// named stage.
func WithStageName(name string) Option {
	return func(opts *Options) {
		opts.stageName = name
		opts.resolvedStageName = ""
	}
}

// WithCoordinator registers one pipeline coordinator to receive runtime
// snapshots for this operation.
func WithCoordinator(coordinator *coordinatorinternal.Coordinator) Option {
	return func(opts *Options) {
		opts.coordinator = coordinator
		opts.resolvedStageName = ""
	}
}

// WithStageBudget constrains one coordinator-managed dynamic stage to a worker
// budget window.
func WithStageBudget(budget coordinatorinternal.StageBudget) Option {
	return func(opts *Options) {
		opts.stageBudget = budget
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

// StageMetricsObserver returns the configured observer for stage metrics.
func (o *Options) StageMetricsObserver() metrics.StageMetricsObserver {
	if o == nil {
		return nil
	}

	return o.stageMetricsObserver
}

// StageName returns the configured stage identity.
func (o *Options) StageName() string {
	if o == nil {
		return ""
	}

	return o.stageName
}

// ResolveStageName returns the stable stage identity for this operation,
// generating one when a coordinator is attached and no explicit name was set.
func (o *Options) ResolveStageName() string {
	if o == nil {
		return ""
	}
	if o.resolvedStageName != "" {
		return o.resolvedStageName
	}
	if o.stageName != "" {
		o.resolvedStageName = o.stageName
		return o.resolvedStageName
	}
	if o.coordinator != nil {
		o.resolvedStageName = o.coordinator.ResolveStageName("")
	}

	return o.resolvedStageName
}

// LinkMetricsObserver returns the configured observer for link metrics.
func (o *Options) LinkMetricsObserver() link.MetricsObserver {
	if o == nil {
		return nil
	}

	return o.linkMetricsObserver
}

// Coordinator returns the configured pipeline coordinator, if any.
func (o *Options) Coordinator() *coordinatorinternal.Coordinator {
	if o == nil {
		return nil
	}

	return o.coordinator
}

// StageBudget returns the configured worker budget for one stage.
func (o *Options) StageBudget() coordinatorinternal.StageBudget {
	if o == nil {
		return coordinatorinternal.StageBudget{}
	}

	return o.stageBudget
}
