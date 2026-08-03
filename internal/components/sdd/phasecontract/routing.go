package phasecontract

import "fmt"

// authorizedTransitions is the phase DAG. design runs in parallel with spec
// (both consume proposal); tasks consumes both spec and design.
var authorizedTransitions = map[PhaseID]map[PhaseID]struct{}{
	PhaseBootstrap: {PhaseExplore: {}},
	PhaseExplore:   {PhaseProposal: {}},
	PhaseProposal:  {PhaseSpec: {}, PhaseDesign: {}},
	PhaseSpec:      {PhaseTasks: {}},
	PhaseDesign:    {PhaseTasks: {}},
	PhaseTasks:     {PhaseApply: {}},
	PhaseApply:     {PhaseVerify: {}},
	PhaseVerify:    {PhaseArchive: {}},
}

// AuthorizeTransition permits only the dependency-ordered transitions in the
// phase DAG. Skipping a phase, going backwards, or jumping across the pipeline
// is rejected so that no unauthorized transition can reach a contract.
func AuthorizeTransition(from, to PhaseID) error {
	if err := ValidatePhaseID(from); err != nil {
		return fmt.Errorf("from phase: %w", err)
	}
	if err := ValidatePhaseID(to); err != nil {
		return fmt.Errorf("to phase: %w", err)
	}
	allowed, ok := authorizedTransitions[from]
	if !ok {
		return fmt.Errorf("phase %q has no authorized successors", from)
	}
	if _, ok := allowed[to]; !ok {
		return fmt.Errorf("transition %q -> %q is not authorized by the phase DAG", from, to)
	}
	return nil
}
