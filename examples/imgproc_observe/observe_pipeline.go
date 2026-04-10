package main

import (
	"context"
	"net/http"
	"time"

	"github.com/ezra-sullivan/flx"
	imgproc "github.com/ezra-sullivan/flx/examples/internal/imgproc"
	"github.com/ezra-sullivan/flx/pipeline/control"
	pipelinecoordinator "github.com/ezra-sullivan/flx/pipeline/coordinator"
	"github.com/ezra-sullivan/flx/pipeline/observe"
)

const observeLogInterval = time.Second

// RunObservePipeline demonstrates observe-only integration on the real image
// download pipeline, without any PipelineCoordinator or Tick loop.
func RunObservePipeline(
	ctx context.Context,
	sourceHTTPClient *http.Client,
	targetHTTPClient *http.Client,
	cfg imgproc.PipelineConfig,
	resizeController *control.ConcurrencyController,
) error {
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	recorder := newObserveRecorder()
	loopDone := startObserveLoop(runCtx, recorder, observeLogInterval)

	images := flx.Stage(
		runCtx,
		imgproc.ListExampleImages(runCtx, sourceHTTPClient, cfg),
		imgproc.DownloadImageStage(sourceHTTPClient, cfg),
		observeStageOptions(recorder, imgproc.StageNameDownload, control.WithWorkers(cfg.DownloadWorkers))...,
	).Through(
		runCtx,
		imgproc.SaveLocalImageStage(cfg.LocalOutputDir, "original"),
		observeStageOptions(recorder, imgproc.StageNameSaveOriginal, control.WithWorkers(2))...,
	).Through(
		runCtx,
		imgproc.TransformImageStage(imgproc.ResizeTo(cfg.ResizeWidth, cfg.ResizeHeight)),
		observeStageOptions(recorder, imgproc.StageNameResize, control.WithForcedDynamicWorkers(resizeController))...,
	).Through(
		runCtx,
		imgproc.TransformImageStage(imgproc.AddWatermarkText(cfg.WatermarkText)),
		observeStageOptions(recorder, imgproc.StageNameWatermark, control.WithWorkers(cfg.WatermarkWorkers))...,
	).Through(
		runCtx,
		imgproc.SaveLocalImageStage(cfg.LocalOutputDir, "processed"),
		observeStageOptions(recorder, imgproc.StageNameSaveProcessed, control.WithWorkers(2))...,
	)

	var err error
	if cfg.UploadEndpoint == "" {
		err = imgproc.ConsumeProcessedImages(images)
	} else {
		results := flx.Stage(
			runCtx,
			images,
			imgproc.UploadImageStage(targetHTTPClient, cfg.UploadEndpoint),
			observeStageOptions(recorder, imgproc.StageNameUpload, control.WithWorkers(cfg.UploadWorkers))...,
		)
		err = imgproc.ConsumeUploadedImages(results)
	}

	cancel()
	<-loopDone
	logObserveSnapshot("final", recorder.Snapshot())
	return err
}

func observeStageOptions(
	recorder *observeRecorder,
	stageName string,
	opts ...control.Option,
) []control.Option {
	stageOpts := make([]control.Option, 0, len(opts)+3)
	stageOpts = append(stageOpts,
		pipelinecoordinator.WithStageName(stageName),
		observe.WithStageMetricsObserver(recorder.ObserveStage),
		observe.WithLinkMetricsObserver(recorder.ObserveLink),
	)
	stageOpts = append(stageOpts, opts...)
	return stageOpts
}

func startObserveLoop(ctx context.Context, recorder *observeRecorder, interval time.Duration) <-chan struct{} {
	done := make(chan struct{})

	go func() {
		defer close(done)
		if recorder == nil || interval <= 0 {
			return
		}

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				logObserveSnapshot("tick", recorder.Snapshot())
			}
		}
	}()

	return done
}
