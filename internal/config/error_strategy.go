package config

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
	// ErrorStrategyContinue ignores worker errors and allows the operation to
	// continue without recording them in stream state.
	ErrorStrategyContinue
	// ErrorStrategyLogAndContinue is a deprecated compatibility alias for
	// ErrorStrategyContinue.
	//
	// Deprecated: use ErrorStrategyContinue.
	ErrorStrategyLogAndContinue = ErrorStrategyContinue
)

// String returns a human-readable representation of s.
func (s ErrorStrategy) String() string {
	switch s {
	case ErrorStrategyFailFast:
		return "fail-fast"
	case ErrorStrategyCollect:
		return "collect"
	case ErrorStrategyContinue:
		return "continue"
	default:
		return fmt.Sprintf("ErrorStrategy(%d)", s)
	}
}

// ValidateErrorStrategy returns an error when strategy is unsupported.
func ValidateErrorStrategy(strategy ErrorStrategy) error {
	switch strategy {
	case ErrorStrategyFailFast, ErrorStrategyCollect, ErrorStrategyContinue:
		return nil
	default:
		return fmt.Errorf("%w: %s", ErrInvalidErrorStrategy, strategy)
	}
}

// MustValidateErrorStrategy panics when strategy is unsupported.
func MustValidateErrorStrategy(strategy ErrorStrategy) {
	if err := ValidateErrorStrategy(strategy); err != nil {
		panic(err)
	}
}
