package quality

import (
	"testing"
	"time"
)

func TestChangeSelectorMatches(t *testing.T) {
	security := ChangeSecurity
	high := RiskHigh
	behavior := true

	tests := []struct {
		name      string
		selector  ChangeSelector
		change    ChangeContext
		wantMatch bool
	}{
		{
			name: "matches every declared dimension",
			selector: ChangeSelector{
				Kinds:              []ChangeKind{ChangeSecurity, ChangeMigration},
				ObservableBehavior: &behavior,
				MinimumRisk:        &high,
				Reversibility:      []Reversibility{ReversibilityDifficult},
				TrustBoundaries:    []TrustBoundary{TrustBoundaryCrossed},
				DependencyBreadth:  []DependencyBreadth{DependencyCrossDomain},
				MigrationImpacts:   []MigrationImpact{MigrationData},
				EvidenceNeeds:      []EvidenceNeed{EvidenceSecurityReview},
				Stages:             []DeliveryStage{StagePullRequest},
			},
			change: ChangeContext{
				Kind:               security,
				ObservableBehavior: true,
				Risk:               RiskCritical,
				Reversibility:      ReversibilityDifficult,
				TrustBoundary:      TrustBoundaryCrossed,
				DependencyBreadth:  DependencyCrossDomain,
				MigrationImpact:    MigrationData,
				EvidenceNeeds:      []EvidenceNeed{EvidenceSecurityReview, EvidenceRegression},
				Stage:              StagePullRequest,
			},
			wantMatch: true,
		},
		{
			name:      "rejects a change below minimum risk",
			selector:  ChangeSelector{MinimumRisk: &high},
			change:    ChangeContext{Risk: RiskMedium},
			wantMatch: false,
		},
		{
			name:      "rejects a different kind",
			selector:  ChangeSelector{Kinds: []ChangeKind{ChangeSecurity}},
			change:    ChangeContext{Kind: ChangeDocumentation},
			wantMatch: false,
		},
		{
			name:      "rejects a missing evidence need",
			selector:  ChangeSelector{EvidenceNeeds: []EvidenceNeed{EvidenceMigration}},
			change:    ChangeContext{EvidenceNeeds: []EvidenceNeed{EvidenceRegression}},
			wantMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.selector.Matches(tt.change); got != tt.wantMatch {
				t.Fatalf("Matches() = %v, want %v", got, tt.wantMatch)
			}
		})
	}
}

