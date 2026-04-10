package main

import (
	"cmp"
	"fmt"
	"log"
	"strings"

	"github.com/ezra-sullivan/flx/pipeline/observe"
)

const logKindObserveSnapshot = "observe_snapshot"

func logObserveSnapshot(label string, snapshot observe.PipelineSnapshot) {
	log.Print(formatObserveSnapshot(label, snapshot))
}

func formatObserveSnapshot(label string, snapshot observe.PipelineSnapshot) string {
	return fmt.Sprintf(
		"kind=%s snapshot=%s stages=[%s] links=[%s]",
		logKindObserveSnapshot,
		label,
		formatStageMetrics(snapshot.Stages),
		formatLinkMetrics(snapshot.Links),
	)
}

func formatStageMetrics(stages []observe.StageMetrics) string {
	if len(stages) == 0 {
		return "-"
	}

	parts := make([]string, 0, len(stages))
	for _, stage := range stages {
		workers := stage.ActiveWorkers + stage.IdleWorkers
		parts = append(parts, fmt.Sprintf(
			"%s{io=%d/%d backlog=%d workers=%d(%d active/%d idle)}",
			stage.StageName,
			stage.InputCount,
			stage.OutputCount,
			stage.Backlog,
			workers,
			stage.ActiveWorkers,
			stage.IdleWorkers,
		))
	}

	return strings.Join(parts, " | ")
}

func formatLinkMetrics(links []observe.LinkMetrics) string {
	if len(links) == 0 {
		return "-"
	}

	parts := make([]string, 0, len(links))
	for _, link := range links {
		target := cmp.Or(link.ToStage, "terminal")
		parts = append(parts, fmt.Sprintf(
			"%s->%s{io=%d/%d backlog=%d/%d}",
			link.FromStage,
			target,
			link.SentCount,
			link.ReceivedCount,
			link.Backlog,
			link.Capacity,
		))
	}

	return strings.Join(parts, " | ")
}
