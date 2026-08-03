package quality

import "fmt"

type PlanningDepth string

const (
	PlanningNone     PlanningDepth = "none"
	PlanningFocused  PlanningDepth = "focused"
	PlanningStandard PlanningDepth = "standard"
	PlanningFull     PlanningDepth = "full"
)

type PlanningArtifact string

const (
	ArtifactProposal      PlanningArtifact = "proposal"
	ArtifactSpecification PlanningArtifact = "specification"
	ArtifactDesign        PlanningArtifact = "design"
	ArtifactTasks         PlanningArtifact = "tasks"
)

type PlanningRule struct {
	ID                string
	Selector          ChangeSelector
	Depth             PlanningDepth
	RequiredArtifacts []PlanningArtifact
	ApprovalRequired  bool
}

type VerticalTDDPolicy struct {
	RequireWhenEligible bool
}

type BehaviorClass string

const (
	BehaviorGenerator    BehaviorClass = "generator"
	BehaviorInstallation BehaviorClass = "installation"
	BehaviorDiagnostic   BehaviorClass = "diagnostic"
	BehaviorDegradation  BehaviorClass = "degradation"
	BehaviorRollback     BehaviorClass = "rollback"
)

type GherkinPolicy struct {
	EligibleBehaviors         []BehaviorClass
	RequireCompatiblePractice bool
	RequireExecutableValue    bool
}

type ArchitecturePrinciple string

const (
	PrincipleDomainVisibility    ArchitecturePrinciple = "domain-visibility"
	PrincipleInwardDependencies  ArchitecturePrinciple = "inward-dependencies"
	PrincipleExternalIsolation   ArchitecturePrinciple = "external-detail-isolation"
	PrincipleBoundaryTestability ArchitecturePrinciple = "boundary-testability"
)

type ArchitectureGate struct {
	Analyzer string
	RuleIDs  []string
}

type ArchitecturePolicy struct {
	Principles []ArchitecturePrinciple
	Gate       *ArchitectureGate
}

type MutationMode string

const (
	MutationOff        MutationMode = "off"
	MutationReportOnly MutationMode = "report-only"
	MutationGate       MutationMode = "gate"
)

type MutationScope string

const MutationChangedCoveredCritical MutationScope = "changed-covered-critical"

type MutationPolicy struct {
	Mode                    MutationMode
	RequireGreenBaseline    bool
	RequireRelevantCoverage bool
	RequirePinnedTool       bool
	Scope                   MutationScope
	Exclusions              []string
	AttributableDeltaRules  []string
	Budget                  ActivityBudget
}

type TestApplicability string

const (
	ApplicabilityMeaningfulInvariant TestApplicability = "meaningful-invariant"
	ApplicabilityUntrustedInput      TestApplicability = "untrusted-input"
)

type PropertyPolicy struct {
	ApplyTo []TestApplicability
	Budget  ActivityBudget
}

type FuzzPolicy struct {
	ApplyTo         []TestApplicability
	PRCorpusReplay  bool
	CommittedCorpus []string
	Budget          ActivityBudget
}

type QualityPolicy struct {
	Version      string
	Planning     []PlanningRule
	TDD          VerticalTDDPolicy
	Gherkin      GherkinPolicy
	Architecture ArchitecturePolicy
	Mutation     MutationPolicy
	Property     PropertyPolicy
	Fuzz         FuzzPolicy
}

func (p QualityPolicy) Validate() error {
	if p.Version == "" {
		return fmt.Errorf("quality policy version is required")
	}
	for _, rule := range p.Planning {
		if rule.ID == "" || rule.Depth == "" {
			return fmt.Errorf("planning rules require an ID and depth")
		}
		if ruleNeedsApproval(rule.Selector) && !rule.ApprovalRequired {
			return fmt.Errorf("high-risk, security, migration, or irreversible planning rule %q requires approval", rule.ID)
		}
	}
	if gate := p.Architecture.Gate; gate != nil && (gate.Analyzer == "" || len(gate.RuleIDs) == 0) {
		return fmt.Errorf("hard architecture gate requires a compatible analyzer and actionable rule IDs")
	}
	if p.Mutation.Mode == MutationGate && len(p.Mutation.AttributableDeltaRules) == 0 {
		return fmt.Errorf("mutation gate requires project-owned attributable delta rules")
	}
	for name, budget := range map[string]ActivityBudget{
		"mutation": p.Mutation.Budget,
		"property": p.Property.Budget,
		"fuzz":     p.Fuzz.Budget,
	} {
		if err := budget.Validate(); err != nil {
			return fmt.Errorf("%s budget: %w", name, err)
		}
	}
	return nil
}

func ruleNeedsApproval(selector ChangeSelector) bool {
	if selector.MinimumRisk != nil && *selector.MinimumRisk >= RiskHigh {
		return true
	}
	if containsOrAnyExceptEmpty(selector.Kinds, ChangeSecurity) || containsOrAnyExceptEmpty(selector.Kinds, ChangeMigration) {
		return true
	}
	return containsOrAnyExceptEmpty(selector.Reversibility, ReversibilityImpossible)
}

func containsOrAnyExceptEmpty[T comparable](values []T, wanted T) bool {
	if len(values) == 0 {
		return false
	}
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
