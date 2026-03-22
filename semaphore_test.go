package flx

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestDynamicSemaphoreBasicAcquireRelease(t *testing.T) {
	sem := NewDynamicSemaphore(3)
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
	sem := NewDynamicSemaphore(1)
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
		sem := NewDynamicSemaphore(1)
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
	sem := NewDynamicSemaphore(1)
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
