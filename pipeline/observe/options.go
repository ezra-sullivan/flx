package observe

import (
	"github.com/ezra-sullivan/flx/internal/config"
	"github.com/ezra-sullivan/flx/internal/link"
	"github.com/ezra-sullivan/flx/internal/metrics"
	"github.com/ezra-sullivan/flx/pipeline/control"
)

// WithStageMetricsObserver registers observer to receive stage snapshots.
func WithStageMetricsObserver(observer StageMetricsObserver) control.Option {
	if observer == nil {
		return config.WithStageMetricsObserver(nil)
	}

	return config.WithStageMetricsObserver(func(snapshot metrics.StageMetrics) {
		observer(StageMetrics{
			StageName:     snapshot.StageName,
			InputCount:    snapshot.InputCount,
			OutputCount:   snapshot.OutputCount,
			ErrorCount:    snapshot.ErrorCount,
			ActiveWorkers: snapshot.ActiveWorkers,
			IdleWorkers:   snapshot.IdleWorkers,
			Backlog:       snapshot.Backlog,
		})
	})
}

// WithLinkMetricsObserver registers observer to receive outbound link
// snapshots.
func WithLinkMetricsObserver(observer LinkMetricsObserver) control.Option {
	if observer == nil {
		return config.WithLinkMetricsObserver(nil)
	}

	return config.WithLinkMetricsObserver(func(snapshot link.Metrics) {
		observer(LinkMetrics{
			FromStage:     snapshot.FromStage,
			ToStage:       snapshot.ToStage,
			SentCount:     snapshot.SentCount,
			ReceivedCount: snapshot.ReceivedCount,
			Backlog:       snapshot.Backlog,
			Capacity:      snapshot.Capacity,
		})
	})
}
