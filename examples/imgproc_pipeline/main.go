package main

import (
	"context"
	"flag"
	"log"
	"strings"
	"time"

	imgproc "github.com/ezra-sullivan/flx/examples/internal/imgproc"
	"github.com/ezra-sullivan/flx/pipeline/control"
)

// main wires together a small end-to-end demo that lists Picsum catalog
// entries, downloads the source images over HTTP, applies a resize stage with
// dynamic concurrency, adds a watermark, and can optionally upload the final
// bytes. It defaults to the plain Stage DSL, while -mode native switches to
// the side-by-side raw flx version.
func main() {
	mode := flag.String("mode", "stage", "pipeline mode: stage or native")
	flag.Parse()

	// One root context is enough for the example because shutdown is driven by
	// process exit. Individual retries and timeout boundaries are handled inside
	// the helper stages rather than here.
	ctx := context.Background()

	sourceHTTPClient := imgproc.NewSourceHTTPClient()
	targetHTTPClient := imgproc.NewTargetHTTPClient()

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
	cfg := imgproc.DefaultPipelineConfig("examples/imgproc_pipeline/.output")
	// cfg.UploadEndpoint = imgproc.ExampleUploadEndpoint()

	var err error
	// The remaining modes both run the same business pipeline; the difference is
	// whether the reader wants the recommended Stage DSL or the raw flx assembly.
	switch strings.ToLower(strings.TrimSpace(*mode)) {
	case "stage":
		err = RunStagePipeline(ctx, sourceHTTPClient, targetHTTPClient, cfg, resizeController)
	case "native":
		err = RunNativePipeline(ctx, sourceHTTPClient, targetHTTPClient, cfg, resizeController)
	default:
		log.Fatalf("unsupported mode %q; expected stage or native", *mode)
	}

	if err != nil {
		log.Fatal(err)
	}
}
