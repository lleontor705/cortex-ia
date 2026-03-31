package pipeline

import (
	"fmt"
	"sync"
)

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

// RunStageContinue executes all steps, collecting errors rather than stopping.
// No rollback is performed — the caller decides how to handle partial failures.
func RunStageContinue(steps []Step) StageResult {
	var completed []string
	var errors []string

	for _, step := range steps {
		if err := step.Run(); err != nil {
			errors = append(errors, fmt.Sprintf("step %q: %v", step.Name(), err))
			continue
		}
		completed = append(completed, step.Name())
	}

	if len(errors) > 0 {
		return StageResult{
			Completed: completed,
			Error:     fmt.Errorf("%d step(s) failed: %s", len(errors), errors[0]),
		}
	}
	return StageResult{Completed: completed}
}

// RunOrchestrator runs prepare steps (stop-on-error with rollback), then apply
// steps using the orchestrator's FailurePolicy.
func RunOrchestrator(o Orchestrator) OrchestratorResult {
	prepResult := RunStage(o.Prepare)
	if prepResult.Error != nil {
		return OrchestratorResult{PrepareResult: prepResult}
	}

	var applyResult StageResult
	if o.Policy == ContinueOnError {
		applyResult = RunStageContinue(o.Apply)
	} else {
		applyResult = RunStage(o.Apply)
	}
	return OrchestratorResult{PrepareResult: prepResult, ApplyResult: applyResult}
}

// RunParallelGroups executes groups of steps level-by-level. Steps within the
// same level run concurrently via goroutines. Levels execute sequentially
// (level N must complete before level N+1 starts). Errors are collected per
// level; execution continues to the next level regardless of failures.
func RunParallelGroups(groups [][]Step) StageResult {
	var allCompleted []string
	var allErrors []string

	for _, group := range groups {
		if len(group) == 0 {
			continue
		}

		// Single-step groups don't need goroutines.
		if len(group) == 1 {
			step := group[0]
			if err := step.Run(); err != nil {
				allErrors = append(allErrors, fmt.Sprintf("step %q: %v", step.Name(), err))
			} else {
				allCompleted = append(allCompleted, step.Name())
			}
			continue
		}

		// Run steps in this level concurrently.
		type result struct {
			name string
			err  error
		}
		results := make([]result, len(group))
		var wg sync.WaitGroup

		for i, step := range group {
			wg.Add(1)
			go func(idx int, s Step) {
				defer wg.Done()
				results[idx] = result{name: s.Name(), err: s.Run()}
			}(i, step)
		}
		wg.Wait()

		for _, r := range results {
			if r.err != nil {
				allErrors = append(allErrors, fmt.Sprintf("step %q: %v", r.name, r.err))
			} else {
				allCompleted = append(allCompleted, r.name)
			}
		}
	}

	if len(allErrors) > 0 {
		return StageResult{
			Completed: allCompleted,
			Error:     fmt.Errorf("%d step(s) failed: %s", len(allErrors), allErrors[0]),
		}
	}
	return StageResult{Completed: allCompleted}
}

// RunParallelChains runs multiple sequential chains concurrently. Each chain
// executes its steps sequentially, but different chains run in parallel.
// Use this when agents can be configured concurrently (different config dirs)
// but components within an agent must run sequentially (same config files).
func RunParallelChains(chains [][]Step) StageResult {
	if len(chains) == 0 {
		return StageResult{}
	}

	// Single chain — just run sequentially, no goroutines needed.
	if len(chains) == 1 {
		return RunStageContinue(chains[0])
	}

	type chainResult struct {
		completed []string
		errors    []string
	}

	results := make([]chainResult, len(chains))
	var wg sync.WaitGroup

	for i, chain := range chains {
		wg.Add(1)
		go func(idx int, steps []Step) {
			defer wg.Done()
			for _, step := range steps {
				if err := step.Run(); err != nil {
					results[idx].errors = append(results[idx].errors, fmt.Sprintf("step %q: %v", step.Name(), err))
					continue
				}
				results[idx].completed = append(results[idx].completed, step.Name())
			}
		}(i, chain)
	}
	wg.Wait()

	// Merge results.
	var allCompleted []string
	var allErrors []string
	for _, r := range results {
		allCompleted = append(allCompleted, r.completed...)
		allErrors = append(allErrors, r.errors...)
	}

	if len(allErrors) > 0 {
		return StageResult{
			Completed: allCompleted,
			Error:     fmt.Errorf("%d step(s) failed: %s", len(allErrors), allErrors[0]),
		}
	}
	return StageResult{Completed: allCompleted}
}

func stepNames(steps []Step) []string {
	names := make([]string, len(steps))
	for i, s := range steps {
		names[i] = s.Name()
	}
	return names
}
