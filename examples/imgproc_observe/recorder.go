package main

import (
	"cmp"
	"slices"
	"sync"

	"github.com/ezra-sullivan/flx/pipeline/observe"
)

type observeRecorder struct {
	mu     sync.RWMutex
	stages map[string]observe.StageMetrics
	links  map[string]observe.LinkMetrics
}

func newObserveRecorder() *observeRecorder {
	return &observeRecorder{
		stages: make(map[string]observe.StageMetrics),
		links:  make(map[string]observe.LinkMetrics),
	}
}

func (r *observeRecorder) ObserveStage(snapshot observe.StageMetrics) {
	if r == nil || snapshot.StageName == "" {
		return
	}

	r.mu.Lock()
	r.stages[snapshot.StageName] = snapshot
	r.mu.Unlock()
}

func (r *observeRecorder) ObserveLink(snapshot observe.LinkMetrics) {
	if r == nil || snapshot.FromStage == "" {
		return
	}

	target := cmp.Or(snapshot.ToStage, "terminal")

	r.mu.Lock()
	r.links[snapshot.FromStage+"->"+target] = snapshot
	r.mu.Unlock()
}

func (r *observeRecorder) Snapshot() observe.PipelineSnapshot {
	if r == nil {
		return observe.PipelineSnapshot{}
	}

	r.mu.RLock()
	stages := make([]observe.StageMetrics, 0, len(r.stages))
	for _, snapshot := range r.stages {
		stages = append(stages, snapshot)
	}

	links := make([]observe.LinkMetrics, 0, len(r.links))
	for _, snapshot := range r.links {
		links = append(links, snapshot)
	}
	r.mu.RUnlock()

	slices.SortFunc(stages, func(a, b observe.StageMetrics) int {
		return cmp.Compare(a.StageName, b.StageName)
	})

	slices.SortFunc(links, func(a, b observe.LinkMetrics) int {
		if diff := cmp.Compare(a.FromStage, b.FromStage); diff != 0 {
			return diff
		}

		return cmp.Compare(a.ToStage, b.ToStage)
	})

	return observe.PipelineSnapshot{
		Stages: stages,
		Links:  links,
	}
}
