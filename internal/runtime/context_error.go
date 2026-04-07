package runtime

import (
	"context"
	"errors"
)

// ContextDoneErr returns the most specific completion error for ctx, preferring
// cancellation causes over the plain ctx.Err value.
func ContextDoneErr(ctx context.Context) error {
	if cause := context.Cause(ctx); cause != nil {
		return cause
	}

	return ctx.Err()
}

// NormalizeContextErr rewrites wrapped context errors to the canonical cause
// from ctx so callers observe stable cancellation values.
func NormalizeContextErr(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}

	if ctxErr := ctx.Err(); ctxErr != nil && errors.Is(err, ctxErr) {
		return ContextDoneErr(ctx)
	}

	return err
}
