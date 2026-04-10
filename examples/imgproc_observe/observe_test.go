package main

import (
	"testing"

	"github.com/ezra-sullivan/flx/pipeline/observe"
)

func TestFormatObserveSnapshot(t *testing.T) {
	snapshot := observe.PipelineSnapshot{
		Stages: []observe.StageMetrics{{
			StageName:     "resize",
			InputCount:    6,
			OutputCount:   4,
			ActiveWorkers: 2,
			IdleWorkers:   1,
			Backlog:       2,
		}},
		Links: []observe.LinkMetrics{{
			FromStage:     "resize",
			ToStage:       "watermark",
			SentCount:     4,
			ReceivedCount: 3,
			Backlog:       1,
			Capacity:      4,
		}},
	}

	got := formatObserveSnapshot("tick", snapshot)
	want := "kind=observe_snapshot snapshot=tick stages=[resize{io=6/4 backlog=2 workers=3(2 active/1 idle)}] links=[resize->watermark{io=4/3 backlog=1/4}]"
	if got != want {
		t.Fatalf("unexpected snapshot format:\nwant: %s\ngot:  %s", want, got)
	}
}

func TestObserveRecorderSnapshotSortsAndKeepsLatestValues(t *testing.T) {
	recorder := newObserveRecorder()
	recorder.ObserveStage(observe.StageMetrics{StageName: "watermark", InputCount: 5, OutputCount: 5})
	recorder.ObserveStage(observe.StageMetrics{StageName: "resize", InputCount: 6, OutputCount: 4, Backlog: 2})
	recorder.ObserveStage(observe.StageMetrics{StageName: "resize", InputCount: 7, OutputCount: 5, Backlog: 2})

	recorder.ObserveLink(observe.LinkMetrics{FromStage: "watermark", SentCount: 5, ReceivedCount: 5, Capacity: 2})
	recorder.ObserveLink(observe.LinkMetrics{FromStage: "resize", ToStage: "watermark", SentCount: 5, ReceivedCount: 4, Backlog: 1, Capacity: 4})

	snapshot := recorder.Snapshot()
	if len(snapshot.Stages) != 2 {
		t.Fatalf("expected two stages, got %d", len(snapshot.Stages))
	}
	if snapshot.Stages[0].StageName != "resize" || snapshot.Stages[0].InputCount != 7 {
		t.Fatalf("unexpected stage snapshot: %#v", snapshot.Stages)
	}
	if len(snapshot.Links) != 2 {
		t.Fatalf("expected two links, got %d", len(snapshot.Links))
	}
	if snapshot.Links[1].ToStage != "" {
		t.Fatalf("expected terminal link to keep empty downstream name, got %#v", snapshot.Links[1])
	}
}
