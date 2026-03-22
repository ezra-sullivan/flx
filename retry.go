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
	ErrInvalidRetryTimes              = errors.New("flx: retry times must be greater than 0")
	ErrNegativeRetryInterval          = errors.New("flx: retry interval must not be negative")
	ErrNegativeRetryTimeout           = errors.New("flx: retry timeout must not be negative")
	ErrNegativeAttemptTimeout         = errors.New("flx: attempt timeout must not be negative")
	ErrAttemptTimeoutRequiresRetryCtx = errors.New("flx: WithAttemptTimeout requires DoWithRetryCtx")
	ErrRetryAttemptTimeout            = errors.New("flx: retry attempt timeout")
)

type RetryOption func(*retryOptions)

type retryOptions struct {
	times          int
	interval       time.Duration
	timeout        time.Duration
	attemptTimeout time.Duration
	ignoreErrors   []error
}

func DoWithRetry(fn func() error, opts ...RetryOption) error {
	return retry(context.Background(), false, func(_ context.Context, _ int) error {
		return fn()
	}, opts...)
}

func DoWithRetryCtx(ctx context.Context, fn func(context.Context, int) error, opts ...RetryOption) error {
	return retry(requireContext(ctx), true, fn, opts...)
}

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
			errs.Add(err)
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
			cause := context.Cause(attemptCtx)
			if cause == nil {
				cause = attemptCtx.Err()
			}

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
				errs.Add(ctx.Err())
				return errs.Err()
			case <-timer.C:
			}
		}
	}

	return errs.Err()
}

func WithIgnoreErrors(ignoreErrors []error) RetryOption {
	return func(options *retryOptions) {
		options.ignoreErrors = slices.Clone(ignoreErrors)
	}
}

func WithInterval(interval time.Duration) RetryOption {
	return func(options *retryOptions) {
		options.interval = interval
	}
}

func WithRetry(times int) RetryOption {
	return func(options *retryOptions) {
		options.times = times
	}
}

func WithTimeout(timeout time.Duration) RetryOption {
	return func(options *retryOptions) {
		options.timeout = timeout
	}
}

func WithAttemptTimeout(timeout time.Duration) RetryOption {
	return func(options *retryOptions) {
		options.attemptTimeout = timeout
	}
}

func newRetryOptions() *retryOptions {
	return &retryOptions{times: defaultRetryTimes}
}

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

func normalizeRetryAttemptError(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}

	cause := context.Cause(ctx)
	if cause != nil && errors.Is(err, ctx.Err()) {
		return cause
	}

	return err
}
