package flx

import (
	"context"
	"errors"
	"log"
	"sync/atomic"
)

type streamState struct {
	errs            batchError
	panicOnTerminal atomic.Bool
}

type streamStateHandle interface {
	add(err error, panicOnTerminal bool)
	err() error
	shouldPanic() bool
}

type mergedStreamState struct {
	local   *streamState
	parents []streamStateHandle
}

func newStreamState() *streamState {
	return &streamState{}
}

func (s *streamState) add(err error, panicOnTerminal bool) {
	if s == nil || err == nil {
		return
	}

	s.errs.Add(err)
	if panicOnTerminal {
		s.panicOnTerminal.Store(true)
	}
}

func (s *streamState) err() error {
	if s == nil {
		return nil
	}

	return s.errs.Err()
}

func (s *streamState) shouldPanic() bool {
	return s != nil && s.panicOnTerminal.Load()
}

func newMergedStreamState(parents ...streamStateHandle) *mergedStreamState {
	return &mergedStreamState{
		local:   newStreamState(),
		parents: parents,
	}
}

func (s *mergedStreamState) add(err error, panicOnTerminal bool) {
	if s == nil {
		return
	}

	s.local.add(err, panicOnTerminal)
}

func (s *mergedStreamState) err() error {
	if s == nil {
		return nil
	}

	errs := make([]error, 0, 1+len(s.parents))
	if err := s.local.err(); err != nil {
		errs = append(errs, err)
	}
	for _, parent := range s.parents {
		if parent == nil {
			continue
		}
		if err := parent.err(); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

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

type operationController struct {
	ctx      context.Context
	cancel   context.CancelCauseFunc
	state    streamStateHandle
	strategy ErrorStrategy
}

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
