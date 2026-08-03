package quality

import "testing"

func TestBuildPlanConsumesPolicyIRTemplateAndSignals(t *testing.T) {
	policy := validQualityPolicy()
	policy.TDD.RequireWhenEligible = false
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
	plan, trace, err := BuildPlan(PipelineInput{
		Policy: policy,
		Capabilities: TestingCapabilities{
			DeterministicFocusedRunner: true,
			WritableTests:              true,
			BaselineEvidence:           true,
		},
		Profile: ProfilePlan{ProfileID: "profile/portable-sequential"},
		Signals: signals,
		Evaluation: EvaluationInput{Change: ChangeContext{
			Kind: ChangeBehavior, ObservableBehavior: true, Risk: RiskMedium,
			Reversibility: ReversibilityEasy, TrustBoundary: TrustBoundaryInternal,
			DependencyBreadth: DependencyPackage, MigrationImpact: MigrationNone,
		}},
	})
	if err != nil {
		t.Fatalf("BuildPlan() error = %v", err)
	}
	if err := plan.Validate(); err != nil {
		t.Fatalf("BuildPlan() returned invalid plan: %v", err)
	}
	if trace.PolicySHA256 == "" || trace.TemplateSHA256 == "" || trace.PlanSHA256 == "" {
		t.Fatalf("quality trace is incomplete: %+v", trace)
	}
	if plan.PolicySHA256 != trace.PolicySHA256 {
		t.Fatalf("plan policy fingerprint = %q, trace = %q", plan.PolicySHA256, trace.PolicySHA256)
	}
}
