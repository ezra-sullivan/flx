package resource

// PressureLevel summarizes how strongly one resource constrains worker growth.
type PressureLevel uint8

const (
	PressureLevelOK PressureLevel = iota
	PressureLevelWarning
	PressureLevelCritical
)

// PressureSample captures one resource pressure sample.
type PressureSample struct {
	Name  string
	Level PressureLevel
	Value float64
	Limit float64
}

// Snapshot captures one batch of resource pressure samples.
type Snapshot struct {
	Samples []PressureSample
}

// Observer reports the latest resource pressure snapshot.
type Observer interface {
	ObserveResource() Snapshot
}

// ObserverFunc adapts one function into an Observer.
type ObserverFunc func() Snapshot

// ObserveResource returns the latest resource pressure snapshot.
func (f ObserverFunc) ObserveResource() Snapshot {
	if f == nil {
		return Snapshot{}
	}

	return f()
}

// ObserveAll merges the latest samples from all configured observers.
func ObserveAll(observers []Observer) Snapshot {
	if len(observers) == 0 {
		return Snapshot{}
	}

	samples := make([]PressureSample, 0, len(observers))
	for _, observer := range observers {
		if observer == nil {
			continue
		}

		snapshot := observeOne(observer)
		if len(snapshot.Samples) == 0 {
			continue
		}

		samples = append(samples, snapshot.Samples...)
	}

	return Snapshot{Samples: samples}
}

// MaxPressureLevel returns the highest pressure level in snapshot.
func MaxPressureLevel(snapshot Snapshot) PressureLevel {
	level := PressureLevelOK
	for _, sample := range snapshot.Samples {
		if sample.Level > level {
			level = sample.Level
		}
	}

	return level
}

func observeOne(observer Observer) (snapshot Snapshot) {
	defer func() {
		if recover() != nil {
			snapshot = Snapshot{}
		}
	}()

	snapshot = observer.ObserveResource()
	if len(snapshot.Samples) == 0 {
		return Snapshot{}
	}

	samples := make([]PressureSample, 0, len(snapshot.Samples))
	samples = append(samples, snapshot.Samples...)
	return Snapshot{Samples: samples}
}
