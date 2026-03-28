package flx

import (
	"fmt"
	"log"
	"runtime/debug"
	"sync"
)

// goSafe starts fn in a new goroutine and logs any recovered panic.
func goSafe(fn func()) {
	go runSafe(fn)
}

// runSafe executes fn synchronously and logs any recovered panic.
func runSafe(fn func()) {
	defer func() {
		if p := recover(); p != nil {
			log.Printf("[flx] panic recovered: %+v\n%s", p, debug.Stack())
		}
	}()

	fn()
}

// runSafeFunc executes fn and converts any panic into an error value.
func runSafeFunc(fn func() error) (err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("panic: %+v\n%s", p, debug.Stack())
		}
	}()

	return fn()
}

// routineGroup wraps sync.WaitGroup with small helpers for launching related
// goroutines.
type routineGroup struct {
	wg sync.WaitGroup
}

// Run starts fn in a new goroutine.
func (g *routineGroup) Run(fn func()) {
	g.wg.Go(fn)
}

// RunSafe starts fn in a new goroutine and logs any recovered panic.
func (g *routineGroup) RunSafe(fn func()) {
	g.wg.Go(func() {
		runSafe(fn)
	})
}

// Wait blocks until all launched goroutines have returned.
func (g *routineGroup) Wait() {
	g.wg.Wait()
}
