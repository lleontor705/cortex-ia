package quality

import (
	"reflect"
	"testing"
)

func TestQualityPlanTemplateMatchesDesign(t *testing.T) {
	template := QualityPlanTemplate{
		SchemaVersion:   "1.0.0",
		PolicyVersion:   "1.0.0",
		PolicySHA256:    "deadbeef",
		ProfileID:       "profile/portable-sequential",
		Capabilities:    TestingCapabilities{DeterministicFocusedRunner: true, WritableTests: true, BaselineEvidence: true},
		TechniqueBounds: TechniqueBounds{},
		Degradations:    []string{"sequential-fallback"},
	}
	if err := template.Validate(); err != nil {
		t.Fatalf("valid QualityPlanTemplate.Validate() error = %v", err)
	}
	if template.ProfileID == "" {
		t.Fatal("ProfileID must be settable")
	}
}

func TestQualityPlanTemplateRejectsMissingPolicyFingerprint(t *testing.T) {
	template := QualityPlanTemplate{
		SchemaVersion: "1.0.0",
		PolicyVersion: "1.0.0",
		ProfileID:     "profile/portable-sequential",
	}
	if err := template.Validate(); err == nil {
		t.Fatal("QualityPlanTemplate without PolicySHA256.Validate() error = nil, want rejection")
	}
}

func TestQualityPlanMatchesDesign(t *testing.T) {
	plan := QualityPlan{
		SchemaVersion:       "1.0.0",
		PolicySHA256:        "policy-hash",
		TemplateSHA256:      "tpl-hash",
		ChangeSignalsSHA256: "signals-hash",
		Planning:            PlanningDecision{Depth: PlanningStandard},
		TDD:                 ApplicabilityRequired,
		Gherkin:             TechniqueDecision{Applicable: true, Reason: "visible"},
		Property:            TechniqueDecision{Reason: "off"},
		Fuzz:                TechniqueDecision{Reason: "off"},
		Mutation:            TechniqueDecision{Reason: "off"},
		RequiredEvidence:    []string{"focused-test", "diff-review"},
		BlockingReasons:     []string{},
	}
	if err := plan.Validate(); err != nil {
		t.Fatalf("valid QualityPlan.Validate() error = %v", err)
	}
}

func TestQualityPlanRejectsFingerprintMismatch(t *testing.T) {
	plan := QualityPlan{
		SchemaVersion:       "1.0.0",
		TemplateSHA256:      "tpl-hash",
		ChangeSignalsSHA256: "",
	}
	if err := plan.Validate(); err == nil {
		t.Fatal("QualityPlan without ChangeSignalsSHA256.Validate() error = nil, want rejection")
	}
}

func TestTestingCapabilitiesAliasesTestCapabilities(t *testing.T) {
	caps := TestCapabilities{
		DeterministicFocusedRunner: true,
		WritableTests:              true,
		BaselineEvidence:           true,
	}
	if !caps.VerticalTDDEligible() {
		t.Fatal("TestingCapabilities alias does not resolve VerticalTDDEligible from TestCapabilities")
	}
}

func TestCompilePolicySignatureAcceptsPolicyCapabilitiesProfile(t *testing.T) {
	policy := validQualityPolicy()
	caps := TestingCapabilities{DeterministicFocusedRunner: true, WritableTests: true, BaselineEvidence: true}
	profile := ProfilePlan{ProfileID: "profile/portable-sequential"}

	policyIR, template, err := CompilePolicy(policy, caps, profile)
	if err != nil {
		t.Fatalf("CompilePolicy() error = %v", err)
	}
	if policyIR.PolicySHA256 == "" {
		t.Fatal("CompilePolicy produced empty QualityPolicyIR.PolicySHA256")
	}
	if policyIR.PolicySHA256 != template.PolicySHA256 {
		t.Fatal("QualityPolicyIR and QualityPlanTemplate PolicySHA256 must match")
	}
	if template.ProfileID != profile.ProfileID {
		t.Fatalf("template ProfileID = %q, want %q", template.ProfileID, profile.ProfileID)
	}
	if !template.Capabilities.VerticalTDDEligible() {
		t.Fatal("template Capabilities did not preserve TDD eligibility")
	}
}

