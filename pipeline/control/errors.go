package control

import (
	"errors"

	"github.com/ezra-sullivan/flx/internal/config"
	controlinternal "github.com/ezra-sullivan/flx/internal/control"
)

var (
	// ErrWorkerLimitReduced reports that a worker was canceled because a forced
	// dynamic controller shrank below the number of active workers.
	ErrWorkerLimitReduced = controlinternal.ErrWorkerLimitReduced
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
type ErrorStrategy = config.ErrorStrategy

const (
	// ErrorStrategyFailFast cancels the operation and makes non-Err terminals
	// panic once the failure is observed.
	ErrorStrategyFailFast = config.ErrorStrategyFailFast
	// ErrorStrategyCollect records worker errors and returns them from Err
	// terminals without canceling sibling workers.
	ErrorStrategyCollect = config.ErrorStrategyCollect
	// ErrorStrategyLogAndContinue logs worker errors and allows the operation to
	// continue without recording them in stream state.
	ErrorStrategyLogAndContinue = config.ErrorStrategyLogAndContinue
)

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

// ValidateErrorStrategy returns an error when strategy is unsupported.
func ValidateErrorStrategy(strategy ErrorStrategy) error {
	return config.ValidateErrorStrategy(config.ErrorStrategy(strategy))
}

// MustValidateErrorStrategy panics when strategy is unsupported.
func MustValidateErrorStrategy(strategy ErrorStrategy) {
	if err := ValidateErrorStrategy(strategy); err != nil {
		panic(err)
	}
}

// WrapWorkerError ensures err is represented as a WorkerError without double
// wrapping values that are already in that form.
func WrapWorkerError(err error) error {
	if err == nil {
		return nil
	}

	if _, ok := errors.AsType[*WorkerError](err); ok {
		return err
	}

	return &WorkerError{Err: err}
}
