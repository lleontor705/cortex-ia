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
