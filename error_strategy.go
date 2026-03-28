package flx

import (
	"errors"
	"fmt"
)

// ErrInvalidErrorStrategy reports that an operation received an unsupported
// worker error handling mode.
var ErrInvalidErrorStrategy = errors.New("flx: invalid error strategy")

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

// validateErrorStrategy returns an error when strategy is unsupported.
func validateErrorStrategy(strategy ErrorStrategy) error {
	switch strategy {
	case ErrorStrategyFailFast, ErrorStrategyCollect, ErrorStrategyLogAndContinue:
		return nil
	default:
		return fmt.Errorf("%w: %s", ErrInvalidErrorStrategy, strategy)
	}
}

// mustValidateErrorStrategy panics when strategy is unsupported.
func mustValidateErrorStrategy(strategy ErrorStrategy) {
	if err := validateErrorStrategy(strategy); err != nil {
		panic(err)
	}
}
