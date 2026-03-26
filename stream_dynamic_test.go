package flx

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestFlatMapWithDynamicWorkersRespectsConcurrency(t *testing.T) {
	ctrl := NewConcurrencyController(4)
	var maxConcurrent atomic.Int32
	var current atomic.Int32

	var (
		result []int
		mu     sync.Mutex
	)

	FlatMap(Values(1, 2, 3, 4, 5, 6, 7, 8), func(item int, pipe chan<- int) {
		cur := current.Add(1)
		for {
			old := maxConcurrent.Load()
			if cur <= old || maxConcurrent.CompareAndSwap(old, cur) {
				break
			}
		}

		time.Sleep(20 * time.Millisecond)
		current.Add(-1)
		pipe <- item
	}, WithDynamicWorkers(ctrl)).ForEach(func(item int) {
		mu.Lock()
		result = append(result, item)
		mu.Unlock()
	})

	if len(result) != 8 {
		t.Fatalf("expected 8 items, got %d", len(result))
	}
	if maxConcurrent.Load() > 4 {
		t.Fatalf("max concurrent should be <= 4, got %d", maxConcurrent.Load())
	}
}

func TestInterruptibleWorkersShrinkCancelsExtraWorkers(t *testing.T) {
	ctrl := NewConcurrencyController(4)

	source := make(chan int, 4)
	for i := range 4 {
		source <- i
	}
	close(source)

	var (
		started       atomic.Int32
		canceledCount atomic.Int32
		completed     atomic.Int32
	)
	allStarted := make(chan struct{})
	release := make(chan struct{})

	done := make(chan struct{})
	go func() {
		FlatMapContext(t.Context(), FromChan(source), func(ctx context.Context, item int, pipe chan<- int) {
			if started.Add(1) == 4 {
				close(allStarted)
			}
			<-release

			select {
			case <-ctx.Done():
				if errors.Is(context.Cause(ctx), ErrWorkerLimitReduced) {
					canceledCount.Add(1)
				}
				return
			case <-time.After(20 * time.Millisecond):
			}

			completed.Add(1)
			pipe <- item
		}, WithForcedDynamicWorkers(ctrl)).Done()
		close(done)
	}()

	select {
	case <-allStarted:
	case <-time.After(time.Second):
		t.Fatal("expected all workers to start")
	}

	ctrl.SetWorkers(1)
	close(release)

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("expected pipeline to finish after shrink")
	}

	if got := canceledCount.Load(); got != 3 {
		t.Fatalf("expected 3 canceled workers after shrinking to 1, got %d", got)
	}
	if got := completed.Load(); got != 1 {
		t.Fatalf("expected 1 surviving worker after shrinking to 1, got %d", got)
	}
}

func TestMapContextForcedDynamicWorkersCanceledSendReleasesSlots(t *testing.T) {
	ctrl := NewConcurrencyController(1)

	source := make(chan int, 4)
	for i := range 4 {
		source <- i
	}
	close(source)

	var started atomic.Int32
	firstStarted := make(chan struct{})
	allStarted := make(chan struct{})
	release := make(chan struct{})

	out := MapContext(t.Context(), FromChan(source), func(ctx context.Context, item int) int {
		switch started.Add(1) {
		case 1:
			close(firstStarted)
		case 4:
			close(allStarted)
		}

		<-release
		return item
	}, WithForcedDynamicWorkers(ctrl))

	select {
	case <-firstStarted:
	case <-time.After(time.Second):
		t.Fatal("expected first worker to start")
	}

	ctrl.SetWorkers(4)

	select {
	case <-allStarted:
	case <-time.After(time.Second):
		t.Fatal("expected all workers to start after scale up")
	}

	ctrl.SetWorkers(1)
	close(release)

	deadline := time.Now().Add(time.Second)
	for ctrl.ActiveWorkers() > 1 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}

	if got := ctrl.ActiveWorkers(); got > 1 {
		t.Fatalf("expected canceled sends to release extra workers, active=%d", got)
	}

	select {
	case <-out.source:
	case <-time.After(time.Second):
		t.Fatal("expected surviving worker output")
	}

	drain(out.source)

	deadline = time.Now().Add(time.Second)
	for ctrl.ActiveWorkers() != 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}

	if got := ctrl.ActiveWorkers(); got != 0 {
		t.Fatalf("expected worker slots to remain released after draining output, active=%d", got)
	}
}

func TestFilterMapReduceWithDynamicWorkers(t *testing.T) {
	ctrl := NewConcurrencyController(4)

	filtered := Values(1, 2, 3, 4, 5, 6, 7, 8, 9, 10).Filter(func(item int) bool {
		return item%2 == 0
	}, WithDynamicWorkers(ctrl))

	mapped := Map(filtered, func(item int) int {
		return item * 10
	}, WithDynamicWorkers(ctrl))

	result, err := Reduce(mapped, func(pipe <-chan int) (int, error) {
		var sum int
		for item := range pipe {
			sum += item
		}
		return sum, nil
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	expected := (2 + 4 + 6 + 8 + 10) * 10
	if result != expected {
		t.Fatalf("expected %d, got %d", expected, result)
	}
}
