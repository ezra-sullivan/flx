package runtime

import (
	"fmt"
	"log"
	"runtime/debug"
	"sync"
)

// GoSafe starts fn in a new goroutine and logs any recovered panic.
func GoSafe(fn func()) {
	go RunSafe(fn)
}

// RunSafe executes fn synchronously and logs any recovered panic.
func RunSafe(fn func()) {
	defer func() {
		if p := recover(); p != nil {
			log.Printf("[flx] panic recovered: %+v\n%s", p, debug.Stack())
		}
	}()

	fn()
}

// RunSafeFunc executes fn and converts any panic into an error value.
func RunSafeFunc(fn func() error) (err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("panic: %+v\n%s", p, debug.Stack())
		}
	}()

	return fn()
}

// RoutineGroup wraps sync.WaitGroup with small helpers for launching related
// goroutines.
type RoutineGroup struct {
	wg sync.WaitGroup
}

// Run starts fn in a new goroutine.
func (g *RoutineGroup) Run(fn func()) {
	g.wg.Go(fn)
}

// RunSafe starts fn in a new goroutine and logs any recovered panic.
func (g *RoutineGroup) RunSafe(fn func()) {
	g.wg.Go(func() {
		RunSafe(fn)
	})
}

// Wait blocks until all launched goroutines have returned.
func (g *RoutineGroup) Wait() {
	g.wg.Wait()
}
