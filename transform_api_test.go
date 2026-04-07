package flx

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestStageMapsValues(t *testing.T) {
	items := Stage(t.Context(), Values(1, 2, 3), func(ctx context.Context, item int) int {
		return item * 10
	}, WithWorkers(1)).Collect()

	if !slices.Equal(items, []int{10, 20, 30}) {
		t.Fatalf("expected mapped values, got %v", items)
	}
}

func TestThroughChainsSameTypeStages(t *testing.T) {
	items := Stage(
		t.Context(),
		Values(1, 2, 3),
		func(ctx context.Context, item int) int {
			return item * 10
		},
		WithWorkers(1),
	).Through(
		t.Context(),
		func(ctx context.Context, item int) int {
			return item + 1
		},
		WithWorkers(1),
	).Collect()

	if !slices.Equal(items, []int{11, 21, 31}) {
		t.Fatalf("expected chained values, got %v", items)
	}
}

func TestStageErrCollectsWorkerErrors(t *testing.T) {
	out := StageErr(
		t.Context(),
		Values("1", "x", "3"),
		func(ctx context.Context, item string) (int, error) {
			return strconv.Atoi(item)
		},
		WithWorkers(1),
		WithErrorStrategy(ErrorStrategyCollect),
	)

	items, err := out.CollectErr()
	if !slices.Equal(items, []int{1, 3}) {
		t.Fatalf("expected successful items to continue, got %v", items)
	}
	if err == nil {
		t.Fatal("expected collected stage error")
	}
	if !strings.Contains(err.Error(), "invalid syntax") {
		t.Fatalf("expected strconv error, got %v", err)
	}
}

func TestThroughErrCollectsWorkerErrors(t *testing.T) {
	errBoom := errors.New("through boom")

	out := Values(1, 2, 3).ThroughErr(
		t.Context(),
		func(ctx context.Context, item int) (int, error) {
			if item == 2 {
				return 0, errBoom
			}

			return item * 10, nil
		},
		WithWorkers(1),
		WithErrorStrategy(ErrorStrategyCollect),
	)

	items, err := out.CollectErr()
	if !slices.Equal(items, []int{10, 30}) {
		t.Fatalf("expected successful items to continue, got %v", items)
	}
	if !errors.Is(err, errBoom) {
		t.Fatalf("expected error to match %v, got %v", errBoom, err)
	}
}

func TestFlatStageEmitsMultipleValues(t *testing.T) {
	items := FlatStage(
		t.Context(),
		Values(1, 2),
		func(ctx context.Context, item int, pipe chan<- int) {
			pipe <- item
			pipe <- item * 10
		},
		WithWorkers(1),
	).Collect()

	if !slices.Equal(items, []int{1, 10, 2, 20}) {
		t.Fatalf("expected flattened values, got %v", items)
	}
}

func TestFlatStageErrCollectsWorkerErrors(t *testing.T) {
	errBoom := errors.New("boom")

	out := FlatStageErr(
		t.Context(),
		Values(1, 2, 3),
		func(ctx context.Context, item int, pipe chan<- int) error {
			if item == 2 {
				return errBoom
			}

			pipe <- item
			return nil
		},
		WithWorkers(1),
		WithErrorStrategy(ErrorStrategyCollect),
	)

	items, err := out.CollectErr()
	if !slices.Equal(items, []int{1, 3}) {
		t.Fatalf("expected successful items to continue, got %v", items)
	}
	if !errors.Is(err, errBoom) {
		t.Fatalf("expected error to match %v, got %v", errBoom, err)
	}
}

func TestTapMethodPassesThroughItems(t *testing.T) {
	var seen []int

	items, err := Values(1, 2, 3).Tap(
		t.Context(),
		func(ctx context.Context, item int) error {
			seen = append(seen, item)
			return nil
		},
		WithWorkers(1),
	).CollectErr()

	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !slices.Equal(seen, []int{1, 2, 3}) {
		t.Fatalf("expected tap side effects for all items, got %v", seen)
	}
	if !slices.Equal(items, []int{1, 2, 3}) {
		t.Fatalf("expected passthrough items, got %v", items)
	}
}

func TestTapPassesThroughItems(t *testing.T) {
	var seen []int

	out := Tap(
		t.Context(),
		Values(1, 2, 3),
		func(ctx context.Context, item int) error {
			seen = append(seen, item)
			return nil
		},
		WithWorkers(1),
	)

	items, err := out.CollectErr()
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !slices.Equal(seen, []int{1, 2, 3}) {
		t.Fatalf("expected tap side effects for all items, got %v", seen)
	}
	if !slices.Equal(items, []int{1, 2, 3}) {
		t.Fatalf("expected passthrough items, got %v", items)
	}
}

