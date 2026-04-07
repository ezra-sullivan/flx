package state

import (
	"errors"
	"sync/atomic"
)

// Handle abstracts local and merged stream error state.
type Handle interface {
	Add(err error, panicOnTerminal bool)
	Err() error
	ShouldPanic() bool
}

// streamState stores the errors produced by one stream and whether non-Err
// terminals should panic when they observe those errors.
type streamState struct {
	errs            BatchError
	panicOnTerminal atomic.Bool
}

// mergedStreamState combines local errors with one or more parent stream
// states so derived streams can reflect upstream failures immediately.
type mergedStreamState struct {
	local   *streamState
	parents []Handle
}

// NewStream returns an empty local stream error state.
func NewStream() Handle {
	return &streamState{}
}

// NewMerged creates a state that adds local errors on top of the current error
// state from parents.
func NewMerged(parents ...Handle) Handle {
	return &mergedStreamState{
		local:   &streamState{},
		parents: append([]Handle(nil), parents...),
	}
}

// Add records err in the local state and optionally marks non-Err terminals to
// panic when they observe the aggregated failure.
func (s *streamState) Add(err error, panicOnTerminal bool) {
	if s == nil || err == nil {
		return
	}

	s.errs.Add(err)
	if panicOnTerminal {
		s.panicOnTerminal.Store(true)
	}
}

// Err returns the joined local error state.
func (s *streamState) Err() error {
	if s == nil {
		return nil
	}

	return s.errs.Err()
}

// ShouldPanic reports whether non-Err terminals should panic on recorded
// errors.
func (s *streamState) ShouldPanic() bool {
	return s != nil && s.panicOnTerminal.Load()
}

// Add records an error that originates from the derived stream itself.
func (s *mergedStreamState) Add(err error, panicOnTerminal bool) {
	if s == nil {
		return
	}

	s.local.Add(err, panicOnTerminal)
}

// Err joins local errors with the current errors from all parent streams.
func (s *mergedStreamState) Err() error {
	if s == nil {
		return nil
	}

	errorsList := make([]error, 0, 1+len(s.parents))
	if err := s.local.Err(); err != nil {
		errorsList = append(errorsList, err)
	}
	for _, parent := range s.parents {
		if parent == nil {
			continue
		}
		if err := parent.Err(); err != nil {
			errorsList = append(errorsList, err)
		}
	}

	return errors.Join(errorsList...)
}

// ShouldPanic reports whether either the local stream or any parent stream has
// requested panic-on-terminal behavior.
func (s *mergedStreamState) ShouldPanic() bool {
	if s == nil {
		return false
	}
	if s.local.ShouldPanic() {
		return true
	}
	for _, parent := range s.parents {
		if parent != nil && parent.ShouldPanic() {
			return true
		}
	}
	return false
}
