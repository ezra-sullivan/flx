package link

import (
	"sync"
	"sync/atomic"
)

// Meter records link traffic and emits coalesced snapshots to an optional
// observer.
type Meter struct {
	observer  MetricsObserver
	fromStage string
	capacity  int

	toStage atomic.Pointer[string]

	sentCount     atomic.Int64
	receivedCount atomic.Int64

	updates chan struct{}
	stop    chan struct{}
	done    chan struct{}

	closeOnce sync.Once
}

// NewMeter returns a link meter. When observer is nil it returns nil so callers
// can stay on the zero-observe path.
func NewMeter(fromStage string, capacity int, observer MetricsObserver) *Meter {
	if observer == nil {
		return nil
	}

	meter := &Meter{
		observer:  observer,
		fromStage: fromStage,
		capacity:  max(capacity, 0),
		updates:   make(chan struct{}, 1),
		stop:      make(chan struct{}),
		done:      make(chan struct{}),
	}

	go meter.run()

	return meter
}

// SetDownstreamStage records the currently known downstream stage name.
func (m *Meter) SetDownstreamStage(name string) {
	if m == nil || name == "" {
		return
	}

	stageName := new(string)
	*stageName = name
	m.toStage.Store(stageName)
	m.notify()
}

// ObserveSend records one item emitted into the link.
func (m *Meter) ObserveSend() {
	if m == nil {
		return
	}

	m.sentCount.Add(1)
	m.notify()
}

// ObserveReceive records one item received from the link.
func (m *Meter) ObserveReceive() {
	if m == nil {
		return
	}

	m.receivedCount.Add(1)
	m.notify()
}

// Close emits a final snapshot and stops the reporter goroutine.
func (m *Meter) Close() {
	if m == nil {
		return
	}

	m.closeOnce.Do(func() {
		close(m.stop)
		<-m.done
	})
}

func (m *Meter) run() {
	defer close(m.done)

	for {
		select {
		case <-m.updates:
			m.drainUpdates()
			m.deliver()
		case <-m.stop:
			m.deliver()
			return
		}
	}
}

func (m *Meter) notify() {
	select {
	case m.updates <- struct{}{}:
	default:
	}
}

func (m *Meter) drainUpdates() {
	for {
		select {
		case <-m.updates:
		default:
			return
		}
	}
}

func (m *Meter) deliver() {
	if m == nil || m.observer == nil {
		return
	}

	snapshot := m.snapshot()

	defer func() {
		_ = recover()
	}()

	m.observer(snapshot)
}

func (m *Meter) snapshot() Metrics {
	sent := m.sentCount.Load()
	received := m.receivedCount.Load()
	backlog := sent - received
	if backlog < 0 {
		backlog = 0
	}

	toStage := ""
	if name := m.toStage.Load(); name != nil {
		toStage = *name
	}

	return Metrics{
		FromStage:     m.fromStage,
		ToStage:       toStage,
		SentCount:     sent,
		ReceivedCount: received,
		Backlog:       int(backlog),
		Capacity:      m.capacity,
	}
}