func TestTapDropsFailedItemsAndCollectsError(t *testing.T) {
	errBoom := errors.New("tap boom")
	var seen []int

	out := Tap(
		t.Context(),
		Values(1, 2, 3),
		func(ctx context.Context, item int) error {
			seen = append(seen, item)
			if item == 2 {
				return errBoom
			}

			return nil
		},
		WithWorkers(1),
		WithErrorStrategy(ErrorStrategyCollect),
	)

	items, err := out.CollectErr()
	if !slices.Equal(seen, []int{1, 2, 3}) {
		t.Fatalf("expected tap side effects for all items, got %v", seen)
	}
	if !slices.Equal(items, []int{1, 3}) {
		t.Fatalf("expected failed tap item to be dropped, got %v", items)
	}
	if !errors.Is(err, errBoom) {
		t.Fatalf("expected error to match %v, got %v", errBoom, err)
	}
}

// These tests cover context-aware transform behavior around canceled sends.
func TestSendContextDoesNotSendWhenContextAlreadyCanceled(t *testing.T) {
	errStopped := errors.New("stopped")

	for range 512 {
		ctx, cancel := context.WithCancelCause(t.Context())
		cancel(errStopped)

		pipe := make(chan int, 1)
		if SendContext(ctx, pipe, 1) {
			t.Fatal("SendContext reported success after context cancellation")
		}

		select {
		case got := <-pipe:
			t.Fatalf("SendContext delivered %d after context cancellation", got)
		default:
		}
	}
}

func TestMapContextCanceledSendDoesNotEmitValue(t *testing.T) {
	errStopped := errors.New("stopped")

	for range 128 {
		ctx, cancel := context.WithCancelCause(t.Context())
		started := make(chan struct{})
		release := make(chan struct{})

		out := MapContext(ctx, Values(1), func(ctx context.Context, item int) int {
			close(started)
			<-release
			return item
		}, WithWorkers(1))

		<-started
		cancel(errStopped)
		close(release)

		items, err := out.CollectErr()
		if err != nil {
			t.Fatalf("expected nil stream error, got %v", err)
		}
		if len(items) != 0 {
			t.Fatalf("expected canceled send to emit no items, got %v", items)
		}
	}
}

// These tests verify that asynchronous transform panics are surfaced and still drain upstream producers.
func TestAsyncTransformPanicsAreReturnedByErrTerminals(t *testing.T) {
	tests := []struct {
		name string
		run  func() error
	}{
		{
			name: "DistinctBy",
			run: func() error {
				return DistinctBy(Values(1, 2, 3), func(v int) int {
					panic("distinct boom")
				}).DoneErr()
			},
		},
		{
			name: "DistinctByCount",
			run: func() error {
				return DistinctByCount(Values(1, 2, 3), 2, func(v int) int {
					panic("distinct-count boom")
				}).DoneErr()
			},
		},
		{
			name: "GroupByCount",
			run: func() error {
				return GroupByCount(Values(1, 2, 3), 2, func(v int) int {
					panic("group-count boom")
				}).DoneErr()
			},
		},
		{
			name: "DistinctByWindow",
			run: func() error {
				return DistinctByWindow(context.Background(), Values(1, 2, 3), time.Second, func(v int) int {
					panic("distinct-window boom")
				}).DoneErr()
			},
		},
		{
			name: "GroupByWindow",
			run: func() error {
				return GroupByWindow(context.Background(), Values(1, 2, 3), time.Second, func(v int) int {
					panic("group-window boom")
				}).DoneErr()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.run()
			if err == nil {
				t.Fatal("expected panic to be returned as error")
			}
			if !strings.Contains(err.Error(), "boom") {
				t.Fatalf("expected error to mention panic, got %v", err)
			}
		})
	}
}

