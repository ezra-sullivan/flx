package flx

func (s Stream[T]) Err() error {
	if s.state == nil {
		return nil
	}

	return s.state.err()
}

func (s Stream[T]) maybePanicOnErr() {
	if s.state == nil || !s.state.shouldPanic() {
		return
	}

	if err := s.state.err(); err != nil {
		panic(err)
	}
}

func (s Stream[T]) drainErr() error {
	drain(s.source)
	return s.Err()
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

func (s Stream[T]) Done() {
	drain(s.source)
	s.maybePanicOnErr()
}

func (s Stream[T]) DoneErr() error {
	drain(s.source)
	return s.Err()
}

func (s Stream[T]) ForEach(fn func(T)) {
	for item := range s.source {
		fn(item)
	}
	s.maybePanicOnErr()
}

func (s Stream[T]) ForEachErr(fn func(T)) error {
	for item := range s.source {
		fn(item)
	}
	return s.Err()
}

func (s Stream[T]) ForAll(fn func(<-chan T)) {
	fn(s.source)
	drain(s.source)
	s.maybePanicOnErr()
}

func (s Stream[T]) ForAllErr(fn func(<-chan T)) error {
	fn(s.source)
	drain(s.source)
	return s.Err()
}

func (s Stream[T]) Parallel(fn func(T), opts ...Option) {
	if err := s.ParallelErr(func(item T) error {
		fn(item)
		return nil
	}, opts...); err != nil {
		panic(err)
	}
}

func (s Stream[T]) ParallelErr(fn func(T) error, opts ...Option) error {
	return transformErr(s, func(item T, _ chan<- struct{}) error {
		return fn(item)
	}, opts...).DoneErr()
}

func (s Stream[T]) Count() (count int) {
	for range s.source {
		count++
	}
	s.maybePanicOnErr()
	return
}

func (s Stream[T]) CountErr() (int, error) {
	var count int
	for range s.source {
		count++
	}
	return count, s.Err()
}

func (s Stream[T]) Collect() []T {
	items, err := s.CollectErr()
	if err != nil {
		panic(err)
	}
	return items
}

func (s Stream[T]) CollectErr() ([]T, error) {
	var items []T
	for item := range s.source {
		items = append(items, item)
	}
	return items, s.Err()
}

func (s Stream[T]) First() (T, bool) {
	for item := range s.source {
		s.drainAndMaybePanic()
		return item, true
	}

	s.maybePanicOnErr()
	var zero T
	return zero, false
}

func (s Stream[T]) FirstErr() (T, bool, error) {
	for item := range s.source {
		return item, true, s.shortCircuitErr()
	}

	var zero T
	return zero, false, s.Err()
}

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

func (s Stream[T]) AllMatchErr(predicate func(T) bool) (bool, error) {
	for item := range s.source {
		if !predicate(item) {
			return false, s.shortCircuitErr()
		}
	}
	return true, s.Err()
}

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

func (s Stream[T]) AnyMatchErr(predicate func(T) bool) (bool, error) {
	for item := range s.source {
		if predicate(item) {
			return true, s.shortCircuitErr()
		}
	}
	return false, s.Err()
}

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

func (s Stream[T]) NoneMatchErr(predicate func(T) bool) (bool, error) {
	for item := range s.source {
		if predicate(item) {
			return false, s.shortCircuitErr()
		}
	}
	return true, s.Err()
}

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
