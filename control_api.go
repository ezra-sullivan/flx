package flx

import (
	"context"
	"errors"
	"fmt"

	"github.com/ezra-sullivan/flx/internal/config"
	"github.com/ezra-sullivan/flx/internal/control"
)

var (
	// ErrWorkerLimitReduced reports that a worker was canceled because a forced
	// dynamic controller shrank below the number of active workers.
	ErrWorkerLimitReduced = control.ErrWorkerLimitReduced
	// ErrInvalidErrorStrategy reports that an operation received an unsupported
	// worker error handling mode.
	ErrInvalidErrorStrategy = config.ErrInvalidErrorStrategy
	// ErrNilController reports that a dynamic-worker option received a nil
	// concurrency controller.
	ErrNilController = config.ErrNilController
	// ErrInterruptibleWorkersRequireContextTransform reports that forced dynamic
	// workers were requested for a transform that does not accept a context.
	ErrInterruptibleWorkersRequireContextTransform = config.ErrInterruptibleWorkersRequireContextTransform
)

// ErrorStrategy controls how concurrent operations react to worker failures.
type ErrorStrategy uint8

const (
	// ErrorStrategyFailFast cancels the operation and makes non-Err terminals
	// panic once the failure is observed.
	ErrorStrategyFailFast ErrorStrategy = iota
	// ErrorStrategyCollect records worker errors and returns them from Err
	// terminals without canceling sibling workers.
	ErrorStrategyCollect
	// ErrorStrategyLogAndContinue logs worker errors and allows the operation to
	// continue without recording them in stream state.
	ErrorStrategyLogAndContinue
)

// String returns a human-readable representation of s.
func (s ErrorStrategy) String() string {
	switch s {
	case ErrorStrategyFailFast:
		return "fail-fast"
	case ErrorStrategyCollect:
		return "collect"
	case ErrorStrategyLogAndContinue:
		return "log-and-continue"
	default:
		return fmt.Sprintf("ErrorStrategy(%d)", s)
	}
}

// WorkerError wraps a failure produced by one worker goroutine.
type WorkerError struct {
	Err error
}

// Error returns the wrapped worker error text.
func (e *WorkerError) Error() string {
	if e == nil || e.Err == nil {
		return "flx: worker failed"
	}

	return e.Err.Error()
}

// Unwrap returns the underlying worker error.
func (e *WorkerError) Unwrap() error {
	if e == nil {
		return nil
	}

	return e.Err
}

// wrapWorkerError ensures err is represented as a WorkerError without double
// wrapping values that are already in that form.
func wrapWorkerError(err error) error {
	if err == nil {
		return nil
	}

	if _, ok := errors.AsType[*WorkerError](err); ok {
		return err
	}

	return &WorkerError{Err: err}
}

func validateErrorStrategy(strategy ErrorStrategy) error {
	switch strategy {
	case ErrorStrategyFailFast, ErrorStrategyCollect, ErrorStrategyLogAndContinue:
		return nil
	default:
		return fmt.Errorf("%w: %s", ErrInvalidErrorStrategy, strategy)
	}
}

func mustValidateErrorStrategy(strategy ErrorStrategy) {
	if err := validateErrorStrategy(strategy); err != nil {
		panic(err)
	}
}

func toInternalErrorStrategy(strategy ErrorStrategy) config.ErrorStrategy {
	return config.ErrorStrategy(strategy)
}

// Option mutates the execution settings for one transform or terminal call.
type Option func(*config.Options)

func unwrapOptions(opts []Option) []config.Option {
	if len(opts) == 0 {
		return nil
	}

	internalOpts := make([]config.Option, 0, len(opts))
	for _, opt := range opts {
		if opt == nil {
			internalOpts = append(internalOpts, nil)
			continue
		}

		internalOpts = append(internalOpts, config.Option(opt))
	}

	return internalOpts
}

