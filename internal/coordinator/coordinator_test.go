package coordinator

import (
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

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

func TestCoordinatorRetainsObservedStateUntilFreshInstance(t *testing.T) {
	coord := New(Policy{})
	coord.ObserveStage(metrics.StageMetrics{StageName: "download", InputCount: 1})

	initial := coord.Snapshot()
	if got := len(initial.Stages); got != 1 {
		t.Fatalf("expected one stage in initial snapshot, got %d", got)
	}

	coord.ObserveStage(metrics.StageMetrics{StageName: "resize", InputCount: 2})

	accumulated := coord.Snapshot()
	if !slices.EqualFunc(accumulated.Stages, []metrics.StageMetrics{
		{StageName: "download", InputCount: 1},
		{StageName: "resize", InputCount: 2},
	}, func(a, b metrics.StageMetrics) bool {
		return a == b
	}) {
		t.Fatalf("expected coordinator instance to retain prior stage snapshots, got %#v", accumulated.Stages)
	}

	fresh := New(Policy{})
	fresh.ObserveStage(metrics.StageMetrics{StageName: "resize", InputCount: 2})

	reset := fresh.Snapshot()
	if !slices.EqualFunc(reset.Stages, []metrics.StageMetrics{
		{StageName: "resize", InputCount: 2},
	}, func(a, b metrics.StageMetrics) bool {
		return a == b
	}) {
		t.Fatalf("expected fresh coordinator instance to start without prior run state, got %#v", reset.Stages)
	}
}

func TestObserveLinkKeepsLatestOutboundSnapshotPerFromStage(t *testing.T) {
	coord := New(Policy{})

	coord.ObserveLink(linkinternal.Metrics{FromStage: "download", ToStage: "resize", SentCount: 3})
	coord.ObserveLink(linkinternal.Metrics{FromStage: "download", ToStage: "watermark", SentCount: 5})

	got := coord.Snapshot()
	if !slices.EqualFunc(got.Links, []linkinternal.Metrics{
		{FromStage: "download", ToStage: "watermark", SentCount: 5},
	}, func(a, b linkinternal.Metrics) bool {
		return a == b
	}) {
		t.Fatalf("expected latest outbound link to replace earlier snapshot for the same fromStage, got %#v", got.Links)
	}
}

func TestSnapshotAndTickSerializeResourceObservation(t *testing.T) {
	entered := make(chan struct{}, 4)
	release := make(chan struct{})
	coord := New(Policy{}, resourceinternal.ObserverFunc(func() resourceinternal.Snapshot {
		entered <- struct{}{}
		<-release

		return resourceinternal.Snapshot{
			Samples: []resourceinternal.PressureSample{{
				Name:  "memory",
				Level: resourceinternal.PressureLevelWarning,
			}},
		}
	}))

	var wg sync.WaitGroup
	wg.Go(func() {
		coord.Snapshot()
	})

	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("expected first resource observation to start")
	}

	wg.Go(func() {
		coord.Snapshot()
	})
	wg.Go(func() {
		coord.Tick()
	})

	select {
	case <-entered:
		t.Fatal("expected resource observation to stay serialized across Snapshot and Tick")
	case <-time.After(100 * time.Millisecond):
	}

	close(release)
	wg.Wait()
}

func TestSnapshotAndTickPollResourceObserversIndependently(t *testing.T) {
	var calls atomic.Int32
	coord := New(Policy{}, resourceinternal.ObserverFunc(func() resourceinternal.Snapshot {
		call := calls.Add(1)

		return resourceinternal.Snapshot{
			Samples: []resourceinternal.PressureSample{{
				Name:  "memory",
				Level: resourceinternal.PressureLevelWarning,
				Value: float64(call),
				Limit: 100,
			}},
		}
	}))

	snapshot := coord.Snapshot()
	if got := len(snapshot.Resources.Samples); got != 1 {
		t.Fatalf("expected one resource sample from Snapshot, got %d", got)
	}

	coord.Tick()

	if got := calls.Load(); got != 2 {
		t.Fatalf("expected Snapshot and Tick to poll independently, got %d calls", got)
	}
}
