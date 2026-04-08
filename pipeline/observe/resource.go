package observe

// PressureLevel summarizes how strongly one resource constrains worker growth.
type PressureLevel uint8

const (
	// PressureLevelOK reports that the resource does not currently constrain
	// worker growth.
	PressureLevelOK PressureLevel = iota
	// PressureLevelWarning reports that the resource should brake further scale
	// up.
	PressureLevelWarning
	// PressureLevelCritical reports that the resource should bias the pipeline
	// toward shrink.
	PressureLevelCritical
)

// String returns one stable text form for the pressure level.
func (l PressureLevel) String() string {
	switch l {
	case PressureLevelOK:
		return "ok"
	case PressureLevelWarning:
		return "warning"
	case PressureLevelCritical:
		return "critical"
	default:
		return "unknown"
	}
}

// PressureSample captures one resource pressure sample.
type PressureSample struct {
	Name  string
	Level PressureLevel
	Value float64
	Limit float64
}

// ResourceSnapshot captures one batch of resource pressure samples.
type ResourceSnapshot struct {
	Samples []PressureSample
}

// ResourceObserver reports the latest resource pressure snapshot.
type ResourceObserver interface {
	ObserveResource() ResourceSnapshot
}

// ResourceObserverFunc adapts one function into a ResourceObserver.
type ResourceObserverFunc func() ResourceSnapshot

// ObserveResource returns the latest resource pressure snapshot.
func (f ResourceObserverFunc) ObserveResource() ResourceSnapshot {
	if f == nil {
		return ResourceSnapshot{}
	}

	return f()
}
