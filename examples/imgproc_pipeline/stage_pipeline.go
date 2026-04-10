package main

import (
	"context"
	"net/http"

	"github.com/ezra-sullivan/flx"
	imgproc "github.com/ezra-sullivan/flx/examples/internal/imgproc"
	"github.com/ezra-sullivan/flx/pipeline/control"
)

// RunStagePipeline assembles the same demo as RunNativePipeline, but expresses
// it as a sequence of explicit flx.Stage calls. This is the default example
// path because it keeps pipeline shape and stage boundaries visible without the
// extra coordinator-specific support code.
func RunStagePipeline(
	ctx context.Context,
	sourceHTTPClient *http.Client,
	targetHTTPClient *http.Client,
	cfg imgproc.PipelineConfig,
	resizeController *control.ConcurrencyController,
) error {
	// This version keeps the orchestration focused on stage order. Small mapper
	// helpers capture each stage contract while the pipeline still shows the
	// Stage API directly at the call site.
	listedImages := imgproc.ListExampleImages(ctx, sourceHTTPClient, cfg)

	// The chain is intentionally linear and named by behavior:
	// 1. download bytes
	// 2. save originals
	// 3. resize with dynamic workers
	// 4. watermark
	// 5. save processed output
	images := flx.Stage(
		ctx,
		listedImages,
		imgproc.DownloadImageStage(sourceHTTPClient, cfg),
		control.WithWorkers(cfg.DownloadWorkers),
	).Through(
		ctx,
		// Save downloaded originals so the example leaves behind artifacts that
		// make later transforms easy to inspect visually.
		imgproc.SaveLocalImageStage(cfg.LocalOutputDir, "original"),
		control.WithWorkers(2),
	).Through(
		ctx,
		imgproc.TransformImageStage(imgproc.ResizeTo(cfg.ResizeWidth, cfg.ResizeHeight)),
		control.WithForcedDynamicWorkers(resizeController),
	).Through(
		ctx,
		imgproc.TransformImageStage(imgproc.AddWatermarkText(cfg.WatermarkText)),
		control.WithWorkers(cfg.WatermarkWorkers),
	).Through(
		ctx,
		// Save processed files after all local transforms finish so the output
		// directory contains both untouched originals and the final result.
		imgproc.SaveLocalImageStage(cfg.LocalOutputDir, "processed"),
		control.WithWorkers(2),
	)

	// No upload endpoint configured: stop after local processing and summarize
	// item outcomes without adding a final network stage.
	if cfg.UploadEndpoint == "" {
		return imgproc.ConsumeProcessedImages(images)
	}

	// Upload remains a separate terminal branch so the default example is still
	// easy to read when the main focus is the local processing pipeline itself.
	results := flx.Stage(
		ctx,
		images,
		imgproc.UploadImageStage(targetHTTPClient, cfg.UploadEndpoint),
		control.WithWorkers(cfg.UploadWorkers),
	)

	return imgproc.ConsumeUploadedImages(results)
}
