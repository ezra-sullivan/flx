package runtime

import (
	"context"
	"sync"

	"github.com/ezra-sullivan/flx/internal/link"
	"github.com/ezra-sullivan/flx/internal/metrics"
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
func WalkLimited[T, U any](parent context.Context, source <-chan T, workers int, newOperation NewOperationFunc, fn func(context.Context, T, chan<- U) error, stopSource func(), tracker *metrics.StageTracker, outboundLink *link.Meter, observeReceive func(), closeInputLink func()) <-chan U {
	pipe := make(chan U, workers)

	go func() {
		var wg sync.WaitGroup
		pool := make(chan struct{}, workers)
		ctx, report := newOperation(parent)
		report = observeReport(report, tracker)

		finish := func(drainSource bool) {
			if drainSource && stopSource != nil {
				stopSource()
			}
			wg.Wait()
			if tracker != nil {
				tracker.Close()
			}
			close(pipe)
		}

		for {
			item, ok, canceled := receiveItem(ctx, source, observeReceive)
			switch {
			case canceled:
				finish(true)
				return
			case !ok:
				if closeInputLink != nil {
					closeInputLink()
				}
				finish(false)
				return
			}
			if tracker != nil {
				tracker.ObserveInput()
			}

			if err := acquirePool(ctx, pool); err != nil {
				if tracker != nil {
					tracker.DropInput()
				}
				finish(true)
				return
			}
			if err := ctx.Err(); err != nil {
				if tracker != nil {
					tracker.DropInput()
				}
				<-pool
				finish(true)
				return
			}
			if tracker != nil {
				tracker.StartWorker()
			}

			val := item
			wg.Go(func() {
				workerPipe, finishOutputs := wrapWorkerPipe(ctx, pipe, tracker, outboundLink)

				defer func() {
					finishOutputs()
					if tracker != nil {
						tracker.FinishWorker()
					}
					<-pool
				}()

				if err := ctx.Err(); err != nil {
					return
				}
				if err := RunSafeFunc(func() error {
					return fn(ctx, val, workerPipe)
				}); err != nil {
					report(err)
				}
			})
		}
	}()

	return pipe
}

// WalkUnlimited runs fn with one goroutine per input item.
func WalkUnlimited[T, U any](parent context.Context, source <-chan T, bufferSize int, newOperation NewOperationFunc, fn func(context.Context, T, chan<- U) error, stopSource func(), tracker *metrics.StageTracker, outboundLink *link.Meter, observeReceive func(), closeInputLink func()) <-chan U {
	pipe := make(chan U, bufferSize)

	go func() {
		var wg sync.WaitGroup
		ctx, report := newOperation(parent)
		report = observeReport(report, tracker)

		finish := func(drainSource bool) {
			if drainSource && stopSource != nil {
				stopSource()
			}
			wg.Wait()
			if tracker != nil {
				tracker.Close()
			}
			close(pipe)
		}

		for {
			item, ok, canceled := receiveItem(ctx, source, observeReceive)
			switch {
			case canceled:
				finish(true)
				return
			case !ok:
				if closeInputLink != nil {
					closeInputLink()
				}
				finish(false)
				return
			}
			if tracker != nil {
				tracker.ObserveInput()
				tracker.StartWorker()
			}

			val := item
			wg.Go(func() {
				workerPipe, finishOutputs := wrapWorkerPipe(ctx, pipe, tracker, outboundLink)

				defer func() {
					finishOutputs()
					if tracker != nil {
						tracker.FinishWorker()
					}
				}()

				if err := ctx.Err(); err != nil {
					return
				}
				if err := RunSafeFunc(func() error {
					return fn(ctx, val, workerPipe)
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
func WalkDynamic[T, U any](parent context.Context, source <-chan T, controller DynamicController, newOperation NewOperationFunc, fn func(context.Context, T, chan<- U) error, stopSource func(), tracker *metrics.StageTracker, outboundLink *link.Meter, observeReceive func(), closeInputLink func()) <-chan U {
	pipe := make(chan U, controller.Workers())

	go func() {
		var wg sync.WaitGroup
		ctx, report := newOperation(parent)
		report = observeReport(report, tracker)

		finish := func(drainSource bool) {
			if drainSource && stopSource != nil {
				stopSource()
			}
			wg.Wait()
			if tracker != nil {
				tracker.Close()
			}
			close(pipe)
		}

		for {
			item, ok, canceled := receiveItem(ctx, source, observeReceive)
			switch {
			case canceled:
				finish(true)
				return
			case !ok:
				if closeInputLink != nil {
					closeInputLink()
				}
				finish(false)
				return
			}
			if tracker != nil {
				tracker.ObserveInput()
			}

			if err := controller.Acquire(ctx); err != nil {
				if tracker != nil {
					tracker.DropInput()
				}
				finish(true)
				return
			}
			if err := ctx.Err(); err != nil {
				if tracker != nil {
					tracker.DropInput()
				}
				controller.Release()
				finish(true)
				return
			}
			if tracker != nil {
				tracker.StartWorker()
			}

			val := item
			wg.Go(func() {
				workerPipe, finishOutputs := wrapWorkerPipe(ctx, pipe, tracker, outboundLink)

				defer controller.Release()
				defer finishOutputs()
				defer func() {
					if tracker != nil {
						tracker.FinishWorker()
					}
				}()

				if err := ctx.Err(); err != nil {
					return
				}
				if err := RunSafeFunc(func() error {
					return fn(ctx, val, workerPipe)
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
func WalkInterruptible[T, U any](parent context.Context, source <-chan T, controller DynamicController, newOperation NewOperationFunc, fn func(context.Context, T, chan<- U) error, stopSource func(), tracker *metrics.StageTracker, outboundLink *link.Meter, observeReceive func(), closeInputLink func()) <-chan U {
	pipe := make(chan U, controller.Workers())

	go func() {
		var wg sync.WaitGroup
		ctx, report := newOperation(parent)
		report = observeReport(report, tracker)

		finish := func(drainSource bool) {
			if drainSource && stopSource != nil {
				stopSource()
			}
			wg.Wait()
			if tracker != nil {
				tracker.Close()
			}
			close(pipe)
		}

		for {
			item, ok, canceled := receiveItem(ctx, source, observeReceive)
			switch {
			case canceled:
				finish(true)
				return
			case !ok:
				if closeInputLink != nil {
					closeInputLink()
				}
				finish(false)
				return
			}
			if tracker != nil {
				tracker.ObserveInput()
			}

			if err := controller.Acquire(ctx); err != nil {
				if tracker != nil {
					tracker.DropInput()
				}
				finish(true)
				return
			}
			if err := ctx.Err(); err != nil {
				if tracker != nil {
					tracker.DropInput()
				}
				controller.Release()
				finish(true)
				return
			}
			if tracker != nil {
				tracker.StartWorker()
			}

			val := item
			workerCtx, cancel := context.WithCancelCause(ctx)
			id := controller.RegisterCancel(cancel)

			wg.Go(func() {
				workerPipe, finishOutputs := wrapWorkerPipe(workerCtx, pipe, tracker, outboundLink)

				defer func() {
					finishOutputs()
					if tracker != nil {
						tracker.FinishWorker()
					}
					controller.UnregisterCancel(id)
					cancel(nil)
					controller.Release()
				}()

				if err := workerCtx.Err(); err != nil {
					return
				}
				if err := RunSafeFunc(func() error {
					return fn(workerCtx, val, workerPipe)
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

func observeReport(report func(error), tracker *metrics.StageTracker) func(error) {
	if tracker == nil {
		return report
	}

	return func(err error) {
		if err != nil {
			tracker.ObserveError()
		}
		if report != nil {
			report(err)
		}
	}
}

func wrapWorkerPipe[T any](ctx context.Context, pipe chan<- T, tracker *metrics.StageTracker, outboundLink *link.Meter) (chan<- T, func()) {
	if tracker == nil && outboundLink == nil {
		return pipe, func() {}
	}

	workerPipe := make(chan T)
	done := make(chan struct{})

	go func() {
		defer close(done)

		forwarding := true
		for item := range workerPipe {
			if !forwarding {
				continue
			}

			if sendContext(ctx, pipe, item) {
				if tracker != nil {
					tracker.ObserveOutput()
				}
				if outboundLink != nil {
					outboundLink.ObserveSend()
				}
				continue
			}

			forwarding = false
		}
	}()

	return workerPipe, func() {
		close(workerPipe)
		<-done
	}
}

func sendContext[T any](ctx context.Context, pipe chan<- T, item T) bool {
	if ctx.Err() != nil {
		return false
	}

	select {
	case <-ctx.Done():
		return false
	case pipe <- item:
		return true
	}
}

func receiveItem[T any](ctx context.Context, source <-chan T, onReceive func()) (item T, ok bool, canceled bool) {
	select {
	case <-ctx.Done():
		var zero T
		return zero, false, true
	case item, ok = <-source:
		if !ok {
			var zero T
			return zero, false, false
		}
		if onReceive != nil {
			onReceive()
		}
		if ctx.Err() != nil {
			var zero T
			return zero, false, true
		}
		return item, true, false
	}
}
