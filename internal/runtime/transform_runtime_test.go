package runtime

import (
	"context"
	"sync/atomic"
	"testing"
)

type interruptibleTestController struct {
	workers int

	released atomic.Int32
	register func(context.CancelCauseFunc) int
}

func (c *interruptibleTestController) Workers() int {
	return c.workers
}

func (c *interruptibleTestController) Acquire(context.Context) error {
	return nil
}

func (c *interruptibleTestController) Release() {
	c.released.Add(1)
}

func (c *interruptibleTestController) RegisterCancel(cancel context.CancelCauseFunc) int {
	if c.register != nil {
		return c.register(cancel)
	}

	return 0
}

func (c *interruptibleTestController) UnregisterCancel(int) {}

func TestWalkInterruptibleSkipsFnWhenWorkerCanceledBeforeStart(t *testing.T) {
	t.Helper()

	var ran atomic.Int32
	controller := &interruptibleTestController{
		workers: 1,
		register: func(cancel context.CancelCauseFunc) int {
			cancel(context.Canceled)
			return 1
		},
	}

	source := make(chan int, 1)
	source <- 1
	close(source)

	out := WalkInterruptible[int, int](
		t.Context(),
		source,
		controller,
		func(parent context.Context) (context.Context, func(error)) {
			return parent, func(error) {}
		},
		func(ctx context.Context, item int, pipe chan<- int) error {
			ran.Add(1)
			return nil
		},
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	for range out {
	}

	if got := ran.Load(); got != 0 {
		t.Fatalf("expected canceled worker to skip user fn, got %d executions", got)
	}
	if got := controller.released.Load(); got != 1 {
		t.Fatalf("expected canceled worker to release its slot once, got %d releases", got)
	}
}
