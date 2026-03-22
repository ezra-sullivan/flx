package flx

import (
	"errors"
	"strings"
	"testing"
)

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
