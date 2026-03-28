package main

import (
	"context"
	"net/http"

	"github.com/ezra-sullivan/flx"
)

// RunStagePipeline assembles the same demo as RunNativePipeline, but expresses
// it as a sequence of named business stages. This version is easier to read
// when the goal is to understand pipeline shape and stage boundaries rather
// than the mechanics of each flx combinator.
func RunStagePipeline(
	ctx context.Context,
	sourceHTTPClient *http.Client,
	targetHTTPClient *http.Client,
	cfg PipelineConfig,
	resizeController *flx.ConcurrencyController,
) error {
	// This version reads like business pipeline code: each helper captures one
	// stage contract, which keeps the orchestration focused on stage order.
	listedImages := ListRemoteImages(ctx, sourceHTTPClient, cfg)

	images := DownloadImages(
		ctx,
		listedImages,
		sourceHTTPClient,
		cfg,
		flx.WithWorkers(cfg.DownloadWorkers),
	)

	// Save downloaded originals so the example leaves behind artifacts that make
	// later transforms easy to inspect visually.
	images = SaveLocalImages(
		ctx,
		images,
		cfg.LocalOutputDir,
		"original",
		flx.WithWorkers(2),
	)

	images = TransformImages(
		ctx,
		images,
		ResizeTo(cfg.ResizeWidth, cfg.ResizeHeight),
		flx.WithForcedDynamicWorkers(resizeController),
	)

	images = TransformImages(
		ctx,
		images,
		AddWatermarkText(cfg.WatermarkText),
		flx.WithWorkers(cfg.WatermarkWorkers),
	)

	// Save processed files after all local transforms finish so the output
	// directory contains both untouched originals and the final result.
	images = SaveLocalImages(
		ctx,
		images,
		cfg.LocalOutputDir,
		"processed",
		flx.WithWorkers(2),
	)

	// No upload endpoint configured: stop after local processing and summarize
	// item outcomes without adding a final network stage.
	if cfg.UploadEndpoint == "" {
		return consumeProcessedImages(images)
	}

	results := UploadImages(
		ctx,
		images,
		targetHTTPClient,
		cfg.UploadEndpoint,
		flx.WithWorkers(cfg.UploadWorkers),
	)

	return consumeUploadedImages(results)
}
