package observe

import (
	"testing"

	"github.com/ezra-sullivan/flx/internal/config"
	"github.com/ezra-sullivan/flx/internal/link"
	"github.com/ezra-sullivan/flx/internal/metrics"
)

func TestWithStageMetricsObserverConfiguresCallback(t *testing.T) {
	var got StageMetrics
	observer := func(snapshot StageMetrics) {
		got = snapshot
	}

	opts := config.BuildOptions(WithStageMetricsObserver(observer))
	hook := opts.StageMetricsObserver()
	if hook == nil {
		t.Fatalf("expected stage metrics observer to be configured")
	}

	hook(metrics.StageMetrics{
		StageName:     "stage-1",
		InputCount:    10,
		OutputCount:   9,
		ErrorCount:    1,
		ActiveWorkers: 3,
		IdleWorkers:   1,
		Backlog:       5,
	})

	want := StageMetrics{
		StageName:     "stage-1",
		InputCount:    10,
		OutputCount:   9,
		ErrorCount:    1,
		ActiveWorkers: 3,
		IdleWorkers:   1,
		Backlog:       5,
	}
	if got != want {
		t.Fatalf("unexpected snapshot: got %#v want %#v", got, want)
	}
}

func TestWithStageMetricsObserverAllowsNil(t *testing.T) {
	opts := config.BuildOptions(WithStageMetricsObserver(nil))
	if opts.StageMetricsObserver() != nil {
		t.Fatalf("expected nil stage metrics observer")
	}
}

func TestWithLinkMetricsObserverConfiguresCallback(t *testing.T) {
	var got LinkMetrics
	observer := func(snapshot LinkMetrics) {
		got = snapshot
	}

	opts := config.BuildOptions(WithLinkMetricsObserver(observer))
	hook := opts.LinkMetricsObserver()
	if hook == nil {
		t.Fatalf("expected link metrics observer to be configured")
	}

	hook(link.Metrics{
		FromStage:     "download",
		ToStage:       "resize",
		SentCount:     10,
		ReceivedCount: 8,
		Backlog:       2,
		Capacity:      16,
	})

	want := LinkMetrics{
		FromStage:     "download",
		ToStage:       "resize",
		SentCount:     10,
		ReceivedCount: 8,
		Backlog:       2,
		Capacity:      16,
	}
	if got != want {
		t.Fatalf("unexpected snapshot: got %#v want %#v", got, want)
	}
}

func TestWithLinkMetricsObserverAllowsNil(t *testing.T) {
	opts := config.BuildOptions(WithLinkMetricsObserver(nil))
	if opts.LinkMetricsObserver() != nil {
		t.Fatalf("expected nil link metrics observer")
	}
}

func TestResourceObserverFuncCallsFunction(t *testing.T) {
	observer := ResourceObserverFunc(func() ResourceSnapshot {
		return ResourceSnapshot{
			Samples: []PressureSample{{
				Name:  "memory",
				Level: PressureLevelWarning,
				Value: 80,
				Limit: 100,
			}},
		}
	})

	got := observer.ObserveResource()
	if len(got.Samples) != 1 {
		t.Fatalf("expected one resource sample, got %#v", got.Samples)
	}
	if sample := got.Samples[0]; sample.Name != "memory" || sample.Level != PressureLevelWarning {
		t.Fatalf("unexpected resource sample: %#v", sample)
	}
}
