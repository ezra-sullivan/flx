package flx

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"
)

const defaultRetryTimes = 3

var (
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

	var errs batchError
	if options.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, options.timeout)
		defer cancel()
	}

	for attempt := range options.times {
		if err := ctx.Err(); err != nil {
			errs.Add(contextDoneErr(ctx))
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
		goSafe(func() {
			errChan <- normalizeRetryAttemptError(attemptCtx, runSafeFunc(func() error {
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
			cause := contextDoneErr(attemptCtx)

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
				errs.Add(contextDoneErr(ctx))
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
	return normalizeContextErr(ctx, err)
}
