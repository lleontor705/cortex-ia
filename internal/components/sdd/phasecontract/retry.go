package phasecontract

import "fmt"

// RetryProfiles binds each phase to its bounded retry policy per REQ-ORCH-003.
// Global ceilings are 3 transient, 2 semantic, and 2 no-progress cycles; no
// profile may exceed them.
var RetryProfiles = map[PhaseID]RetryPolicy{
	PhaseBootstrap:   {TransientMax: 3, SemanticMax: 1, NoProgressCycles: 1},
	PhaseInvestigate: {TransientMax: 3, SemanticMax: 1, NoProgressCycles: 1},
	PhasePropose:     {TransientMax: 3, SemanticMax: 2, NoProgressCycles: 1},
	PhaseSpec:        {TransientMax: 3, SemanticMax: 2, NoProgressCycles: 1},
	PhaseDesign:      {TransientMax: 3, SemanticMax: 2, NoProgressCycles: 1},
	PhaseTasks:       {TransientMax: 3, SemanticMax: 2, NoProgressCycles: 1},
	PhaseApply:       {TransientMax: 3, SemanticMax: 2, NoProgressCycles: 2},
	PhaseVerify:      {TransientMax: 3, SemanticMax: 2, NoProgressCycles: 2},
	PhaseArchive:     {TransientMax: 1, SemanticMax: 1, NoProgressCycles: 1},
}

// RetryState tracks the bounded retry counters for one phase execution. It
// enforces the transient, semantic, and no-progress ceilings from the policy.
type RetryState struct {
	Phase             PhaseID `json:"phase"`
	TransientAttempts int     `json:"transient_attempts"`
	SemanticAttempts  int     `json:"semantic_attempts"`
	NoProgressCycles  int     `json:"no_progress_cycles"`
}

// Validate ensures the state references a canonical phase.
func (s RetryState) Validate() error {
	return ValidatePhaseID(s.Phase)
}

// RecordTransient increments the transient attempt counter.
func (s *RetryState) RecordTransient() { s.TransientAttempts++ }

// RecordSemantic increments the semantic reflection counter.
func (s *RetryState) RecordSemantic() { s.SemanticAttempts++ }

// RecordNoProgress increments the no-progress cycle counter.
func (s *RetryState) RecordNoProgress() { s.NoProgressCycles++ }

// Exhausted reports whether any retry ceiling has been reached without further
// budget remaining, triggering a runaway halt per REQ-ORCH-003.
func (s RetryState) Exhausted(policy RetryPolicy) bool {
	if policy.TransientMax > 0 && s.TransientAttempts >= policy.TransientMax {
		return true
	}
	if policy.SemanticMax > 0 && s.SemanticAttempts >= policy.SemanticMax {
		return true
	}
	if policy.NoProgressCycles > 0 && s.NoProgressCycles >= policy.NoProgressCycles {
		return true
	}
	return false
}

// PolicyFor returns the canonical retry policy for a phase, or an error if the
// phase is unknown.
func PolicyFor(phase PhaseID) (RetryPolicy, error) {
	if err := ValidatePhaseID(phase); err != nil {
		return RetryPolicy{}, err
	}
	return RetryProfiles[phase], nil
}

// CeilingError describes a runaway halt reason for diagnostics.
func CeilingError(s RetryState, policy RetryPolicy) error {
	if !s.Exhausted(policy) {
		return nil
	}
	return fmt.Errorf("phase %q retry ceiling reached: transient=%d/%d semantic=%d/%d no-progress=%d/%d",
		s.Phase, s.TransientAttempts, policy.TransientMax, s.SemanticAttempts,
		policy.SemanticMax, s.NoProgressCycles, policy.NoProgressCycles)
}
