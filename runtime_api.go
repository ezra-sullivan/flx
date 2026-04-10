package flx

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/ezra-sullivan/flx/internal/config"
	rt "github.com/ezra-sullivan/flx/internal/runtime"
	"github.com/ezra-sullivan/flx/internal/state"
	"github.com/ezra-sullivan/flx/pipeline/control"
)

const defaultRetryTimes = 3

var (
	// ErrCanceled is an alias for context.Canceled.
	ErrCanceled = context.Canceled
	// ErrTimeout is an alias for context.DeadlineExceeded.
	ErrTimeout = context.DeadlineExceeded
	// ErrNilContext reports that a required parent context was nil.
	ErrNilContext = errors.New("flx: nil context")
	// ErrNegativeTimeout reports that a timeout duration was negative.
	ErrNegativeTimeout = errors.New("flx: timeout must not be negative")
	// ErrInvalidRetryTimes reports that a retry count was zero or negative.
	ErrInvalidRetryTimes = errors.New("flx: retry times must be greater than 0")
	// ErrNegativeRetryInterval reports that a retry interval was negative.
	ErrNegativeRetryInterval = errors.New("flx: retry interval must not be negative")
	// ErrNegativeRetryTimeout reports that a total retry timeout was negative.
	ErrNegativeRetryTimeout = errors.New("flx: retry timeout must not be negative")
	// ErrNegativeAttemptTimeout reports that a per-attempt timeout was negative.
	ErrNegativeAttemptTimeout = errors.New("flx: attempt timeout must not be negative")
	// ErrAttemptTimeoutRequiresRetryCtx reports that attempt timeouts only work
	// with the context-aware retry API.
	ErrAttemptTimeoutRequiresRetryCtx = errors.New("flx: WithAttemptTimeout requires DoWithRetryCtx")
	// ErrRetryAttemptTimeout reports that one retry attempt exceeded its own
	// attempt timeout.
	ErrRetryAttemptTimeout = errors.New("flx: retry attempt timeout")
)

// TimeoutOption mutates the behavior of one timeout call.
type TimeoutOption func(*timeoutOptions)

// RetryOption mutates the behavior of one retry call.
type RetryOption func(*retryOptions)

// RetryEvent describes one failed attempt that will be retried. Attempt fields
// describe overall attempts including the first try, while Retry fields
// describe the upcoming retry ordinal only.
type RetryEvent struct {
	Attempt     int
	MaxAttempts int
	Retry       int
	MaxRetries  int
	NextAttempt int
	NextDelay   time.Duration
	Err         error
}

// TimeoutLatePanicEvent reports a panic that happened after DoWithTimeout or
// DoWithTimeoutCtx had already returned, so the panic could not be rethrown to
// the finished caller anymore.
type TimeoutLatePanicEvent struct {
	Panic any
	Stack string
}

// retryOptions stores the validated settings for one retry loop.
type retryOptions struct {
	times          int
	interval       time.Duration
	timeout        time.Duration
	attemptTimeout time.Duration
	ignoreErrors   []error
	onRetry        func(RetryEvent)
}

// timeoutOptions stores the validated settings for one timeout call.
type timeoutOptions struct {
	parentContext     context.Context
	latePanicObserver func(TimeoutLatePanicEvent)
}

// Parallel runs each function in its own goroutine and panics if the chosen
// fail-fast strategy records an error.
func Parallel(fns ...func()) {
	if err := ParallelWithErrorStrategy(control.ErrorStrategyFailFast, fns...); err != nil {
		panic(err)
	}
}

