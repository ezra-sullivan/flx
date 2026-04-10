package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/ezra-sullivan/flx"
	imgproc "github.com/ezra-sullivan/flx/examples/internal/imgproc"
	"github.com/ezra-sullivan/flx/pipeline/control"
)

// RunNativePipeline builds the example directly out of flx primitives instead
// of delegating each business step to a named helper. It is useful as a
// side-by-side comparison for readers who want to see the full stream assembly
// inline, including how item errors are preserved across stages.
func RunNativePipeline(
	ctx context.Context,
	sourceHTTPClient *http.Client,
	targetHTTPClient *http.Client,
	cfg imgproc.PipelineConfig,
	resizeController *control.ConcurrencyController,
) error {
	// Native version: build the source stage inline with flx.From so the reader
	// can inspect the exact local-catalog listing logic without additional
	// indirection.
	listedImages := flx.From(func(out chan<- imgproc.RemoteImage) {
		pageSize := cfg.ListPageSize
		if pageSize <= 0 {
			pageSize = 10
		}

		downloadWidth := cfg.DownloadWidth
		if downloadWidth <= 0 {
			downloadWidth = 320
		}

		downloadHeight := cfg.DownloadHeight
		if downloadHeight <= 0 {
			downloadHeight = 240
		}

		for page := 1; ; page++ {
			if cfg.ListMaxPages > 0 && page > cfg.ListMaxPages {
				return
			}

			photos, err := imgproc.FetchImageCatalogPage(ctx, sourceHTTPClient, cfg.ListEndpoint, page, pageSize)
			if err != nil {
				panic(fmt.Errorf("list images page=%d: %w", page, err))
			}

			if len(photos) == 0 {
				return
			}

			for _, photo := range photos {
				image := imgproc.RemoteImage{
					ID:        photo.ID,
					SourceURL: imgproc.BuildImageDownloadURL(photo.ID, downloadWidth, downloadHeight),
				}
				if !flx.SendContext(ctx, out, image) {
					return
				}
			}

			if len(photos) < pageSize {
				return
			}
		}
	})

	// Keep the local-save stages explicit in this version, so compute the guard
	// once and let each save stage focus on persistence rather than config checks.
	shouldSaveLocal := strings.TrimSpace(cfg.LocalOutputDir) != ""

	// Stage 1 downloads bytes and converts item-scoped transport failures into
	// item-level errors so later stages can continue processing unaffected
	// images.
	images := flx.MapContext(
		ctx,
		listedImages,
		func(ctx context.Context, image imgproc.RemoteImage) imgproc.ImageResult[[]byte] {
			imageBytes, err := imgproc.DownloadImageWithRetry(ctx, sourceHTTPClient, image, cfg)
			if err != nil {
				return imgproc.ImageResult[[]byte]{
					ImageID:   image.ID,
					SourceURL: image.SourceURL,
					Err:       fmt.Errorf("download %s: %w", image.SourceURL, err),
				}
			}

			return imgproc.ImageResult[[]byte]{
				ImageID:   image.ID,
				SourceURL: image.SourceURL,
				Value:     imageBytes,
			}
		},
		control.WithWorkers(cfg.DownloadWorkers),
	)

	// Stage 2 persists the unmodified originals so the example can leave behind
	// a clear before/after comparison for the later transform stages.
	images = flx.MapContext(
		ctx,
		images,
		func(ctx context.Context, image imgproc.ImageResult[[]byte]) imgproc.ImageResult[[]byte] {
			if image.Err != nil || !shouldSaveLocal {
				return image
			}

			if err := imgproc.SaveImageBytes(cfg.LocalOutputDir, "original", image); err != nil {
				return imgproc.ImageResult[[]byte]{
					ImageID:   image.ImageID,
					SourceURL: image.SourceURL,
					Err:       fmt.Errorf("save original image %s: %w", image.SourceURL, err),
				}
			}

			return image
		},
		control.WithWorkers(2),
	)

	// Stage 3 resizes images under a dedicated dynamic worker controller. That
	// makes this stage the easiest place to observe runtime concurrency changes.
	images = flx.MapContext(
		ctx,
		images,
		func(ctx context.Context, image imgproc.ImageResult[[]byte]) imgproc.ImageResult[[]byte] {
			if image.Err != nil {
				return image
			}

			nextBytes, err := imgproc.ResizeTo(cfg.ResizeWidth, cfg.ResizeHeight)(ctx, image.SourceURL, image.Value)
			if err != nil {
				return imgproc.ImageResult[[]byte]{
					ImageID:   image.ImageID,
					SourceURL: image.SourceURL,
					Err:       err,
				}
			}

			return imgproc.ImageResult[[]byte]{
				ImageID:   image.ImageID,
				SourceURL: image.SourceURL,
				Value:     nextBytes,
			}
		},
		control.WithForcedDynamicWorkers(resizeController),
	)

	// Stage 4 adds a visible marker so the processed output is obviously
	// different from the original even without opening the logs.
	images = flx.MapContext(
		ctx,
		images,
		func(ctx context.Context, image imgproc.ImageResult[[]byte]) imgproc.ImageResult[[]byte] {
			if image.Err != nil {
				return image
			}

			nextBytes, err := imgproc.AddWatermarkText(cfg.WatermarkText)(ctx, image.SourceURL, image.Value)
			if err != nil {
				return imgproc.ImageResult[[]byte]{
					ImageID:   image.ImageID,
					SourceURL: image.SourceURL,
					Err:       err,
				}
			}

			return imgproc.ImageResult[[]byte]{
				ImageID:   image.ImageID,
				SourceURL: image.SourceURL,
				Value:     nextBytes,
			}
		},
		control.WithWorkers(cfg.WatermarkWorkers),
	)

	// Stage 5 writes the transformed bytes to disk so the final artifacts can be
	// inspected independently of the stream logs.
	images = flx.MapContext(
		ctx,
		images,
		func(ctx context.Context, image imgproc.ImageResult[[]byte]) imgproc.ImageResult[[]byte] {
			if image.Err != nil || !shouldSaveLocal {
				return image
			}

			if err := imgproc.SaveImageBytes(cfg.LocalOutputDir, "processed", image); err != nil {
				return imgproc.ImageResult[[]byte]{
					ImageID:   image.ImageID,
					SourceURL: image.SourceURL,
					Err:       fmt.Errorf("save processed image %s: %w", image.SourceURL, err),
				}
			}

			return image
		},
		control.WithWorkers(2),
	)

	// No upload endpoint configured: stop after local processing and let the sink
	// summarize both successful and failed item outcomes.
	if cfg.UploadEndpoint == "" {
		return imgproc.ConsumeProcessedImages(images)
	}

	results := flx.MapContext(
		ctx,
		images,
		func(ctx context.Context, image imgproc.ImageResult[[]byte]) imgproc.ImageResult[string] {
			if image.Err != nil {
				return imgproc.ImageResult[string]{
					ImageID:   image.ImageID,
					SourceURL: image.SourceURL,
					Err:       image.Err,
				}
			}

			postedURL, err := imgproc.PostImage(
				ctx,
				targetHTTPClient,
				cfg.UploadEndpoint,
				image.SourceURL,
				image.Value,
			)
			if err != nil {
				return imgproc.ImageResult[string]{
					ImageID:   image.ImageID,
					SourceURL: image.SourceURL,
					Err:       fmt.Errorf("post image %s: %w", image.SourceURL, err),
				}
			}

			return imgproc.ImageResult[string]{
				ImageID:   image.ImageID,
				SourceURL: image.SourceURL,
				Value:     postedURL,
			}
		},
		control.WithWorkers(cfg.UploadWorkers),
	)

	return imgproc.ConsumeUploadedImages(results)
}
