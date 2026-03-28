package flx

import (
	"context"
	"errors"
	"slices"
	"testing"
)

// These tests verify how ConcurrencyController cancels the newest interruptible workers when shrinking.
func TestConcurrencyControllerCancelExcessSparseIDsNewestFirst(t *testing.T) {
	ctrl := NewConcurrencyController(4)
	var canceled []int

	cancelFor := func(id int) context.CancelCauseFunc {
		return func(cause error) {
			if !errors.Is(cause, ErrWorkerLimitReduced) {
				t.Fatalf("expected ErrWorkerLimitReduced, got %v", cause)
			}
			canceled = append(canceled, id)
		}
	}

	ctrl.mu.Lock()
	ctrl.cancels = map[int]context.CancelCauseFunc{
		1:    cancelFor(1),
		1000: cancelFor(1000),
		2000: cancelFor(2000),
	}
	ctrl.mu.Unlock()

	ctrl.cancelExcess(1)

	if !slices.Equal(canceled, []int{2000, 1000}) {
		t.Fatalf("expected newest workers to be canceled first, got %v", canceled)
	}

	ctrl.mu.Lock()
	defer ctrl.mu.Unlock()

	if len(ctrl.cancels) != 1 {
		t.Fatalf("expected 1 remaining cancel func, got %d", len(ctrl.cancels))
	}
	if _, ok := ctrl.cancels[1]; !ok {
		t.Fatalf("expected oldest worker to remain registered, remaining=%v", ctrl.cancels)
	}
}
