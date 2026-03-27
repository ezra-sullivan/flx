package flx

import "slices"

type Stream[T any] struct {
	source <-chan T
	state  streamStateHandle
}

func Values[T any](items ...T) Stream[T] {
	source := make(chan T, len(items))
	for _, item := range items {
		source <- item
	}
	close(source)

	return newStream(source)
}

func From[T any](generate func(chan<- T)) Stream[T] {
	source := make(chan T)
	state := newStreamState()

	go func() {
		defer close(source)
		if err := runSafeFunc(func() error {
			generate(source)
			return nil
		}); err != nil {
			state.add(err, true)
		}
	}()

	return streamWithState(source, state)
}

func FromChan[T any](source <-chan T) Stream[T] {
	return newStream(source)
}

func Concat[T any](s Stream[T], others ...Stream[T]) Stream[T] {
	return s.Concat(others...)
}

func newStream[T any](source <-chan T) Stream[T] {
	return Stream[T]{
		source: source,
		state:  newStreamState(),
	}
}

func streamWithState[T any](source <-chan T, state streamStateHandle) Stream[T] {
	if state == nil {
		state = newStreamState()
	}

	return Stream[T]{
		source: source,
		state:  state,
	}
}

func (s Stream[T]) withSource(source <-chan T) Stream[T] {
	return streamWithState(source, s.state)
}

func (s Stream[T]) Concat(others ...Stream[T]) Stream[T] {
	source := make(chan T)
	parents := make([]streamStateHandle, 0, 1+len(others))
	parents = append(parents, s.state)
	for _, each := range others {
		parents = append(parents, each.state)
	}
	state := newMergedStreamState(parents...)

	go func() {
		var group routineGroup
		group.Run(func() {
			for item := range s.source {
				source <- item
			}
		})
		for _, each := range others {
			each := each
			group.Run(func() {
				for item := range each.source {
					source <- item
				}
			})
		}

		group.Wait()
		close(source)
	}()

	return streamWithState(source, state)
}

func (s Stream[T]) Filter(fn func(T) bool, opts ...Option) Stream[T] {
	return FlatMap(s, func(item T, pipe chan<- T) {
		if fn(item) {
			pipe <- item
		}
	}, opts...)
}

func (s Stream[T]) Buffer(n int) Stream[T] {
	if n < 0 {
		n = 0
	}

	source := make(chan T, n)
	go func() {
		for item := range s.source {
			source <- item
		}
		close(source)
	}()

	return s.withSource(source)
}

func (s Stream[T]) Sort(less func(T, T) bool) Stream[T] {
	var items []T
	for item := range s.source {
		items = append(items, item)
	}
	slices.SortFunc(items, func(a, b T) int {
		switch {
		case less(a, b):
			return -1
		case less(b, a):
			return 1
		default:
			return 0
		}
	})

	source := make(chan T, len(items))
	for _, item := range items {
		source <- item
	}
	close(source)

	return s.withSource(source)
}

func (s Stream[T]) Reverse() Stream[T] {
	var items []T
	for item := range s.source {
		items = append(items, item)
	}
	slices.Reverse(items)

	source := make(chan T, len(items))
	for _, item := range items {
		source <- item
	}
	close(source)

	return s.withSource(source)
}

func (s Stream[T]) Head(n int64) Stream[T] {
	if n < 1 {
		panic("n must be greater than 0")
	}

	source := make(chan T)
	go func() {
		for item := range s.source {
			n--
			if n >= 0 {
				source <- item
			}
			if n == 0 {
				drain(s.source)
				close(source)
				return
			}
		}
		close(source)
	}()

	return s.withSource(source)
}

func (s Stream[T]) Tail(n int64) Stream[T] {
	if n < 1 {
		panic("n should be greater than 0")
	}

	source := make(chan T)
	go func() {
		ring := newRing[T](int(n))
		for item := range s.source {
			ring.Add(item)
		}
		for _, item := range ring.Take() {
			source <- item
		}
		close(source)
	}()

	return s.withSource(source)
}

func (s Stream[T]) Skip(n int64) Stream[T] {
	if n < 0 {
		panic("n must not be negative")
	}
	if n == 0 {
		return s
	}

	source := make(chan T)
	go func() {
		for item := range s.source {
			n--
			if n >= 0 {
				continue
			}
			source <- item
		}
		close(source)
	}()

	return s.withSource(source)
}
