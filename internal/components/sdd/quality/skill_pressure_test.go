package quality

import (
	"context"
	"testing"
)

func TestSkillPressureReportRequiresBaselineFailureAndControls(t *testing.T) {
	report := validSkillPressureReport()
	if err := report.Validate(); err != nil {
		t.Fatalf("valid report rejected: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*SkillPressureReport)
	}{
		{name: "baseline did not fail", mutate: func(r *SkillPressureReport) { r.Scenarios[0].Baseline.ObservedFailure = false }},
		{name: "missing control", mutate: func(r *SkillPressureReport) { r.Scenarios[0].Controls = nil }},
		{name: "missing rerun", mutate: func(r *SkillPressureReport) { r.Scenarios[0].Reruns = nil }},
		{name: "changed replay input", mutate: func(r *SkillPressureReport) { r.Scenarios[0].Reruns[0].ReplayInputDigest = "different" }},
		{name: "prose only outcome", mutate: func(r *SkillPressureReport) { r.Scenarios[0].WithSkill.SemanticOutcome = "" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidate := validSkillPressureReport()
			tt.mutate(&candidate)
			if err := candidate.Validate(); err == nil {
				t.Fatal("Validate() error = nil, want rejection")
			}
		})
	}
}

func TestSkillPressureReportSupportsThreeScenariosAndReportOnlyDisposition(t *testing.T) {
	report := validSkillPressureReport()
	report.Scenarios = []PressureScenario{validPressureScenario("scenario-1"), validPressureScenario("scenario-2"), validPressureScenario("scenario-3")}
	report.Disposition = PressureDispositionInconclusive
	if err := report.Validate(); err != nil {
		t.Fatalf("report with three scenarios rejected: %v", err)
	}
	if report.BlockingImpact() {
		t.Fatal("report-only pressure finding changed blocking impact")
	}
}

func TestSkillPressurePromotionFailsClosedUntilPolicyApproval(t *testing.T) {
	criteria := PromotionCriteria{
		ReportOnlyCycles: 2, RequiredCycles: 2,
		StableReplays: 3, RequiredReplays: 3,
		ZeroUnexplainedVariance: true, CompleteAttribution: true,
		CriticalHighDefects: 0, SeededControlsByClass: map[string]int{"skill-quality": 1},
		ApprovedPolicyRevision: "",
	}
	decision := EvaluateSkillPressurePromotion(criteria)
	if decision.Eligible || decision.Reason != PromotionReasonApprovalMissing {
		t.Fatalf("promotion decision = %#v, want approval_missing", decision)
	}
	criteria.ApprovedPolicyRevision = "quality-policy-v2"
	decision = EvaluateSkillPressurePromotion(criteria)
	if !decision.Eligible || decision.Reason != PromotionReasonEligible {
		t.Fatalf("complete promotion criteria = %#v, want eligible", decision)
	}
}

func TestDeterministicPressureRunnerUsesSemanticOutcomes(t *testing.T) {
	runner := DeterministicPressureRunner{Outcomes: map[string]PressureRunOutcome{
		"baseline":   {SemanticOutcome: "waived", ObservedFailure: true},
		"with-skill": {SemanticOutcome: "required", ObservedFailure: false},
		"control":    {SemanticOutcome: "unchanged", ObservedFailure: false},
	}}
	got, err := runner.Run(context.Background(), PressureRunRequest{RunID: "with-skill"})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got.SemanticOutcome != "required" || got.ObservedFailure {
		t.Fatalf("Run() = %#v, want semantic required success", got)
	}
}

func validSkillPressureReport() SkillPressureReport {
	return SkillPressureReport{
		SchemaVersion: "1.0.0", SkillID: "skill-improver", SkillVersion: "1.1.0", SkillSHA256: "sha256:skill",
		HarnessRevision: "fixture-v1", ProfileID: "profile/portable-sequential", PolicySHA256: "sha256:policy",
		Scenarios:    []PressureScenario{validPressureScenario("scenario-1"), validPressureScenario("scenario-2"), validPressureScenario("scenario-3")},
		EvidenceRefs: []EvidenceRef{{ID: "evidence-1", Digest: "sha256:evidence"}}, Confidence: 0.9,
		Environment: PressureEnvironment{OS: "test", GoVersion: "fixture", ExternalEvaluator: false},
		Disposition: PressureDispositionPass, SeededControls: SeededControls{ByClass: map[string]int{"skill-quality": 1}},
		Promotion: PromotionDecision{Reason: PromotionReasonApprovalMissing},
	}
}

func validPressureScenario(id string) PressureScenario {
	return PressureScenario{
		ScenarioID: id, ExpectedInvariant: "with-skill selects required behavior", ReplayInputDigest: "sha256:input",
		ObservedOutcome: "required",
		Baseline:        PressureRun{Kind: PressureRunBaseline, ReplayInputDigest: "sha256:input", SemanticOutcome: "waived", ObservedFailure: true, ExpectedFailure: true},
		Controls:        []PressureRun{{Kind: PressureRunControl, ReplayInputDigest: "sha256:input", SemanticOutcome: "unchanged"}},
		WithSkill:       PressureRun{Kind: PressureRunWithSkill, ReplayInputDigest: "sha256:input", SemanticOutcome: "required"},
		MicroTests:      []PressureRun{{Kind: PressureRunMicro, ReplayInputDigest: "sha256:input", SemanticOutcome: "required"}},
		Reruns:          []PressureRun{{Kind: PressureRunRerun, ReplayInputDigest: "sha256:input", SemanticOutcome: "required"}},
	}
}
