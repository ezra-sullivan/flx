package resource

import "runtime"

const (
	defaultMemoryWarningRatio  = 0.80
	defaultMemoryCriticalRatio = 0.95
)

// MemoryObserver reports one coarse memory pressure sample from runtime heap
// allocation versus one soft limit.
type MemoryObserver struct {
	softLimit     uint64
	warningRatio  float64
	criticalRatio float64
	readMemStats  func(*runtime.MemStats)
}

// NewMemoryObserver returns one memory observer that classifies pressure from
// runtime heap allocation against softLimitBytes.
func NewMemoryObserver(softLimitBytes uint64) *MemoryObserver {
	return newMemoryObserver(
		softLimitBytes,
		defaultMemoryWarningRatio,
		defaultMemoryCriticalRatio,
		runtime.ReadMemStats,
	)
}

// ObserveResource returns one memory pressure snapshot. A zero soft limit
// disables the sample.
func (o *MemoryObserver) ObserveResource() Snapshot {
	if o == nil || o.softLimit == 0 {
		return Snapshot{}
	}

	readMemStats := o.readMemStats
	if readMemStats == nil {
		readMemStats = runtime.ReadMemStats
	}

	var stats runtime.MemStats
	readMemStats(&stats)

	usage := float64(stats.Alloc)
	limit := float64(o.softLimit)
	ratio := 0.0
	if limit > 0 {
		ratio = usage / limit
	}

	level := PressureLevelOK
	switch {
	case ratio >= o.criticalRatio:
		level = PressureLevelCritical
	case ratio >= o.warningRatio:
		level = PressureLevelWarning
	}

	return Snapshot{
		Samples: []PressureSample{{
			Name:  "memory",
			Level: level,
			Value: usage,
			Limit: limit,
		}},
	}
}

func newMemoryObserver(softLimitBytes uint64, warningRatio, criticalRatio float64, readMemStats func(*runtime.MemStats)) *MemoryObserver {
	warningRatio = normalizeRatio(warningRatio, defaultMemoryWarningRatio)
	criticalRatio = normalizeRatio(criticalRatio, defaultMemoryCriticalRatio)
	if criticalRatio < warningRatio {
		criticalRatio = warningRatio
	}

	return &MemoryObserver{
		softLimit:     softLimitBytes,
		warningRatio:  warningRatio,
		criticalRatio: criticalRatio,
		readMemStats:  readMemStats,
	}
}

func normalizeRatio(value, fallback float64) float64 {
	if value <= 0 || value > 1 {
		return fallback
	}

	return value
}
