package control

import (
	"container/list"
	"context"
	"sync"
)

const minWorkers = 1

// DynamicSemaphore is a resizable semaphore used by dynamic worker pipelines.
// It is safe for concurrent use.
type DynamicSemaphore struct {
	mu      sync.Mutex
	max     int
	cur     int
	waiters list.List
}

// NewDynamicSemaphore returns a semaphore with capacity n, clamped to at least
// one slot.
func NewDynamicSemaphore(n int) *DynamicSemaphore {
	return &DynamicSemaphore{max: max(n, minWorkers)}
}

// Acquire blocks until a slot is available.
func (s *DynamicSemaphore) Acquire() {
	s.mu.Lock()
	if s.canAcquireLocked() {
		s.cur++
		s.mu.Unlock()
		return
	}

	waiter := s.enqueueWaiterLocked()
	s.mu.Unlock()
	<-waiter.ready
}

// AcquireCtx blocks until a slot is available or ctx is canceled.
func (s *DynamicSemaphore) AcquireCtx(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	s.mu.Lock()
	if s.canAcquireLocked() {
		s.cur++
		s.mu.Unlock()
		return nil
	}

	waiter := s.enqueueWaiterLocked()
	s.mu.Unlock()

	select {
	case <-waiter.ready:
		return nil
	case <-ctx.Done():
		s.mu.Lock()
		granted := waiter.granted
		if !granted && waiter.elem != nil {
			s.waiters.Remove(waiter.elem)
			waiter.elem = nil
			s.dispatchLocked()
		}
		s.mu.Unlock()

		if granted {
			return nil
		}

		return ctx.Err()
	}
}

// Release frees one acquired slot and wakes the next waiter if capacity is
// available. It panics if called without a matching Acquire or AcquireCtx.
func (s *DynamicSemaphore) Release() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cur <= 0 {
		panic("flx: DynamicSemaphore: Release called without matching Acquire")
	}

	s.cur--
	s.dispatchLocked()
}

// Resize updates the semaphore capacity and wakes queued waiters when the new
// capacity allows additional work to start.
func (s *DynamicSemaphore) Resize(n int) {
	s.mu.Lock()
	s.max = max(n, minWorkers)
	s.dispatchLocked()
	s.mu.Unlock()
}

// Cap returns the configured semaphore capacity.
func (s *DynamicSemaphore) Cap() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.max
}

// Current returns the number of slots currently held.
func (s *DynamicSemaphore) Current() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.cur
}

// semaphoreWaiter tracks one blocked Acquire or AcquireCtx call.
type semaphoreWaiter struct {
	ready   chan struct{}
	elem    *list.Element
	granted bool
}

// canAcquireLocked reports whether a caller can acquire immediately while s.mu
// is held.
func (s *DynamicSemaphore) canAcquireLocked() bool {
	return s.waiters.Len() == 0 && s.cur < s.max
}

// enqueueWaiterLocked appends one waiter to the FIFO wait queue while s.mu is
// held.
func (s *DynamicSemaphore) enqueueWaiterLocked() *semaphoreWaiter {
	waiter := &semaphoreWaiter{ready: make(chan struct{})}
	waiter.elem = s.waiters.PushBack(waiter)
	return waiter
}

// dispatchLocked grants queued waiters in FIFO order until the semaphore is
// full or the wait queue is empty.
func (s *DynamicSemaphore) dispatchLocked() {
	for s.cur < s.max {
		front := s.waiters.Front()
		if front == nil {
			return
		}

		waiter := front.Value.(*semaphoreWaiter)
		s.waiters.Remove(front)
		waiter.elem = nil
		waiter.granted = true
		s.cur++
		close(waiter.ready)
	}
}
