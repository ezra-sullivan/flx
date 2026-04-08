package coordinator

import (
	linkinternal "github.com/ezra-sullivan/flx/internal/link"
	"github.com/ezra-sullivan/flx/internal/metrics"
	resourceinternal "github.com/ezra-sullivan/flx/internal/resource"
)

// Snapshot captures the current pipeline-wide view known to the coordinator.
type Snapshot struct {
	Stages    []metrics.StageMetrics
	Links     []linkinternal.Metrics
	Resources resourceinternal.Snapshot
}
