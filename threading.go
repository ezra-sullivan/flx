package flx

import (
	"fmt"
	"log"
	"runtime/debug"
	"sync"
)

func goSafe(fn func()) {
	go runSafe(fn)
}

func runSafe(fn func()) {
	defer func() {
		if p := recover(); p != nil {
			log.Printf("[flx] panic recovered: %+v\n%s", p, debug.Stack())
		}
	}()

	fn()
}

func runSafeFunc(fn func() error) (err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("panic: %+v\n%s", p, debug.Stack())
		}
	}()

	return fn()
}

type routineGroup struct {
	wg sync.WaitGroup
}

func (g *routineGroup) Run(fn func()) {
	g.wg.Go(fn)
}

func (g *routineGroup) RunSafe(fn func()) {
	g.wg.Go(func() {
		runSafe(fn)
	})
}

func (g *routineGroup) Wait() {
	g.wg.Wait()
}
