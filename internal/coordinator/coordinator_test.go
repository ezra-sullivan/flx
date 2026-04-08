package coordinator

import (
	"slices"
	"testing"

	linkinternal "github.com/ezra-sullivan/flx/internal/link"
	"github.com/ezra-sullivan/flx/internal/metrics"
	resourceinternal "github.com/ezra-sullivan/flx/internal/resource"
)

func TestSnapshotReturnsSortedCopies(t *testing.T) {
	coord := New(Policy{}, resourceinternal.ObserverFunc(func() resourceinternal.Snapshot {
		return resourceinternal.Snapshot{
			Samples: []resourceinternal.PressureSample{{
				Name:  "memory",
				Level: resourceinternal.PressureLevelWarning,
				Value: 80,
				Limit: 100,
			}},
		}
	}))

	coord.ObserveStage(metrics.StageMetrics{StageName: "resize", InputCount: 4})
	coord.ObserveStage(metrics.StageMetrics{StageName: "download", InputCount: 3})
	coord.ObserveLink(linkinternal.Metrics{FromStage: "resize", ToStage: "", SentCount: 4})
	coord.ObserveLink(linkinternal.Metrics{FromStage: "download", ToStage: "resize", SentCount: 3})

	got := coord.Snapshot()

	if !slices.EqualFunc(got.Stages, []metrics.StageMetrics{
		{StageName: "download", InputCount: 3},
		{StageName: "resize", InputCount: 4},
	}, func(a, b metrics.StageMetrics) bool {
		return a == b
	}) {
		t.Fatalf("unexpected stages: %#v", got.Stages)
	}

	if !slices.EqualFunc(got.Links, []linkinternal.Metrics{
		{FromStage: "download", ToStage: "resize", SentCount: 3},
		{FromStage: "resize", ToStage: "", SentCount: 4},
	}, func(a, b linkinternal.Metrics) bool {
		return a == b
	}) {
		t.Fatalf("unexpected links: %#v", got.Links)
	}

	if !slices.EqualFunc(got.Resources.Samples, []resourceinternal.PressureSample{{
		Name:  "memory",
		Level: resourceinternal.PressureLevelWarning,
		Value: 80,
		Limit: 100,
	}}, func(a, b resourceinternal.PressureSample) bool {
		return a == b
	}) {
		t.Fatalf("unexpected resources: %#v", got.Resources.Samples)
	}
}

func TestResolveStageNameGeneratesStableAnonymousNames(t *testing.T) {
	coord := New(Policy{})

	first := coord.ResolveStageName("")
	second := coord.ResolveStageName("")

	if first == "" || second == "" {
		t.Fatal("expected generated stage names")
	}
	if first == second {
		t.Fatalf("expected distinct anonymous stage names, got %q", first)
	}
}
