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

	results := make([]ChainResult, len(chains))
	var wg sync.WaitGroup

	for i, chain := range chains {
		wg.Add(1)
		go func(idx int, steps []Step) {
			defer wg.Done()
			results[idx] = runChain(steps)
		}(i, chain)
	}
	wg.Wait()

	// Merge results in input order so the primary error is deterministic.
	var allCompleted []string
	var primaryFailure string
	var primaryError error
	failures := 0
	for _, r := range results {
		allCompleted = append(allCompleted, r.Completed...)
		if r.Error == nil {
			continue
		}
		failures++
		if primaryError == nil {
			primaryFailure = r.Failed
			primaryError = r.Error
		}
	}

	if primaryError != nil {
		return StageResult{
			Completed: allCompleted,
			Failed:    primaryFailure,
			Error:     fmt.Errorf("%d step(s) failed: %w", failures, primaryError),
		}
	}
	return StageResult{Completed: allCompleted}
}

// ChainResult is the result of a sequential chain. A failed step stops only
// its own chain; other chains continue until they finish.
type ChainResult struct {
	Completed []string
	Failed    string
	Error     error
}

func runChain(steps []Step) ChainResult {
	result := ChainResult{}
	for _, step := range steps {
		if err := step.Run(); err != nil {
			result.Failed = step.Name()
			result.Error = fmt.Errorf("step %q: %w", step.Name(), err)
			return result
		}
		result.Completed = append(result.Completed, step.Name())
	}
	return result
}

func stepNames(steps []Step) []string {
	names := make([]string, len(steps))
	for i, s := range steps {
		names[i] = s.Name()
	}
	return names
}
