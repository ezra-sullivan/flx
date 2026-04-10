package imgproc

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

// DownloadImageStage returns the mapper used by the download step.
func DownloadImageStage(
	sourceHTTPClient *http.Client,
	cfg PipelineConfig,
) func(context.Context, RemoteImage) ImageResult[[]byte] {
	return func(ctx context.Context, image RemoteImage) ImageResult[[]byte] {
		imageBytes, err := DownloadImageWithRetry(ctx, sourceHTTPClient, image, cfg)
		if err != nil {
			return ImageResult[[]byte]{
				ImageID:   image.ID,
				SourceURL: image.SourceURL,
				Err:       fmt.Errorf("download %s: %w", image.SourceURL, err),
			}
		}

		return ImageResult[[]byte]{
			ImageID:   image.ID,
			SourceURL: image.SourceURL,
			Value:     imageBytes,
		}
	}
}

// TransformImageStage returns the mapper used by transform steps.
func TransformImageStage(
	transform ImageTransformer,
) func(context.Context, ImageResult[[]byte]) ImageResult[[]byte] {
	return func(ctx context.Context, image ImageResult[[]byte]) ImageResult[[]byte] {
		if image.Err != nil {
			return image
		}

		nextBytes, err := transform(ctx, image.SourceURL, image.Value)
		if err != nil {
			return ImageResult[[]byte]{
				ImageID:   image.ImageID,
				SourceURL: image.SourceURL,
				Err:       err,
			}
		}

		return ImageResult[[]byte]{
			ImageID:   image.ImageID,
			SourceURL: image.SourceURL,
			Value:     nextBytes,
		}
	}
}

// SaveLocalImageStage returns the mapper used by local persistence steps.
func SaveLocalImageStage(
	baseDir string,
	stage string,
) func(context.Context, ImageResult[[]byte]) ImageResult[[]byte] {
	if strings.TrimSpace(baseDir) == "" {
		return func(_ context.Context, image ImageResult[[]byte]) ImageResult[[]byte] {
			return image
		}
	}

	return func(ctx context.Context, image ImageResult[[]byte]) ImageResult[[]byte] {
		if image.Err != nil {
			return image
		}

		if err := SaveImageBytes(baseDir, stage, image); err != nil {
			return ImageResult[[]byte]{
				ImageID:   image.ImageID,
				SourceURL: image.SourceURL,
				Err:       fmt.Errorf("save %s image %s: %w", stage, image.SourceURL, err),
			}
		}

		return image
	}
}

// UploadImageStage returns the mapper used by the optional upload step.
func UploadImageStage(
	targetHTTPClient *http.Client,
	uploadEndpoint string,
) func(context.Context, ImageResult[[]byte]) ImageResult[string] {
	return func(ctx context.Context, image ImageResult[[]byte]) ImageResult[string] {
		if image.Err != nil {
			return ImageResult[string]{
				ImageID:   image.ImageID,
				SourceURL: image.SourceURL,
				Err:       image.Err,
			}
		}

		postedURL, err := PostImage(ctx, targetHTTPClient, uploadEndpoint, image.SourceURL, image.Value)
		if err != nil {
			return ImageResult[string]{
				ImageID:   image.ImageID,
				SourceURL: image.SourceURL,
				Err:       fmt.Errorf("post image %s: %w", image.SourceURL, err),
			}
		}

		return ImageResult[string]{
			ImageID:   image.ImageID,
			SourceURL: image.SourceURL,
			Value:     postedURL,
		}
	}
}
