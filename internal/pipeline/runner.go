package pipeline

import "fmt"

// RunStage executes steps sequentially. On failure, rolls back completed steps
// in reverse order.
func RunStage(steps []Step) StageResult {
	var completed []Step

	for _, step := range steps {
		if err := step.Run(); err != nil {
			// Rollback completed steps in reverse.
			for i := len(completed) - 1; i >= 0; i-- {
				if rb, ok := completed[i].(RollbackStep); ok {
					_ = rb.Rollback()
				}
			}
			return StageResult{
				Completed: stepNames(completed),
				Failed:    step.Name(),
				Error:     fmt.Errorf("step %q failed: %w", step.Name(), err),
			}
		}
		completed = append(completed, step)
	}

	return StageResult{Completed: stepNames(completed)}
}

func stepNames(steps []Step) []string {
	names := make([]string, len(steps))
	for i, s := range steps {
		names[i] = s.Name()
	}
	return names
}
