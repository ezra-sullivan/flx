package main

import (
	"cmp"
	"fmt"
	"log"
	"strings"

	"github.com/ezra-sullivan/flx/pipeline/observe"
)

const (
	logKindCoordinatorSnapshot = "coordinator_snapshot"
	logKindCoordinatorDecision = "coordinator_decision"
)

func logCoordinatorSnapshot(label string, snapshot observe.PipelineSnapshot) {
	log.Print(formatCoordinatorSnapshot(label, snapshot))
}

func logCoordinatorDecision(decision observe.PipelineDecision) {
	message := formatCoordinatorDecision(decision)
	if message == "" {
		return
	}

	log.Print(message)
}

func formatCoordinatorSnapshot(label string, snapshot observe.PipelineSnapshot) string {
	return fmt.Sprintf(
		"kind=%s snapshot=%s stages=[%s] links=[%s] resources=[%s]",
		logKindCoordinatorSnapshot,
		label,
		formatStageMetrics(snapshot.Stages),
		formatLinkMetrics(snapshot.Links),
		formatResourceSnapshot(snapshot.Resources),
	)
}

func formatCoordinatorDecision(decision observe.PipelineDecision) string {
	if len(decision.Stages) == 0 {
		return ""
	}

	parts := make([]string, 0, len(decision.Stages))
	for _, stage := range decision.Stages {
		parts = append(parts, fmt.Sprintf(
			"%s{workers=%d->%d reason=%s backlog=%d/%d}",
			stage.StageName,
			stage.FromWorkers,
			stage.ToWorkers,
			stage.Reason,
			stage.StageBacklog,
			stage.LinkBacklog,
		))
	}

	return fmt.Sprintf("kind=%s decision=[%s]", logKindCoordinatorDecision, strings.Join(parts, " | "))
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

func formatResourceSnapshot(snapshot observe.ResourceSnapshot) string {
	if len(snapshot.Samples) == 0 {
		return "-"
	}

	parts := make([]string, 0, len(snapshot.Samples))
	for _, sample := range snapshot.Samples {
		parts = append(parts, formatPressureSample(sample))
	}

	return strings.Join(parts, " | ")
}

func formatPressureSample(sample observe.PressureSample) string {
	if sample.Limit > 0 {
		return fmt.Sprintf(
			"%s{%s %s/%s}",
			sample.Name,
			sample.Level.String(),
			formatMiB(sample.Value),
			formatMiB(sample.Limit),
		)
	}

	return fmt.Sprintf(
		"%s{%s %s}",
		sample.Name,
		sample.Level.String(),
		formatMiB(sample.Value),
	)
}

func formatMiB(value float64) string {
	return fmt.Sprintf("%.1fMiB", value/1024/1024)
}
