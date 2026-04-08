package observe

// StageMetrics carries counters and worker counts for a stage snapshot.
type StageMetrics struct {
	StageName     string
	InputCount    int64
	OutputCount   int64
	ErrorCount    int64
	ActiveWorkers int
	IdleWorkers   int
	Backlog       int
}

// StageMetricsObserver receives the snapshot reported for a stage.
type StageMetricsObserver func(StageMetrics)
