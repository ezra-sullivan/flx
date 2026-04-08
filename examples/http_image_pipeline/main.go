package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/ezra-sullivan/flx/pipeline/control"
)

// main wires together a small end-to-end demo that lists remote images,
// applies a resize stage with dynamic concurrency, adds a watermark, and can
// optionally upload the final bytes. Use -mode to compare the plain Stage DSL,
// the lower-level native assembly, and the coordinator-attached variant.
func main() {
	mode := flag.String("mode", "coordinator", "pipeline mode: coordinator, stage, or native")
	flag.Parse()

	ctx := context.Background()

	sourceHTTPClient := &http.Client{Timeout: 15 * time.Second}
	targetHTTPClient := &http.Client{Timeout: 15 * time.Second}

	// Resize is the stage that uses forced dynamic workers in this sample. The
	// controller is injected into that stage so the worker count can change at
	// runtime without rebuilding the rest of the pipeline.
	resizeController := control.NewConcurrencyController(4)

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

	var err error
	switch strings.ToLower(strings.TrimSpace(*mode)) {
	case "coordinator":
		err = RunCoordinatorPipeline(ctx, sourceHTTPClient, targetHTTPClient, cfg, resizeController)
	case "stage":
		err = RunStagePipeline(ctx, sourceHTTPClient, targetHTTPClient, cfg, resizeController)
	case "native":
		err = RunNativePipeline(ctx, sourceHTTPClient, targetHTTPClient, cfg, resizeController)
	default:
		log.Fatalf("unsupported mode %q; expected coordinator, stage, or native", *mode)
	}

	if err != nil {
		log.Fatal(err)
	}
}
