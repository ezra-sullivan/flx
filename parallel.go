package flx

import "sync"

func Parallel(fns ...func()) {
	if err := ParallelWithErrorStrategy(ErrorStrategyFailFast, fns...); err != nil {
		panic(err)
	}
}

func ParallelWithErrorStrategy(strategy ErrorStrategy, fns ...func()) error {
	state := newStreamState()
	op := newOperationController(nil, state, strategy)
	var wg sync.WaitGroup

launch:
	for _, fn := range fns {
		select {
		case <-op.ctx.Done():
			break launch
		default:
		}

		run := fn
		wg.Go(func() {
			if err := runSafeFunc(func() error {
				run()
				return nil
			}); err != nil {
				op.report(err)
			}
		})
	}

	wg.Wait()
	return state.err()
}

func ParallelErr(fns ...func() error) error {
	var errs batchError
	var wg sync.WaitGroup

	for _, fn := range fns {
		run := fn
		wg.Go(func() {
			errs.Add(wrapWorkerError(runSafeFunc(run)))
		})
	}

	wg.Wait()
	return errs.Err()
}
