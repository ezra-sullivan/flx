package observe

// LinkMetrics carries counters and backlog for a single outbound stream link.
type LinkMetrics struct {
	FromStage     string
	ToStage       string
	SentCount     int64
	ReceivedCount int64
	Backlog       int
	Capacity      int
}

// LinkMetricsObserver receives the snapshot reported for one link.
type LinkMetricsObserver func(LinkMetrics)
