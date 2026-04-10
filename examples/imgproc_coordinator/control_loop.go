package main

import (
	"context"
	"time"

	pipelinecoordinator "github.com/ezra-sullivan/flx/pipeline/coordinator"
)

const (
	coordinatorTickInterval        = 300 * time.Millisecond
	coordinatorSnapshotLogInterval = time.Second
)

func startCoordinatorLoop(
	ctx context.Context,
	pipelineCoordinator *pipelinecoordinator.PipelineCoordinator,
) <-chan struct{} {
	done := make(chan struct{})

	go func() {
		defer close(done)
		if pipelineCoordinator == nil {
			return
		}

		runCoordinatorLoop(ctx, pipelineCoordinator)
	}()

	return done
}

func runCoordinatorLoop(ctx context.Context, pipelineCoordinator *pipelinecoordinator.PipelineCoordinator) {
	if pipelineCoordinator == nil {
		return
	}

	ticker := time.NewTicker(coordinatorTickInterval)
	defer ticker.Stop()

	var lastSnapshotLogAt time.Time
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			if lastSnapshotLogAt.IsZero() || now.Sub(lastSnapshotLogAt) >= coordinatorSnapshotLogInterval {
				logCoordinatorSnapshot("tick", pipelineCoordinator.Snapshot())
				lastSnapshotLogAt = now
			}

			pipelineCoordinator.Tick()
		}
	}
}
