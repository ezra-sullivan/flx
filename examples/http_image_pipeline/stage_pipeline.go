package main

import (
	"context"
	"net/http"

	"github.com/ezra-sullivan/flx"
)

// RunStagePipeline assembles the same demo as RunNativePipeline, but expresses
// it as a sequence of explicit flx.Stage calls. This version is easier to read
// when the goal is to understand pipeline shape and stage boundaries rather
// than the mechanics of each lower-level flx combinator.
func RunStagePipeline(
	ctx context.Context,
	sourceHTTPClient *http.Client,
	targetHTTPClient *http.Client,
	cfg PipelineConfig,
	resizeController *flx.ConcurrencyController,
) error {
	// This version keeps the orchestration focused on stage order. Small mapper
	// helpers capture each stage contract while the pipeline still shows the
	// Stage API directly at the call site.
	listedImages := ListRemoteImages(ctx, sourceHTTPClient, cfg)

	images := flx.Stage(
		ctx,
		listedImages,
		downloadImageStage(sourceHTTPClient, cfg),
		flx.WithWorkers(cfg.DownloadWorkers),
	).Through(
		ctx,
		// Save downloaded originals so the example leaves behind artifacts that
		// make later transforms easy to inspect visually.
		saveLocalImageStage(cfg.LocalOutputDir, "original"),
		flx.WithWorkers(2),
	).Through(
		ctx,
		transformImageStage(ResizeTo(cfg.ResizeWidth, cfg.ResizeHeight)),
		flx.WithForcedDynamicWorkers(resizeController),
	).Through(
		ctx,
		transformImageStage(AddWatermarkText(cfg.WatermarkText)),
		flx.WithWorkers(cfg.WatermarkWorkers),
	).Through(
		ctx,
		// Save processed files after all local transforms finish so the output
		// directory contains both untouched originals and the final result.
		saveLocalImageStage(cfg.LocalOutputDir, "processed"),
		flx.WithWorkers(2),
	)

	// No upload endpoint configured: stop after local processing and summarize
	// item outcomes without adding a final network stage.
	if cfg.UploadEndpoint == "" {
		return consumeProcessedImages(images)
	}

	results := flx.Stage(
		ctx,
		images,
		uploadImageStage(targetHTTPClient, cfg.UploadEndpoint),
		flx.WithWorkers(cfg.UploadWorkers),
	)

	return consumeUploadedImages(results)
}
