package flx

import (
	"errors"
	"fmt"
)

var ErrInvalidErrorStrategy = errors.New("flx: invalid error strategy")

type ErrorStrategy uint8

const (
	ErrorStrategyFailFast ErrorStrategy = iota
	ErrorStrategyCollect
	ErrorStrategyLogAndContinue
)

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

type WorkerError struct {
	Err error
}

func (e *WorkerError) Error() string {
	if e == nil || e.Err == nil {
		return "flx: worker failed"
	}

	return e.Err.Error()
}

func (e *WorkerError) Unwrap() error {
	if e == nil {
		return nil
	}

	return e.Err
}

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
