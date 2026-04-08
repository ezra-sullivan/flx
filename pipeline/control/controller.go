package control

import controlinternal "github.com/ezra-sullivan/flx/internal/control"

// ConcurrencyController coordinates resizable worker limits for dynamic stream
// operations. It is safe for concurrent use.
type ConcurrencyController struct {
	ctrl *controlinternal.Controller
}

// NewConcurrencyController returns a controller whose worker limit starts at
// workers, clamped to at least one slot.
func NewConcurrencyController(workers int) *ConcurrencyController {
	return &ConcurrencyController{
		ctrl: controlinternal.NewController(workers),
	}
}

// SetWorkers updates the target worker count. When the limit shrinks, the
// newest registered interruptible workers are canceled until the active count
// fits within the new limit.
func (c *ConcurrencyController) SetWorkers(n int) {
	c.ctrl.SetWorkers(n)
}

// Workers returns the configured worker limit.
func (c *ConcurrencyController) Workers() int {
	return c.ctrl.Workers()
}

// ActiveWorkers returns the number of workers currently holding semaphore
// slots.
func (c *ConcurrencyController) ActiveWorkers() int {
	return c.ctrl.ActiveWorkers()
}

func (c *ConcurrencyController) internalController() *controlinternal.Controller {
	if c == nil {
		return nil
	}

	return c.ctrl
}
