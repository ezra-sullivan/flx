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
	ErrCanceled        = context.Canceled
	ErrTimeout         = context.DeadlineExceeded
	ErrNilContext      = errors.New("flx: nil context")
	ErrNegativeTimeout = errors.New("flx: timeout must not be negative")
)

type TimeoutOption func() context.Context

func DoWithTimeout(fn func() error, timeout time.Duration, opts ...TimeoutOption) error {
	return doWithTimeout(timeout, opts, func(context.Context) error {
		return fn()
	})
}

func DoWithTimeoutCtx(fn func(context.Context) error, timeout time.Duration, opts ...TimeoutOption) error {
	return doWithTimeout(timeout, opts, fn)
}

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
		return err
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

		err := fn(ctx)
		select {
		case <-settled:
		case done <- err:
		}
	})

	select {
	case p := <-panicChan:
		panic(p)
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func WithContext(ctx context.Context) TimeoutOption {
	return func() context.Context {
		return ctx
	}
}

func requireContext(ctx context.Context) context.Context {
	if ctx == nil {
		panic(ErrNilContext)
	}

	return ctx
}

func validateTimeout(timeout time.Duration) {
	if timeout < 0 {
		panic(fmt.Errorf("%w: %s", ErrNegativeTimeout, timeout))
	}
}
