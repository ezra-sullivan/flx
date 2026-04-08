package coordinator

import "time"

type signalDirection uint8

const (
	signalNone signalDirection = iota
	signalUp
	signalDown
)

type stageSignal struct {
	direction signalDirection
	reason    string
}

type stagePolicyState struct {
	upSignalTicks   int
	downSignalTicks int
	lastScaleUpAt   time.Time
	lastScaleDownAt time.Time
}

func (s stagePolicyState) observe(direction signalDirection) stagePolicyState {
	switch direction {
	case signalUp:
		s.upSignalTicks++
		s.downSignalTicks = 0
	case signalDown:
		s.downSignalTicks++
		s.upSignalTicks = 0
	default:
		s.upSignalTicks = 0
		s.downSignalTicks = 0
	}

	return s
}

func (s stagePolicyState) recordScale(direction signalDirection, now time.Time) stagePolicyState {
	switch direction {
	case signalUp:
		s.lastScaleUpAt = now
	case signalDown:
		s.lastScaleDownAt = now
	}

	s.upSignalTicks = 0
	s.downSignalTicks = 0
	return s
}
