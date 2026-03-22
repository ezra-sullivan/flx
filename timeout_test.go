package flx

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestDoWithTimeoutPanicsOnInvalidInputs(t *testing.T) {
	t.Run("negative timeout", func(t *testing.T) {
		assertPanicIs(t, ErrNegativeTimeout, func() {
			_ = DoWithTimeout(func() error { return nil }, -time.Millisecond)
		})
	})

	t.Run("nil parent ctx", func(t *testing.T) {
		assertPanicIs(t, ErrNilContext, func() {
			_ = DoWithTimeout(func() error { return nil }, time.Millisecond, WithContext(nil))
		})
	})
}

func TestDoWithTimeoutDoesNotStartWhenContextAlreadyDone(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	var started atomic.Int32
	err := DoWithTimeout(func() error {
		started.Add(1)
		return nil
	}, time.Second, WithContext(ctx))

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled error, got %v", err)
	}
	if got := started.Load(); got != 0 {
		t.Fatalf("expected fn not to start when parent ctx already done, got %d", got)
	}
}

func TestDoWithTimeoutCtxPassesDerivedContext(t *testing.T) {
	var sawDeadline atomic.Bool

	err := DoWithTimeoutCtx(func(ctx context.Context) error {
		if _, ok := ctx.Deadline(); ok {
			sawDeadline.Store(true)
		}
		return nil
	}, 50*time.Millisecond)

	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !sawDeadline.Load() {
		t.Fatal("expected timeout callback to receive derived context")
	}
}

func TestDoWithTimeoutCtxReturnsTimeout(t *testing.T) {
	err := DoWithTimeoutCtx(func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	}, 20*time.Millisecond)

	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("expected timeout error, got %v", err)
	}
}

func TestDoWithTimeoutCtxRepanics(t *testing.T) {
	var recovered any

	func() {
		defer func() {
			recovered = recover()
		}()

		_ = DoWithTimeoutCtx(func(ctx context.Context) error {
			panic("timeout boom")
		}, time.Second)
	}()

	if recovered == nil {
		t.Fatal("expected panic to be rethrown")
	}
	if !strings.Contains(recovered.(string), "timeout boom") {
		t.Fatalf("expected panic payload to mention timeout boom, got %v", recovered)
	}
}
