package flx

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// These tests cover dynamic and interruptible worker behavior for stream transforms.
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

	items, err := out.CollectErr()
	if err != nil {
		t.Fatalf("expected nil stream error, got %v", err)
	}
	if got := len(items); got != 1 {
		t.Fatalf("expected exactly one surviving worker output, got %d (%v)", got, items)
	}

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

// These tests verify that short-circuit and partial consumers still drain upstream producers.
func TestShortCircuitOperatorsDrainUpstreamProducer(t *testing.T) {
	tests := []struct {
		name string
		run  func(Stream[int])
	}{
		{
			name: "First",
			run: func(s Stream[int]) {
				got, ok := s.First()
				if !ok || got != 0 {
					t.Fatalf("expected first item 0, got %v (ok=%v)", got, ok)
				}
			},
		},
		{
			name: "AnyMatch",
			run: func(s Stream[int]) {
				if !s.AnyMatch(func(item int) bool { return item == 0 }) {
					t.Fatal("expected AnyMatch to short-circuit with true")
				}
			},
		},
		{
			name: "AllMatch",
			run: func(s Stream[int]) {
				if s.AllMatch(func(item int) bool { return item > 0 }) {
					t.Fatal("expected AllMatch to short-circuit with false")
				}
			},
		},
		{
			name: "NoneMatch",
			run: func(s Stream[int]) {
				if s.NoneMatch(func(item int) bool { return item == 0 }) {
					t.Fatal("expected NoneMatch to short-circuit with false")
				}
			},
		},
		{
			name: "FirstErr",
			run: func(s Stream[int]) {
				got, ok, err := s.FirstErr()
				if err != nil {
					t.Fatalf("expected nil error, got %v", err)
				}
				if !ok || got != 0 {
					t.Fatalf("expected first item 0, got %v (ok=%v)", got, ok)
				}
			},
		},
		{
			name: "AnyMatchErr",
			run: func(s Stream[int]) {
				got, err := s.AnyMatchErr(func(item int) bool { return item == 0 })
				if err != nil {
					t.Fatalf("expected nil error, got %v", err)
				}
				if !got {
					t.Fatal("expected AnyMatchErr to short-circuit with true")
				}
			},
		},
		{
			name: "AllMatchErr",
			run: func(s Stream[int]) {
				got, err := s.AllMatchErr(func(item int) bool { return item > 0 })
				if err != nil {
					t.Fatalf("expected nil error, got %v", err)
				}
				if got {
					t.Fatal("expected AllMatchErr to short-circuit with false")
				}
			},
		},
		{
			name: "NoneMatchErr",
			run: func(s Stream[int]) {
				got, err := s.NoneMatchErr(func(item int) bool { return item == 0 })
				if err != nil {
					t.Fatalf("expected nil error, got %v", err)
				}
				if got {
					t.Fatal("expected NoneMatchErr to short-circuit with false")
				}
			},
		},
		{
			name: "Head",
			run: func(s Stream[int]) {
				if count := s.Head(1).Count(); count != 1 {
					t.Fatalf("expected Head(1) to produce 1 item, got %d", count)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stream, producerDone := testProducerStream(32)
			tt.run(stream)
			waitProducerDone(t, producerDone, tt.name)
		})
	}
}

func TestForAllPartialConsumerStillDrainsUpstreamProducer(t *testing.T) {
	stream, producerDone := testProducerStream(32)

	var consumed []int
	stream.ForAll(func(source <-chan int) {
		for range 3 {
			item, ok := <-source
			if !ok {
				t.Fatal("expected source to still have items during partial consumption")
			}
			consumed = append(consumed, item)
		}
	})

	if len(consumed) != 3 {
		t.Fatalf("expected 3 consumed items, got %d", len(consumed))
	}
	waitProducerDone(t, producerDone, "ForAll")
}

func TestReduceEarlyReturnStillDrainsUpstreamProducer(t *testing.T) {
	stream, producerDone := testProducerStream(32)

	got, err := Reduce(stream, func(source <-chan int) (int, error) {
		item, ok := <-source
		if !ok {
			t.Fatal("expected Reduce source to have an item")
		}

		return item, nil
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if got != 0 {
		t.Fatalf("expected first item 0, got %d", got)
	}

	waitProducerDone(t, producerDone, "Reduce")
}

// testProducerStream builds a producer that finishes only after downstream drains all emitted items.
func testProducerStream(total int) (Stream[int], <-chan struct{}) {
	done := make(chan struct{})
	stream := From(func(source chan<- int) {
		defer close(done)
		for i := range total {
			source <- i
		}
	})
	return stream, done
}

// waitProducerDone fails the test when an upstream producer remains blocked after the consumer returned.
func waitProducerDone(t *testing.T, done <-chan struct{}, name string) {
	t.Helper()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatalf("%s should drain upstream and let the producer exit", name)
	}
}

func assertComparable[T comparable]() {}

// These tests cover stream behaviors that do not fit a single transform or terminal category.
func TestStreamRemainsComparable(t *testing.T) {
	assertComparable[Stream[int]]()
}

func TestConcatReflectsUpstreamErrorsImmediately(t *testing.T) {
	leftSource := make(chan int)
	rightSource := make(chan int)
	leftState := newStreamState()
	rightState := newStreamState()
	left := streamWithState(leftSource, leftState)
	right := streamWithState(rightSource, rightState)

	out := left.Concat(right)

	leftErr := errors.New("left failed")
	rightErr := errors.New("right failed")
	leftState.add(leftErr, true)
	rightState.add(rightErr, false)

	if !errors.Is(out.Err(), leftErr) {
		t.Fatalf("expected concat state to reflect left error immediately, got %v", out.Err())
	}
	if !errors.Is(out.Err(), rightErr) {
		t.Fatalf("expected concat state to reflect right error immediately, got %v", out.Err())
	}

	close(leftSource)
	close(rightSource)

	assertPanicIs(t, leftErr, func() {
		out.Done()
	})
}

func TestGroupByPreservesFirstSeenKeyOrder(t *testing.T) {
	grouped := GroupBy(Values("b1", "a1", "b2", "c1", "a2"), func(v string) byte {
		return v[0]
	}).Collect()

	expected := [][]string{
		{"b1", "b2"},
		{"a1", "a2"},
		{"c1"},
	}
	if !slices.EqualFunc(grouped, expected, func(a, b []string) bool {
		return slices.Equal(a, b)
	}) {
		t.Fatalf("unexpected group order/content: %v", grouped)
	}
}

func TestDistinctByChunkAndCollect(t *testing.T) {
	items := Chunk(DistinctBy(Values(1, 2, 2, 3, 1, 4), func(v int) int {
		return v
	}), 2).Collect()

	expected := [][]int{
		{1, 2},
		{3, 4},
	}
	if !slices.EqualFunc(items, expected, func(a, b []int) bool {
		return slices.Equal(a, b)
	}) {
		t.Fatalf("unexpected chunked items: %v", items)
	}
}

func TestFirstLastMaxMinErrVariants(t *testing.T) {
	first, ok, err := Values(3, 1, 4).FirstErr()
	if err != nil || !ok || first != 3 {
		t.Fatalf("unexpected FirstErr result: first=%v ok=%v err=%v", first, ok, err)
	}

	last, ok, err := Values(3, 1, 4).LastErr()
	if err != nil || !ok || last != 4 {
		t.Fatalf("unexpected LastErr result: last=%v ok=%v err=%v", last, ok, err)
	}

	maxVal, ok, err := Values(3, 1, 4).MaxErr(func(a, b int) bool { return a < b })
	if err != nil || !ok || maxVal != 4 {
		t.Fatalf("unexpected MaxErr result: max=%v ok=%v err=%v", maxVal, ok, err)
	}

	minVal, ok, err := Values(3, 1, 4).MinErr(func(a, b int) bool { return a < b })
	if err != nil || !ok || minVal != 1 {
		t.Fatalf("unexpected MinErr result: min=%v ok=%v err=%v", minVal, ok, err)
	}
}

func TestFromProducerPanicIsReturnedByErrTerminal(t *testing.T) {
	items, err := From(func(source chan<- int) {
		source <- 1
		panic("from boom")
	}).CollectErr()

	if !slices.Equal(items, []int{1}) {
		t.Fatalf("unexpected collected items: %v", items)
	}
	if err == nil || !strings.Contains(err.Error(), "from boom") {
		t.Fatalf("expected CollectErr to include producer panic, got %v", err)
	}
}

func TestFromProducerPanicTriggersNonErrTerminalPanic(t *testing.T) {
	var recovered any

	func() {
		defer func() {
			recovered = recover()
		}()

		From(func(source chan<- int) {
			panic("from boom")
		}).Done()
	}()

	if recovered == nil {
		t.Fatal("expected Done to panic on producer panic")
	}

	err, ok := recovered.(error)
	if !ok {
		t.Fatalf("expected panic value to be error, got %T", recovered)
	}
	if !strings.Contains(err.Error(), "from boom") {
		t.Fatalf("expected panic to mention producer panic, got %v", err)
	}
}

// These tests distinguish terminals that wait for delayed fail-fast errors from Err variants that return early.
func TestShortCircuitTerminalsWaitForLateFailFastError(t *testing.T) {
	errBoom := errors.New("boom")

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
				_ = s.AllMatch(func(item int) bool { return item > 1 })
			},
		},
		{
			name: "AnyMatch",
			run: func(s Stream[int]) {
				_ = s.AnyMatch(func(item int) bool { return item == 1 })
			},
		},
		{
			name: "NoneMatch",
			run: func(s Stream[int]) {
				_ = s.NoneMatch(func(item int) bool { return item == 1 })
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stream, release := newLateFailFastStream(errBoom)
			panicCh := make(chan any, 1)
			done := make(chan struct{})

			go func() {
				defer close(done)
				defer func() {
					panicCh <- recover()
				}()

				tt.run(stream)
			}()

			select {
			case <-done:
				t.Fatalf("%s returned before the delayed fail-fast error was released", tt.name)
			case <-time.After(50 * time.Millisecond):
			}

			close(release)

			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatalf("%s did not finish after releasing the delayed error", tt.name)
			}

			panicValue := <-panicCh
			if panicValue == nil {
				t.Fatalf("%s should panic on the delayed fail-fast error", tt.name)
			}

			err, ok := panicValue.(error)
			if !ok {
				t.Fatalf("%s panic should be an error, got %T", tt.name, panicValue)
			}
			if !errors.Is(err, errBoom) {
				t.Fatalf("%s panic should wrap boom, got %v", tt.name, err)
			}
		})
	}
}

