package flx

import (
	"context"
	"sync"
)

func transformErr[T, U any](s Stream[T], fn func(T, chan<- U) error, opts ...Option) Stream[U] {
	options := buildOptions(opts...)
	if options.interruptible {
		panic(ErrInterruptibleWorkersRequireContextTransform)
	}

	switch {
	case options.unlimited:
		return walkUnlimited[T, U](nil, s, func(_ context.Context, item T, pipe chan<- U) error {
			return fn(item, pipe)
		}, options)
	case options.controller != nil:
		return walkDynamic[T, U](nil, s, func(_ context.Context, item T, pipe chan<- U) error {
			return fn(item, pipe)
		}, options)
	default:
		return walkLimited[T, U](nil, s, func(_ context.Context, item T, pipe chan<- U) error {
			return fn(item, pipe)
		}, options)
	}
}

func transformContextErr[T, U any](ctx context.Context, s Stream[T], fn func(context.Context, T, chan<- U) error, opts ...Option) Stream[U] {
	options := buildOptions(opts...)

	switch {
	case options.unlimited:
		return walkUnlimited(ctx, s, fn, options)
	case options.controller != nil && options.interruptible:
		return walkInterruptible(ctx, s, fn, options)
	case options.controller != nil:
		return walkDynamic(ctx, s, fn, options)
	default:
		return walkLimited(ctx, s, fn, options)
	}
}

func walkLimited[T, U any](parent context.Context, s Stream[T], fn func(context.Context, T, chan<- U) error, option *opOptions) Stream[U] {
	pipe := make(chan U, option.workers)

	go func() {
		var wg sync.WaitGroup
		pool := make(chan struct{}, option.workers)
		op := newOperationController(parent, s.state, option.errorStrategy)

		stop := func() {
			go drain(s.source)
			wg.Wait()
			close(pipe)
		}

		for {
			item, ok, canceled := receiveItem(op.ctx, s.source)
			switch {
			case canceled:
				stop()
				return
			case !ok:
				wg.Wait()
				close(pipe)
				return
			}

			if err := acquirePool(op.ctx, pool); err != nil {
				stop()
				return
			}
			if err := op.ctx.Err(); err != nil {
				<-pool
				stop()
				return
			}

			val := item
			wg.Go(func() {
				defer func() {
					<-pool
				}()

				if err := op.ctx.Err(); err != nil {
					return
				}
				if err := runSafeFunc(func() error {
					return fn(op.ctx, val, pipe)
				}); err != nil {
					op.report(err)
				}
			})
		}
	}()

	return streamWithState(pipe, s.state)
}

func walkUnlimited[T, U any](parent context.Context, s Stream[T], fn func(context.Context, T, chan<- U) error, option *opOptions) Stream[U] {
	pipe := make(chan U, unlimitedWorkerBuffer)

	go func() {
		var wg sync.WaitGroup
		op := newOperationController(parent, s.state, option.errorStrategy)

		stop := func() {
			go drain(s.source)
			wg.Wait()
			close(pipe)
		}

		for {
			item, ok, canceled := receiveItem(op.ctx, s.source)
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
				if err := op.ctx.Err(); err != nil {
					return
				}
				if err := runSafeFunc(func() error {
					return fn(op.ctx, val, pipe)
				}); err != nil {
					op.report(err)
				}
			})
		}
	}()

	return streamWithState(pipe, s.state)
}

func walkDynamic[T, U any](parent context.Context, s Stream[T], fn func(context.Context, T, chan<- U) error, option *opOptions) Stream[U] {
	ctrl := option.controller
	pipe := make(chan U, ctrl.Workers())

	go func() {
		var wg sync.WaitGroup
		op := newOperationController(parent, s.state, option.errorStrategy)

		stop := func() {
			go drain(s.source)
			wg.Wait()
			close(pipe)
		}

		for {
			item, ok, canceled := receiveItem(op.ctx, s.source)
			switch {
			case canceled:
				stop()
				return
			case !ok:
				wg.Wait()
				close(pipe)
				return
			}

			if err := ctrl.sem.AcquireCtx(op.ctx); err != nil {
				stop()
				return
			}
			if err := op.ctx.Err(); err != nil {
				ctrl.sem.Release()
				stop()
				return
			}

			val := item
			wg.Go(func() {
				defer ctrl.sem.Release()

				if err := op.ctx.Err(); err != nil {
					return
				}
				if err := runSafeFunc(func() error {
					return fn(op.ctx, val, pipe)
				}); err != nil {
					op.report(err)
				}
			})
		}
	}()

	return streamWithState(pipe, s.state)
}

func walkInterruptible[T, U any](parent context.Context, s Stream[T], fn func(context.Context, T, chan<- U) error, option *opOptions) Stream[U] {
	ctrl := option.controller
	pipe := make(chan U, ctrl.Workers())

	go func() {
		var wg sync.WaitGroup
		op := newOperationController(parent, s.state, option.errorStrategy)

		stop := func() {
			go drain(s.source)
			wg.Wait()
			close(pipe)
		}

		for {
			item, ok, canceled := receiveItem(op.ctx, s.source)
			switch {
			case canceled:
				stop()
				return
			case !ok:
				wg.Wait()
				close(pipe)
				return
			}

			if err := ctrl.sem.AcquireCtx(op.ctx); err != nil {
				stop()
				return
			}
			if err := op.ctx.Err(); err != nil {
				ctrl.sem.Release()
				stop()
				return
			}

			val := item
			ctx, cancel := context.WithCancelCause(op.ctx)
			id := ctrl.registerCancel(cancel)

			wg.Go(func() {
				defer func() {
					ctrl.unregisterCancel(id)
					cancel(nil)
					ctrl.sem.Release()
				}()

				if err := op.ctx.Err(); err != nil {
					return
				}
				if err := runSafeFunc(func() error {
					return fn(ctx, val, pipe)
				}); err != nil {
					op.report(err)
				}
			})
		}
	}()

	return streamWithState(pipe, s.state)
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

func drain[T any](source <-chan T) {
	for range source {
	}
}
