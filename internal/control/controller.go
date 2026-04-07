package control

import (
	"context"
	"errors"
	"maps"
	"slices"
	"sync"
)

// ErrWorkerLimitReduced reports that a worker was canceled because a forced
// dynamic controller shrank below the number of active workers.
var ErrWorkerLimitReduced = errors.New("flx: worker canceled because concurrency limit was reduced")

// Controller coordinates resizable worker limits for dynamic stream
// operations. It is safe for concurrent use.
type Controller struct {
	sem *DynamicSemaphore

	mu      sync.Mutex
	nextID  int
	cancels map[int]context.CancelCauseFunc
}

// NewController returns a controller whose worker limit starts at workers,
// clamped to at least one slot.
func NewController(workers int) *Controller {
	return &Controller{
		sem:     NewDynamicSemaphore(workers),
		cancels: make(map[int]context.CancelCauseFunc),
	}
}

// SetWorkers updates the target worker count. When the limit shrinks, the
// newest registered interruptible workers are canceled until the active count
// fits within the new limit.
func (c *Controller) SetWorkers(n int) {
	n = max(n, minWorkers)
	c.sem.Resize(n)
	c.cancelExcess(n)
}

// Workers returns the configured worker limit.
func (c *Controller) Workers() int {
	return c.sem.Cap()
}

// ActiveWorkers returns the number of workers currently holding semaphore slots.
func (c *Controller) ActiveWorkers() int {
	return c.sem.Current()
}

// Acquire blocks until a worker slot is available or ctx is canceled.
func (c *Controller) Acquire(ctx context.Context) error {
	return c.sem.AcquireCtx(ctx)
}

// Release frees one acquired worker slot.
func (c *Controller) Release() {
	c.sem.Release()
}

// RegisterCancel records one interruptible worker cancel function and returns
// a monotonically increasing registration ID.
func (c *Controller) RegisterCancel(cancel context.CancelCauseFunc) int {
	c.mu.Lock()
	defer c.mu.Unlock()

	id := c.nextID
	c.nextID++
	c.cancels[id] = cancel
	return id
}

// UnregisterCancel removes a worker cancel function after that worker exits.
func (c *Controller) UnregisterCancel(id int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.cancels, id)
}

// cancelExcess cancels the newest registered workers until the interruptible
// worker count no longer exceeds newLimit.
func (c *Controller) cancelExcess(newLimit int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	excess := len(c.cancels) - newLimit
	if excess <= 0 {
		return
	}

	ids := slices.Sorted(maps.Keys(c.cancels))
	for i := len(ids) - 1; i >= 0 && excess > 0; i-- {
		id := ids[i]
		cancel, ok := c.cancels[id]
		if !ok {
			continue
		}

		cancel(ErrWorkerLimitReduced)
		delete(c.cancels, id)
		excess--
	}
}
