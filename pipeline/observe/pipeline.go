package observe

// PipelineSnapshot captures the latest stage and link view for one pipeline.
type PipelineSnapshot struct {
	Stages    []StageMetrics
	Links     []LinkMetrics
	Resources ResourceSnapshot
}

// StageDecision captures one coordinator decision for a stage.
type StageDecision struct {
	StageName    string
	FromWorkers  int
	ToWorkers    int
	Reason       string
	StageBacklog int
	LinkBacklog  int
}

// PipelineDecision captures one pipeline-wide coordinator decision batch.
type PipelineDecision struct {
	Stages []StageDecision
}
