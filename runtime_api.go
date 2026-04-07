package flx

import (
	"context"
	"errors"
	"fmt"
	"log"
	"runtime/debug"
	"slices"
	"strings"
	"sync"
	"time"

	rt "github.com/ezra-sullivan/flx/internal/runtime"
	"github.com/ezra-sullivan/flx/internal/state"
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

// TimeoutOption supplies the parent context for a timeout call.
type TimeoutOption func() context.Context

// RetryOption mutates the behavior of one retry call.
type RetryOption func(*retryOptions)

// retryOptions stores the validated settings for one retry loop.
type retryOptions struct {
	times          int
	interval       time.Duration
	timeout        time.Duration
	attemptTimeout time.Duration
	ignoreErrors   []error
}

// Parallel runs each function in its own goroutine and panics if the chosen
// fail-fast strategy records an error.
func Parallel(fns ...func()) {
	if err := ParallelWithErrorStrategy(ErrorStrategyFailFast, fns...); err != nil {
		panic(err)
	}
}

// ParallelWithErrorStrategy runs each function in its own goroutine and applies
// strategy to worker panics and returned errors.
func ParallelWithErrorStrategy(strategy ErrorStrategy, fns ...func()) error {
	mustValidateErrorStrategy(strategy)

	streamState := state.NewStream()
	var op *rt.OperationController
	op = rt.NewOperationController(nil, func(err error) {
		if err == nil {
			return
		}

		err = wrapWorkerError(err)

		switch strategy {
		case ErrorStrategyCollect:
			streamState.Add(err, false)
		case ErrorStrategyLogAndContinue:
			log.Printf("[flx] worker error: %v", err)
		case ErrorStrategyFailFast:
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
		case <-attemptCtx.Done():
			cancelAttempt()
			cause := rt.ContextDoneErr(attemptCtx)

			if errors.Is(cause, ErrRetryAttemptTimeout) {
				errs.Add(cause)
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

// newRetryOptions returns the default retry configuration.
func newRetryOptions() *retryOptions {
	return &retryOptions{times: defaultRetryTimes}
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

	parentCtx := context.Background()
	for _, opt := range opts {
		parentCtx = opt()
	}
	parentCtx = requireContext(parentCtx)

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
				payload := fmt.Sprintf("%+v\n\n%s", p, strings.TrimSpace(string(debug.Stack())))
				select {
				case <-settled:
					log.Printf("[flx] panic after DoWithTimeout returned: %v", payload)
				case panicChan <- payload:
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
	return func() context.Context {
		return ctx
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
