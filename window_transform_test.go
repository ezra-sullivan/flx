package flx

import (
	"context"
	"slices"
	"testing"
	"time"
)

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

func collectAsync[T any](s Stream[T]) <-chan []T {
	resultCh := make(chan []T, 1)
	go func() {
		resultCh <- s.Collect()
	}()
	return resultCh
}

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

func waitDone(t *testing.T, done <-chan struct{}, name string) {
	t.Helper()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("%s timed out", name)
	}
}

func groupsEqual[K comparable, T comparable](got, want []Group[K, T]) bool {
	return slices.EqualFunc(got, want, func(a, b Group[K, T]) bool {
		return a.Key == b.Key && slices.Equal(a.Items, b.Items)
	})
}
