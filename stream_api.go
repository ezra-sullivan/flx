package flx

import "github.com/ezra-sullivan/flx/internal/streaming"

// Stream is a lazy sequence of values backed by a channel plus shared error
// state that records upstream worker failures.
type Stream[T any] struct {
	inner streaming.Stream[T]
}

func init() {
	streaming.SetWorkerErrorWrapper(wrapWorkerError)
}

func wrapStream[T any](inner streaming.Stream[T]) Stream[T] {
	return Stream[T]{inner: inner}
}

func unwrapStream[T any](s Stream[T]) streaming.Stream[T] {
	return s.inner
}

func unwrapStreams[T any](streams []Stream[T]) []streaming.Stream[T] {
	out := make([]streaming.Stream[T], 0, len(streams))
	for _, stream := range streams {
		out = append(out, stream.inner)
	}

	return out
}

// Values returns a stream that emits items in order and then closes.
func Values[T any](items ...T) Stream[T] {
	return wrapStream(streaming.Values(items...))
}

// From adapts a producer callback into a stream. Panics from generate are
// captured in the stream state and surfaced by terminal operations.
func From[T any](generate func(chan<- T)) Stream[T] {
	return wrapStream(streaming.From(generate))
}

// FromChan wraps source as a Stream without changing its production semantics.
func FromChan[T any](source <-chan T) Stream[T] {
	return wrapStream(streaming.FromChan(source))
}

// Concat merges s with others and returns a stream that emits items from all
// inputs as they arrive.
func Concat[T any](s Stream[T], others ...Stream[T]) Stream[T] {
	return wrapStream(streaming.Concat(unwrapStream(s), unwrapStreams(others)...))
}

// Concat merges s with others while preserving per-stream item order. Items
// from different input streams may interleave based on runtime scheduling.
func (s Stream[T]) Concat(others ...Stream[T]) Stream[T] {
	return wrapStream(s.inner.Concat(unwrapStreams(others)...))
}

// Filter keeps only the items for which fn returns true.
func (s Stream[T]) Filter(fn func(T) bool, opts ...Option) Stream[T] {
	return wrapStream(s.inner.Filter(fn, unwrapOptions(opts)...))
}

// Buffer inserts a channel buffer of size n between s and the returned stream.
func (s Stream[T]) Buffer(n int) Stream[T] {
	return wrapStream(s.inner.Buffer(n))
}

// Sort drains s, sorts all items with less, and then replays them as a new
// stream.
func (s Stream[T]) Sort(less func(T, T) bool) Stream[T] {
	return wrapStream(s.inner.Sort(less))
}

// Reverse drains s, reverses the collected items, and replays them as a new
// stream.
func (s Stream[T]) Reverse() Stream[T] {
	return wrapStream(s.inner.Reverse())
}

// Head returns a stream containing at most the first n items from s. The
// upstream source is drained after the head is satisfied so producers can exit.
func (s Stream[T]) Head(n int64) Stream[T] {
	return wrapStream(s.inner.Head(n))
}

// Tail returns a stream containing the last n items from s in original order.
func (s Stream[T]) Tail(n int64) Stream[T] {
	return wrapStream(s.inner.Tail(n))
}

// Skip discards the first n items from s and emits the remainder.
func (s Stream[T]) Skip(n int64) Stream[T] {
	return wrapStream(s.inner.Skip(n))
}

// Err returns the currently accumulated stream error without draining the
// stream.
func (s Stream[T]) Err() error {
	return s.inner.Err()
}

// Done drains the stream and panics if a fail-fast error was recorded.
func (s Stream[T]) Done() {
	s.inner.Done()
}

// DoneErr drains the stream and returns the final error state.
func (s Stream[T]) DoneErr() error {
	return s.inner.DoneErr()
}

// ForEach calls fn for every item in the stream and panics if a fail-fast error
// was recorded.
func (s Stream[T]) ForEach(fn func(T)) {
	s.inner.ForEach(fn)
}

// ForEachErr calls fn for every item in the stream and returns the final error
// state.
func (s Stream[T]) ForEachErr(fn func(T)) error {
	return s.inner.ForEachErr(fn)
}

// ForAll hands the raw source channel to fn, then drains any leftovers and
// applies fail-fast panic behavior.
func (s Stream[T]) ForAll(fn func(<-chan T)) {
	s.inner.ForAll(fn)
}

