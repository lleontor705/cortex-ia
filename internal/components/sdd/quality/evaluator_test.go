package quality

import (
	"strings"
	"testing"
	"time"
)

func TestEvaluatorSelectsPlanningFromRiskAndObservableBehavior(t *testing.T) {
	high := RiskHigh
	observable := true
	policy := QualityPolicy{
		Version: "1.0.0",
		Planning: []PlanningRule{
			{ID: "observable", Selector: ChangeSelector{ObservableBehavior: &observable}, Depth: PlanningFocused, RequiredArtifacts: []PlanningArtifact{ArtifactSpecification}},
			{ID: "high-risk", Selector: ChangeSelector{MinimumRisk: &high}, Depth: PlanningFull, RequiredArtifacts: []PlanningArtifact{ArtifactProposal, ArtifactSpecification, ArtifactDesign, ArtifactTasks}, ApprovalRequired: true},
		},
	}

	tests := []struct {
		name          string
		change        ChangeContext
		wantDepth     PlanningDepth
		wantRules     []string
		wantApproval  bool
		wantArtifacts int
	}{
		{name: "observable low risk gets focused planning", change: ChangeContext{ObservableBehavior: true, Risk: RiskLow}, wantDepth: PlanningFocused, wantRules: []string{"observable"}, wantArtifacts: 1},
		{name: "high risk retains full planning", change: ChangeContext{ObservableBehavior: true, Risk: RiskCritical}, wantDepth: PlanningFull, wantRules: []string{"observable", "high-risk"}, wantApproval: true, wantArtifacts: 4},
		{name: "unmatched change gets no planning", change: ChangeContext{Risk: RiskLow}, wantDepth: PlanningNone},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision, err := Evaluate(policy, EvaluationInput{Change: tt.change})
			if err != nil {
				t.Fatalf("Evaluate() error = %v", err)
			}
			if decision.Planning.Depth != tt.wantDepth || decision.Planning.ApprovalRequired != tt.wantApproval {
				t.Fatalf("planning = %#v, want depth %q approval %v", decision.Planning, tt.wantDepth, tt.wantApproval)
			}
			if len(decision.Planning.MatchedRuleIDs) != len(tt.wantRules) || len(decision.Planning.RequiredArtifacts) != tt.wantArtifacts {
				t.Fatalf("planning = %#v, want rules %v and %d artifacts", decision.Planning, tt.wantRules, tt.wantArtifacts)
			}
		})
	}
}

func TestEvaluatorRequiresStrictTDDEvidenceOrCompensatedException(t *testing.T) {
	policy := QualityPolicy{Version: "1.0.0", TDD: VerticalTDDPolicy{RequireWhenEligible: true}}
	eligible := ChangeContext{Kind: ChangeBehavior, ObservableBehavior: true, Tests: TestCapabilities{DeterministicFocusedRunner: true, WritableTests: true, BaselineEvidence: true}}
	redAt := time.Date(2026, 7, 26, 10, 0, 0, 0, time.UTC)
	valid := &VerticalTDDEvidence{
		RequirementIDs: []string{"REQ-QUAL-002"}, CommitOrTreeState: "tree:abc", Command: "go test ./quality", WorkingDirectory: "D:/repo",
		RedAt: redAt, ProductionChangedAt: redAt.Add(time.Second), RedExitStatus: 1, RedFailureFingerprint: "undefined: Evaluate",
		Green: TestResult{At: redAt.Add(2 * time.Second)}, Refactor: TestResult{At: redAt.Add(3 * time.Second)},
		Artifacts: []string{"evaluator_test.go", "evaluator.go"}, VerificationLevel: VerificationUnit,
	}

	tests := []struct {
		name    string
		input   EvaluationInput
		wantTDD ApplicabilityStatus
		wantErr string
	}{
		{name: "valid strict evidence passes", input: EvaluationInput{Change: eligible, TDDEvidence: valid}, wantTDD: ApplicabilityRequired},
		{name: "production before red fails", input: EvaluationInput{Change: eligible, TDDEvidence: func() *VerticalTDDEvidence {
			copy := *valid
			copy.ProductionChangedAt = redAt.Add(-time.Second)
			return &copy
		}()}, wantErr: "production changed before RED"},
		{name: "production simultaneous with red is not ordered evidence", input: EvaluationInput{Change: eligible, TDDEvidence: func() *VerticalTDDEvidence { copy := *valid; copy.ProductionChangedAt = redAt; return &copy }()}, wantErr: "production changed before RED"},
		{name: "command must be normalized", input: EvaluationInput{Change: eligible, TDDEvidence: func() *VerticalTDDEvidence { copy := *valid; copy.Command = " go  test   ./quality "; return &copy }()}, wantErr: "command must be normalized"},
		{name: "narrative mode cannot replace evidence", input: EvaluationInput{Change: eligible}, wantErr: "strict TDD evidence is required"},
		{name: "ineligible work needs compensated exception", input: EvaluationInput{Change: ChangeContext{Kind: ChangeBehavior}, TDDEvidence: &VerticalTDDEvidence{Exception: &TDDException{Reason: TDDUnavailableRunner, Detail: "no focused runner", CompensatingEvidence: []string{"contract test"}}}}, wantTDD: ApplicabilityException},
		{name: "ineligible work without exception fails", input: EvaluationInput{Change: ChangeContext{Kind: ChangeBehavior}}, wantErr: "TDD exception is required"},
		{name: "non behavior work does not require TDD", input: EvaluationInput{Change: ChangeContext{Kind: ChangeDocumentation}}, wantTDD: ApplicabilityNotApplicable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision, err := Evaluate(policy, tt.input)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("Evaluate() error = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Evaluate() error = %v", err)
			}
			if decision.TDD != tt.wantTDD {
				t.Fatalf("TDD = %q, want %q", decision.TDD, tt.wantTDD)
			}
		})
	}
}