func TestCompilePolicyRejectsInvalidPolicy(t *testing.T) {
	_, _, err := CompilePolicy(QualityPolicy{}, TestingCapabilities{}, ProfilePlan{})
	if err == nil {
		t.Fatal("CompilePolicy with invalid policy error = nil, want rejection")
	}
}

func TestEvaluatePlanSignatureAcceptsIRTemplateSignals(t *testing.T) {
	policy := validQualityPolicy()
	caps := TestingCapabilities{DeterministicFocusedRunner: true, WritableTests: true, BaselineEvidence: true}
	profile := ProfilePlan{ProfileID: "profile/portable-sequential"}
	policyIR, template, err := CompilePolicy(policy, caps, profile)
	if err != nil {
		t.Fatalf("CompilePolicy() error = %v", err)
	}

	signals := ChangeSignals{
		ChangeName:         "improve-agent-phase-workflows",
		Kind:               ChangeBehavior,
		ObservableBehavior: true,
		Risk:               RiskMedium,
		Reversibility:      ReversibilityEasy,
		TrustBoundary:      TrustBoundaryInternal,
		DependencyBreadth:  DependencyPackage,
		MigrationImpact:    MigrationNone,
	}

	plan, err := EvaluatePlan(policyIR, template, signals)
	if err != nil {
		t.Fatalf("EvaluatePlan() error = %v", err)
	}
	if plan.TemplateSHA256 == "" {
		t.Fatal("EvaluatePlan produced empty TemplateSHA256")
	}
	if plan.ChangeSignalsSHA256 == "" {
		t.Fatal("EvaluatePlan produced empty ChangeSignalsSHA256")
	}
}

func TestEvaluatePlanRejectsFingerprintMismatch(t *testing.T) {
	signals := ChangeSignals{ChangeName: "x"}
	mismatchedIR := QualityPolicyIR{PolicySHA256: "ir-hash"}
	mismatchedTemplate := QualityPlanTemplate{
		SchemaVersion: "1.0.0",
		PolicyVersion: "1.0.0",
		PolicySHA256:  "different-hash",
		ProfileID:     "profile/portable-sequential",
	}
	if _, err := EvaluatePlan(mismatchedIR, mismatchedTemplate, signals); err == nil {
		t.Fatal("EvaluatePlan with mismatched fingerprints error = nil, want rejection")
	}
}

func TestEvaluatePlanIsImmutableAndPure(t *testing.T) {
	policy := validQualityPolicy()
	caps := TestingCapabilities{DeterministicFocusedRunner: true, WritableTests: true, BaselineEvidence: true}
	profile := ProfilePlan{ProfileID: "profile/portable-sequential"}
	policyIR, template, err := CompilePolicy(policy, caps, profile)
	if err != nil {
		t.Fatalf("CompilePolicy() error = %v", err)
	}
	signals := ChangeSignals{ChangeName: "improve", Kind: ChangeBehavior, ObservableBehavior: true, Risk: RiskLow}

	first, err := EvaluatePlan(policyIR, template, signals)
	if err != nil {
		t.Fatalf("EvaluatePlan first error = %v", err)
	}
	second, err := EvaluatePlan(policyIR, template, signals)
	if err != nil {
		t.Fatalf("EvaluatePlan second error = %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("EvaluatePlan is not pure: same inputs produced different QualityPlan values")
	}
}

func validQualityPolicy() QualityPolicy {
	return QualityPolicy{
		Version:  "1.0.0",
		Planning: []PlanningRule{{ID: "default", Depth: PlanningStandard}},
		TDD:      VerticalTDDPolicy{RequireWhenEligible: true},
		Gherkin: GherkinPolicy{
			EligibleBehaviors:         []BehaviorClass{BehaviorInstallation},
			RequireCompatiblePractice: true,
			RequireExecutableValue:    true,
		},
		Property: PropertyPolicy{ApplyTo: []TestApplicability{ApplicabilityUntrustedInput}, Budget: ActivityBudget{WallTime: 60000000000}},
		Fuzz:     FuzzPolicy{ApplyTo: []TestApplicability{ApplicabilityUntrustedInput}, Budget: ActivityBudget{WallTime: 60000000000}},
		Mutation: MutationPolicy{Mode: MutationReportOnly, Budget: ActivityBudget{WallTime: 120000000000}},
	}
}
