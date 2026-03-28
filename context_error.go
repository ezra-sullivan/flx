package flx

import (
	"context"
	"errors"
)

// contextDoneErr returns the most specific completion error for ctx, preferring
// cancellation causes over the plain ctx.Err value.
func contextDoneErr(ctx context.Context) error {
	if cause := context.Cause(ctx); cause != nil {
		return cause
	}

	return ctx.Err()
}

// normalizeContextErr rewrites wrapped context errors to the canonical cause
// from ctx so callers observe stable cancellation values.
func normalizeContextErr(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}

	if ctxErr := ctx.Err(); ctxErr != nil && errors.Is(err, ctxErr) {
		return contextDoneErr(ctx)
	}

	return err
}
