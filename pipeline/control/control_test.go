package control_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ezra-sullivan/flx/pipeline/control"
)

func TestConcurrencyControllerDelegatesWorkerLimit(t *testing.T) {
	ctrl := control.NewConcurrencyController(0)

	if got := ctrl.Workers(); got != 1 {
		t.Fatalf("expected constructor to clamp workers to 1, got %d", got)
	}

	ctrl.SetWorkers(5)
	if got := ctrl.Workers(); got != 5 {
		t.Fatalf("expected SetWorkers to update limit, got %d", got)
	}
}

func TestDynamicSemaphoreBasicAcquireRelease(t *testing.T) {
	sem := control.NewDynamicSemaphore(3)
	sem.Acquire()
	sem.Acquire()
	sem.Acquire()

	done := make(chan struct{})
	go func() {
		sem.Acquire()
		close(done)
	}()

	select {
	case <-done:
		t.Fatal("Acquire should block when semaphore is full")
	case <-time.After(50 * time.Millisecond):
	}

	sem.Release()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Acquire should proceed after Release")
	}
}

func TestDynamicSemaphoreAcquireCtxCancel(t *testing.T) {
	sem := control.NewDynamicSemaphore(1)
	sem.Acquire()

	ctx, cancel := context.WithCancel(t.Context())
	errCh := make(chan error, 1)
	go func() {
		errCh <- sem.AcquireCtx(ctx)
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context cancellation, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("AcquireCtx should return after cancellation")
	}

	if got := sem.Current(); got != 1 {
		t.Fatalf("expected only the original slot to remain held, got %d", got)
	}
}

func TestDynamicSemaphoreAcquireCtxConcurrentCancelAndReleaseNoLeak(t *testing.T) {
	for i := range 200 {
		sem := control.NewDynamicSemaphore(1)
		sem.Acquire()

		ctx, cancel := context.WithCancel(t.Context())
		errCh := make(chan error, 1)
		go func() {
			errCh <- sem.AcquireCtx(ctx)
		}()

		time.Sleep(time.Microsecond)

		var wg sync.WaitGroup
		wg.Go(cancel)
		wg.Go(func() {
			sem.Release()
		})
		wg.Wait()

		select {
		case err := <-errCh:
			switch {
			case err == nil:
				sem.Release()
			case errors.Is(err, context.Canceled):
			default:
				t.Fatalf("iteration %d: unexpected error %v", i, err)
			}
		case <-time.After(time.Second):
			t.Fatalf("iteration %d: AcquireCtx did not finish", i)
		}

		if got := sem.Current(); got != 0 {
			t.Fatalf("iteration %d: leaked semaphore slot, current=%d", i, got)
		}
	}
}

func TestDynamicSemaphoreResizeUp(t *testing.T) {
	sem := control.NewDynamicSemaphore(1)
	sem.Acquire()

	var acquired atomic.Int32
	for range 3 {
		go func() {
			sem.Acquire()
			acquired.Add(1)
		}()
	}

	time.Sleep(50 * time.Millisecond)
	if acquired.Load() != 0 {
		t.Fatal("no goroutines should acquire before resize")
	}

	sem.Resize(4)
	time.Sleep(100 * time.Millisecond)
	if got := acquired.Load(); got != 3 {
		t.Fatalf("expected 3 goroutines to acquire after resize, got %d", got)
	}
}

func TestWrapWorkerErrorAvoidsDoubleWrap(t *testing.T) {
	boom := errors.New("boom")

	wrapped := control.WrapWorkerError(boom)
	if wrapped == nil {
		t.Fatal("expected wrapped error")
	}

	var workerErr *control.WorkerError
	if !errors.As(wrapped, &workerErr) {
		t.Fatalf("expected WorkerError, got %T", wrapped)
	}
	if !errors.Is(wrapped, boom) {
		t.Fatalf("expected wrapped error to match boom, got %v", wrapped)
	}

	again := control.WrapWorkerError(workerErr)
	if again != workerErr {
		t.Fatal("expected WrapWorkerError to avoid double wrapping")
	}
}

func TestValidateErrorStrategyRejectsUnknownValue(t *testing.T) {
	if err := control.ValidateErrorStrategy(control.ErrorStrategy(100)); !errors.Is(err, control.ErrInvalidErrorStrategy) {
		t.Fatalf("expected invalid strategy error, got %v", err)
	}
}