func TestHeadCollectErrWaitsForLateFailFastError(t *testing.T) {
	errBoom := errors.New("boom")
	stream, release := newLateFailFastStream(errBoom)

	type result struct {
		items []int
		err   error
	}

	resultCh := make(chan result, 1)
	go func() {
		items, err := stream.Head(1).CollectErr()
		resultCh <- result{items: items, err: err}
	}()

	select {
	case <-resultCh:
		t.Fatal("Head(1).CollectErr returned before the delayed fail-fast error was released")
	case <-time.After(50 * time.Millisecond):
	}

	close(release)

	select {
	case got := <-resultCh:
		if !slices.Equal(got.items, []int{1}) {
			t.Fatalf("unexpected Head(1) items: %v", got.items)
		}
		if !errors.Is(got.err, errBoom) {
			t.Fatalf("expected Head(1).CollectErr to include boom, got %v", got.err)
		}
	case <-time.After(time.Second):
		t.Fatal("Head(1).CollectErr did not finish after releasing the delayed error")
	}
}

func TestShortCircuitErrTerminalsReturnBeforeLateFailFastErrorSettles(t *testing.T) {
	errBoom := errors.New("boom")

	tests := []struct {
		name string
		run  func(Stream[int]) error
	}{
		{
			name: "FirstErr",
			run: func(s Stream[int]) error {
				got, ok, err := s.FirstErr()
				if !ok || got != 1 {
					return fmt.Errorf("expected FirstErr to return item 1, got %v (ok=%v)", got, ok)
				}
				return err
			},
		},
		{
			name: "AllMatchErr",
			run: func(s Stream[int]) error {
				got, err := s.AllMatchErr(func(item int) bool { return item > 1 })
				if got {
					return errors.New("expected AllMatchErr to short-circuit with false")
				}
				return err
			},
		},
		{
			name: "AnyMatchErr",
			run: func(s Stream[int]) error {
				got, err := s.AnyMatchErr(func(item int) bool { return item == 1 })
				if !got {
					return errors.New("expected AnyMatchErr to short-circuit with true")
				}
				return err
			},
		},
		{
			name: "NoneMatchErr",
			run: func(s Stream[int]) error {
				got, err := s.NoneMatchErr(func(item int) bool { return item == 1 })
				if got {
					return errors.New("expected NoneMatchErr to short-circuit with false")
				}
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stream, release := newLateFailFastStream(errBoom)
			resultCh := make(chan error, 1)

			go func() {
				resultCh <- tt.run(stream)
			}()

			select {
			case err := <-resultCh:
				if err != nil {
					t.Fatalf("%s should return before the delayed fail-fast error settles, got %v", tt.name, err)
				}
			case <-time.After(50 * time.Millisecond):
				t.Fatalf("%s did not short-circuit before the delayed fail-fast error was released", tt.name)
			}

			if err := stream.Err(); err != nil {
				t.Fatalf("%s should not see the delayed fail-fast error before release, got %v", tt.name, err)
			}

			close(release)
			waitStreamErr(t, stream, errBoom, tt.name)
		})
	}
}

func TestRingWrapKeepsIndexBoundedAndOrderStable(t *testing.T) {
	ring := newRing[int](3)
	for i := range 10 {
		ring.Add(i)
	}

	got := ring.Take()
	if len(got) != 3 {
		t.Fatalf("expected ring to keep only the latest 3 items, got %d items (%v)", len(got), got)
	}

	want := []int{7, 8, 9}
	if !slices.Equal(got, want) {
		t.Fatalf("unexpected ring order: got %v want %v", got, want)
	}
}

// newLateFailFastStream emits one successful item immediately and delays the fail-fast error until release is closed.
func newLateFailFastStream(err error) (Stream[int], chan struct{}) {
	release := make(chan struct{})
	firstDone := make(chan struct{})

	stream := MapErr(Values(1, 2), func(item int) (int, error) {
		if item == 1 {
			close(firstDone)
			return item, nil
		}

		<-firstDone
		<-release
		return 0, err
	}, WithWorkers(2))

	return stream, release
}

// waitStreamErr polls the shared stream state until want is recorded or the test times out.
func waitStreamErr(t *testing.T, stream Stream[int], want error, name string) {
	t.Helper()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if err := stream.Err(); errors.Is(err, want) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("%s should eventually record %v in stream state", name, want)
}
