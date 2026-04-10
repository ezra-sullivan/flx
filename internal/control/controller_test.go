package control

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"
)

// These tests verify how Controller cancels the newest interruptible workers when shrinking.
func TestControllerCancelExcessSparseIDsNewestFirst(t *testing.T) {
	ctrl := NewController(4)
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

func TestRegisterCancelCancelsNewestWorkerWhenRegistrationExceedsLimit(t *testing.T) {
	ctrl := NewController(2)

	firstCtx, firstCancel := context.WithCancelCause(t.Context())
	defer firstCancel(nil)
	firstID := ctrl.RegisterCancel(firstCancel)
	if got := len(ctrl.cancels); got != 1 {
		t.Fatalf("expected one registered worker, got %d", got)
	}

	ctrl.SetWorkers(1)

	secondCanceled := make(chan error, 1)
	secondCtx, secondCancel := context.WithCancelCause(t.Context())
	defer secondCancel(nil)

	secondID := ctrl.RegisterCancel(func(cause error) {
		secondCancel(cause)
		secondCanceled <- cause
	})

	if secondID == firstID {
		t.Fatalf("expected a distinct registration id, got %d", secondID)
	}

	select {
	case cause := <-secondCanceled:
		if !errors.Is(cause, ErrWorkerLimitReduced) {
			t.Fatalf("expected ErrWorkerLimitReduced, got %v", cause)
		}
	case <-time.After(time.Second):
		t.Fatal("expected newest worker to be canceled immediately")
	}

	if !errors.Is(context.Cause(secondCtx), ErrWorkerLimitReduced) {
		t.Fatalf("expected second worker context to be canceled with ErrWorkerLimitReduced, got %v", context.Cause(secondCtx))
	}
	if err := firstCtx.Err(); err != nil {
		t.Fatalf("expected first worker to remain active, got %v", err)
	}

	ctrl.mu.Lock()
	defer ctrl.mu.Unlock()

	if got := len(ctrl.cancels); got != 1 {
		t.Fatalf("expected exactly one registered worker to remain, got %d", got)
	}
	if _, ok := ctrl.cancels[firstID]; !ok {
		t.Fatalf("expected oldest worker registration to remain, remaining=%v", ctrl.cancels)
	}
	if _, ok := ctrl.cancels[secondID]; ok {
		t.Fatalf("expected newest worker registration to be removed after immediate cancel, remaining=%v", ctrl.cancels)
	}
}
