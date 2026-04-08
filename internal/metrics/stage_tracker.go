package metrics

import (
	"sync"
	"sync/atomic"
)

// StageTracker records per-stage counters and emits coalesced snapshots to an
// optional observer.
type StageTracker struct {
	observer       StageMetricsObserver
	stageName      string
	workerCapacity func() int

	inputCount    atomic.Int64
	startedCount  atomic.Int64
	droppedCount  atomic.Int64
	outputCount   atomic.Int64
	errorCount    atomic.Int64
	activeWorkers atomic.Int64

	updates chan struct{}
	stop    chan struct{}
	done    chan struct{}

	closeOnce sync.Once
}

// NewStageTracker returns a tracker that reports stage snapshots to observer.
// When observer is nil, it returns nil so callers can stay on the zero-observe
// path without extra work.
func NewStageTracker(stageName string, observer StageMetricsObserver, workerCapacity func() int) *StageTracker {
	if observer == nil {
		return nil
	}
	if workerCapacity == nil {
		workerCapacity = func() int { return 0 }
	}

	tracker := &StageTracker{
		observer:       observer,
		stageName:      stageName,
		workerCapacity: workerCapacity,
		updates:        make(chan struct{}, 1),
		stop:           make(chan struct{}),
		done:           make(chan struct{}),
	}

	go tracker.run()

	return tracker
}

// ObserveInput records that the stage accepted one upstream item.
func (t *StageTracker) ObserveInput() {
	if t == nil {
		return
	}

	t.inputCount.Add(1)
	t.notify()
}

// DropInput records that one accepted item was abandoned before any worker
// started processing it.
func (t *StageTracker) DropInput() {
	if t == nil {
		return
	}

	t.droppedCount.Add(1)
	t.notify()
}

// StartWorker records that one worker started processing an item.
func (t *StageTracker) StartWorker() {
	if t == nil {
		return
	}

	t.startedCount.Add(1)
	t.activeWorkers.Add(1)
	t.notify()
}

// FinishWorker records that one worker finished processing.
func (t *StageTracker) FinishWorker() {
	if t == nil {
		return
	}

	t.activeWorkers.Add(-1)
	t.notify()
}

// ObserveOutput records one item that was successfully emitted downstream.
func (t *StageTracker) ObserveOutput() {
	if t == nil {
		return
	}

	t.outputCount.Add(1)
	t.notify()
}

// ObserveError records one worker error reported by the runtime.
func (t *StageTracker) ObserveError() {
	if t == nil {
		return
	}

	t.errorCount.Add(1)
	t.notify()
}

// Close emits a final snapshot and waits for the reporter goroutine to stop.
func (t *StageTracker) Close() {
	if t == nil {
		return
	}

	t.closeOnce.Do(func() {
		close(t.stop)
		<-t.done
	})
}

func (t *StageTracker) run() {
	defer close(t.done)

	for {
		select {
		case <-t.updates:
			t.drainUpdates()
			t.deliver()
		case <-t.stop:
			t.deliver()
			return
		}
	}
}

func (t *StageTracker) notify() {
	select {
	case t.updates <- struct{}{}:
	default:
	}
}

func (t *StageTracker) drainUpdates() {
	for {
		select {
		case <-t.updates:
		default:
			return
		}
	}
}

func (t *StageTracker) deliver() {
	if t == nil || t.observer == nil {
		return
	}

	snapshot := t.snapshot()

	defer func() {
		_ = recover()
	}()

	t.observer(snapshot)
}

func (t *StageTracker) snapshot() StageMetrics {
	activeWorkers := int(t.activeWorkers.Load())
	workerCapacity := 0
	if t.workerCapacity != nil {
		workerCapacity = max(t.workerCapacity(), 0)
	}

	idleWorkers := 0
	if workerCapacity > 0 {
		idleWorkers = max(workerCapacity-activeWorkers, 0)
	}

	backlog := t.inputCount.Load() - t.startedCount.Load() - t.droppedCount.Load()
	if backlog < 0 {
		backlog = 0
	}

	return StageMetrics{
		StageName:     t.stageName,
		InputCount:    t.inputCount.Load(),
		OutputCount:   t.outputCount.Load(),
		ErrorCount:    t.errorCount.Load(),
		ActiveWorkers: activeWorkers,
		IdleWorkers:   idleWorkers,
		Backlog:       int(backlog),
	}
}
