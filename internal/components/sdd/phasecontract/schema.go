package phasecontract

import "fmt"

// PhaseID identifies one canonical SDD phase. Phases not in this set cannot
// receive a contract or transition.
type PhaseID string

const (
	PhaseBootstrap   = PhaseID("bootstrap")
	PhaseInvestigate = PhaseID("investigate")
	PhasePropose     = PhaseID("propose")
	PhaseSpec        = PhaseID("spec")
	PhaseDesign      = PhaseID("design")
	PhaseTasks       = PhaseID("tasks")
	PhaseApply       = PhaseID("apply")
	PhaseVerify      = PhaseID("verify")
	PhaseArchive     = PhaseID("archive")
)

var retainedPhases = map[PhaseID]struct{}{
	PhaseBootstrap: {}, PhaseInvestigate: {}, PhasePropose: {}, PhaseSpec: {},
	PhaseDesign: {}, PhaseTasks: {}, PhaseApply: {}, PhaseVerify: {}, PhaseArchive: {},
}

// ValidatePhaseID rejects phases outside the nine-phase pipeline so that no
// foreign phase can receive a contract.
func ValidatePhaseID(id PhaseID) error {
	if _, ok := retainedPhases[id]; !ok {
		return fmt.Errorf("phase %q is not a canonical SDD phase", id)
	}
	return nil
}

// BootstrapInput carries the operator request and the freshness-stamped
// capability probes that bootstrap consumes.
type BootstrapInput struct {
	Request ArtifactRef   `json:"request"`
	Probes  []ArtifactRef `json:"probes"`
}

func (i BootstrapInput) Validate() error {
	if err := i.Request.Validate(); err != nil {
		return fmt.Errorf("bootstrap request: %w", err)
	}
	if len(i.Probes) == 0 {
		return fmt.Errorf("bootstrap requires at least one capability probe")
	}
	for j, probe := range i.Probes {
		if err := probe.Validate(); err != nil {
			return fmt.Errorf("bootstrap probe %d: %w", j, err)
		}
	}
	return nil
}

// InvestigateInput carries the request plus the bootstrap context.
type InvestigateInput struct {
	Request   ArtifactRef `json:"request"`
	Bootstrap ArtifactRef `json:"bootstrap"`
}

func (i InvestigateInput) Validate() error {
	if err := i.Request.Validate(); err != nil {
		return fmt.Errorf("investigate request: %w", err)
	}
	if err := i.Bootstrap.Validate(); err != nil {
		return fmt.Errorf("investigate bootstrap: %w", err)
	}
	return nil
}

// ProposalInput carries the exploration and operator input (scope/risk/acceptance
// traceability). Operator input is semi-trusted and never authority.
type ProposalInput struct {
	Exploration ArtifactRef `json:"exploration"`
	Operator    ArtifactRef `json:"operator"`
}

func (i ProposalInput) Validate() error {
	if err := i.Exploration.Validate(); err != nil {
		return fmt.Errorf("proposal exploration: %w", err)
	}
	if i.Operator.SHA256 == "" {
		return fmt.Errorf("proposal requires operator input")
	}
	return nil
}

// SpecInput carries the approved proposal and the QualityPlan.
type SpecInput struct {
	Proposal    ArtifactRef `json:"proposal"`
	QualityPlan ArtifactRef `json:"quality_plan"`
}

func (i SpecInput) Validate() error {
	if err := i.Proposal.Validate(); err != nil {
		return fmt.Errorf("spec proposal: %w", err)
	}
	if err := i.QualityPlan.Validate(); err != nil {
		return fmt.Errorf("spec quality plan: %w", err)
	}
	return nil
}

// DesignInput carries the proposal and at least two alternatives/evidence so
// that a single-approach design cannot pass.
type DesignInput struct {
	Proposal ArtifactRef   `json:"proposal"`
	Evidence []ArtifactRef `json:"evidence"`
}

func (i DesignInput) Validate() error {
	if err := i.Proposal.Validate(); err != nil {
		return fmt.Errorf("design proposal: %w", err)
	}
	if len(i.Evidence) < 2 {
		return fmt.Errorf("design requires at least two alternatives/evidence")
	}
	for j, ref := range i.Evidence {
		if err := ref.Validate(); err != nil {
			return fmt.Errorf("design evidence %d: %w", j, err)
		}
	}
	return nil
}

// TasksInput consumes the approved specification AND design AND QualityPlan,
// satisfying REQ-PLAN-001 (tasks consumes spec and design).
type TasksInput struct {
	Specification ArtifactRef `json:"specification"`
	Design        ArtifactRef `json:"design"`
	QualityPlan   ArtifactRef `json:"quality_plan"`
}

func (i TasksInput) Validate() error {
	if err := i.Specification.Validate(); err != nil {
		return fmt.Errorf("tasks specification: %w", err)
	}
	if err := i.Design.Validate(); err != nil {
		return fmt.Errorf("tasks design: %w", err)
	}
	if err := i.QualityPlan.Validate(); err != nil {
		return fmt.Errorf("tasks quality plan: %w", err)
	}
	return nil
}

// ApplyInput carries one ready task plus spec/design/plan and the optional
// previous apply-progress for batch continuation.
type ApplyInput struct {
	Task             ArtifactRef  `json:"task"`
	Specification    ArtifactRef  `json:"specification"`
	Design           ArtifactRef  `json:"design"`
	QualityPlan      ArtifactRef  `json:"quality_plan"`
	PreviousProgress *ArtifactRef `json:"previous_progress,omitempty"`
}