// ParallelWithErrorStrategy runs each function in its own goroutine and applies
// strategy to worker panics and returned errors.
func ParallelWithErrorStrategy(strategy control.ErrorStrategy, fns ...func()) error {
	mustValidateErrorStrategy(strategy)

	streamState := state.NewStream()
	var op *rt.OperationController
	op = rt.NewOperationController(context.Background(), func(err error) {
		if err == nil {
			return
		}

		err = wrapWorkerError(err)

		switch strategy {
		case control.ErrorStrategyCollect:
			streamState.Add(err, false)
		case control.ErrorStrategyContinue:
		case control.ErrorStrategyFailFast:
			streamState.Add(err, true)
			op.Cancel(err)
		default:
			streamState.Add(err, true)
			op.Cancel(err)
		}
	})
	var wg sync.WaitGroup

launch:
	for _, fn := range fns {
		select {
		case <-op.Context().Done():
			break launch
		default:
		}

		run := fn
		wg.Go(func() {
			if err := rt.RunSafeFunc(func() error {
				run()
				return nil
			}); err != nil {
				op.Report(err)
			}
		})
	}

	wg.Wait()
	return streamState.Err()
}

func wrapWorkerError(err error) error {
	return control.WrapWorkerError(err)
}

func validateErrorStrategy(strategy control.ErrorStrategy) error {
	return config.ValidateErrorStrategy(config.ErrorStrategy(strategy))
}

func mustValidateErrorStrategy(strategy control.ErrorStrategy) {
	if err := validateErrorStrategy(strategy); err != nil {
		panic(err)
	}
}

// ParallelErr runs each function in its own goroutine and returns the joined
// worker errors without applying stream fail-fast semantics.
func ParallelErr(fns ...func() error) error {
	var errs state.BatchError
	var wg sync.WaitGroup

	for _, fn := range fns {
		run := fn
		wg.Go(func() {
			errs.Add(wrapWorkerError(rt.RunSafeFunc(run)))
		})
	}

	wg.Wait()
	return errs.Err()
}

// DoWithRetry executes fn until it succeeds or the retry budget is exhausted.
func DoWithRetry(fn func() error, opts ...RetryOption) error {
	return retry(context.Background(), false, func(_ context.Context, _ int) error {
		return fn()
	}, opts...)
}

// DoWithRetryCtx executes fn until it succeeds or the retry budget is
// exhausted, passing the current attempt context and zero-based attempt index.
func DoWithRetryCtx(ctx context.Context, fn func(context.Context, int) error, opts ...RetryOption) error {
	return retry(requireContext(ctx), true, fn, opts...)
}

// retry implements the shared retry loop for context-aware and context-free
// entry points.
func retry(ctx context.Context, cooperative bool, fn func(context.Context, int) error, opts ...RetryOption) error {
	options := newRetryOptions()
	for _, opt := range opts {
		opt(options)
	}
	validateRetryOptions(options, cooperative)

	var errs state.BatchError
	if options.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, options.timeout)
		defer cancel()
	}

	for attempt := range options.times {
		if err := ctx.Err(); err != nil {
			errs.Add(rt.ContextDoneErr(ctx))
			return errs.Err()
		}

		attemptCtx := ctx
		cancelAttempt := func() {}
		if options.attemptTimeout > 0 {
			var cancel context.CancelFunc
			attemptCtx, cancel = context.WithTimeoutCause(ctx, options.attemptTimeout, ErrRetryAttemptTimeout)
			cancelAttempt = cancel
		}

		errChan := make(chan error, 1)
		rt.GoSafe(func() {
			errChan <- normalizeRetryAttemptError(attemptCtx, rt.RunSafeFunc(func() error {
				return fn(attemptCtx, attempt)
			}))
		})

		select {
		case err := <-errChan:
			cancelAttempt()
			if err == nil {
				return nil
			}
			if slices.ContainsFunc(options.ignoreErrors, func(target error) bool {
				return errors.Is(err, target)
			}) {
				return nil
			}
			errs.Add(err)
			if attempt+1 < options.times {
				notifyRetry(options.onRetry, RetryEvent{
					Attempt:     attempt + 1,
					MaxAttempts: options.times,
					Retry:       attempt + 1,
					MaxRetries:  max(options.times-1, 0),
					NextAttempt: attempt + 2,
					NextDelay:   options.interval,
					Err:         err,
				})
			}
		case <-attemptCtx.Done():
			cancelAttempt()
			cause := rt.ContextDoneErr(attemptCtx)

			if errors.Is(cause, ErrRetryAttemptTimeout) {
				errs.Add(cause)
				if attempt+1 < options.times {
					notifyRetry(options.onRetry, RetryEvent{
						Attempt:     attempt + 1,
						MaxAttempts: options.times,
						Retry:       attempt + 1,
						MaxRetries:  max(options.times-1, 0),
						NextAttempt: attempt + 2,
						NextDelay:   options.interval,
						Err:         cause,
					})
				}
			} else {
				errs.Add(cause)
				return errs.Err()
			}
		}

		if options.interval > 0 && attempt+1 < options.times {
			timer := time.NewTimer(options.interval)
			select {
			case <-ctx.Done():
				if !timer.Stop() {
					<-timer.C
				}
				errs.Add(rt.ContextDoneErr(ctx))
				return errs.Err()
			case <-timer.C:
			}
		}
	}

	return errs.Err()
}

