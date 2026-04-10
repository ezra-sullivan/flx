package main

import (
	"context"
	"net/http"

	"github.com/ezra-sullivan/flx"
	imgproc "github.com/ezra-sullivan/flx/examples/internal/imgproc"
	"github.com/ezra-sullivan/flx/pipeline/control"
	pipelinecoordinator "github.com/ezra-sullivan/flx/pipeline/coordinator"
)

// RunCoordinatorPipeline demonstrates one coordinator-managed image-processing
// pipeline. Only the resize stage is dynamically adjusted.
func RunCoordinatorPipeline(
	ctx context.Context,
	sourceHTTPClient *http.Client,
	targetHTTPClient *http.Client,
	cfg imgproc.PipelineConfig,
	resizeController *control.ConcurrencyController,
) error {
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	pipelineCoordinator := newExamplePipelineCoordinator()
	loopDone := startCoordinatorLoop(runCtx, pipelineCoordinator)

	images := flx.Stage(
		runCtx,
		imgproc.ListExampleImages(runCtx, sourceHTTPClient, cfg),
		imgproc.DownloadImageStage(sourceHTTPClient, cfg),
		coordinatorStageOptions(pipelineCoordinator, imgproc.StageNameDownload, control.WithWorkers(cfg.DownloadWorkers))...,
	).Through(
		runCtx,
		imgproc.SaveLocalImageStage(cfg.LocalOutputDir, "original"),
		coordinatorStageOptions(pipelineCoordinator, imgproc.StageNameSaveOriginal, control.WithWorkers(2))...,
	).Through(
		runCtx,
		imgproc.TransformImageStage(imgproc.ResizeTo(cfg.ResizeWidth, cfg.ResizeHeight)),
		coordinatorStageOptions(
			pipelineCoordinator,
			imgproc.StageNameResize,
			control.WithForcedDynamicWorkers(resizeController),
			pipelinecoordinator.WithStageBudget(pipelinecoordinator.StageBudget{
				MinWorkers: 1,
				MaxWorkers: 6,
			}),
		)...,
	).Through(
		runCtx,
		imgproc.TransformImageStage(imgproc.AddWatermarkText(cfg.WatermarkText)),
		coordinatorStageOptions(pipelineCoordinator, imgproc.StageNameWatermark, control.WithWorkers(cfg.WatermarkWorkers))...,
	).Through(
		runCtx,
		imgproc.SaveLocalImageStage(cfg.LocalOutputDir, "processed"),
		coordinatorStageOptions(pipelineCoordinator, imgproc.StageNameSaveProcessed, control.WithWorkers(2))...,
	)

	var err error
	if cfg.UploadEndpoint == "" {
		err = imgproc.ConsumeProcessedImages(images)
	} else {
		results := flx.Stage(
			runCtx,
			images,
			imgproc.UploadImageStage(targetHTTPClient, cfg.UploadEndpoint),
			coordinatorStageOptions(pipelineCoordinator, imgproc.StageNameUpload, control.WithWorkers(cfg.UploadWorkers))...,
		)
		err = imgproc.ConsumeUploadedImages(results)
	}

	cancel()
	<-loopDone
	logCoordinatorSnapshot("final", pipelineCoordinator.Snapshot())
	return err
}

func coordinatorStageOptions(
	pipelineCoordinator *pipelinecoordinator.PipelineCoordinator,
	stageName string,
	opts ...control.Option,
) []control.Option {
	stageOpts := make([]control.Option, 0, len(opts)+2)
	stageOpts = append(stageOpts,
		pipelinecoordinator.WithCoordinator(pipelineCoordinator),
		pipelinecoordinator.WithStageName(stageName),
	)
	stageOpts = append(stageOpts, opts...)
	return stageOpts
}
