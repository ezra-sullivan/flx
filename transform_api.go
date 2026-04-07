package flx

import (
	"context"
	"errors"
	"time"

	"github.com/ezra-sullivan/flx/internal/streaming"
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
type Group[K comparable, T any] = streaming.Group[K, T]

// SendContext sends item to pipe unless ctx has already been canceled.
func SendContext[T any](ctx context.Context, pipe chan<- T, item T) bool {
	return streaming.SendContext(ctx, pipe, item)
}

// Map applies fn to each item in s and emits the mapped values.
func Map[T, U any](s Stream[T], fn func(T) U, opts ...Option) Stream[U] {
	return wrapStream(streaming.Map(unwrapStream(s), fn, unwrapOptions(opts)...))
}

// MapErr applies fn to each item in s and records any returned error in the
// stream state.
func MapErr[T, U any](s Stream[T], fn func(T) (U, error), opts ...Option) Stream[U] {
	return wrapStream(streaming.MapErr(unwrapStream(s), fn, unwrapOptions(opts)...))
}

// FlatMap calls fn for each item and lets fn emit zero or more output values.
func FlatMap[T, U any](s Stream[T], fn func(T, chan<- U), opts ...Option) Stream[U] {
	return wrapStream(streaming.FlatMap(unwrapStream(s), fn, unwrapOptions(opts)...))
}

// FlatMapErr calls fn for each item and records any returned worker error in
// the stream state.
func FlatMapErr[T, U any](s Stream[T], fn func(T, chan<- U) error, opts ...Option) Stream[U] {
	return wrapStream(streaming.FlatMapErr(unwrapStream(s), fn, unwrapOptions(opts)...))
}

// MapContext is Map with a caller-provided context passed into each worker.
func MapContext[T, U any](ctx context.Context, s Stream[T], fn func(context.Context, T) U, opts ...Option) Stream[U] {
	return wrapStream(streaming.MapContext(requireContext(ctx), unwrapStream(s), fn, unwrapOptions(opts)...))
}

// MapContextErr is MapErr with a caller-provided context passed into each
// worker.
func MapContextErr[T, U any](ctx context.Context, s Stream[T], fn func(context.Context, T) (U, error), opts ...Option) Stream[U] {
	return wrapStream(streaming.MapContextErr(requireContext(ctx), unwrapStream(s), fn, unwrapOptions(opts)...))
}

// FlatMapContext is FlatMap with a caller-provided context passed into each
// worker.
func FlatMapContext[T, U any](ctx context.Context, s Stream[T], fn func(context.Context, T, chan<- U), opts ...Option) Stream[U] {
	return wrapStream(streaming.FlatMapContext(requireContext(ctx), unwrapStream(s), fn, unwrapOptions(opts)...))
}

// FlatMapContextErr is FlatMapErr with a caller-provided context passed into
// each worker.
func FlatMapContextErr[T, U any](ctx context.Context, s Stream[T], fn func(context.Context, T, chan<- U) error, opts ...Option) Stream[U] {
	return wrapStream(streaming.FlatMapContextErr(requireContext(ctx), unwrapStream(s), fn, unwrapOptions(opts)...))
}

// DistinctBy keeps the first item for each key produced by fn.
func DistinctBy[T any, K comparable](s Stream[T], fn func(T) K) Stream[T] {
	return wrapStream(streaming.DistinctBy(unwrapStream(s), fn))
}

// DistinctByCount keeps the first item for each key within windows of n input
// items, resetting the seen-key set after every window.
func DistinctByCount[T any, K comparable](s Stream[T], n int, fn func(T) K) Stream[T] {
	validateWindowCount(n)
	return wrapStream(streaming.DistinctByCount(unwrapStream(s), n, fn))
}

// GroupBy drains s, groups items by fn, and emits groups in first-seen key
// order.
func GroupBy[T any, K comparable](s Stream[T], fn func(T) K) Stream[[]T] {
	return wrapStream(streaming.GroupBy(unwrapStream(s), fn))
}

// GroupByCount groups items by fn within windows of n input items and emits one
// Group per key in first-seen order for each window.
func GroupByCount[T any, K comparable](s Stream[T], n int, fn func(T) K) Stream[Group[K, T]] {
	validateWindowCount(n)
	return wrapStream(streaming.GroupByCount(unwrapStream(s), n, fn))
}

// DistinctByWindow keeps the first item for each key within a time window that
// starts when the first item in that window arrives.
func DistinctByWindow[T any, K comparable](ctx context.Context, s Stream[T], every time.Duration, fn func(T) K) Stream[T] {
	validateWindowDuration(every)
	return wrapStream(streaming.DistinctByWindow(requireContext(ctx), unwrapStream(s), every, fn))
}

// GroupByWindow groups items by fn within a time window that starts when the
// first item in that window arrives and flushes on timer tick or source close.
func GroupByWindow[T any, K comparable](ctx context.Context, s Stream[T], every time.Duration, fn func(T) K) Stream[Group[K, T]] {
	validateWindowDuration(every)
	return wrapStream(streaming.GroupByWindow(requireContext(ctx), unwrapStream(s), every, fn))
}

// Chunk groups items into slices of size n, emitting a final short chunk when
// the source ends.
func Chunk[T any](s Stream[T], n int) Stream[[]T] {
	return wrapStream(streaming.Chunk(unwrapStream(s), n))
}

// Reduce hands s's source channel to fn, drains any remaining items after fn
// returns, and joins fn's returned error with the stream error state.
func Reduce[T, R any](s Stream[T], fn func(<-chan T) (R, error)) (R, error) {
	return streaming.Reduce(unwrapStream(s), fn)
}

// Stage applies fn to each item in in and emits the mapped values.
// It is a thin semantic wrapper around MapContext for pipelines that want
// explicit stage-shaped call sites without introducing a second runtime model.
func Stage[I, O any](
	ctx context.Context,
	in Stream[I],
	fn func(context.Context, I) O,
	opts ...Option,
) Stream[O] {
	return wrapStream(streaming.Stage(requireContext(ctx), unwrapStream(in), fn, unwrapOptions(opts)...))
}

// StageErr applies fn to each item in in and records returned worker errors in
// the stream state.
// It is a thin semantic wrapper around MapContextErr for stage-oriented
// pipelines that still want stream-level error handling.
func StageErr[I, O any](
	ctx context.Context,
	in Stream[I],
	fn func(context.Context, I) (O, error),
	opts ...Option,
) Stream[O] {
	return wrapStream(streaming.StageErr(requireContext(ctx), unwrapStream(in), fn, unwrapOptions(opts)...))
}

// FlatStage applies fn to each item in in and lets fn emit zero or more output
// values.
// It is a thin semantic wrapper around FlatMapContext for stage-oriented
// pipelines.
func FlatStage[I, O any](
	ctx context.Context,
	in Stream[I],
	fn func(context.Context, I, chan<- O),
	opts ...Option,
) Stream[O] {
	return wrapStream(streaming.FlatStage(requireContext(ctx), unwrapStream(in), fn, unwrapOptions(opts)...))
}

// FlatStageErr applies fn to each item in in, lets fn emit zero or more output
// values, and records returned worker errors in the stream state.
// It is a thin semantic wrapper around FlatMapContextErr.
func FlatStageErr[I, O any](
	ctx context.Context,
	in Stream[I],
	fn func(context.Context, I, chan<- O) error,
	opts ...Option,
) Stream[O] {
	return wrapStream(streaming.FlatStageErr(requireContext(ctx), unwrapStream(in), fn, unwrapOptions(opts)...))
}

// Tap runs fn for each item in in and re-emits the original item when fn
// succeeds.
// If fn returns an error, Tap records that error in the stream state and does
// not forward the failed item.
func Tap[T any](
	ctx context.Context,
	in Stream[T],
	fn func(context.Context, T) error,
	opts ...Option,
) Stream[T] {
	return wrapStream(streaming.Tap(requireContext(ctx), unwrapStream(in), fn, unwrapOptions(opts)...))
}

// Through applies fn to each item in s and returns another Stream[T].
// It exists to make same-type stage segments read fluently in a chain.
func (s Stream[T]) Through(
	ctx context.Context,
	fn func(context.Context, T) T,
	opts ...Option,
) Stream[T] {
	return wrapStream(s.inner.Through(requireContext(ctx), fn, unwrapOptions(opts)...))
}

// ThroughErr applies fn to each item in s, records returned worker errors in
// the stream state, and returns another Stream[T].
// It exists to make same-type stage segments read fluently in a chain.
func (s Stream[T]) ThroughErr(
	ctx context.Context,
	fn func(context.Context, T) (T, error),
	opts ...Option,
) Stream[T] {
	return wrapStream(s.inner.ThroughErr(requireContext(ctx), fn, unwrapOptions(opts)...))
}

// Tap runs fn for each item in s and re-emits the original item when fn
// succeeds.
func (s Stream[T]) Tap(
	ctx context.Context,
	fn func(context.Context, T) error,
	opts ...Option,
) Stream[T] {
	return wrapStream(s.inner.Tap(requireContext(ctx), fn, unwrapOptions(opts)...))
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
