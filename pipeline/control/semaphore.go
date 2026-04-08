package control

import controlinternal "github.com/ezra-sullivan/flx/internal/control"

// DynamicSemaphore is a resizable semaphore used by dynamic worker pipelines.
// It is safe for concurrent use.
type DynamicSemaphore = controlinternal.DynamicSemaphore

// NewDynamicSemaphore returns a semaphore with capacity n, clamped to at least
// one slot.
func NewDynamicSemaphore(n int) *DynamicSemaphore {
	return controlinternal.NewDynamicSemaphore(n)
}
