package flx

import (
	"errors"
	"slices"
	"sync"
)

// batchError collects multiple errors and is safe for concurrent use.
type batchError struct {
	mu   sync.RWMutex
	errs []error
}

func (be *batchError) Add(err error) {
	if err == nil {
		return
	}

	be.mu.Lock()
	be.errs = append(be.errs, err)
	be.mu.Unlock()
}

func (be *batchError) Err() error {
	be.mu.RLock()
	defer be.mu.RUnlock()

	return errors.Join(be.errs...)
}

func (be *batchError) Unwrap() []error {
	be.mu.RLock()
	defer be.mu.RUnlock()

	return slices.Clone(be.errs)
}
