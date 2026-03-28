package flx

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

// These tests cover stream behaviors that do not fit a single transform or terminal category.
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
	if !out.state.shouldPanic() {
		t.Fatal("expected concat state to inherit panic-on-terminal from upstream")
	}

	close(leftSource)
	close(rightSource)
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
