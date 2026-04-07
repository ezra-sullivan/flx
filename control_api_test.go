package flx

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestConcurrencyControllerDelegatesWorkerLimit(t *testing.T) {
	ctrl := NewConcurrencyController(0)

	if got := ctrl.Workers(); got != 1 {
		t.Fatalf("expected constructor to clamp workers to 1, got %d", got)
	}

	ctrl.SetWorkers(5)
	if got := ctrl.Workers(); got != 5 {
		t.Fatalf("expected SetWorkers to update limit, got %d", got)
	}
}

// These tests exercise DynamicSemaphore acquisition, cancellation, resize, and leak resistance.
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

// These tests cover worker error strategy behavior across terminals and helper APIs.
func TestDonePanicsOnFailFastError(t *testing.T) {
	errBoom := errors.New("boom")

	assertPanicIs(t, errBoom, func() {
		MapErr(Values(1, 2, 3), func(v int) (int, error) {
			if v == 2 {
				return 0, errBoom
			}
			return v, nil
		}, WithWorkers(1)).Done()
	})
}

func TestCollectStrategyReturnsErrorWithoutPanic(t *testing.T) {
	errBoom := errors.New("boom")

	items, err := MapErr(Values(1, 2, 3), func(v int) (int, error) {
		if v == 2 {
			return 0, errBoom
		}
		return v * 10, nil
	}, WithWorkers(1), WithErrorStrategy(ErrorStrategyCollect)).CollectErr()

	if !errors.Is(err, errBoom) {
		t.Fatalf("expected collected error to match boom, got %v", err)
	}
	if len(items) != 2 || items[0] != 10 || items[1] != 30 {
		t.Fatalf("unexpected collected items: %v", items)
	}
}

func TestParallelErrCollectsPanicAndError(t *testing.T) {
	errBoom := errors.New("boom")

	err := ParallelErr(
		func() error { return errBoom },
		func() error {
			panic("parallel panic")
		},
	)

	if !errors.Is(err, errBoom) {
		t.Fatalf("expected aggregated error to contain boom, got %v", err)
	}
	if !strings.Contains(err.Error(), "parallel panic") {
		t.Fatalf("expected aggregated error to contain panic text, got %v", err)
	}
}

func TestParallelWithErrorStrategyPanicsOnInvalidStrategy(t *testing.T) {
	assertPanicIs(t, ErrInvalidErrorStrategy, func() {
		_ = ParallelWithErrorStrategy(ErrorStrategy(100), func() {})
	})
}

func TestInterruptibleWorkersRequireContextTransform(t *testing.T) {
	ctrl := NewConcurrencyController(2)

	assertPanicIs(t, ErrInterruptibleWorkersRequireContextTransform, func() {
		_ = Map(Values(1, 2, 3), func(v int) int { return v }, WithInterruptibleWorkers(ctrl))
	})
}

func TestShortCircuitTerminalsPanicWhenFailFastErrorAlreadyRecorded(t *testing.T) {
	errBoom := errors.New("boom")

	newFailedStream := func(items ...int) Stream[int] {
		source := make(chan int, len(items))
		for _, item := range items {
			source <- item
		}
		close(source)

		state := newStreamState()
		state.add(errBoom, true)
		return streamWithState(source, state)
	}

	tests := []struct {
		name string
		run  func(Stream[int])
	}{
		{
			name: "First",
			run: func(s Stream[int]) {
				_, _ = s.First()
			},
		},
		{
			name: "AllMatch",
			run: func(s Stream[int]) {
				_ = s.AllMatch(func(v int) bool { return v > 0 })
			},
		},
		{
			name: "AnyMatch",
			run: func(s Stream[int]) {
				_ = s.AnyMatch(func(v int) bool { return v == 1 })
			},
		},
		{
			name: "NoneMatch",
			run: func(s Stream[int]) {
				_ = s.NoneMatch(func(v int) bool { return v == 1 })
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertPanicIs(t, errBoom, func() {
				tt.run(newFailedStream(1))
			})
		})
	}
}
