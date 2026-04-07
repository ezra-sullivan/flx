package streaming

import "context"

// Stage applies fn to each item in in and emits the mapped values.
func Stage[I, O any](ctx context.Context, in Stream[I], fn func(context.Context, I) O, opts ...Option) Stream[O] {
	return MapContext(ctx, in, fn, opts...)
}

// StageErr applies fn to each item in in and records returned worker errors in
// the stream state.
func StageErr[I, O any](ctx context.Context, in Stream[I], fn func(context.Context, I) (O, error), opts ...Option) Stream[O] {
	return MapContextErr(ctx, in, fn, opts...)
}

// FlatStage applies fn to each item in in and lets fn emit zero or more output
// values.
func FlatStage[I, O any](ctx context.Context, in Stream[I], fn func(context.Context, I, chan<- O), opts ...Option) Stream[O] {
	return FlatMapContext(ctx, in, fn, opts...)
}

// FlatStageErr applies fn to each item in in, lets fn emit zero or more output
// values, and records returned worker errors in the stream state.
func FlatStageErr[I, O any](ctx context.Context, in Stream[I], fn func(context.Context, I, chan<- O) error, opts ...Option) Stream[O] {
	return FlatMapContextErr(ctx, in, fn, opts...)
}

// Tap runs fn for each item in in and re-emits the original item when fn
// succeeds.
func Tap[T any](ctx context.Context, in Stream[T], fn func(context.Context, T) error, opts ...Option) Stream[T] {
	return StageErr(ctx, in, func(ctx context.Context, item T) (T, error) {
		return item, fn(ctx, item)
	}, opts...)
}

// Through applies fn to each item in s and returns another Stream[T].
func (s Stream[T]) Through(ctx context.Context, fn func(context.Context, T) T, opts ...Option) Stream[T] {
	return Stage(ctx, s, fn, opts...)
}

// ThroughErr applies fn to each item in s, records returned worker errors in
// the stream state, and returns another Stream[T].
func (s Stream[T]) ThroughErr(ctx context.Context, fn func(context.Context, T) (T, error), opts ...Option) Stream[T] {
	return StageErr(ctx, s, fn, opts...)
}

// Tap runs fn for each item in s and re-emits the original item when fn
// succeeds.
func (s Stream[T]) Tap(ctx context.Context, fn func(context.Context, T) error, opts ...Option) Stream[T] {
	return Tap(ctx, s, fn, opts...)
}
