package main

import (
	"context"
	"log"
	"time"

	imgproc "github.com/ezra-sullivan/flx/examples/internal/imgproc"
	"github.com/ezra-sullivan/flx/pipeline/control"
)

func main() {
	ctx := context.Background()
	sourceHTTPClient := imgproc.NewSourceHTTPClient()
	targetHTTPClient := imgproc.NewTargetHTTPClient()
	resizeController := control.NewConcurrencyController(2)
	startObserveResizeDemo(resizeController)

	cfg := imgproc.DefaultPipelineConfig("examples/imgproc_observe/.output")
	cfg.ListPageSize = 8
	cfg.DownloadWidth = 960
	cfg.DownloadHeight = 540
	cfg.ResizeWidth = 480
	cfg.ResizeHeight = 270
	cfg.DownloadWorkers = 3

	if err := RunObservePipeline(ctx, sourceHTTPClient, targetHTTPClient, cfg, resizeController); err != nil {
		log.Fatal(err)
	}
}

func startObserveResizeDemo(controller *control.ConcurrencyController) {
	if controller == nil {
		return
	}

	go func() {
		time.Sleep(700 * time.Millisecond)
		controller.SetWorkers(4)
		time.Sleep(700 * time.Millisecond)
		controller.SetWorkers(1)
		time.Sleep(700 * time.Millisecond)
		controller.SetWorkers(3)
	}()
}
