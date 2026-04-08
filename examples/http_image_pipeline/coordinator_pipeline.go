package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/ezra-sullivan/flx"
	"github.com/ezra-sullivan/flx/pipeline/control"
	pipelinecoordinator "github.com/ezra-sullivan/flx/pipeline/coordinator"
	"github.com/ezra-sullivan/flx/pipeline/observe"
)

const (
	coordinatorTickInterval    = 250 * time.Millisecond
	exampleMemorySoftLimit     = 256 << 20
	exampleMemoryWarningRatio  = 0.80
	exampleMemoryCriticalRatio = 0.95
)

// RunCoordinatorPipeline runs the Stage-oriented example with one pipeline
// coordinator attached so the demo can expose stage, link, and resource
// snapshots while periodically applying Tick decisions.
func RunCoordinatorPipeline(
	ctx context.Context,
	sourceHTTPClient *http.Client,
	targetHTTPClient *http.Client,
	cfg PipelineConfig,
	resizeController *control.ConcurrencyController,
) error {
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	pipelineCoordinator := pipelinecoordinator.NewPipelineCoordinator(
		pipelinecoordinator.PipelineCoordinatorPolicy{
			ScaleUpStep:         1,
			ScaleDownStep:       1,
			ScaleUpCooldown:     500 * time.Millisecond,
			ScaleDownCooldown:   750 * time.Millisecond,
			ScaleUpHysteresis:   1,
			ScaleDownHysteresis: 2,
		},
		pipelinecoordinator.WithDecisionObserver(logPipelineDecision),
		pipelinecoordinator.WithResourceObserver(newMemoryPressureObserver(exampleMemorySoftLimit)),
	)

	loopDone := make(chan struct{})
	go func() {
		defer close(loopDone)
		runCoordinatorLoop(runCtx, pipelineCoordinator, coordinatorTickInterval)
	}()

	listedImages := ListRemoteImages(runCtx, sourceHTTPClient, cfg)

	images := flx.Stage(
		runCtx,
		listedImages,
		downloadImageStage(sourceHTTPClient, cfg),
		control.WithWorkers(cfg.DownloadWorkers),
		pipelinecoordinator.WithCoordinator(pipelineCoordinator),
		pipelinecoordinator.WithStageName("download"),
	).Through(
		runCtx,
		saveLocalImageStage(cfg.LocalOutputDir, "original"),
		control.WithWorkers(2),
		pipelinecoordinator.WithCoordinator(pipelineCoordinator),
		pipelinecoordinator.WithStageName("save-original"),
	).Through(
		runCtx,
		transformImageStage(ResizeTo(cfg.ResizeWidth, cfg.ResizeHeight)),
		control.WithForcedDynamicWorkers(resizeController),
		pipelinecoordinator.WithCoordinator(pipelineCoordinator),
		pipelinecoordinator.WithStageName("resize"),
		pipelinecoordinator.WithStageBudget(pipelinecoordinator.StageBudget{
			MinWorkers: 2,
			MaxWorkers: 8,
		}),
	).Through(
		runCtx,
		transformImageStage(AddWatermarkText(cfg.WatermarkText)),
		control.WithWorkers(cfg.WatermarkWorkers),
		pipelinecoordinator.WithCoordinator(pipelineCoordinator),
		pipelinecoordinator.WithStageName("watermark"),
	).Through(
		runCtx,
		saveLocalImageStage(cfg.LocalOutputDir, "processed"),
		control.WithWorkers(2),
		pipelinecoordinator.WithCoordinator(pipelineCoordinator),
		pipelinecoordinator.WithStageName("save-processed"),
	)

	var err error
	if strings.TrimSpace(cfg.UploadEndpoint) == "" {
		err = consumeProcessedImages(images)
	} else {
		results := flx.Stage(
			runCtx,
			images,
			uploadImageStage(targetHTTPClient, cfg.UploadEndpoint),
			control.WithWorkers(cfg.UploadWorkers),
			pipelinecoordinator.WithCoordinator(pipelineCoordinator),
			pipelinecoordinator.WithStageName("upload"),
		)

		err = consumeUploadedImages(results)
	}

	cancel()
	<-loopDone
	logPipelineSnapshot("final", pipelineCoordinator.Snapshot())

	return err
}

