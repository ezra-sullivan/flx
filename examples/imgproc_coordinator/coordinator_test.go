package main

import (
	"testing"

	"github.com/ezra-sullivan/flx/pipeline/observe"
)

func TestFormatCoordinatorSnapshot(t *testing.T) {
	snapshot := observe.PipelineSnapshot{
		Stages: []observe.StageMetrics{{
			StageName:     "resize",
			InputCount:    8,
			OutputCount:   5,
			ActiveWorkers: 2,
			IdleWorkers:   1,
			Backlog:       3,
		}},
		Links: []observe.LinkMetrics{{
			FromStage:     "resize",
			ToStage:       "watermark",
			SentCount:     5,
			ReceivedCount: 4,
			Backlog:       1,
			Capacity:      4,
		}},
		Resources: observe.ResourceSnapshot{
			Samples: []observe.PressureSample{{
				Name:  "heap",
				Level: observe.PressureLevelWarning,
				Value: 128 * 1024 * 1024,
				Limit: 256 * 1024 * 1024,
			}},
		},
	}

	got := formatCoordinatorSnapshot("tick", snapshot)
	want := "kind=coordinator_snapshot snapshot=tick stages=[resize{io=8/5 backlog=3 workers=3(2 active/1 idle)}] links=[resize->watermark{io=5/4 backlog=1/4}] resources=[heap{warning 128.0MiB/256.0MiB}]"
	if got != want {
		t.Fatalf("unexpected snapshot format:\nwant: %s\ngot:  %s", want, got)
	}
}

func TestFormatCoordinatorDecision(t *testing.T) {
	decision := observe.PipelineDecision{
		Stages: []observe.StageDecision{{
			StageName:    "resize",
			FromWorkers:  1,
			ToWorkers:    2,
			Reason:       "incoming_link_backlog",
			StageBacklog: 4,
			LinkBacklog:  3,
		}},
	}

	got := formatCoordinatorDecision(decision)
	want := "kind=coordinator_decision decision=[resize{workers=1->2 reason=incoming_link_backlog backlog=4/3}]"
	if got != want {
		t.Fatalf("unexpected decision format:\nwant: %s\ngot:  %s", want, got)
	}
}
