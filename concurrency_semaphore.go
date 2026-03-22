package flx

import (
	"container/list"
	"context"
	"sync"
)

type DynamicSemaphore struct {
	mu      sync.Mutex
	max     int
	cur     int
	waiters list.List
}

func NewDynamicSemaphore(n int) *DynamicSemaphore {
	return &DynamicSemaphore{max: max(n, minWorkers)}
}

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

func (s *DynamicSemaphore) Release() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cur <= 0 {
		panic("flx: DynamicSemaphore: Release called without matching Acquire")
	}

	s.cur--
	s.dispatchLocked()
}

func (s *DynamicSemaphore) Resize(n int) {
	s.mu.Lock()
	s.max = max(n, minWorkers)
	s.dispatchLocked()
	s.mu.Unlock()
}

func (s *DynamicSemaphore) Cap() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.max
}

func (s *DynamicSemaphore) Current() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.cur
}

type semaphoreWaiter struct {
	ready   chan struct{}
	elem    *list.Element
	granted bool
}

func (s *DynamicSemaphore) canAcquireLocked() bool {
	return s.waiters.Len() == 0 && s.cur < s.max
}

func (s *DynamicSemaphore) enqueueWaiterLocked() *semaphoreWaiter {
	waiter := &semaphoreWaiter{ready: make(chan struct{})}
	waiter.elem = s.waiters.PushBack(waiter)
	return waiter
}

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