func (i ApplyInput) Validate() error {
	if err := i.Task.Validate(); err != nil {
		return fmt.Errorf("apply task: %w", err)
	}
	if err := i.Specification.Validate(); err != nil {
		return fmt.Errorf("apply specification: %w", err)
	}
	if err := i.Design.Validate(); err != nil {
		return fmt.Errorf("apply design: %w", err)
	}
	if err := i.QualityPlan.Validate(); err != nil {
		return fmt.Errorf("apply quality plan: %w", err)
	}
	if i.PreviousProgress != nil {
		if err := i.PreviousProgress.Validate(); err != nil {
			return fmt.Errorf("apply previous progress: %w", err)
		}
	}
	return nil
}

// VerifyInput consumes spec/tasks/plan and the apply-progress produced by apply.
type VerifyInput struct {
	Specification ArtifactRef `json:"specification"`
	Tasks         ArtifactRef `json:"tasks"`
	QualityPlan   ArtifactRef `json:"quality_plan"`
	ApplyProgress ArtifactRef `json:"apply_progress"`
}

func (i VerifyInput) Validate() error {
	if err := i.Specification.Validate(); err != nil {
		return fmt.Errorf("verify specification: %w", err)
	}
	if err := i.Tasks.Validate(); err != nil {
		return fmt.Errorf("verify tasks: %w", err)
	}
	if err := i.QualityPlan.Validate(); err != nil {
		return fmt.Errorf("verify quality plan: %w", err)
	}
	if err := i.ApplyProgress.Validate(); err != nil {
		return fmt.Errorf("verify apply progress: %w", err)
	}
	return nil
}

// ArchiveInput consumes the verification and full lineage.
type ArchiveInput struct {
	Verification ArtifactRef   `json:"verification"`
	Lineage      []ArtifactRef `json:"lineage"`
}

func (i ArchiveInput) Validate() error {
	if err := i.Verification.Validate(); err != nil {
		return fmt.Errorf("archive verification: %w", err)
	}
	if len(i.Lineage) == 0 {
		return fmt.Errorf("archive requires lineage evidence")
	}
	for j, ref := range i.Lineage {
		if err := ref.Validate(); err != nil {
			return fmt.Errorf("archive lineage %d: %w", j, err)
		}
	}
	return nil
}

// PhaseSchema binds the per-phase budget and stop policy from design §9. These
// are compile-time facts, not runtime claims.
type PhaseSchema struct {
	Budget Budget     `json:"budget"`
	Stops  StopPolicy `json:"stops"`
}

// PhaseSchemas is the canonical budget/stop table for all nine phases, matching
// design §9 and §5.
var PhaseSchemas = map[PhaseID]PhaseSchema{
	PhaseBootstrap: {
		Budget: Budget{MaxFileReads: 8, MaxToolCalls: 10},
		Stops:  StopPolicy{Completion: []string{"context"}, Blocking: []string{"missing-root", "unsafe-probe", "contradiction", "store-unavailable"}, Failure: []string{"fatal"}},
	},
	PhaseInvestigate: {
		Budget: Budget{MaxFileReads: 4, CheckpointAtFiles: 4},
		Stops:  StopPolicy{Completion: []string{"current-map"}, Blocking: []string{"unsupported-claim", "conflicting-evidence"}, Failure: []string{"fatal"}},
	},
	PhasePropose: {
		Budget: Budget{MaxOutputTokens: 3500},
		Stops:  StopPolicy{Completion: []string{"proposal"}, Blocking: []string{"unresolved-product-decision", "rollback-absent", "acceptance-unverifiable"}, Failure: []string{"fatal"}},
	},
	PhaseSpec: {
		Budget: Budget{MaxOutputTokens: 3500},
		Stops:  StopPolicy{Completion: []string{"spec"}, Blocking: []string{"uncovered-criterion", "ambiguity", "non-executable-scenario", "gherkin-misuse"}, Failure: []string{"fatal"}},
	},
	PhaseDesign: {
		Budget: Budget{MaxOutputTokens: 4000},
		Stops:  StopPolicy{Completion: []string{"design"}, Blocking: []string{"unresolved-compatibility", "unresolved-isolation", "unresolved-rollback", "mandatory-high-risk-gate"}, Failure: []string{"fatal"}},
	},
	PhaseTasks: {
		Budget: Budget{},
		Stops:  StopPolicy{Completion: []string{"board"}, Blocking: []string{"missing-input", "cycle", "coverage-gap", "size-decision", "false-readiness"}, Failure: []string{"fatal"}},
	},
	PhaseApply: {
		Budget: Budget{},
		Stops:  StopPolicy{Completion: []string{"done"}, Blocking: []string{"claim-failure", "scope-creep", "policy-violation", "test-failure", "review-failure", "isolation-failure", "retry-failure"}, Failure: []string{"fatal"}},
	},
	PhaseVerify: {
		Budget: Budget{MaxOutputTokens: 4000},
		Stops:  StopPolicy{Completion: []string{"pass"}, Blocking: []string{"blocked", "inconclusive-gate"}, Failure: []string{"fail"}},
	},
	PhaseArchive: {
		Budget: Budget{MaxOutputTokens: 3000},
		Stops:  StopPolicy{Completion: []string{"archived"}, Blocking: []string{"verify-not-pass", "nonterminal-task", "missing-lineage"}, Failure: []string{"fatal"}},
	},
}