func TestAsyncTransformPanicsTriggerNonErrTerminalPanic(t *testing.T) {
	tests := []struct {
		name string
		run  func()
	}{
		{
			name: "DistinctBy",
			run: func() {
				DistinctBy(Values(1, 2, 3), func(v int) int {
					panic("distinct boom")
				}).Done()
			},
		},
		{
			name: "DistinctByWindow",
			run: func() {
				DistinctByWindow(context.Background(), Values(1, 2, 3), time.Second, func(v int) int {
					panic("distinct-window boom")
				}).Done()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var recovered any

			func() {
				defer func() {
					recovered = recover()
				}()
				tt.run()
			}()

			if recovered == nil {
				t.Fatal("expected Done to panic")
			}

			err, ok := recovered.(error)
			if !ok {
				t.Fatalf("expected panic value to be error, got %T", recovered)
			}
			if !strings.Contains(err.Error(), "boom") {
				t.Fatalf("expected panic to mention original panic, got %v", err)
			}
		})
	}
}

func TestAsyncTransformPanicStillDrainsUpstreamProducer(t *testing.T) {
	tests := []struct {
		name string
		run  func(Stream[int]) error
	}{
		{
			name: "DistinctBy",
			run: func(s Stream[int]) error {
				return DistinctBy(s, func(v int) int {
					panic("distinct boom")
				}).DoneErr()
			},
		},
		{
			name: "DistinctByCount",
			run: func(s Stream[int]) error {
				return DistinctByCount(s, 2, func(v int) int {
					panic("distinct-count boom")
				}).DoneErr()
			},
		},
		{
			name: "GroupByCount",
			run: func(s Stream[int]) error {
				return GroupByCount(s, 2, func(v int) int {
					panic("group-count boom")
				}).DoneErr()
			},
		},
		{
			name: "DistinctByWindow",
			run: func(s Stream[int]) error {
				return DistinctByWindow(context.Background(), s, time.Second, func(v int) int {
					panic("distinct-window boom")
				}).DoneErr()
			},
		},
		{
			name: "GroupByWindow",
			run: func(s Stream[int]) error {
				return GroupByWindow(context.Background(), s, time.Second, func(v int) int {
					panic("group-window boom")
				}).DoneErr()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stream, producerDone := testProducerStream(32)

			err := tt.run(stream)
			if err == nil {
				t.Fatal("expected panic to be returned as error")
			}

			waitProducerDone(t, producerDone, tt.name)
		})
	}
}

// These tests cover count- and time-window transforms, including cancellation and flush behavior.
func TestDistinctByCountResetsPerWindow(t *testing.T) {
	got := DistinctByCount(Values("a", "b", "a", "c", "a", "b"), 3, func(v string) string {
		return v
	}).Collect()

	want := []string{"a", "b", "c", "a", "b"}
	if !slices.Equal(got, want) {
		t.Fatalf("unexpected DistinctByCount result: %v", got)
	}
}

func TestDistinctByCountEmitsTailWindow(t *testing.T) {
	got := DistinctByCount(Values("a", "b", "a", "c", "a"), 4, func(v string) string {
		return v
	}).Collect()

	want := []string{"a", "b", "c", "a"}
	if !slices.Equal(got, want) {
		t.Fatalf("unexpected DistinctByCount tail result: %v", got)
	}
}

func TestGroupByCountSplitsByWindowAndPreservesOrder(t *testing.T) {
	got := GroupByCount(Values("b1", "a1", "b2", "b3", "c1"), 3, func(v string) byte {
		return v[0]
	}).Collect()

	want := []Group[byte, string]{
		{Key: 'b', Items: []string{"b1", "b2"}},
		{Key: 'a', Items: []string{"a1"}},
		{Key: 'b', Items: []string{"b3"}},
		{Key: 'c', Items: []string{"c1"}},
	}
	if !groupsEqual(got, want) {
		t.Fatalf("unexpected GroupByCount result: %#v", got)
	}
}

func TestCountWindowPanicsOnInvalidSize(t *testing.T) {
	t.Run("DistinctByCount", func(t *testing.T) {
		assertPanicIs(t, ErrInvalidWindowCount, func() {
			DistinctByCount(Values(1, 2, 3), 0, func(v int) int { return v })
		})
	})

	t.Run("GroupByCount", func(t *testing.T) {
		assertPanicIs(t, ErrInvalidWindowCount, func() {
			GroupByCount(Values(1, 2, 3), 0, func(v int) int { return v })
		})
	})
}

func TestDistinctByWindowResetsAfterDuration(t *testing.T) {
	const every = 40 * time.Millisecond

	input := make(chan string)
	resultCh := collectAsync(DistinctByWindow(context.Background(), FromChan(input), every, func(v string) string {
		return v
	}))

	go func() {
		input <- "a"
		input <- "a"
		time.Sleep(2 * every)
		input <- "a"
		input <- "b"
		time.Sleep(2 * every)
		input <- "b"
		close(input)
	}()

	got := waitValue(t, resultCh, "DistinctByWindow collect")
	want := []string{"a", "a", "b", "b"}
	if !slices.Equal(got, want) {
		t.Fatalf("unexpected DistinctByWindow result: %v", got)
	}
}

