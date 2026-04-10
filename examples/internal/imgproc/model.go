package imgproc

import (
	"context"
	"net/http"
	"time"
)

const (
	exampleListEndpoint    = "https://picsum.photos/v2/list"
	StageNameDownload      = "download"
	StageNameSaveOriginal  = "save-original"
	StageNameResize        = "resize"
	StageNameWatermark     = "watermark"
	StageNameSaveProcessed = "save-processed"
	StageNameUpload        = "upload"

	defaultDownloadRetryTimes          = 3
	defaultDownloadRetryInterval       = 1000 * time.Millisecond
	defaultDownloadRetryAttemptTimeout = 30 * time.Second
)

// PipelineConfig collects the knobs used by the image-processing examples. Zero
// or empty values fall back to small demo-friendly defaults.
type PipelineConfig struct {
	ListEndpoint           string
	ListPageSize           int
	ListMaxPages           int
	LocalOutputDir         string
	DownloadWidth          int
	DownloadHeight         int
	DownloadWorkers        int
	DownloadRetryTimes     int
	DownloadRetryInterval  time.Duration
	DownloadAttemptTimeout time.Duration
	WatermarkWorkers       int
	UploadWorkers          int
	ResizeWidth            int
	ResizeHeight           int
	WatermarkText          string
	UploadEndpoint         string
}

// ImageCatalogEntry models the minimal response shape this example family needs
// from the Picsum list endpoint.
type ImageCatalogEntry struct {
	ID string `json:"id"`
}

// RemoteImage is the payload emitted by the listing stage and consumed by the
// download stage.
type RemoteImage struct {
	ID        string
	SourceURL string
}

// ImageResult carries either a successful value or an item-scoped failure for
// one image.
type ImageResult[T any] struct {
	ImageID   string
	SourceURL string
	Value     T
	Err       error
}

// ImageTransformer describes one pure image-processing step.
type ImageTransformer func(ctx context.Context, sourceURL string, imageBytes []byte) ([]byte, error)

// DefaultPipelineConfig returns the baseline image-processing example config.
func DefaultPipelineConfig(outputDir string) PipelineConfig {
	return PipelineConfig{
		ListEndpoint:           ExampleListEndpoint(),
		ListPageSize:           5,
		ListMaxPages:           1,
		LocalOutputDir:         outputDir,
		DownloadWidth:          1920,
		DownloadHeight:         1080,
		DownloadWorkers:        2,
		DownloadRetryTimes:     3,
		DownloadRetryInterval:  250 * time.Millisecond,
		DownloadAttemptTimeout: 30 * time.Second,
		WatermarkWorkers:       4,
		UploadWorkers:          8,
		ResizeWidth:            960,
		ResizeHeight:           540,
		WatermarkText:          "flx-demo",
	}
}

// ExampleListEndpoint returns the Picsum list endpoint used by the examples.
func ExampleListEndpoint() string {
	return exampleListEndpoint
}

// ExampleUploadEndpoint returns the local upload endpoint used by the examples.
func ExampleUploadEndpoint() string {
	return exampleUploadEndpoint
}

// NewSourceHTTPClient returns the source-side HTTP client used by the examples.
func NewSourceHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 45 * time.Second,
	}
}

// NewTargetHTTPClient returns the optional upload-side HTTP client used by the
// examples.
func NewTargetHTTPClient() *http.Client {
	return &http.Client{
		Timeout:   15 * time.Second,
		Transport: newLocalUploadTransport(),
	}
}