// ForAllErr hands the raw source channel to fn, then drains any leftovers and
// returns the final error state.
func (s Stream[T]) ForAllErr(fn func(<-chan T)) error {
	return s.inner.ForAllErr(fn)
}

// Parallel applies fn to each item using the same worker machinery as the
// transform operators and panics on fail-fast errors.
func (s Stream[T]) Parallel(fn func(T), opts ...Option) {
	s.inner.Parallel(fn, unwrapOptions(opts)...)
}

// ParallelErr applies fn to each item using worker options and returns the
// final error state.
func (s Stream[T]) ParallelErr(fn func(T) error, opts ...Option) error {
	return s.inner.ParallelErr(fn, unwrapOptions(opts)...)
}

// Count drains the stream and returns the number of items it produced.
func (s Stream[T]) Count() int {
	return s.inner.Count()
}

// CountErr drains the stream, returns the item count, and returns the final
// error state.
func (s Stream[T]) CountErr() (int, error) {
	return s.inner.CountErr()
}

// Collect drains the stream into a slice and panics on fail-fast errors.
func (s Stream[T]) Collect() []T {
	return s.inner.Collect()
}

// CollectErr drains the stream into a slice and returns the final error state.
func (s Stream[T]) CollectErr() ([]T, error) {
	return s.inner.CollectErr()
}

// First returns the first item and then drains the rest of the stream so any
// delayed fail-fast error is observed before the call returns.
func (s Stream[T]) First() (T, bool) {
	return s.inner.First()
}

// FirstErr returns the first item and the current error state, then drains the
// remaining source asynchronously.
func (s Stream[T]) FirstErr() (T, bool, error) {
	return s.inner.FirstErr()
}

// Last drains the stream and returns the last item it observed.
func (s Stream[T]) Last() (T, bool) {
	return s.inner.Last()
}

// LastErr drains the stream, returns the last item it observed, and returns the
// final error state.
func (s Stream[T]) LastErr() (T, bool, error) {
	return s.inner.LastErr()
}

// AllMatch reports whether every item satisfies predicate. It drains the
// remainder of the stream after the first mismatch so delayed fail-fast errors
// can still surface.
func (s Stream[T]) AllMatch(predicate func(T) bool) bool {
	return s.inner.AllMatch(predicate)
}

// AllMatchErr reports whether every item satisfies predicate and returns the
// current error state when it short-circuits.
func (s Stream[T]) AllMatchErr(predicate func(T) bool) (bool, error) {
	return s.inner.AllMatchErr(predicate)
}

// AnyMatch reports whether any item satisfies predicate. It drains the
// remainder of the stream after the first match so delayed fail-fast errors can
// still surface.
func (s Stream[T]) AnyMatch(predicate func(T) bool) bool {
	return s.inner.AnyMatch(predicate)
}

// AnyMatchErr reports whether any item satisfies predicate and returns the
// current error state when it short-circuits.
func (s Stream[T]) AnyMatchErr(predicate func(T) bool) (bool, error) {
	return s.inner.AnyMatchErr(predicate)
}

// NoneMatch reports whether no item satisfies predicate. It drains the
// remainder of the stream after the first match so delayed fail-fast errors can
// still surface.
func (s Stream[T]) NoneMatch(predicate func(T) bool) bool {
	return s.inner.NoneMatch(predicate)
}

// NoneMatchErr reports whether no item satisfies predicate and returns the
// current error state when it short-circuits.
func (s Stream[T]) NoneMatchErr(predicate func(T) bool) (bool, error) {
	return s.inner.NoneMatchErr(predicate)
}

// Max drains the stream and returns the greatest item according to less.
func (s Stream[T]) Max(less func(T, T) bool) (T, bool) {
	return s.inner.Max(less)
}

// MaxErr drains the stream, returns the greatest item according to less, and
// returns the final error state.
func (s Stream[T]) MaxErr(less func(T, T) bool) (T, bool, error) {
	return s.inner.MaxErr(less)
}

// Min drains the stream and returns the least item according to less.
func (s Stream[T]) Min(less func(T, T) bool) (T, bool) {
	return s.inner.Min(less)
}

// MinErr drains the stream, returns the least item according to less, and
// returns the final error state.
func (s Stream[T]) MinErr(less func(T, T) bool) (T, bool, error) {
	return s.inner.MinErr(less)
}
