package flx

import (
	"errors"
	"testing"

	"github.com/ezra-sullivan/flx/internal/collections"
	"github.com/ezra-sullivan/flx/internal/state"
	"github.com/ezra-sullivan/flx/internal/streaming"
)

type streamTestState struct {
	handle state.Handle
}

func newStreamState() *streamTestState {
	return &streamTestState{handle: state.NewStream()}
}

func (s *streamTestState) add(err error, panicOnTerminal bool) {
	if s == nil || s.handle == nil {
		return
	}

	s.handle.Add(err, panicOnTerminal)
}

func streamWithState[T any](source <-chan T, state *streamTestState) Stream[T] {
	if state == nil {
		state = newStreamState()
	}

	return wrapStream(streaming.WithState(source, state.handle))
}

func newRing[T any](n int) *collections.Ring[T] {
	return collections.NewRing[T](n)
}

// assertPanicIs verifies that fn panics with an error matching target.
func assertPanicIs(t *testing.T, target error, fn func()) {
	t.Helper()

	var recovered any
	func() {
		defer func() {
			recovered = recover()
		}()
		fn()
	}()

	if recovered == nil {
		t.Fatal("expected panic")
	}

	err, ok := recovered.(error)
	if !ok {
		t.Fatalf("expected panic value to be error, got %T", recovered)
	}
	if !errors.Is(err, target) {
		t.Fatalf("expected panic to match %v, got %v", target, err)
	}
}
