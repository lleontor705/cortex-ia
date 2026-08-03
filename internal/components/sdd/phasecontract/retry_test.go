package phasecontract

import "testing"

func TestRetryPolicyProfilesEnforceGlobalCeilings(t *testing.T) {
	// REQ-ORCH-003: max 3 transient, max 2 semantic, max 2 no-progress cycles.
	for phase, policy := range RetryProfiles {
		if policy.TransientMax > 3 {
			t.Errorf("phase %q transient max %d exceeds ceiling 3", phase, policy.TransientMax)
		}
		if policy.SemanticMax > 2 {
			t.Errorf("phase %q semantic max %d exceeds ceiling 2", phase, policy.SemanticMax)
		}
		if policy.NoProgressCycles > 2 {
			t.Errorf("phase %q no-progress cycles %d exceeds ceiling 2", phase, policy.NoProgressCycles)
		}
		if policy.NoProgressCycles < 1 {
			t.Errorf("phase %q must declare a positive no-progress ceiling", phase)
		}
	}
	if len(RetryProfiles) < 9 {
		t.Fatalf("RetryProfiles must cover all 9 phases, got %d", len(RetryProfiles))
	}
}

func TestRetryStateExhaustedByTransientSemanticAndNoProgress(t *testing.T) {
	policy := RetryPolicy{TransientMax: 3, SemanticMax: 2, NoProgressCycles: 2}

	// Transient ceiling.
	state := RetryState{Phase: PhaseApply}
	for i := 0; i < policy.TransientMax; i++ {
		if state.Exhausted(policy) {
			t.Fatalf("RetryState exhausted before transient ceiling at attempt %d", i)
		}
		state.RecordTransient()
	}
	if !state.Exhausted(policy) {
		t.Fatal("RetryState not exhausted after reaching transient ceiling")
	}

	// Semantic ceiling.
	sem := RetryState{Phase: PhaseSpec}
	for i := 0; i < policy.SemanticMax; i++ {
		if sem.Exhausted(policy) {
			t.Fatalf("RetryState exhausted before semantic ceiling at attempt %d", i)
		}
		sem.RecordSemantic()
	}
	if !sem.Exhausted(policy) {
		t.Fatal("RetryState not exhausted after reaching semantic ceiling")
	}

	// No-progress ceiling.
	progress := RetryState{Phase: PhaseApply}
	for i := 0; i < policy.NoProgressCycles; i++ {
		if progress.Exhausted(policy) {
			t.Fatalf("RetryState exhausted before no-progress ceiling at cycle %d", i)
		}
		progress.RecordNoProgress()
	}
	if !progress.Exhausted(policy) {
		t.Fatal("RetryState not exhausted after reaching no-progress ceiling")
	}
}

func TestRetryStateRejectsUnknownPhase(t *testing.T) {
	state := RetryState{Phase: "unknown"}
	if err := state.Validate(); err == nil {
		t.Fatal("RetryState with unknown phase.Validate() error = nil, want rejection")
	}
}
