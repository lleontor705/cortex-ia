package quality

import "fmt"

// ApplicabilityStatus records whether a quality activity is required, skipped,
// or replaced by an explicit compensated exception.
type ApplicabilityStatus string

const (
	ApplicabilityNotApplicable ApplicabilityStatus = "not-applicable"
	ApplicabilityRequired      ApplicabilityStatus = "required"
	ApplicabilityException     ApplicabilityStatus = "exception"
)

type PlanningDecision struct {
	Depth             PlanningDepth
	MatchedRuleIDs    []string
	RequiredArtifacts []PlanningArtifact
	ApprovalRequired  bool
}

type TechniqueDecision struct {
	Applicable bool
	Reason     string
	Budget     ActivityBudget
}

type MutationContext struct {
	GreenBaseline              bool
	RelevantCoverage           bool
	PinnedCompatibleTool       bool
	ChangedCoveredCriticalCode bool
}

// EvaluationInput contains observable facts. It intentionally has no model
// confidence field because confidence cannot waive policy obligations.
type EvaluationInput struct {
	Change                     ChangeContext
	TDDEvidence                *VerticalTDDEvidence
	Behavior                   BehaviorClass
	StakeholderVisible         bool
	CompatibleGherkinPractice  bool
	ExecutableExamplesAddValue bool
	Mutation                   MutationContext
	TestSurfaces               []TestApplicability
}

type EvaluationDecision struct {
	Planning PlanningDecision
	TDD      ApplicabilityStatus
	Gherkin  TechniqueDecision
	Mutation TechniqueDecision
	Property TechniqueDecision
	Fuzz     TechniqueDecision
}

// Evaluate applies proportional planning and test-policy applicability to
// objective change and observability facts.
func Evaluate(policy QualityPolicy, input EvaluationInput) (EvaluationDecision, error) {
	if err := policy.Validate(); err != nil {
		return EvaluationDecision{}, err
	}

	decision := EvaluationDecision{Planning: selectPlanning(policy.Planning, input.Change)}
	tdd, err := evaluateTDD(policy.TDD, input.Change, input.TDDEvidence)
	if err != nil {
		return EvaluationDecision{}, err
	}
	decision.TDD = tdd
	decision.Gherkin = evaluateGherkin(policy.Gherkin, input)
	decision.Mutation = evaluateMutation(policy.Mutation, input.Mutation)
	decision.Property = evaluateSurface("property", policy.Property.ApplyTo, policy.Property.Budget, input.TestSurfaces)
	decision.Fuzz = evaluateSurface("fuzz", policy.Fuzz.ApplyTo, policy.Fuzz.Budget, input.TestSurfaces)
	return decision, nil
}

func selectPlanning(rules []PlanningRule, change ChangeContext) PlanningDecision {
	decision := PlanningDecision{Depth: PlanningNone}
	seenArtifacts := make(map[PlanningArtifact]struct{})
	for _, rule := range rules {
		if !rule.Selector.Matches(change) {
			continue
		}
		decision.MatchedRuleIDs = append(decision.MatchedRuleIDs, rule.ID)
		if planningRank(rule.Depth) > planningRank(decision.Depth) {
			decision.Depth = rule.Depth
		}
		decision.ApprovalRequired = decision.ApprovalRequired || rule.ApprovalRequired
		for _, artifact := range rule.RequiredArtifacts {
			if _, exists := seenArtifacts[artifact]; exists {
				continue
			}
			seenArtifacts[artifact] = struct{}{}
			decision.RequiredArtifacts = append(decision.RequiredArtifacts, artifact)
		}
	}
	return decision
}

func planningRank(depth PlanningDepth) int {
	switch depth {
	case PlanningFocused:
		return 1
	case PlanningStandard:
		return 2
	case PlanningFull:
		return 3
	default:
		return 0
	}
}

func evaluateTDD(policy VerticalTDDPolicy, change ChangeContext, evidence *VerticalTDDEvidence) (ApplicabilityStatus, error) {
	if !behaviorChanging(change) || !policy.RequireWhenEligible {
		return ApplicabilityNotApplicable, nil
	}
	if change.Tests.VerticalTDDEligible() {
		if evidence == nil || evidence.Exception != nil {
			return "", fmt.Errorf("strict TDD evidence is required for eligible behavior-changing work")
		}
		if err := evidence.Validate(); err != nil {
			return "", fmt.Errorf("strict TDD evidence: %w", err)
		}
		return ApplicabilityRequired, nil
	}
	if evidence == nil || evidence.Exception == nil {
		return "", fmt.Errorf("TDD exception is required when behavior-changing work is ineligible")
	}
	if err := evidence.Validate(); err != nil {
		return "", err
	}
	return ApplicabilityException, nil
}

func behaviorChanging(change ChangeContext) bool {
	if change.ObservableBehavior {
		return true
	}
	switch change.Kind {
	case ChangeBehavior, ChangeSecurity, ChangeMigration:
		return true
	default:
		return false
	}
}

func evaluateGherkin(policy GherkinPolicy, input EvaluationInput) TechniqueDecision {
	decision := TechniqueDecision{Reason: "requires stakeholder-visible eligible behavior"}
	if !input.StakeholderVisible || !contains(policy.EligibleBehaviors, input.Behavior) {
		return decision
	}
	if policy.RequireCompatiblePractice && !input.CompatibleGherkinPractice {
		decision.Reason = "compatible Gherkin practice is unavailable"
		return decision
	}
	if policy.RequireExecutableValue && !input.ExecutableExamplesAddValue {
		decision.Reason = "executable examples add no shared-understanding value"
		return decision
	}
	decision.Applicable = true
	decision.Reason = "stakeholder-visible executable behavior"
	return decision
}

func evaluateMutation(policy MutationPolicy, context MutationContext) TechniqueDecision {
	decision := TechniqueDecision{Budget: policy.Budget, Reason: "mutation policy is off or prerequisites are unmet"}
	if policy.Mode == MutationOff || !context.ChangedCoveredCriticalCode {
		return decision
	}
	if policy.RequireGreenBaseline && !context.GreenBaseline ||
		policy.RequireRelevantCoverage && !context.RelevantCoverage ||
		policy.RequirePinnedTool && !context.PinnedCompatibleTool {
		return decision
	}
	decision.Applicable = true
	decision.Reason = "changed covered critical code satisfies mutation prerequisites"
	return decision
}

func evaluateSurface(name string, allowed []TestApplicability, budget ActivityBudget, observed []TestApplicability) TechniqueDecision {
	decision := TechniqueDecision{Budget: budget, Reason: name + " requires a meaningful declared surface"}
	for _, surface := range observed {
		if contains(allowed, surface) {
			decision.Applicable = true
			decision.Reason = name + " applies to " + string(surface)
			return decision
		}
	}
	return decision
}