// WithIgnoreErrors treats matching errors as successful completion.
func WithIgnoreErrors(ignoreErrors []error) RetryOption {
	return func(options *retryOptions) {
		options.ignoreErrors = slices.Clone(ignoreErrors)
	}
}

// WithInterval sets the delay between failed attempts.
func WithInterval(interval time.Duration) RetryOption {
	return func(options *retryOptions) {
		options.interval = interval
	}
}

// WithRetry sets the maximum number of attempts, including the first one.
func WithRetry(times int) RetryOption {
	return func(options *retryOptions) {
		options.times = times
	}
}

// WithTimeout sets an overall timeout for the full retry loop.
func WithTimeout(timeout time.Duration) RetryOption {
	return func(options *retryOptions) {
		options.timeout = timeout
	}
}

// WithAttemptTimeout sets a timeout for each individual retry attempt.
func WithAttemptTimeout(timeout time.Duration) RetryOption {
	return func(options *retryOptions) {
		options.attemptTimeout = timeout
	}
}

// WithOnRetry registers one callback that is invoked after a failed attempt
// when another retry attempt is still scheduled. Panics in the callback are
// recovered and ignored so observability hooks do not change retry behavior.
func WithOnRetry(fn func(RetryEvent)) RetryOption {
	return func(options *retryOptions) {
		options.onRetry = fn
	}
}

// WithTimeoutLatePanicObserver registers one callback that is invoked when a
// timeout callback panics after DoWithTimeout or DoWithTimeoutCtx has already
// returned. Panics in the callback are recovered and ignored so observability
// hooks do not change timeout behavior.
func WithTimeoutLatePanicObserver(fn func(TimeoutLatePanicEvent)) TimeoutOption {
	return func(options *timeoutOptions) {
		options.latePanicObserver = fn
	}
}

// newRetryOptions returns the default retry configuration.
func newRetryOptions() *retryOptions {
	return &retryOptions{times: defaultRetryTimes}
}

// newTimeoutOptions returns the default timeout configuration.
func newTimeoutOptions() *timeoutOptions {
	return &timeoutOptions{parentContext: context.Background()}
}

// validateRetryOptions panics when retry settings are invalid for the chosen
// API variant.
func validateRetryOptions(options *retryOptions, cooperative bool) {
	switch {
	case options.times <= 0:
		panic(fmt.Errorf("%w: %d", ErrInvalidRetryTimes, options.times))
	case options.interval < 0:
		panic(fmt.Errorf("%w: %s", ErrNegativeRetryInterval, options.interval))
	case options.timeout < 0:
		panic(fmt.Errorf("%w: %s", ErrNegativeRetryTimeout, options.timeout))
	case options.attemptTimeout < 0:
		panic(fmt.Errorf("%w: %s", ErrNegativeAttemptTimeout, options.attemptTimeout))
	case options.attemptTimeout > 0 && !cooperative:
		panic(ErrAttemptTimeoutRequiresRetryCtx)
	}
}

