package flx

import (
	"context"
	"errors"
	"fmt"
	"log"
	"runtime/debug"
	"strings"
	"time"
)

var (
	// ErrCanceled is an alias for context.Canceled.
	ErrCanceled = context.Canceled
	// ErrTimeout is an alias for context.DeadlineExceeded.
	ErrTimeout = context.DeadlineExceeded
	// ErrNilContext reports that a required parent context was nil.
	ErrNilContext = errors.New("flx: nil context")
	// ErrNegativeTimeout reports that a timeout duration was negative.
	ErrNegativeTimeout = errors.New("flx: timeout must not be negative")
)

// TimeoutOption supplies the parent context for a timeout call.
type TimeoutOption func() context.Context

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
		return contextDoneErr(ctx)
	}

	done := make(chan error, 1)
	panicChan := make(chan any, 1)
	settled := make(chan struct{})
	defer close(settled)

	goSafe(func() {
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

		err := normalizeContextErr(ctx, fn(ctx))
		select {
		case <-settled:
		case done <- err:
		}
	})

	select {
	case p := <-panicChan:
		panic(p)
	case err := <-done:
		return normalizeContextErr(ctx, err)
	case <-ctx.Done():
		return contextDoneErr(ctx)
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
