package flx

import (
	"context"
	"errors"
	"time"
)

var (
	// ErrInvalidWindowCount reports that a count-based window size was less than
	// one.
	ErrInvalidWindowCount = errors.New("flx: window count must be greater than 0")
	// ErrInvalidWindowDuration reports that a time-based window duration was not
	// positive.
	ErrInvalidWindowDuration = errors.New("flx: window duration must be positive")
)

// Group holds one grouping key plus the items assigned to that key.
type Group[K comparable, T any] struct {
	Key   K
	Items []T
}

// SendContext sends item to pipe unless ctx has already been canceled.
func SendContext[T any](ctx context.Context, pipe chan<- T, item T) bool {
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

// Map applies fn to each item in s and emits the mapped values.
func Map[T, U any](s Stream[T], fn func(T) U, opts ...Option) Stream[U] {
	return MapErr(s, func(item T) (U, error) {
		return fn(item), nil
	}, opts...)
}

// MapErr applies fn to each item in s and records any returned error in the
// stream state.
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

// FlatMap calls fn for each item and lets fn emit zero or more output values.
func FlatMap[T, U any](s Stream[T], fn func(T, chan<- U), opts ...Option) Stream[U] {
	return FlatMapErr(s, func(item T, pipe chan<- U) error {
		fn(item, pipe)
		return nil
	}, opts...)
}

// FlatMapErr calls fn for each item and records any returned worker error in
// the stream state.
func FlatMapErr[T, U any](s Stream[T], fn func(T, chan<- U) error, opts ...Option) Stream[U] {
	return transformErr(s, fn, opts...)
}

// MapContext is Map with a caller-provided context passed into each worker.
func MapContext[T, U any](ctx context.Context, s Stream[T], fn func(context.Context, T) U, opts ...Option) Stream[U] {
	return MapContextErr(ctx, s, func(ctx context.Context, item T) (U, error) {
		return fn(ctx, item), nil
	}, opts...)
}

// MapContextErr is MapErr with a caller-provided context passed into each
// worker.
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

// FlatMapContext is FlatMap with a caller-provided context passed into each
// worker.
func FlatMapContext[T, U any](ctx context.Context, s Stream[T], fn func(context.Context, T, chan<- U), opts ...Option) Stream[U] {
	return FlatMapContextErr(ctx, s, func(ctx context.Context, item T, pipe chan<- U) error {
		fn(ctx, item, pipe)
		return nil
	}, opts...)
}

// FlatMapContextErr is FlatMapErr with a caller-provided context passed into
// each worker.
func FlatMapContextErr[T, U any](ctx context.Context, s Stream[T], fn func(context.Context, T, chan<- U) error, opts ...Option) Stream[U] {
	return transformContextErr(requireContext(ctx), s, fn, opts...)
}

// DistinctBy keeps the first item for each key produced by fn.
func DistinctBy[T any, K comparable](s Stream[T], fn func(T) K) Stream[T] {
	source := make(chan T)

	go func() {
		defer close(source)
		if err := runSafeFunc(func() error {
			keys := make(map[K]struct{})
			for item := range s.source {
				key := fn(item)
				if _, ok := keys[key]; ok {
					continue
				}
				keys[key] = struct{}{}
				source <- item
			}
			return nil
		}); err != nil {
			s.state.add(err, true)
			go drain(s.source)
		}
	}()

	return streamWithState(source, s.state)
}

// DistinctByCount keeps the first item for each key within windows of n input
// items, resetting the seen-key set after every window.
func DistinctByCount[T any, K comparable](s Stream[T], n int, fn func(T) K) Stream[T] {
	validateWindowCount(n)

	source := make(chan T)

	go func() {
		defer close(source)
		if err := runSafeFunc(func() error {
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
			return nil
		}); err != nil {
			s.state.add(err, true)
			go drain(s.source)
		}
	}()

	return streamWithState(source, s.state)
}

// GroupBy drains s, groups items by fn, and emits groups in first-seen key
// order.
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

// GroupByCount groups items by fn within windows of n input items and emits one
// Group per key in first-seen order for each window.
func GroupByCount[T any, K comparable](s Stream[T], n int, fn func(T) K) Stream[Group[K, T]] {
	validateWindowCount(n)

	source := make(chan Group[K, T])

	go func() {
		defer close(source)
		if err := runSafeFunc(func() error {
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
			return nil
		}); err != nil {
			s.state.add(err, true)
			go drain(s.source)
		}
	}()

	return streamWithState(source, s.state)
}

// DistinctByWindow keeps the first item for each key within a time window that
// starts when the first item in that window arrives.
func DistinctByWindow[T any, K comparable](ctx context.Context, s Stream[T], every time.Duration, fn func(T) K) Stream[T] {
	validateWindowDuration(every)

	ctx = requireContext(ctx)
	source := make(chan T)

	go func() {
		defer close(source)
		if err := runSafeFunc(func() error {
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
					return nil
				case <-tick:
					stopWindow()
				case item, ok := <-s.source:
					if !ok {
						stopWindow()
						return nil
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
						return nil
					}
				}
			}
		}); err != nil {
			s.state.add(err, true)
			go drain(s.source)
		}
	}()

	return streamWithState(source, s.state)
}

// GroupByWindow groups items by fn within a time window that starts when the
// first item in that window arrives and flushes on timer tick or source close.
func GroupByWindow[T any, K comparable](ctx context.Context, s Stream[T], every time.Duration, fn func(T) K) Stream[Group[K, T]] {
	validateWindowDuration(every)

	ctx = requireContext(ctx)
	source := make(chan Group[K, T])

	go func() {
		defer close(source)
		if err := runSafeFunc(func() error {
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
					return nil
				case <-tick:
					if !flush() {
						go drain(s.source)
						return nil
					}
				case item, ok := <-s.source:
					if !ok {
						if !flush() {
							go drain(s.source)
						}
						return nil
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
		}); err != nil {
			s.state.add(err, true)
			go drain(s.source)
		}
	}()

	return streamWithState(source, s.state)
}

// Chunk groups items into slices of size n, emitting a final short chunk when
// the source ends.
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

// Reduce hands s's source channel to fn, drains any remaining items after fn
// returns, and joins fn's returned error with the stream error state.
func Reduce[T, R any](s Stream[T], fn func(<-chan T) (R, error)) (R, error) {
	result, err := fn(s.source)
	drain(s.source)
	return result, errors.Join(err, s.Err())
}

// validateWindowCount panics when n is less than one.
func validateWindowCount(n int) {
	if n < 1 {
		panic(ErrInvalidWindowCount)
	}
}

// validateWindowDuration panics when every is not positive.
func validateWindowDuration(every time.Duration) {
	if every <= 0 {
		panic(ErrInvalidWindowDuration)
	}
}

// stopTimer stops timer and drains its channel when needed.
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
