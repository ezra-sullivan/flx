package resource

import (
	"runtime"
	"testing"
)

func TestObserveAllMergesSamplesAndIgnoresNil(t *testing.T) {
	snapshot := ObserveAll([]Observer{
		nil,
		ObserverFunc(func() Snapshot {
			return Snapshot{
				Samples: []PressureSample{{
					Name:  "memory",
					Level: PressureLevelWarning,
				}},
			}
		}),
		ObserverFunc(func() Snapshot {
			return Snapshot{
				Samples: []PressureSample{{
					Name:  "network",
					Level: PressureLevelCritical,
				}},
			}
		}),
	})

	if len(snapshot.Samples) != 2 {
		t.Fatalf("expected two resource samples, got %#v", snapshot.Samples)
	}
	if level := MaxPressureLevel(snapshot); level != PressureLevelCritical {
		t.Fatalf("unexpected max pressure level: %v", level)
	}
}

func TestObserveAllRecoversObserverPanic(t *testing.T) {
	snapshot := ObserveAll([]Observer{
		ObserverFunc(func() Snapshot {
			panic("boom")
		}),
	})

	if len(snapshot.Samples) != 0 {
		t.Fatalf("expected panic observer to be dropped, got %#v", snapshot.Samples)
	}
}

func TestMemoryObserverMapsUsageToPressureLevels(t *testing.T) {
	tests := []struct {
		name    string
		alloc   uint64
		want    PressureLevel
		softCap uint64
	}{
		{name: "ok", alloc: 60, softCap: 100, want: PressureLevelOK},
		{name: "warning", alloc: 85, softCap: 100, want: PressureLevelWarning},
		{name: "critical", alloc: 96, softCap: 100, want: PressureLevelCritical},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			observer := newMemoryObserver(tt.softCap, 0.8, 0.95, func(stats *runtime.MemStats) {
				stats.Alloc = tt.alloc
			})

			snapshot := observer.ObserveResource()
			if len(snapshot.Samples) != 1 {
				t.Fatalf("expected one sample, got %#v", snapshot.Samples)
			}
			if got := snapshot.Samples[0].Level; got != tt.want {
				t.Fatalf("unexpected pressure level: got %v want %v", got, tt.want)
			}
		})
	}
}
