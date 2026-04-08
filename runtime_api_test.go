package flx

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ezra-sullivan/flx/pipeline/control"
)

// These tests cover retry validation, cancellation, timeout, and panic conversion behavior.
func TestDoWithRetryPanicsOnInvalidOptions(t *testing.T) {
	t.Run("zero retry times", func(t *testing.T) {
		assertPanicIs(t, ErrInvalidRetryTimes, func() {
			_ = DoWithRetry(func() error { return nil }, WithRetry(0))
		})
	})

	t.Run("negative retry times", func(t *testing.T) {
		assertPanicIs(t, ErrInvalidRetryTimes, func() {
			_ = DoWithRetry(func() error { return nil }, WithRetry(-1))
		})
	})

	t.Run("negative interval", func(t *testing.T) {
		assertPanicIs(t, ErrNegativeRetryInterval, func() {
			_ = DoWithRetry(func() error { return nil }, WithInterval(-time.Millisecond))
		})
	})

	t.Run("negative timeout", func(t *testing.T) {
		assertPanicIs(t, ErrNegativeRetryTimeout, func() {
			_ = DoWithRetry(func() error { return nil }, WithTimeout(-time.Millisecond))
		})
	})

	t.Run("negative attempt timeout", func(t *testing.T) {
		assertPanicIs(t, ErrNegativeAttemptTimeout, func() {
			_ = DoWithRetryCtx(t.Context(), func(ctx context.Context, retryCount int) error {
				return nil
			}, WithAttemptTimeout(-time.Millisecond))
		})
	})

	t.Run("nil ctx", func(t *testing.T) {
		assertPanicIs(t, ErrNilContext, func() {
			_ = DoWithRetryCtx(nil, func(ctx context.Context, retryCount int) error { return nil })
		})
	})

	t.Run("attempt timeout requires ctx variant", func(t *testing.T) {
		assertPanicIs(t, ErrAttemptTimeoutRequiresRetryCtx, func() {
			_ = DoWithRetry(func() error { return nil }, WithAttemptTimeout(time.Millisecond))
		})
	})
}

func TestDoWithRetryCtxPassesDerivedTimeoutContext(t *testing.T) {
	var sawDeadline atomic.Bool

	err := DoWithRetryCtx(t.Context(), func(ctx context.Context, retryCount int) error {
		if _, ok := ctx.Deadline(); ok {
			sawDeadline.Store(true)
		}
		return nil
	}, WithTimeout(50*time.Millisecond))

	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !sawDeadline.Load() {
		t.Fatal("expected retry callback to receive derived timeout context")
	}
}

func TestDoWithRetryCtxReturnsParentCancelCauseWhenContextAlreadyDone(t *testing.T) {
	ctx, cancel := context.WithCancelCause(t.Context())
	cause := errors.New("parent stopped retry before start")
	cancel(cause)

	var started atomic.Int32
	err := DoWithRetryCtx(ctx, func(ctx context.Context, retryCount int) error {
		started.Add(1)
		return nil
	})

	if !errors.Is(err, cause) {
		t.Fatalf("expected cancel cause %v, got %v", cause, err)
	}
	if got := started.Load(); got != 0 {
		t.Fatalf("expected callback not to start when parent ctx already done, got %d", got)
	}
}

func TestDoWithRetryReturnsPanicAsError(t *testing.T) {
	err := DoWithRetry(func() error {
		panic("retry boom")
	}, WithRetry(1))

	if err == nil {
		t.Fatal("expected panic to be converted into error")
	}
	if !strings.Contains(err.Error(), "retry boom") {
		t.Fatalf("expected panic error to mention retry boom, got %v", err)
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
	assertPanicIs(t, control.ErrInvalidErrorStrategy, func() {
		_ = ParallelWithErrorStrategy(control.ErrorStrategy(100), func() {})
	})
}

func TestDoWithRetryCtxRetriesOnAttemptTimeout(t *testing.T) {
	var attempts atomic.Int32

	err := DoWithRetryCtx(t.Context(), func(ctx context.Context, retryCount int) error {
		attempts.Add(1)
		<-ctx.Done()
		return ctx.Err()
	}, WithRetry(2), WithAttemptTimeout(20*time.Millisecond))

	if !errors.Is(err, ErrRetryAttemptTimeout) {
		t.Fatalf("expected attempt timeout error, got %v", err)
	}
	if got := attempts.Load(); got != 2 {
		t.Fatalf("expected 2 timed out attempts, got %d", got)
	}
}

func TestDoWithRetryCtxReturnsParentCancelCauseWhileWaitingForNextAttempt(t *testing.T) {
	parent, cancel := context.WithCancelCause(t.Context())
	cause := errors.New("parent stopped retry between attempts")
	attemptErr := errors.New("attempt failed")
	unexpectedRetryErr := errors.New("unexpected extra retry attempt")
	firstAttemptDone := make(chan struct{})

	errCh := make(chan error, 1)
	go func() {
		errCh <- DoWithRetryCtx(parent, func(ctx context.Context, retryCount int) error {
			if retryCount == 0 {
				close(firstAttemptDone)
				return attemptErr
			}

			return unexpectedRetryErr
		}, WithRetry(2), WithInterval(time.Second))
	}()

	<-firstAttemptDone
	cancel(cause)

	err := <-errCh
	if errors.Is(err, unexpectedRetryErr) {
		t.Fatalf("expected cancellation before the next retry attempt, got %v", err)
	}
	if !errors.Is(err, attemptErr) {
		t.Fatalf("expected aggregated error to contain attempt error %v, got %v", attemptErr, err)
	}
	if !errors.Is(err, cause) {
		t.Fatalf("expected aggregated error to contain cancel cause %v, got %v", cause, err)
	}
}

// These tests cover timeout validation, context propagation, cancellation causes, and panic rethrow behavior.
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

func TestDoWithTimeoutReturnsParentCancelCauseWhenContextAlreadyDone(t *testing.T) {
	ctx, cancel := context.WithCancelCause(t.Context())
	cause := errors.New("parent stopped timeout before start")
	cancel(cause)

	var started atomic.Int32
	err := DoWithTimeout(func() error {
		started.Add(1)
		return nil
	}, time.Second, WithContext(ctx))

	if !errors.Is(err, cause) {
		t.Fatalf("expected cancel cause %v, got %v", cause, err)
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

func TestDoWithTimeoutCtxReturnsParentCancelCause(t *testing.T) {
	parent, cancel := context.WithCancelCause(t.Context())
	cause := errors.New("parent stopped timeout in flight")
	started := make(chan struct{})

	errCh := make(chan error, 1)
	go func() {
		errCh <- DoWithTimeoutCtx(func(ctx context.Context) error {
			close(started)
			<-ctx.Done()
			return ctx.Err()
		}, time.Second, WithContext(parent))
	}()

	<-started
	cancel(cause)

	err := <-errCh
	if !errors.Is(err, cause) {
		t.Fatalf("expected cancel cause %v, got %v", cause, err)
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
