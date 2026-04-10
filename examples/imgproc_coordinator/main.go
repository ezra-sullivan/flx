package main

import (
	"context"
	"log"

	imgproc "github.com/ezra-sullivan/flx/examples/internal/imgproc"
	"github.com/ezra-sullivan/flx/pipeline/control"
)

func main() {
	ctx := context.Background()
	sourceHTTPClient := imgproc.NewSourceHTTPClient()
	targetHTTPClient := imgproc.NewTargetHTTPClient()
	resizeController := control.NewConcurrencyController(1)

	cfg := imgproc.DefaultPipelineConfig("examples/imgproc_coordinator/.output")
	cfg.ListPageSize = 12
	cfg.DownloadWorkers = 4
	cfg.DownloadWidth = 960
	cfg.DownloadHeight = 540
	cfg.ResizeWidth = 480
	cfg.ResizeHeight = 270

	if err := RunCoordinatorPipeline(ctx, sourceHTTPClient, targetHTTPClient, cfg, resizeController); err != nil {
		log.Fatal(err)
	}
}
