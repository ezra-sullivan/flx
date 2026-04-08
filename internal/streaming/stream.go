package streaming

import (
	"github.com/ezra-sullivan/flx/internal/collections"
	"github.com/ezra-sullivan/flx/internal/link"
	rt "github.com/ezra-sullivan/flx/internal/runtime"
	"github.com/ezra-sullivan/flx/internal/state"
	"slices"
)

type workerErrorWrapper func(error) error

var workerErrorHook workerErrorWrapper

// SetWorkerErrorWrapper installs the root-owned worker error wrapper used by
// concurrent stream operations.
func SetWorkerErrorWrapper(wrap func(error) error) {
	workerErrorHook = wrap
}

func wrapWorkerError(err error) error {
	if err == nil {
		return nil
	}
	if workerErrorHook == nil {
		return err
	}

	return workerErrorHook(err)
}

// Stream is a lazy sequence of values backed by a channel plus shared error
// state that records upstream worker failures.
type Stream[T any] struct {
	source <-chan T
	state  state.Handle
	link   *link.Meter
}

// New wraps source with a fresh local error state.
func New[T any](source <-chan T) Stream[T] {
	return WithState(source, nil)
}

// WithState wraps source with handle, creating a fresh state when nil is
// provided.
func WithState[T any](source <-chan T, handle state.Handle) Stream[T] {
	return WithStateAndLink(source, handle, nil)
}

// WithStateAndLink wraps source with handle and optional outbound link meter.
func WithStateAndLink[T any](source <-chan T, handle state.Handle, linkMeter *link.Meter) Stream[T] {
	if handle == nil {
		handle = state.NewStream()
	}

	return Stream[T]{
		source: source,
		state:  handle,
		link:   linkMeter,
	}
}

// Values returns a stream that emits items in order and then closes.
func Values[T any](items ...T) Stream[T] {
	source := make(chan T, len(items))
	for _, item := range items {
		source <- item
	}
	close(source)

	return New(source)
}

// From adapts a producer callback into a stream. Panics from generate are
// captured in the stream state and surfaced by terminal operations.
func From[T any](generate func(chan<- T)) Stream[T] {
	source := make(chan T)
	streamState := state.NewStream()

	go func() {
		defer close(source)
		if err := rt.RunSafeFunc(func() error {
			generate(source)
			return nil
		}); err != nil {
			streamState.Add(err, true)
		}
	}()

	return WithState(source, streamState)
}

// FromChan wraps source as a Stream without changing its production semantics.
func FromChan[T any](source <-chan T) Stream[T] {
	return New(source)
}

// Concat merges s with others and returns a stream that emits items from all
// inputs as they arrive.
func Concat[T any](s Stream[T], others ...Stream[T]) Stream[T] {
	return s.Concat(others...)
}

func (s Stream[T]) withSource(source <-chan T) Stream[T] {
	return WithState(source, s.state)
}

func (s Stream[T]) withSourceAndLink(source <-chan T, linkMeter *link.Meter) Stream[T] {
	return WithStateAndLink(source, s.state, linkMeter)
}

// Concat merges s with others while preserving per-stream item order. Items
// from different input streams may interleave based on runtime scheduling.
func (s Stream[T]) Concat(others ...Stream[T]) Stream[T] {
	source := make(chan T)
	parents := make([]state.Handle, 0, 1+len(others))
	parents = append(parents, s.state)
	for _, each := range others {
		parents = append(parents, each.state)
	}
	streamState := state.NewMerged(parents...)

	go func() {
		var group rt.RoutineGroup
		group.Run(func() {
			for {
				item, ok := s.next()
				if !ok {
					return
				}
				source <- item
			}
		})
		for _, each := range others {
			each := each
			group.Run(func() {
				for {
					item, ok := each.next()
					if !ok {
						return
					}
					source <- item
				}
			})
		}

		group.Wait()
		close(source)
	}()

	return WithState(source, streamState)
}

// Filter keeps only the items for which fn returns true.
func (s Stream[T]) Filter(fn func(T) bool, opts ...Option) Stream[T] {
	return FlatMap(s, func(item T, pipe chan<- T) {
		if fn(item) {
			pipe <- item
		}
	}, opts...)
}

// Buffer inserts a channel buffer of size n between s and the returned stream.
func (s Stream[T]) Buffer(n int) Stream[T] {
	if n < 0 {
		n = 0
	}

	source := make(chan T, n)
	go func() {
		for {
			item, ok := s.next()
			if !ok {
				break
			}
			source <- item
		}
		close(source)
	}()

	return s.withSource(source)
}

// Sort drains s, sorts all items with less, and then replays them as a new
// stream.
func (s Stream[T]) Sort(less func(T, T) bool) Stream[T] {
	var items []T
	for {
		item, ok := s.next()
		if !ok {
			break
		}
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

// Reverse drains s, reverses the collected items, and replays them as a new
// stream.
func (s Stream[T]) Reverse() Stream[T] {
	var items []T
	for {
		item, ok := s.next()
		if !ok {
			break
		}
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

// Head returns a stream containing at most the first n items from s. The
// upstream source is drained after the head is satisfied so producers can exit.
func (s Stream[T]) Head(n int64) Stream[T] {
	if n < 1 {
		panic("n must be greater than 0")
	}

	source := make(chan T)
	go func() {
		for {
			item, ok := s.next()
			if !ok {
				break
			}
			n--
			if n >= 0 {
				source <- item
			}
			if n == 0 {
				s.drainSource()
				close(source)
				return
			}
		}
		close(source)
	}()

	return s.withSource(source)
}

// Tail returns a stream containing the last n items from s in original order.
func (s Stream[T]) Tail(n int64) Stream[T] {
	if n < 1 {
		panic("n should be greater than 0")
	}

	source := make(chan T)
	go func() {
		ring := collections.NewRing[T](int(n))
		for {
			item, ok := s.next()
			if !ok {
				break
			}
			ring.Add(item)
		}
		for _, item := range ring.Take() {
			source <- item
		}
		close(source)
	}()

	return s.withSource(source)
}

// Skip discards the first n items from s and emits the remainder.
func (s Stream[T]) Skip(n int64) Stream[T] {
	if n < 0 {
		panic("n must not be negative")
	}
	if n == 0 {
		return s
	}

	source := make(chan T)
	go func() {
		for {
			item, ok := s.next()
			if !ok {
				break
			}
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
