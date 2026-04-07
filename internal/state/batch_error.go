package state

import (
	"errors"
	"slices"
	"sync"
)

// BatchError collects multiple errors from concurrent workers and can expose
// them either as one joined error or as a cloned slice.
type BatchError struct {
	mu   sync.RWMutex
	errs []error
}

// Add records a non-nil error for later aggregation.
func (be *BatchError) Add(err error) {
	if err == nil {
		return
	}

	be.mu.Lock()
	be.errs = append(be.errs, err)
	be.mu.Unlock()
}

// Err joins all recorded errors. It returns nil when no error was added.
func (be *BatchError) Err() error {
	be.mu.RLock()
	defer be.mu.RUnlock()

	return errors.Join(be.errs...)
}

// Unwrap returns a snapshot of the recorded errors in insertion order.
func (be *BatchError) Unwrap() []error {
	be.mu.RLock()
	defer be.mu.RUnlock()

	return slices.Clone(be.errs)
}
