package flx

import (
	"context"
	"errors"
)

func SendContext[T any](ctx context.Context, pipe chan<- T, item T) bool {
	select {
	case <-ctx.Done():
		return false
	case pipe <- item:
		return true
	}
}

func Map[T, U any](s Stream[T], fn func(T) U, opts ...Option) Stream[U] {
	return MapErr(s, func(item T) (U, error) {
		return fn(item), nil
	}, opts...)
}

func MapErr[T, U any](s Stream[T], fn func(T) (U, error), opts ...Option) Stream[U] {
	return FlatMapErr(s, func(item T, pipe chan<- U) error {
		mapped, err := fn(item)
		if err != nil {
			return err
		}
		pipe <- mapped
		return nil
	}, opts...)
}

func FlatMap[T, U any](s Stream[T], fn func(T, chan<- U), opts ...Option) Stream[U] {
	return FlatMapErr(s, func(item T, pipe chan<- U) error {
		fn(item, pipe)
		return nil
	}, opts...)
}

func FlatMapErr[T, U any](s Stream[T], fn func(T, chan<- U) error, opts ...Option) Stream[U] {
	return transformErr(s, fn, opts...)
}

func MapContext[T, U any](ctx context.Context, s Stream[T], fn func(context.Context, T) U, opts ...Option) Stream[U] {
	return MapContextErr(ctx, s, func(ctx context.Context, item T) (U, error) {
		return fn(ctx, item), nil
	}, opts...)
}

func MapContextErr[T, U any](ctx context.Context, s Stream[T], fn func(context.Context, T) (U, error), opts ...Option) Stream[U] {
	return FlatMapContextErr(ctx, s, func(ctx context.Context, item T, pipe chan<- U) error {
		mapped, err := fn(ctx, item)
		if err != nil {
			return err
		}
		pipe <- mapped
		return nil
	}, opts...)
}

func FlatMapContext[T, U any](ctx context.Context, s Stream[T], fn func(context.Context, T, chan<- U), opts ...Option) Stream[U] {
	return FlatMapContextErr(ctx, s, func(ctx context.Context, item T, pipe chan<- U) error {
		fn(ctx, item, pipe)
		return nil
	}, opts...)
}

func FlatMapContextErr[T, U any](ctx context.Context, s Stream[T], fn func(context.Context, T, chan<- U) error, opts ...Option) Stream[U] {
	return transformContextErr(requireContext(ctx), s, fn, opts...)
}

func DistinctBy[T any, K comparable](s Stream[T], fn func(T) K) Stream[T] {
	source := make(chan T)

	go func() {
		defer close(source)

		keys := make(map[K]struct{})
		for item := range s.source {
			key := fn(item)
			if _, ok := keys[key]; ok {
				continue
			}
			keys[key] = struct{}{}
			source <- item
		}
	}()

	return streamWithState(source, s.state)
}

func GroupBy[T any, K comparable](s Stream[T], fn func(T) K) Stream[[]T] {
	groups := make(map[K][]T)
	order := make([]K, 0)
	for item := range s.source {
		key := fn(item)
		if _, ok := groups[key]; !ok {
			order = append(order, key)
		}
		groups[key] = append(groups[key], item)
	}

	source := make(chan []T)
	go func() {
		for _, key := range order {
			source <- groups[key]
		}
		close(source)
	}()

	return streamWithState(source, s.state)
}

func Chunk[T any](s Stream[T], n int) Stream[[]T] {
	if n < 1 {
		panic("n should be greater than 0")
	}

	source := make(chan []T)
	go func() {
		var chunk []T
		for item := range s.source {
			chunk = append(chunk, item)
			if len(chunk) == n {
				source <- chunk
				chunk = nil
			}
		}
		if chunk != nil {
			source <- chunk
		}
		close(source)
	}()

	return streamWithState(source, s.state)
}

func Reduce[T, R any](s Stream[T], fn func(<-chan T) (R, error)) (R, error) {
	result, err := fn(s.source)
	return result, errors.Join(err, s.Err())
}
