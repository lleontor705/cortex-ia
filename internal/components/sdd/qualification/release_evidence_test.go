package qualification

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/quality"
)

func TestEvaluateRaceEvidenceRejectsAbsentOrInvalidQualification(t *testing.T) {
	tests := []struct {
		name     string
		evidence RaceEvidence
		want     string
	}{
		{name: "missing evidence", want: "missing"},
		{name: "CGO disabled", evidence: validRaceEvidence(func(e *RaceEvidence) { e.CGOEnabled = false }), want: "CGO"},
		{name: "wrong operating system", evidence: validRaceEvidence(func(e *RaceEvidence) { e.GOOS = "windows" }), want: "linux/amd64"},
		{name: "wrong command", evidence: validRaceEvidence(func(e *RaceEvidence) { e.Command = "go test ./..." }), want: RaceCommand},
		{name: "compiler unavailable", evidence: validRaceEvidence(func(e *RaceEvidence) { e.Termination = TerminationCompilerUnavailable }), want: "compiler"},
		{name: "timeout", evidence: validRaceEvidence(func(e *RaceEvidence) { e.Termination = TerminationTimeout }), want: "timeout"},
		{name: "cancellation", evidence: validRaceEvidence(func(e *RaceEvidence) { e.Termination = TerminationCancelled }), want: "cancel"},
		{name: "flaky infrastructure", evidence: validRaceEvidence(func(e *RaceEvidence) { e.Termination = TerminationFlakyInfrastructure }), want: "flaky"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision := EvaluateRaceEvidence(tt.evidence)
			if decision.Status != quality.OutcomeInconclusive || !decision.Blocking {
				t.Fatalf("decision = %#v, want blocking inconclusive", decision)
			}
			if !strings.Contains(decision.Reason, tt.want) {
				t.Fatalf("reason = %q, want %q", decision.Reason, tt.want)
			}
		})
	}
}

func TestEvaluateRaceEvidencePreservesOriginalFailureAcrossBoundedRetries(t *testing.T) {
	evidence := validRaceEvidence(func(e *RaceEvidence) {
		e.ExitCode = 1
		e.OriginalFailure = "WARNING: DATA RACE in TestApply"
		e.Attempts = []RaceAttempt{
			{Number: 1, ExitCode: 1, Failure: e.OriginalFailure},
			{Number: 2, ExitCode: 0},
		}
		e.RetryBudget = 1
	})

	decision := EvaluateRaceEvidence(evidence)
	if decision.Status != quality.OutcomeInconclusive || !decision.Blocking {
		t.Fatalf("decision = %#v, want blocking inconclusive", decision)
	}
	if decision.OriginalFailure != evidence.OriginalFailure || !strings.Contains(decision.Reason, "retry") {
		t.Fatalf("original failure was not preserved: %#v", decision)
	}
}

func TestEvaluateRaceEvidenceAcceptsPinnedLinuxAMD64CGORun(t *testing.T) {
	decision := EvaluateRaceEvidence(validRaceEvidence(nil))
	if decision.Status != quality.OutcomePass || !decision.Blocking {
		t.Fatalf("decision = %#v, want blocking pass", decision)
	}
}

func TestEvaluatePolicyEvidenceKeepsMutationAndRuntimeBoundariesTruthful(t *testing.T) {
	tests := []struct {
		name     string
		evidence PolicyEvidence
		status   quality.OutcomeStatus
		blocking bool
	}{
		{name: "report-only mutation can report pass without release gate", evidence: validPolicyEvidence(EvidenceMutation, EnforcementReportOnly), status: quality.OutcomePass},
		{name: "report-only missing mutation evidence is nonblocking inconclusive", evidence: PolicyEvidence{Kind: EvidenceMutation, Enforcement: EnforcementReportOnly}, status: quality.OutcomeInconclusive},
		{name: "blocking runtime timeout is inconclusive", evidence: validPolicyEvidence(EvidenceRuntime, EnforcementBlocking, func(e *PolicyEvidence) { e.Termination = TerminationTimeout }), status: quality.OutcomeInconclusive, blocking: true},
		{name: "runtime credentials in evidence are rejected", evidence: validPolicyEvidence(EvidenceRuntime, EnforcementBlocking, func(e *PolicyEvidence) { e.CredentialMaterial = "secret" }), status: quality.OutcomeFail, blocking: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision := EvaluatePolicyEvidence(tt.evidence)
			if decision.Status != tt.status || decision.Blocking != tt.blocking {
				t.Fatalf("decision = %#v, want status=%q blocking=%t", decision, tt.status, tt.blocking)
			}
		})
	}
}

func TestReleaseWorkflowPinsLinuxAMD64CGORaceAndEvidenceArtifact(t *testing.T) {
	workflowPath := filepath.Join("..", "..", "..", "..", ".github", "workflows", "release.yml")
	data, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatal(err)
	}
	workflow := string(data)
	for _, required := range []string{
		"race-qualification:",
		"GOOS: linux",
		"GOARCH: amd64",
		"CGO_ENABLED: \"1\"",
		"CGO_ENABLED=1 go test -race -count=1 ./...",
		"gcc --version",
		"go version",
		"actions/upload-artifact@v4",
		"needs: [unit-tests, lint, race-qualification]",
	} {
		if !strings.Contains(workflow, required) {
			t.Errorf("release workflow missing %q", required)
		}
	}
}

func validRaceEvidence(mutate func(*RaceEvidence)) RaceEvidence {
	evidence := RaceEvidence{
		GOOS: "linux", GOARCH: "amd64", CGOEnabled: true,
		Command: RaceCommand, GoVersion: "go1.26.4", Compiler: "gcc 13.3.0",
		Termination: TerminationCompleted, ExitCode: 0,
		Attempts: []RaceAttempt{{Number: 1, ExitCode: 0}}, RetryBudget: 0,
	}
	if mutate != nil {
		mutate(&evidence)
	}
	return evidence
}

func validPolicyEvidence(kind EvidenceKind, enforcement Enforcement, mutators ...func(*PolicyEvidence)) PolicyEvidence {
	evidence := PolicyEvidence{
		Kind: kind, Enforcement: enforcement, Termination: TerminationCompleted,
		Numerator: 3, Denominator: 3, Exclusions: []string{},
		Budget: EvidenceBudget{WallTimeSeconds: 30, Retries: 1, Cases: 3},
		Tool: "pinned-tool@1.0.0", EvidenceRefs: []string{"sha256:abc"},
	}
	for _, mutate := range mutators {
		mutate(&evidence)
	}
	return evidence
}
