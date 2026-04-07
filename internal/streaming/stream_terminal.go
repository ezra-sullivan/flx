package streaming

// Err returns the currently accumulated stream error without draining the
// stream.
func (s Stream[T]) Err() error {
	if s.state == nil {
		return nil
	}

	return s.state.Err()
}

func (s Stream[T]) maybePanicOnErr() {
	if s.state == nil || !s.state.ShouldPanic() {
		return
	}

	if err := s.state.Err(); err != nil {
		panic(err)
	}
}

func (s Stream[T]) shortCircuitErr() error {
	err := s.Err()
	go drain(s.source)
	return err
}

func (s Stream[T]) drainAndMaybePanic() {
	drain(s.source)
	s.maybePanicOnErr()
}

// Done drains the stream and panics if a fail-fast error was recorded.
func (s Stream[T]) Done() {
	drain(s.source)
	s.maybePanicOnErr()
}

// DoneErr drains the stream and returns the final error state.
func (s Stream[T]) DoneErr() error {
	drain(s.source)
	return s.Err()
}

// ForEach calls fn for every item in the stream and panics if a fail-fast error
// was recorded.
func (s Stream[T]) ForEach(fn func(T)) {
	for item := range s.source {
		fn(item)
	}
	s.maybePanicOnErr()
}

// ForEachErr calls fn for every item in the stream and returns the final error
// state.
func (s Stream[T]) ForEachErr(fn func(T)) error {
	for item := range s.source {
		fn(item)
	}
	return s.Err()
}

// ForAll hands the raw source channel to fn, then drains any leftovers and
// applies fail-fast panic behavior.
func (s Stream[T]) ForAll(fn func(<-chan T)) {
	fn(s.source)
	drain(s.source)
	s.maybePanicOnErr()
}

// ForAllErr hands the raw source channel to fn, then drains any leftovers and
// returns the final error state.
func (s Stream[T]) ForAllErr(fn func(<-chan T)) error {
	fn(s.source)
	drain(s.source)
	return s.Err()
}

// Parallel applies fn to each item using the same worker machinery as the
// transform operators and panics on fail-fast errors.
func (s Stream[T]) Parallel(fn func(T), opts ...Option) {
	if err := s.ParallelErr(func(item T) error {
		fn(item)
		return nil
	}, opts...); err != nil {
		panic(err)
	}
}

// ParallelErr applies fn to each item using worker options and returns the
// final error state.
func (s Stream[T]) ParallelErr(fn func(T) error, opts ...Option) error {
	return transformErr(s, func(item T, _ chan<- struct{}) error {
		return fn(item)
	}, opts...).DoneErr()
}

// Count drains the stream and returns the number of items it produced.
func (s Stream[T]) Count() (count int) {
	for range s.source {
		count++
	}
	s.maybePanicOnErr()
	return
}

// CountErr drains the stream, returns the item count, and returns the final
// error state.
func (s Stream[T]) CountErr() (int, error) {
	var count int
	for range s.source {
		count++
	}
	return count, s.Err()
}

// Collect drains the stream into a slice and panics on fail-fast errors.
func (s Stream[T]) Collect() []T {
	items, err := s.CollectErr()
	if err != nil {
		panic(err)
	}
	return items
}

// CollectErr drains the stream into a slice and returns the final error state.
func (s Stream[T]) CollectErr() ([]T, error) {
	var items []T
	for item := range s.source {
		items = append(items, item)
	}
	return items, s.Err()
}

// First returns the first item and then drains the rest of the stream so any
// delayed fail-fast error is observed before the call returns.
func (s Stream[T]) First() (T, bool) {
	for item := range s.source {
		s.drainAndMaybePanic()
		return item, true
	}

	s.maybePanicOnErr()
	var zero T
	return zero, false
}

// FirstErr returns the first item and the current error state, then drains the
// remaining source asynchronously.
func (s Stream[T]) FirstErr() (T, bool, error) {
	for item := range s.source {
		return item, true, s.shortCircuitErr()
	}

	var zero T
	return zero, false, s.Err()
}