func TestDistinctByWindowCancelDrainsUpstream(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	producerDone := make(chan struct{})

	out := DistinctByWindow(ctx, From(func(source chan<- int) {
		defer close(producerDone)

		source <- 1
		for range 32 {
			source <- 1
		}
	}), time.Second, func(v int) int {
		return v
	})

	first, ok := out.First()
	if !ok || first != 1 {
		t.Fatalf("unexpected first result: %v (ok=%v)", first, ok)
	}

	cancel()
	waitDone(t, producerDone, "DistinctByWindow cancel drain")
}

func TestGroupByWindowFlushesOnTickAndSourceClose(t *testing.T) {
	const every = 40 * time.Millisecond

	input := make(chan string)
	resultCh := collectAsync(GroupByWindow(context.Background(), FromChan(input), every, func(v string) byte {
		return v[0]
	}))

	go func() {
		input <- "b1"
		input <- "a1"
		input <- "b2"
		time.Sleep(2 * every)
		input <- "b3"
		input <- "c1"
		close(input)
	}()

	got := waitValue(t, resultCh, "GroupByWindow collect")
	want := []Group[byte, string]{
		{Key: 'b', Items: []string{"b1", "b2"}},
		{Key: 'a', Items: []string{"a1"}},
		{Key: 'b', Items: []string{"b3"}},
		{Key: 'c', Items: []string{"c1"}},
	}
	if !groupsEqual(got, want) {
		t.Fatalf("unexpected GroupByWindow result: %#v", got)
	}
}

func TestGroupByWindowCancelDropsPartialWindowAndDrainsUpstream(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	producerDone := make(chan struct{})
	windowStarted := make(chan struct{})
	release := make(chan struct{})

	resultCh := collectAsync(GroupByWindow(ctx, From(func(source chan<- string) {
		defer close(producerDone)

		source <- "a1"
		source <- "a2"
		close(windowStarted)

		<-release
		for range 16 {
			source <- "a3"
		}
	}), time.Second, func(v string) byte {
		return v[0]
	}))

	waitDone(t, windowStarted, "GroupByWindow start")
	cancel()
	close(release)

	got := waitValue(t, resultCh, "GroupByWindow cancel collect")
	if len(got) != 0 {
		t.Fatalf("expected cancel to drop partial window, got %#v", got)
	}

	waitDone(t, producerDone, "GroupByWindow cancel drain")
}

func TestTimeWindowPanicsOnInvalidDuration(t *testing.T) {
	t.Run("DistinctByWindow", func(t *testing.T) {
		assertPanicIs(t, ErrInvalidWindowDuration, func() {
			DistinctByWindow(context.Background(), Values(1, 2, 3), 0, func(v int) int { return v })
		})
	})

	t.Run("GroupByWindow", func(t *testing.T) {
		assertPanicIs(t, ErrInvalidWindowDuration, func() {
			GroupByWindow(context.Background(), Values(1, 2, 3), 0, func(v int) int { return v })
		})
	})
}

// collectAsync collects a stream on a goroutine so time-based tests can coordinate with it asynchronously.
func collectAsync[T any](s Stream[T]) <-chan []T {
	resultCh := make(chan []T, 1)
	go func() {
		resultCh <- s.Collect()
	}()
	return resultCh
}

// waitValue returns the next value from ch or fails the test on timeout.
func waitValue[T any](t *testing.T, ch <-chan T, name string) T {
	t.Helper()

	select {
	case value := <-ch:
		return value
	case <-time.After(2 * time.Second):
		t.Fatalf("%s timed out", name)
	}

	var zero T
	return zero
}

// waitDone waits for done to close or fails the test on timeout.
func waitDone(t *testing.T, done <-chan struct{}, name string) {
	t.Helper()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("%s timed out", name)
	}
}

// groupsEqual compares grouped results while preserving key order and item order.
func groupsEqual[K comparable, T comparable](got, want []Group[K, T]) bool {
	return slices.EqualFunc(got, want, func(a, b Group[K, T]) bool {
		return a.Key == b.Key && slices.Equal(a.Items, b.Items)
	})
}