// WithWorkers limits the current operation to a fixed number of workers.
func WithWorkers(workers int) Option {
	return Option(config.WithWorkers(workers))
}

// WithUnlimitedWorkers spawns one worker per item for the current operation.
func WithUnlimitedWorkers() Option {
	return Option(config.WithUnlimitedWorkers())
}

// DynamicSemaphore is a resizable semaphore used by dynamic worker pipelines.
// It is safe for concurrent use.
type DynamicSemaphore struct {
	sem *control.DynamicSemaphore
}

// NewDynamicSemaphore returns a semaphore with capacity n, clamped to at least
// one slot.
func NewDynamicSemaphore(n int) *DynamicSemaphore {
	return &DynamicSemaphore{sem: control.NewDynamicSemaphore(n)}
}

// Acquire blocks until a slot is available.
func (s *DynamicSemaphore) Acquire() {
	s.sem.Acquire()
}

// AcquireCtx blocks until a slot is available or ctx is canceled.
func (s *DynamicSemaphore) AcquireCtx(ctx context.Context) error {
	return s.sem.AcquireCtx(ctx)
}

// Release frees one acquired slot and wakes the next waiter if capacity is
// available.
func (s *DynamicSemaphore) Release() {
	s.sem.Release()
}

// Resize updates the semaphore capacity and wakes queued waiters when the new
// capacity allows additional work to start.
func (s *DynamicSemaphore) Resize(n int) {
	s.sem.Resize(n)
}

// Cap returns the configured semaphore capacity.
func (s *DynamicSemaphore) Cap() int {
	return s.sem.Cap()
}

// Current returns the number of slots currently held.
func (s *DynamicSemaphore) Current() int {
	return s.sem.Current()
}

// ConcurrencyController coordinates resizable worker limits for dynamic stream
// operations. It is safe for concurrent use.
type ConcurrencyController struct {
	ctrl *control.Controller
}

// NewConcurrencyController returns a controller whose worker limit starts at
// workers, clamped to at least one slot.
func NewConcurrencyController(workers int) *ConcurrencyController {
	return &ConcurrencyController{
		ctrl: control.NewController(workers),
	}
}

// WithDynamicWorkers enables graceful dynamic resizing for the current
// operation. Shrinking does not interrupt workers that already hold a slot.
func WithDynamicWorkers(controller *ConcurrencyController) Option {
	return Option(config.WithDynamicWorkers(controller.internalController()))
}

// WithForcedDynamicWorkers enables forced dynamic resizing for the current
// operation. Shrinking cancels excess workers via context and only works with
// MapContext or FlatMapContext variants.
func WithForcedDynamicWorkers(controller *ConcurrencyController) Option {
	return Option(config.WithForcedDynamicWorkers(controller.internalController()))
}

// WithInterruptibleWorkers is a compatibility alias for
// WithForcedDynamicWorkers.
//
// Deprecated: use WithForcedDynamicWorkers.
func WithInterruptibleWorkers(controller *ConcurrencyController) Option {
	return WithForcedDynamicWorkers(controller)
}

// WithErrorStrategy configures how worker panics and errors are handled for the
// current operation.
func WithErrorStrategy(strategy ErrorStrategy) Option {
	return Option(config.WithErrorStrategy(toInternalErrorStrategy(strategy)))
}

// SetWorkers updates the target worker count. When the limit shrinks, the
// newest registered interruptible workers are canceled until the active count
// fits within the new limit.
func (c *ConcurrencyController) SetWorkers(n int) {
	c.ctrl.SetWorkers(n)
}

// Workers returns the configured worker limit.
func (c *ConcurrencyController) Workers() int {
	return c.ctrl.Workers()
}

// ActiveWorkers returns the number of workers currently holding semaphore slots.
func (c *ConcurrencyController) ActiveWorkers() int {
	return c.ctrl.ActiveWorkers()
}

func (c *ConcurrencyController) internalController() *control.Controller {
	if c == nil {
		return nil
	}

	return c.ctrl
}
