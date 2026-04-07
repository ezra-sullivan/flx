package runtime

import (
	"context"
	"sync"
)

// NewOperationFunc creates the context and report callback for one walker run.
type NewOperationFunc func(parent context.Context) (context.Context, func(error))

// DynamicController exposes the runtime hooks needed by dynamic walkers.
type DynamicController interface {
	Workers() int
	Acquire(context.Context) error
	Release()
	RegisterCancel(context.CancelCauseFunc) int
	UnregisterCancel(int)
}

// WalkLimited runs fn with a fixed-size worker pool.
func WalkLimited[T, U any](parent context.Context, source <-chan T, workers int, newOperation NewOperationFunc, fn func(context.Context, T, chan<- U) error, stopSource func()) <-chan U {
	pipe := make(chan U, workers)

	go func() {
		var wg sync.WaitGroup
		pool := make(chan struct{}, workers)
		ctx, report := newOperation(parent)

		stop := func() {
			if stopSource != nil {
				stopSource()
			}
			wg.Wait()
			close(pipe)
		}

		for {
			item, ok, canceled := receiveItem(ctx, source)
			switch {
			case canceled:
				stop()
				return
			case !ok:
				wg.Wait()
				close(pipe)
				return
			}

			if err := acquirePool(ctx, pool); err != nil {
				stop()
				return
			}
			if err := ctx.Err(); err != nil {
				<-pool
				stop()
				return
			}

			val := item
			wg.Go(func() {
				defer func() {
					<-pool
				}()

				if err := ctx.Err(); err != nil {
					return
				}
				if err := RunSafeFunc(func() error {
					return fn(ctx, val, pipe)
				}); err != nil {
					report(err)
				}
			})
		}
	}()

	return pipe
}

// WalkUnlimited runs fn with one goroutine per input item.
func WalkUnlimited[T, U any](parent context.Context, source <-chan T, bufferSize int, newOperation NewOperationFunc, fn func(context.Context, T, chan<- U) error, stopSource func()) <-chan U {
	pipe := make(chan U, bufferSize)

	go func() {
		var wg sync.WaitGroup
		ctx, report := newOperation(parent)

		stop := func() {
			if stopSource != nil {
				stopSource()
			}
			wg.Wait()
			close(pipe)
		}

		for {
			item, ok, canceled := receiveItem(ctx, source)
			switch {
			case canceled:
				stop()
				return
			case !ok:
				wg.Wait()
				close(pipe)
				return
			}

			val := item
			wg.Go(func() {
				if err := ctx.Err(); err != nil {
					return
				}
				if err := RunSafeFunc(func() error {
					return fn(ctx, val, pipe)
				}); err != nil {
					report(err)
				}
			})
		}
	}()

	return pipe
}

// WalkDynamic runs fn under a resizable semaphore without canceling workers
// that already acquired a slot.
func WalkDynamic[T, U any](parent context.Context, source <-chan T, controller DynamicController, newOperation NewOperationFunc, fn func(context.Context, T, chan<- U) error, stopSource func()) <-chan U {
	pipe := make(chan U, controller.Workers())

	go func() {
		var wg sync.WaitGroup
		ctx, report := newOperation(parent)

		stop := func() {
			if stopSource != nil {
				stopSource()
			}
			wg.Wait()
			close(pipe)
		}

		for {
			item, ok, canceled := receiveItem(ctx, source)
			switch {
			case canceled:
				stop()
				return
			case !ok:
				wg.Wait()
				close(pipe)
				return
			}

			if err := controller.Acquire(ctx); err != nil {
				stop()
				return
			}
			if err := ctx.Err(); err != nil {
				controller.Release()
				stop()
				return
			}

			val := item
			wg.Go(func() {
				defer controller.Release()

				if err := ctx.Err(); err != nil {
					return
				}
				if err := RunSafeFunc(func() error {
					return fn(ctx, val, pipe)
				}); err != nil {
					report(err)
				}
			})
		}
	}()

	return pipe
}

// WalkInterruptible runs fn under a resizable semaphore and cancels excess
// workers when the controller shrinks.
func WalkInterruptible[T, U any](parent context.Context, source <-chan T, controller DynamicController, newOperation NewOperationFunc, fn func(context.Context, T, chan<- U) error, stopSource func()) <-chan U {
	pipe := make(chan U, controller.Workers())

	go func() {
		var wg sync.WaitGroup
		ctx, report := newOperation(parent)

		stop := func() {
			if stopSource != nil {
				stopSource()
			}
			wg.Wait()
			close(pipe)
		}

		for {
			item, ok, canceled := receiveItem(ctx, source)
			switch {
			case canceled:
				stop()
				return
			case !ok:
				wg.Wait()
				close(pipe)
				return
			}

			if err := controller.Acquire(ctx); err != nil {
				stop()
				return
			}
			if err := ctx.Err(); err != nil {
				controller.Release()
				stop()
				return
			}

			val := item
			workerCtx, cancel := context.WithCancelCause(ctx)
			id := controller.RegisterCancel(cancel)

			wg.Go(func() {
				defer func() {
					controller.UnregisterCancel(id)
					cancel(nil)
					controller.Release()
				}()

				if err := ctx.Err(); err != nil {
					return
				}
				if err := RunSafeFunc(func() error {
					return fn(workerCtx, val, pipe)
				}); err != nil {
					report(err)
				}
			})
		}
	}()

	return pipe
}

func acquirePool(ctx context.Context, pool chan struct{}) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case pool <- struct{}{}:
		return nil
	}
}

func receiveItem[T any](ctx context.Context, source <-chan T) (item T, ok bool, canceled bool) {
	select {
	case <-ctx.Done():
		var zero T
		return zero, false, true
	case item, ok = <-source:
		if !ok {
			var zero T
			return zero, false, false
		}
		if ctx.Err() != nil {
			var zero T
			return zero, false, true
		}
		return item, true, false
	}
}
