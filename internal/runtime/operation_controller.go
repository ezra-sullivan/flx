package runtime

import "context"

// OperationController owns the cancellation scope and report entrypoint for
// one concurrent stream operation.
type OperationController struct {
	ctx    context.Context
	cancel context.CancelCauseFunc
	report func(error)
}

// NewOperationController derives a worker context from parent and installs the
// root-owned error handling callback.
func NewOperationController(parent context.Context, report func(error)) *OperationController {
	if parent == nil {
		parent = context.Background()
	}

	ctx, cancel := context.WithCancelCause(parent)
	return &OperationController{
		ctx:    ctx,
		cancel: cancel,
		report: report,
	}
}

// Context returns the derived worker context for the operation.
func (c *OperationController) Context() context.Context {
	if c == nil {
		return nil
	}

	return c.ctx
}

// Cancel stops the operation and records err as the cancellation cause.
func (c *OperationController) Cancel(err error) {
	if c == nil || c.cancel == nil {
		return
	}

	c.cancel(err)
}

// Report forwards one worker failure into the root-owned strategy adapter.
func (c *OperationController) Report(err error) {
	if c == nil || err == nil {
		return
	}
	if c.report == nil {
		return
	}

	c.report(err)
}
