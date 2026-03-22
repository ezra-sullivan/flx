package flx

import (
	"slices"
	"sync"
)

type ring[T any] struct {
	elements []T
	index    int
	full     bool
	lock     sync.RWMutex
}

func newRing[T any](n int) *ring[T] {
	if n < 1 {
		panic("ring size must be positive")
	}

	return &ring[T]{elements: make([]T, 0, n)}
}

func (r *ring[T]) Add(item T) {
	r.lock.Lock()
	defer r.lock.Unlock()

	if r.full {
		r.elements[r.index%cap(r.elements)] = item
	} else {
		r.elements = append(r.elements, item)
		r.full = len(r.elements) == cap(r.elements)
	}

	r.index++
}

func (r *ring[T]) Take() []T {
	r.lock.RLock()
	defer r.lock.RUnlock()

	size := len(r.elements)
	if size == 0 {
		return nil
	}

	if !r.full {
		return slices.Clone(r.elements)
	}

	start := r.index % size
	result := make([]T, size)
	copy(result, r.elements[start:])
	copy(result[size-start:], r.elements[:start])
	return result
}
