package flx

import (
	"context"
	"errors"
	"log"
	"sync/atomic"
)

// streamState stores the errors produced by one stream and whether non-Err
// terminals should panic when they observe those errors.
type streamState struct {
	errs            batchError
	panicOnTerminal atomic.Bool
}

// streamStateHandle abstracts local and merged stream error state.
type streamStateHandle interface {
	add(err error, panicOnTerminal bool)
	err() error
	shouldPanic() bool
}

// mergedStreamState combines local errors with one or more parent stream
// states so derived streams can reflect upstream failures immediately.
type mergedStreamState struct {
	local   *streamState
	parents []streamStateHandle
}

// newStreamState returns an empty local stream error state.
func newStreamState() *streamState {
	return &streamState{}
}

// add records err in the local state and optionally marks non-Err terminals to
// panic when they observe the aggregated failure.
func (s *streamState) add(err error, panicOnTerminal bool) {
	if s == nil || err == nil {
		return
	}

	s.errs.Add(err)
	if panicOnTerminal {
		s.panicOnTerminal.Store(true)
	}
}

// err returns the joined local error state.
func (s *streamState) err() error {
	if s == nil {
		return nil
	}

	return s.errs.Err()
}

// shouldPanic reports whether non-Err terminals should panic on recorded
// errors.
func (s *streamState) shouldPanic() bool {
	return s != nil && s.panicOnTerminal.Load()
}

// newMergedStreamState creates a state that adds local errors on top of the
// current error state from parents.
func newMergedStreamState(parents ...streamStateHandle) *mergedStreamState {
	return &mergedStreamState{
		local:   newStreamState(),
		parents: parents,
	}
}

// add records an error that originates from the derived stream itself.
func (s *mergedStreamState) add(err error, panicOnTerminal bool) {
	if s == nil {
		return
	}

	s.local.add(err, panicOnTerminal)
}

// err joins local errors with the current errors from all parent streams.
func (s *mergedStreamState) err() error {
	if s == nil {
		return nil
	}

	errorsList := make([]error, 0, 1+len(s.parents))
	if err := s.local.err(); err != nil {
		errorsList = append(errorsList, err)
	}
	for _, parent := range s.parents {
		if parent == nil {
			continue
		}
		if err := parent.err(); err != nil {
			errorsList = append(errorsList, err)
		}
	}

	return errors.Join(errorsList...)
}

// shouldPanic reports whether either the local stream or any parent stream has
// requested panic-on-terminal behavior.
func (s *mergedStreamState) shouldPanic() bool {
	if s == nil {
		return false
	}
	if s.local.shouldPanic() {
		return true
	}
	for _, parent := range s.parents {
		if parent != nil && parent.shouldPanic() {
			return true
		}
	}
	return false
}

// operationController owns the cancellation scope and worker error strategy for
// one concurrent stream operation.
type operationController struct {
	ctx      context.Context
	cancel   context.CancelCauseFunc
	state    streamStateHandle
	strategy ErrorStrategy
}

// newOperationController derives a worker context from parent and validates the
// requested error strategy.
func newOperationController(parent context.Context, state streamStateHandle, strategy ErrorStrategy) *operationController {
	if parent == nil {
		parent = context.Background()
	}

	mustValidateErrorStrategy(strategy)
	ctx, cancel := context.WithCancelCause(parent)
	return &operationController{
		ctx:      ctx,
		cancel:   cancel,
		state:    state,
		strategy: strategy,
	}
}

// report applies the configured strategy to one worker failure.
func (c *operationController) report(err error) {
	if err == nil {
		return
	}

	err = wrapWorkerError(err)

	switch c.strategy {
	case ErrorStrategyCollect:
		c.state.add(err, false)
	case ErrorStrategyLogAndContinue:
		log.Printf("[flx] worker error: %v", err)
	case ErrorStrategyFailFast:
		c.state.add(err, true)
		c.cancel(err)
	default:
		c.state.add(err, true)
		c.cancel(err)
	}
}
