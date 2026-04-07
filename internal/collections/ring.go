package collections

import (
	"slices"
	"sync"
)

// Ring stores the last n items in insertion order and is safe for concurrent
// use.
type Ring[T any] struct {
	elements []T
	index    int
	full     bool
	lock     sync.RWMutex
}

// NewRing allocates a ring buffer that can hold exactly n items.
func NewRing[T any](n int) *Ring[T] {
	if n < 1 {
		panic("ring size must be positive")
	}

	return &Ring[T]{elements: make([]T, 0, n)}
}

// Add appends item, overwriting the oldest value once the ring is full.
func (r *Ring[T]) Add(item T) {
	r.lock.Lock()
	defer r.lock.Unlock()

	if !r.full {
		r.elements = append(r.elements, item)
		r.full = len(r.elements) == cap(r.elements)
		if r.full {
			r.index = 0
		}
		return
	}

	r.elements[r.index] = item
	r.index++
	if r.index == len(r.elements) {
		r.index = 0
	}
}

// Take returns the current contents from oldest to newest.
func (r *Ring[T]) Take() []T {
	r.lock.RLock()
	defer r.lock.RUnlock()

	size := len(r.elements)
	if size == 0 {
		return nil
	}

	if !r.full {
		return slices.Clone(r.elements)
	}

	start := r.index
	result := make([]T, size)
	copy(result, r.elements[start:])
	copy(result[size-start:], r.elements[:start])
	return result
}
