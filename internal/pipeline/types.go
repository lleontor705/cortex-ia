package pipeline

// Step is a unit of work in the installation pipeline.
type Step interface {
	Name() string
	Run() error
}

// RollbackStep is a Step that can undo its work on failure.
type RollbackStep interface {
	Step
	Rollback() error
}

// StageResult tracks what happened during a stage.
type StageResult struct {
	Completed []string
	Failed    string
	Error     error
}

// FailurePolicy controls behavior when an apply step fails.
type FailurePolicy int

const (
	// StopOnError halts execution and rolls back on the first failure.
	StopOnError FailurePolicy = iota
	// ContinueOnError collects errors and continues with remaining steps.
	ContinueOnError
)

// Orchestrator runs a two-stage pipeline: prepare (validation, backup) then
// apply (component injection). Prepare always stops on error. Apply uses the
// configured FailurePolicy.
type Orchestrator struct {
	Prepare []Step
	Apply   []Step
	Policy  FailurePolicy
}

// OrchestratorResult captures the outcome of both stages.
type OrchestratorResult struct {
	PrepareResult StageResult
	ApplyResult   StageResult
}