func runCoordinatorLoop(ctx context.Context, pipelineCoordinator *pipelinecoordinator.PipelineCoordinator, every time.Duration) {
	if pipelineCoordinator == nil {
		return
	}
	if every <= 0 {
		every = coordinatorTickInterval
	}

	ticker := time.NewTicker(every)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			logPipelineSnapshot("tick", pipelineCoordinator.Snapshot())
			pipelineCoordinator.Tick()
		}
	}
}

func logPipelineSnapshot(label string, snapshot observe.PipelineSnapshot) {
	log.Printf(
		"coordinator snapshot label=%s stages=%s links=%s resources=%s",
		label,
		formatStageMetrics(snapshot.Stages),
		formatLinkMetrics(snapshot.Links),
		formatResourceSnapshot(snapshot.Resources),
	)
}

func logPipelineDecision(decision observe.PipelineDecision) {
	for _, stage := range decision.Stages {
		log.Printf(
			"coordinator decision stage=%s workers=%d->%d reason=%s stage_backlog=%d link_backlog=%d",
			stage.StageName,
			stage.FromWorkers,
			stage.ToWorkers,
			stage.Reason,
			stage.StageBacklog,
			stage.LinkBacklog,
		)
	}
}

func newMemoryPressureObserver(softLimitBytes uint64) observe.ResourceObserver {
	return observe.ResourceObserverFunc(func() observe.ResourceSnapshot {
		if softLimitBytes == 0 {
			return observe.ResourceSnapshot{}
		}

		var stats runtime.MemStats
		runtime.ReadMemStats(&stats)

		usage := float64(stats.Alloc)
		limit := float64(softLimitBytes)
		level := observe.PressureLevelOK
		if limit > 0 {
			ratio := usage / limit
			switch {
			case ratio >= exampleMemoryCriticalRatio:
				level = observe.PressureLevelCritical
			case ratio >= exampleMemoryWarningRatio:
				level = observe.PressureLevelWarning
			}
		}

		return observe.ResourceSnapshot{
			Samples: []observe.PressureSample{{
				Name:  "memory",
				Level: level,
				Value: usage,
				Limit: limit,
			}},
		}
	})
}

func formatStageMetrics(stages []observe.StageMetrics) string {
	if len(stages) == 0 {
		return "-"
	}

	parts := make([]string, 0, len(stages))
	for _, stage := range stages {
		parts = append(parts, fmt.Sprintf(
			"%s(in=%d out=%d backlog=%d active=%d idle=%d)",
			stage.StageName,
			stage.InputCount,
			stage.OutputCount,
			stage.Backlog,
			stage.ActiveWorkers,
			stage.IdleWorkers,
		))
	}

	return strings.Join(parts, ", ")
}

func formatLinkMetrics(links []observe.LinkMetrics) string {
	if len(links) == 0 {
		return "-"
	}

	parts := make([]string, 0, len(links))
	for _, link := range links {
		target := link.ToStage
		if target == "" {
			target = "terminal"
		}

		parts = append(parts, fmt.Sprintf(
			"%s->%s(sent=%d recv=%d backlog=%d cap=%d)",
			link.FromStage,
			target,
			link.SentCount,
			link.ReceivedCount,
			link.Backlog,
			link.Capacity,
		))
	}

	return strings.Join(parts, ", ")
}

func formatResourceSnapshot(snapshot observe.ResourceSnapshot) string {
	if len(snapshot.Samples) == 0 {
		return "-"
	}

	parts := make([]string, 0, len(snapshot.Samples))
	for _, sample := range snapshot.Samples {
		parts = append(parts, fmt.Sprintf(
			"%s(%s %.1fMiB/%.1fMiB)",
			sample.Name,
			sample.Level.String(),
			sample.Value/1024/1024,
			sample.Limit/1024/1024,
		))
	}

	return strings.Join(parts, ", ")
}
