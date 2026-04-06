package flx

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"strings"
	"testing"
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
