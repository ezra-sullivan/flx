package link

// Metrics captures one snapshot for a single outbound stream link.
type Metrics struct {
	FromStage     string
	ToStage       string
	SentCount     int64
	ReceivedCount int64
	Backlog       int
	Capacity      int
}

// MetricsObserver receives one link snapshot.
type MetricsObserver func(Metrics)
