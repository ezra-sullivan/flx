package flx

import (
	"bytes"
	"context"
	"errors"
	"log"
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
			//nolint:staticcheck // This test verifies that the API rejects a nil context.
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

func TestParallelWithErrorStrategyContinueIgnoresWorkerErrorWithoutLogging(t *testing.T) {
	var buf bytes.Buffer
	restoreLogger := captureStdLogger(t, &buf)
	defer restoreLogger()

	err := ParallelWithErrorStrategy(control.ErrorStrategyContinue, func() {
		panic("parallel continue boom")
	})

	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if got := strings.TrimSpace(buf.String()); got != "" {
		t.Fatalf("expected no log output, got %q", got)
	}
}

func TestParallelWithErrorStrategyLogAndContinueAliasDoesNotLog(t *testing.T) {
	var buf bytes.Buffer
	restoreLogger := captureStdLogger(t, &buf)
	defer restoreLogger()

	//nolint:staticcheck // compatibility alias coverage
	strategy := control.ErrorStrategyLogAndContinue
	err := ParallelWithErrorStrategy(strategy, func() {
		panic("parallel alias boom")
	})

	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if got := strings.TrimSpace(buf.String()); got != "" {
		t.Fatalf("expected no log output, got %q", got)
	}
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

func TestDoWithRetryCtxOnRetryReportsFailedAttemptsBeforeNextRetry(t *testing.T) {
	attemptErr := errors.New("attempt failed")
	events := make([]RetryEvent, 0, 2)

	err := DoWithRetryCtx(t.Context(), func(ctx context.Context, retryCount int) error {
		return attemptErr
	}, WithRetry(3), WithInterval(25*time.Millisecond), WithOnRetry(func(event RetryEvent) {
		events = append(events, event)
	}))

	if !errors.Is(err, attemptErr) {
		t.Fatalf("expected final error to contain attempt failure, got %v", err)
	}
	if got := len(events); got != 2 {
		t.Fatalf("expected retry observer to run for the first two failed attempts, got %d events", got)
	}

	first := events[0]
	if first.Attempt != 1 || first.MaxAttempts != 3 || first.Retry != 1 || first.MaxRetries != 2 || first.NextAttempt != 2 || first.NextDelay != 25*time.Millisecond || !errors.Is(first.Err, attemptErr) {
		t.Fatalf("unexpected first retry event: %#v", first)
	}

	second := events[1]
	if second.Attempt != 2 || second.MaxAttempts != 3 || second.Retry != 2 || second.MaxRetries != 2 || second.NextAttempt != 3 || second.NextDelay != 25*time.Millisecond || !errors.Is(second.Err, attemptErr) {
		t.Fatalf("unexpected second retry event: %#v", second)
	}
}

func TestDoWithRetryCtxOnRetryReportsAttemptTimeout(t *testing.T) {
	var events []RetryEvent

	err := DoWithRetryCtx(t.Context(), func(ctx context.Context, retryCount int) error {
		<-ctx.Done()
		return ctx.Err()
	}, WithRetry(2), WithAttemptTimeout(20*time.Millisecond), WithOnRetry(func(event RetryEvent) {
		events = append(events, event)
	}))

	if !errors.Is(err, ErrRetryAttemptTimeout) {
		t.Fatalf("expected attempt timeout error, got %v", err)
	}
	if got := len(events); got != 1 {
		t.Fatalf("expected one retry event for the timed out first attempt, got %d", got)
	}
	if event := events[0]; event.Attempt != 1 || event.MaxAttempts != 2 || event.Retry != 1 || event.MaxRetries != 1 || event.NextAttempt != 2 || event.NextDelay != 0 || !errors.Is(event.Err, ErrRetryAttemptTimeout) {
		t.Fatalf("unexpected retry event: %#v", event)
	}
}

func TestDoWithRetryCtxOnRetryRecoversObserverPanic(t *testing.T) {
	var attempts atomic.Int32

	err := DoWithRetryCtx(t.Context(), func(ctx context.Context, retryCount int) error {
		attempts.Add(1)
		return errors.New("boom")
	}, WithRetry(2), WithOnRetry(func(event RetryEvent) {
		panic("retry observer boom")
	}))

	if err == nil {
		t.Fatal("expected final retry error")
	}
	if got := attempts.Load(); got != 2 {
		t.Fatalf("expected retry loop to continue after observer panic, got %d attempts", got)
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
			//nolint:staticcheck // This test verifies that WithContext(nil) is rejected.
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

func TestDoWithTimeoutCtxReportsLatePanicToObserver(t *testing.T) {
	allowLatePanic := make(chan struct{})
	reports := make(chan TimeoutLatePanicEvent, 1)

	err := DoWithTimeoutCtx(func(ctx context.Context) error {
		<-ctx.Done()
		<-allowLatePanic
		panic("late timeout boom")
	}, 20*time.Millisecond, WithTimeoutLatePanicObserver(func(event TimeoutLatePanicEvent) {
		reports <- event
	}))

	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("expected timeout error, got %v", err)
	}

	close(allowLatePanic)

	select {
	case event := <-reports:
		if got := event.Stack; got == "" {
			t.Fatal("expected late panic stack to be captured")
		}
		if got := event.Panic; got != "late timeout boom" {
			t.Fatalf("expected late panic payload, got %v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for late panic observer")
	}
}

func TestDoWithTimeoutCtxRecoversLatePanicObserverPanic(t *testing.T) {
	allowLatePanic := make(chan struct{})
	observerCalled := make(chan struct{})

	err := DoWithTimeoutCtx(func(ctx context.Context) error {
		<-ctx.Done()
		<-allowLatePanic
		panic("late timeout boom")
	}, 20*time.Millisecond, WithTimeoutLatePanicObserver(func(event TimeoutLatePanicEvent) {
		close(observerCalled)
		panic("observer boom")
	}))

	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("expected timeout error, got %v", err)
	}

	close(allowLatePanic)

	select {
	case <-observerCalled:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for late panic observer")
	}
}

func TestDoWithTimeoutCtxLatePanicDoesNotLogByDefault(t *testing.T) {
	var buf bytes.Buffer
	restoreLogger := captureStdLogger(t, &buf)
	defer restoreLogger()

	allowLatePanic := make(chan struct{})
	lateExited := make(chan struct{})

	err := DoWithTimeoutCtx(func(ctx context.Context) error {
		<-ctx.Done()
		<-allowLatePanic
		defer close(lateExited)
		panic("late timeout boom")
	}, 20*time.Millisecond)

	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("expected timeout error, got %v", err)
	}

	close(allowLatePanic)

	select {
	case <-lateExited:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for late timeout panic to unwind")
	}

	time.Sleep(20 * time.Millisecond)

	if got := strings.TrimSpace(buf.String()); got != "" {
		t.Fatalf("expected no default log output, got %q", got)
	}
}

func captureStdLogger(t *testing.T, dst *bytes.Buffer) func() {
	t.Helper()

	oldWriter := log.Writer()
	oldFlags := log.Flags()
	oldPrefix := log.Prefix()

	log.SetOutput(dst)
	log.SetFlags(0)
	log.SetPrefix("")

	return func() {
		log.SetOutput(oldWriter)
		log.SetFlags(oldFlags)
		log.SetPrefix(oldPrefix)
	}
}