// Last drains the stream and returns the last item it observed.
func (s Stream[T]) Last() (T, bool) {
	var (
		last T
		ok   bool
	)
	for item := range s.source {
		last = item
		ok = true
	}
	s.maybePanicOnErr()
	return last, ok
}

// LastErr drains the stream, returns the last item it observed, and returns the
// final error state.
func (s Stream[T]) LastErr() (T, bool, error) {
	var (
		last T
		ok   bool
	)
	for item := range s.source {
		last = item
		ok = true
	}
	return last, ok, s.Err()
}

// AllMatch reports whether every item satisfies predicate. It drains the
// remainder of the stream after the first mismatch so delayed fail-fast errors
// can still surface.
func (s Stream[T]) AllMatch(predicate func(T) bool) bool {
	for item := range s.source {
		if !predicate(item) {
			s.drainAndMaybePanic()
			return false
		}
	}

	s.maybePanicOnErr()
	return true
}

// AllMatchErr reports whether every item satisfies predicate and returns the
// current error state when it short-circuits.
func (s Stream[T]) AllMatchErr(predicate func(T) bool) (bool, error) {
	for item := range s.source {
		if !predicate(item) {
			return false, s.shortCircuitErr()
		}
	}
	return true, s.Err()
}

// AnyMatch reports whether any item satisfies predicate. It drains the
// remainder of the stream after the first match so delayed fail-fast errors can
// still surface.
func (s Stream[T]) AnyMatch(predicate func(T) bool) bool {
	for item := range s.source {
		if predicate(item) {
			s.drainAndMaybePanic()
			return true
		}
	}

	s.maybePanicOnErr()
	return false
}

// AnyMatchErr reports whether any item satisfies predicate and returns the
// current error state when it short-circuits.
func (s Stream[T]) AnyMatchErr(predicate func(T) bool) (bool, error) {
	for item := range s.source {
		if predicate(item) {
			return true, s.shortCircuitErr()
		}
	}
	return false, s.Err()
}

// NoneMatch reports whether no item satisfies predicate. It drains the
// remainder of the stream after the first match so delayed fail-fast errors can
// still surface.
func (s Stream[T]) NoneMatch(predicate func(T) bool) bool {
	for item := range s.source {
		if predicate(item) {
			s.drainAndMaybePanic()
			return false
		}
	}

	s.maybePanicOnErr()
	return true
}

// NoneMatchErr reports whether no item satisfies predicate and returns the
// current error state when it short-circuits.
func (s Stream[T]) NoneMatchErr(predicate func(T) bool) (bool, error) {
	for item := range s.source {
		if predicate(item) {
			return false, s.shortCircuitErr()
		}
	}
	return true, s.Err()
}

// Max drains the stream and returns the greatest item according to less.
func (s Stream[T]) Max(less func(T, T) bool) (T, bool) {
	var (
		best T
		ok   bool
	)
	for item := range s.source {
		if !ok || less(best, item) {
			best = item
			ok = true
		}
	}
	s.maybePanicOnErr()
	return best, ok
}

// MaxErr drains the stream, returns the greatest item according to less, and
// returns the final error state.
func (s Stream[T]) MaxErr(less func(T, T) bool) (T, bool, error) {
	var (
		best T
		ok   bool
	)
	for item := range s.source {
		if !ok || less(best, item) {
			best = item
			ok = true
		}
	}
	return best, ok, s.Err()
}

// Min drains the stream and returns the least item according to less.
func (s Stream[T]) Min(less func(T, T) bool) (T, bool) {
	var (
		best T
		ok   bool
	)
	for item := range s.source {
		if !ok || less(item, best) {
			best = item
			ok = true
		}
	}
	s.maybePanicOnErr()
	return best, ok
}

// MinErr drains the stream, returns the least item according to less, and
// returns the final error state.
func (s Stream[T]) MinErr(less func(T, T) bool) (T, bool, error) {
	var (
		best T
		ok   bool
	)
	for item := range s.source {
		if !ok || less(item, best) {
			best = item
			ok = true
		}
	}
	return best, ok, s.Err()
}
