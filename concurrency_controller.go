package flx

import (
	"context"
	"errors"
	"maps"
	"slices"
	"sync"
)

var ErrWorkerLimitReduced = errors.New("flx: worker canceled because concurrency limit was reduced")

type ConcurrencyController struct {
	sem *DynamicSemaphore

	mu      sync.Mutex
	nextID  int
	cancels map[int]context.CancelCauseFunc
}

func NewConcurrencyController(workers int) *ConcurrencyController {
	return &ConcurrencyController{
		sem:     NewDynamicSemaphore(workers),
		cancels: make(map[int]context.CancelCauseFunc),
	}
}

func (c *ConcurrencyController) SetWorkers(n int) {
	n = max(n, minWorkers)
	c.sem.Resize(n)
	c.cancelExcess(n)
}

func (c *ConcurrencyController) Workers() int {
	return c.sem.Cap()
}

func (c *ConcurrencyController) ActiveWorkers() int {
	return c.sem.Current()
}

func (c *ConcurrencyController) registerCancel(cancel context.CancelCauseFunc) int {
	c.mu.Lock()
	defer c.mu.Unlock()

	id := c.nextID
	c.nextID++
	c.cancels[id] = cancel
	return id
}

func (c *ConcurrencyController) unregisterCancel(id int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.cancels, id)
}

func (c *ConcurrencyController) cancelExcess(newLimit int) {
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
