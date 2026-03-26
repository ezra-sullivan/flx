package flx

import (
	"context"
	"errors"
	"time"
)

var (
	ErrInvalidWindowCount    = errors.New("flx: window count must be greater than 0")
	ErrInvalidWindowDuration = errors.New("flx: window duration must be positive")
)

type Group[K comparable, T any] struct {
	Key   K
	Items []T
}

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

		if !SendContext(ctx, pipe, mapped) {
			return nil
		}

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

func DistinctByCount[T any, K comparable](s Stream[T], n int, fn func(T) K) Stream[T] {
	validateWindowCount(n)

	source := make(chan T)

	go func() {
		defer close(source)

		count := 0
		seen := make(map[K]struct{})
		for item := range s.source {
			if count == n {
				count = 0
				seen = make(map[K]struct{})
			}

			key := fn(item)
			if _, ok := seen[key]; !ok {
				seen[key] = struct{}{}
				source <- item
			}

			count++
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

func GroupByCount[T any, K comparable](s Stream[T], n int, fn func(T) K) Stream[Group[K, T]] {
	validateWindowCount(n)

	source := make(chan Group[K, T])

	go func() {
		defer close(source)

		count := 0
		groups := make(map[K][]T)
		order := make([]K, 0)

		flush := func() {
			for _, key := range order {
				source <- Group[K, T]{Key: key, Items: groups[key]}
			}
			count = 0
			groups = make(map[K][]T)
			order = nil
		}

		for item := range s.source {
			if count == n {
				flush()
			}

			key := fn(item)
			if _, ok := groups[key]; !ok {
				order = append(order, key)
			}
			groups[key] = append(groups[key], item)
			count++
		}

		if count > 0 {
			flush()
		}
	}()

	return streamWithState(source, s.state)
}

func DistinctByWindow[T any, K comparable](ctx context.Context, s Stream[T], every time.Duration, fn func(T) K) Stream[T] {
	validateWindowDuration(every)

	ctx = requireContext(ctx)
	source := make(chan T)

	go func() {
		defer close(source)

		var (
			timer *time.Timer
			tick  <-chan time.Time
			seen  map[K]struct{}
		)

		startWindow := func() {
			seen = make(map[K]struct{})
			timer = time.NewTimer(every)
			tick = timer.C
		}
		stopWindow := func() {
			stopTimer(timer)
			timer = nil
			tick = nil
			seen = nil
		}

		for {
			select {
			case <-ctx.Done():
				stopWindow()
				go drain(s.source)
				return
			case <-tick:
				stopWindow()
			case item, ok := <-s.source:
				if !ok {
					stopWindow()
					return
				}

				if seen == nil {
					startWindow()
				}

				key := fn(item)
				if _, ok := seen[key]; ok {
					continue
				}

				seen[key] = struct{}{}
				if !SendContext(ctx, source, item) {
					stopWindow()
					go drain(s.source)
					return
				}
			}
		}
	}()

	return streamWithState(source, s.state)
}

func GroupByWindow[T any, K comparable](ctx context.Context, s Stream[T], every time.Duration, fn func(T) K) Stream[Group[K, T]] {
	validateWindowDuration(every)

	ctx = requireContext(ctx)
	source := make(chan Group[K, T])

	go func() {
		defer close(source)

		var (
			timer  *time.Timer
			tick   <-chan time.Time
			groups map[K][]T
			order  []K
		)

		startWindow := func() {
			groups = make(map[K][]T)
			order = nil
			timer = time.NewTimer(every)
			tick = timer.C
		}
		stopWindow := func() {
			stopTimer(timer)
			timer = nil
			tick = nil
			groups = nil
			order = nil
		}
		flush := func() bool {
			if len(order) == 0 {
				stopWindow()
				return true
			}

			for _, key := range order {
				if !SendContext(ctx, source, Group[K, T]{Key: key, Items: groups[key]}) {
					stopWindow()
					return false
				}
			}

			stopWindow()
			return true
		}

		for {
			select {
			case <-ctx.Done():
				stopWindow()
				go drain(s.source)
				return
			case <-tick:
				if !flush() {
					go drain(s.source)
					return
				}
			case item, ok := <-s.source:
				if !ok {
					if !flush() {
						go drain(s.source)
					}
					return
				}

				if groups == nil {
					startWindow()
				}

				key := fn(item)
				if _, ok := groups[key]; !ok {
					order = append(order, key)
				}
				groups[key] = append(groups[key], item)
			}
		}
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
	drain(s.source)
	return result, errors.Join(err, s.Err())
}

func validateWindowCount(n int) {
	if n < 1 {
		panic(ErrInvalidWindowCount)
	}
}

func validateWindowDuration(every time.Duration) {
	if every <= 0 {
		panic(ErrInvalidWindowDuration)
	}
}

func stopTimer(timer *time.Timer) {
	if timer == nil {
		return
	}
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
}