func TestTDDExceptionRejectsUnknownReason(t *testing.T) {
	evidence := VerticalTDDEvidence{Exception: &TDDException{Reason: TDDExceptionReason("because"), Detail: "unsupported excuse", CompensatingEvidence: []string{"review"}}}
	if err := evidence.Validate(); err == nil || !strings.Contains(err.Error(), "recognized reason") {
		t.Fatalf("Validate() error = %v, want recognized reason error", err)
	}
}

func TestEvaluatorAppliesOptionalTechniquesConditionally(t *testing.T) {
	policy := QualityPolicy{
		Version:  "1.0.0",
		Gherkin:  GherkinPolicy{EligibleBehaviors: []BehaviorClass{BehaviorInstallation}, RequireCompatiblePractice: true, RequireExecutableValue: true},
		Mutation: MutationPolicy{Mode: MutationReportOnly, RequireGreenBaseline: true, RequireRelevantCoverage: true, RequirePinnedTool: true, Scope: MutationChangedCoveredCritical, Budget: ActivityBudget{Mutants: 10}},
		Property: PropertyPolicy{ApplyTo: []TestApplicability{ApplicabilityMeaningfulInvariant}, Budget: ActivityBudget{Cases: 100}},
		Fuzz:     FuzzPolicy{ApplyTo: []TestApplicability{ApplicabilityUntrustedInput}, PRCorpusReplay: true, Budget: ActivityBudget{FuzzDuration: time.Second}},
	}

	tests := []struct {
		name         string
		input        EvaluationInput
		wantGherkin  bool
		wantMutation bool
		wantProperty bool
		wantFuzz     bool
	}{
		{name: "all applicable", input: EvaluationInput{Behavior: BehaviorInstallation, StakeholderVisible: true, CompatibleGherkinPractice: true, ExecutableExamplesAddValue: true, Mutation: MutationContext{GreenBaseline: true, RelevantCoverage: true, PinnedCompatibleTool: true, ChangedCoveredCriticalCode: true}, TestSurfaces: []TestApplicability{ApplicabilityMeaningfulInvariant, ApplicabilityUntrustedInput}}, wantGherkin: true, wantMutation: true, wantProperty: true, wantFuzz: true},
		{name: "internal algorithm skips optional techniques", input: EvaluationInput{Behavior: BehaviorDiagnostic, Mutation: MutationContext{GreenBaseline: true}, TestSurfaces: nil}},
		{name: "mutation prerequisites are mandatory", input: EvaluationInput{Mutation: MutationContext{ChangedCoveredCriticalCode: true}}, wantMutation: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision, err := Evaluate(policy, tt.input)
			if err != nil {
				t.Fatalf("Evaluate() error = %v", err)
			}
			if decision.Gherkin.Applicable != tt.wantGherkin || decision.Mutation.Applicable != tt.wantMutation || decision.Property.Applicable != tt.wantProperty || decision.Fuzz.Applicable != tt.wantFuzz {
				t.Fatalf("techniques = %#v, want gherkin=%v mutation=%v property=%v fuzz=%v", decision, tt.wantGherkin, tt.wantMutation, tt.wantProperty, tt.wantFuzz)
			}
		})
	}
}

func TestEvaluateActivityNeverPassesExhaustion(t *testing.T) {
	budget := ActivityBudget{WallTime: time.Second, ToolCalls: 2, Cases: 10}
	tests := []struct {
		name      string
		usage     ActivityUsage
		completed bool
		want      OutcomeStatus
		wantCause TerminationCause
	}{
		{name: "within budget can pass", usage: ActivityUsage{WallTime: time.Second, ToolCalls: 2, Cases: 10}, completed: true, want: OutcomePass, wantCause: TerminationNone},
		{name: "wall time exhaustion is inconclusive", usage: ActivityUsage{WallTime: time.Second + 1}, completed: true, want: OutcomeInconclusive, wantCause: TerminationBudgetExhausted},
		{name: "case exhaustion is inconclusive", usage: ActivityUsage{Cases: 11}, completed: true, want: OutcomeInconclusive, wantCause: TerminationBudgetExhausted},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outcome := EvaluateActivity(budget, tt.usage, tt.completed, []string{"partial.json"})
			if outcome.Status != tt.want || outcome.Cause != tt.wantCause {
				t.Fatalf("EvaluateActivity() = %#v, want status %q cause %q", outcome, tt.want, tt.wantCause)
			}
			if tt.wantCause == TerminationBudgetExhausted && len(outcome.PartialEvidence) == 0 {
				t.Fatal("budget exhaustion must preserve partial evidence")
			}
		})
	}
}

func TestActivityBudgetExhaustionIsMonotonic(t *testing.T) {
	budget := ActivityBudget{Cases: 100}
	for cases := 101; cases < 1000; cases += 17 {
		if !budget.ExhaustedBy(ActivityUsage{Cases: cases}) {
			t.Fatalf("budget ceased to be exhausted at %d cases", cases)
		}
	}
}
