package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/ezra-sullivan/flx"
)

// main wires together a small end-to-end demo that lists remote images,
// applies a resize stage with dynamic concurrency, adds a watermark, and can
// optionally upload the final bytes. The configuration intentionally stays
// small so the example is easy to run locally while still showing retries,
// stage boundaries, and output artifacts.
func main() {
	ctx := context.Background()

	sourceHTTPClient := &http.Client{Timeout: 15 * time.Second}
	targetHTTPClient := &http.Client{Timeout: 15 * time.Second}

	// Resize is the stage that uses forced dynamic workers in this sample. The
	// controller is injected into that stage so the worker count can change at
	// runtime without rebuilding the rest of the pipeline.
	resizeController := flx.NewConcurrencyController(4)

	// Simulate runtime scaling while the pipeline is still active. This makes the
	// effect of dynamic worker control obvious in logs and elapsed time.
	go func() {
		time.Sleep(500 * time.Millisecond)
		resizeController.SetWorkers(8)
		time.Sleep(500 * time.Millisecond)
		resizeController.SetWorkers(2)
		time.Sleep(500 * time.Millisecond)
		resizeController.SetWorkers(6)
	}()

	// Keep the sample small enough to run locally while still showing retries,
	// multi-stage processing, and before/after files on disk.
	cfg := PipelineConfig{
		ListEndpoint:           "https://picsum.photos/v2/list",
		ListPageSize:           5,
		ListMaxPages:           1,
		LocalOutputDir:         "examples/http_image_pipeline/.output",
		DownloadWidth:          1920,
		DownloadHeight:         1080,
		DownloadWorkers:        4,
		DownloadRetryTimes:     3,
		DownloadRetryInterval:  250 * time.Millisecond,
		DownloadAttemptTimeout: 8 * time.Second,
		WatermarkWorkers:       4,
		UploadWorkers:          8,
		ResizeWidth:            960,
		ResizeHeight:           540,
		WatermarkText:          "flx-demo",
		// UploadEndpoint: "https://target.example.com/api/upload",
	}

	if err := RunStagePipeline(ctx, sourceHTTPClient, targetHTTPClient, cfg, resizeController); err != nil {
		log.Fatal(err)
	}

	// Uncomment this path to compare the more explicit inline-pipeline version
	// against the Stage-oriented pipeline above.
	// if err := RunNativePipeline(ctx, sourceHTTPClient, targetHTTPClient, cfg, resizeController); err != nil {
	// 	log.Fatal(err)
	// }
}