func TestQualityPolicyValidate(t *testing.T) {
	valid := QualityPolicy{
		Version: "1.0.0",
		Planning: []PlanningRule{{
			ID:                "security-full-plan",
			Selector:          ChangeSelector{Kinds: []ChangeKind{ChangeSecurity}},
			Depth:             PlanningFull,
			RequiredArtifacts: []PlanningArtifact{ArtifactProposal, ArtifactSpecification, ArtifactDesign, ArtifactTasks},
			ApprovalRequired:  true,
		}},
		TDD:          VerticalTDDPolicy{RequireWhenEligible: true},
		Gherkin:      GherkinPolicy{EligibleBehaviors: []BehaviorClass{BehaviorGenerator, BehaviorInstallation, BehaviorDiagnostic, BehaviorDegradation, BehaviorRollback}, RequireCompatiblePractice: true, RequireExecutableValue: true},
		Architecture: ArchitecturePolicy{Principles: []ArchitecturePrinciple{PrincipleDomainVisibility, PrincipleInwardDependencies, PrincipleExternalIsolation, PrincipleBoundaryTestability}},
		Mutation:     MutationPolicy{Mode: MutationReportOnly, RequireGreenBaseline: true, RequireRelevantCoverage: true, RequirePinnedTool: true, Scope: MutationChangedCoveredCritical, Budget: ActivityBudget{WallTime: time.Minute, Mutants: 100}},
		Property:     PropertyPolicy{ApplyTo: []TestApplicability{ApplicabilityMeaningfulInvariant}, Budget: ActivityBudget{Cases: 100}},
		Fuzz:         FuzzPolicy{ApplyTo: []TestApplicability{ApplicabilityUntrustedInput}, PRCorpusReplay: true, Budget: ActivityBudget{FuzzDuration: time.Second}},
	}

	tests := []struct {
		name    string
		mutate  func(*QualityPolicy)
		wantErr bool
	}{
		{name: "valid contract"},
		{name: "requires semantic version", mutate: func(p *QualityPolicy) { p.Version = "" }, wantErr: true},
		{name: "high risk planning requires approval", mutate: func(p *QualityPolicy) { p.Planning[0].ApprovalRequired = false }, wantErr: true},
		{name: "architecture gate requires analyzer and rule IDs", mutate: func(p *QualityPolicy) { p.Architecture.Gate = &ArchitectureGate{} }, wantErr: true},
		{name: "mutation gate requires attributable delta rules", mutate: func(p *QualityPolicy) { p.Mutation.Mode = MutationGate }, wantErr: true},
		{name: "rejects negative budget", mutate: func(p *QualityPolicy) { p.Mutation.Budget.Cost = -1 }, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := valid
			policy.Planning = append([]PlanningRule(nil), valid.Planning...)
			if tt.mutate != nil {
				tt.mutate(&policy)
			}
			err := policy.Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestVerticalTDDEvidenceValidate(t *testing.T) {
	redAt := time.Date(2026, 7, 26, 10, 0, 0, 0, time.UTC)
	productionAt := redAt.Add(time.Minute)
	base := VerticalTDDEvidence{
		RequirementIDs:        []string{"REQ-QUAL-002"},
		ExampleIDs:            []string{"vertical-slice"},
		CommitOrTreeState:     "tree:abc123",
		Command:               "go test ./internal/components/sdd/quality -run TestContract",
		WorkingDirectory:      "D:/repo",
		RedAt:                 redAt,
		ProductionChangedAt:   productionAt,
		RedExitStatus:         1,
		RedFailureFingerprint: "undefined: QualityPolicy",
		Green:                 TestResult{At: redAt.Add(2 * time.Minute), ExitStatus: 0},
		Refactor:              TestResult{At: redAt.Add(3 * time.Minute), ExitStatus: 0},
		Artifacts:             []string{"quality_test.go", "quality.go"},
		VerificationLevel:     VerificationUnit,
	}

	tests := []struct {
		name     string
		evidence VerticalTDDEvidence
		wantErr  bool
	}{
		{name: "complete evidence", evidence: base},
		{name: "production before red", evidence: func() VerticalTDDEvidence { e := base; e.ProductionChangedAt = redAt.Add(-time.Second); return e }(), wantErr: true},
		{name: "red must fail", evidence: func() VerticalTDDEvidence { e := base; e.RedExitStatus = 0; return e }(), wantErr: true},
		{name: "green must pass", evidence: func() VerticalTDDEvidence { e := base; e.Green.ExitStatus = 1; return e }(), wantErr: true},
		{name: "explicit exception with compensating evidence", evidence: VerticalTDDEvidence{Exception: &TDDException{Reason: TDDUnavailableRunner, Detail: "runner unavailable", CompensatingEvidence: []string{"manual contract review"}}}},
		{name: "exception requires compensation", evidence: VerticalTDDEvidence{Exception: &TDDException{Reason: TDDUnavailableRunner, Detail: "runner unavailable"}}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.evidence.Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOutcomeForTermination(t *testing.T) {
	tests := []struct {
		name      string
		cause     TerminationCause
		completed bool
		want      OutcomeStatus
	}{
		{name: "completed work passes", cause: TerminationNone, completed: true, want: OutcomePass},
		{name: "ordinary failure fails", cause: TerminationNone, completed: false, want: OutcomeFail},
		{name: "budget exhaustion is inconclusive", cause: TerminationBudgetExhausted, want: OutcomeInconclusive},
		{name: "timeout is inconclusive", cause: TerminationTimeout, want: OutcomeInconclusive},
		{name: "insufficient trials are inconclusive", cause: TerminationInsufficientTrials, want: OutcomeInconclusive},
		{name: "missing capability is degraded", cause: TerminationMissingCapability, want: OutcomeDegraded},
		{name: "flaky infrastructure is degraded", cause: TerminationFlakyInfrastructure, want: OutcomeDegraded},
		{name: "cancellation is degraded", cause: TerminationCancelled, want: OutcomeDegraded},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := OutcomeForTermination(tt.cause, tt.completed); got != tt.want {
				t.Fatalf("OutcomeForTermination() = %q, want %q", got, tt.want)
			}
		})
	}
}
