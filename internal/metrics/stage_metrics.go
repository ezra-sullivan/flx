package metrics

// StageMetrics captures counters and runtime state reported for a single stage.
type StageMetrics struct {
	StageName     string
	InputCount    int64
	OutputCount   int64
	ErrorCount    int64
	ActiveWorkers int
	IdleWorkers   int
	Backlog       int
}

// StageMetricsObserver receives a snapshot when the stage reports metrics.
type StageMetricsObserver func(StageMetrics)