// normalizeRetryAttemptError rewrites context-related attempt errors to the
// canonical context cause.
func normalizeRetryAttemptError(ctx context.Context, err error) error {
	return rt.NormalizeContextErr(ctx, err)
}

func notifyRetry(onRetry func(RetryEvent), event RetryEvent) {
	if onRetry == nil {
		return
	}

	_ = rt.RunSafeFunc(func() error {
		onRetry(event)
		return nil
	})
}

func formatTimeoutLatePanic(event TimeoutLatePanicEvent) string {
	return fmt.Sprintf("%+v\n\n%s", event.Panic, event.Stack)
}

func notifyTimeoutLatePanic(observer func(TimeoutLatePanicEvent), event TimeoutLatePanicEvent) {
	if observer == nil {
		return
	}

	_ = rt.RunSafeFunc(func() error {
		observer(event)
		return nil
	})
}

// DoWithTimeout runs fn with a derived timeout context and returns its result.
func DoWithTimeout(fn func() error, timeout time.Duration, opts ...TimeoutOption) error {
	return doWithTimeout(timeout, opts, func(context.Context) error {
		return fn()
	})
}

// DoWithTimeoutCtx runs fn with a derived timeout context and passes that
// context into the callback.
func DoWithTimeoutCtx(fn func(context.Context) error, timeout time.Duration, opts ...TimeoutOption) error {
	return doWithTimeout(timeout, opts, fn)
}

// doWithTimeout contains the shared timeout orchestration for both public entry
// points.
func doWithTimeout(timeout time.Duration, opts []TimeoutOption, fn func(context.Context) error) error {
	validateTimeout(timeout)

	options := newTimeoutOptions()
	for _, opt := range opts {
		opt(options)
	}
	parentCtx := requireContext(options.parentContext)

	ctx, cancel := context.WithTimeout(parentCtx, timeout)
	defer cancel()

	if err := ctx.Err(); err != nil {
		return rt.ContextDoneErr(ctx)
	}

	done := make(chan error, 1)
	panicChan := make(chan any, 1)
	settled := make(chan struct{})
	defer close(settled)

	rt.GoSafe(func() {
		defer func() {
			if p := recover(); p != nil {
				event := TimeoutLatePanicEvent{
					Panic: p,
					Stack: strings.TrimSpace(string(debug.Stack())),
				}
				select {
				case <-settled:
					notifyTimeoutLatePanic(options.latePanicObserver, event)
					return
				default:
				}

				select {
				case panicChan <- formatTimeoutLatePanic(event):
				case <-settled:
					notifyTimeoutLatePanic(options.latePanicObserver, event)
				}
			}
		}()

		err := rt.NormalizeContextErr(ctx, fn(ctx))
		select {
		case <-settled:
		case done <- err:
		}
	})

	select {
	case p := <-panicChan:
		panic(p)
	case err := <-done:
		return rt.NormalizeContextErr(ctx, err)
	case <-ctx.Done():
		return rt.ContextDoneErr(ctx)
	}
}

// WithContext sets the parent context for a timeout call.
func WithContext(ctx context.Context) TimeoutOption {
	return func(options *timeoutOptions) {
		options.parentContext = ctx
	}
}

// requireContext panics when ctx is nil and otherwise returns it unchanged.
func requireContext(ctx context.Context) context.Context {
	if ctx == nil {
		panic(ErrNilContext)
	}

	return ctx
}

// validateTimeout panics when timeout is negative.
func validateTimeout(timeout time.Duration) {
	if timeout < 0 {
		panic(fmt.Errorf("%w: %s", ErrNegativeTimeout, timeout))
	}
}
