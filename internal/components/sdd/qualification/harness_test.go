package qualification

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/quality"
)

func TestHarnessRequiresExplicitOptInAndCredentials(t *testing.T) {
	tests := []struct {
		name          string
		authorization Authorization
		wantReason    string
	}{
		{name: "opt in is absent", authorization: Authorization{CredentialRef: "env:RUNTIME_TOKEN", CredentialValue: "secret"}, wantReason: "explicit opt-in"},
		{name: "credential is absent", authorization: Authorization{ExplicitOptIn: true}, wantReason: "credential"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &fakeRunner{}
			plan := validPlan(t)
			plan.Authorization = tt.authorization

			report := Harness{Runner: runner}.Run(context.Background(), plan)

			if report.Status != quality.OutcomeInconclusive {
				t.Fatalf("Run() status = %q, want inconclusive", report.Status)
			}
			if !strings.Contains(report.Reason, tt.wantReason) {
				t.Fatalf("Run() reason = %q, want %q", report.Reason, tt.wantReason)
			}
			if runner.calls != 0 {
				t.Fatalf("external runner calls = %d, want 0", runner.calls)
			}
		})
	}
}

func TestHarnessProducesAttributedRedactedThreeTrialClaim(t *testing.T) {
	plan := validPlan(t)
	runner := &fakeRunner{observations: []Observation{
		passingObservation("evidence token-value trial-1"),
		passingObservation("evidence trial-2"),
		passingObservation("evidence trial-3"),
	}}

	report := Harness{Runner: runner}.Run(context.Background(), plan)

	if report.Status != quality.OutcomePass {
		t.Fatalf("Run() status = %q, reason = %q, want pass", report.Status, report.Reason)
	}
	if runner.calls != 3 || len(report.Trials) != 3 {
		t.Fatalf("calls/trials = %d/%d, want 3/3", runner.calls, len(report.Trials))
	}
	for index, trial := range report.Trials {
		if trial.Attribution != plan.Attribution {
			t.Fatalf("trial %d attribution = %#v, want %#v", index, trial.Attribution, plan.Attribution)
		}
		if trial.TrialID == "" {
			t.Fatalf("trial %d has empty trial ID", index)
		}
		if strings.Contains(strings.Join(trial.Evidence, " "), plan.Authorization.CredentialValue) {
			t.Fatalf("trial %d leaked credential: %#v", index, trial.Evidence)
		}
	}
	if got := report.Metrics; got.EligibleStarted != 3 || got.ContractCleanWithoutHuman != 3 || got.Excluded != 0 {
		t.Fatalf("primary metric = %#v, want numerator=3 denominator=3 exclusions=0", got)
	}
	if math.Abs(report.Metrics.TotalCostUSD-0.30) > 1e-9 || report.Metrics.TotalToolCalls != 6 || report.Metrics.TotalRetries != 3 {
		t.Fatalf("cost/tool/retry metrics = %#v", report.Metrics)
	}
	if report.Metrics.TotalLatency != 6*time.Second || report.Metrics.EvidenceComplete != 3 {
		t.Fatalf("latency/evidence metrics = %#v", report.Metrics)
	}
	data, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), plan.Authorization.CredentialValue) || strings.Contains(string(data), plan.Authorization.CredentialRef) {
		t.Fatalf("serialized report leaked credential material: %s", data)
	}
}

func TestHarnessKeepsInsufficientFlakyMissingAndBudgetedRunsInconclusive(t *testing.T) {
	tests := []struct {
		name       string
		mutatePlan func(*Plan)
		runner     *fakeRunner
		wantReason string
	}{
		{
			name:       "insufficient trials",
			mutatePlan: func(plan *Plan) { plan.Trials = 2 },
			runner:     &fakeRunner{observations: []Observation{passingObservation("one"), passingObservation("two")}},
			wantReason: "at least 3 trials",
		},
		{
			name: "flaky infrastructure",
			runner: &fakeRunner{observations: []Observation{
				passingObservation("one"),
				func() Observation {
					observation := passingObservation("two")
					observation.Flaky = true
					return observation
				}(),
				passingObservation("three"),
			}},
			wantReason: "flaky",
		},
		{
			name:       "missing attribution",
			mutatePlan: func(plan *Plan) { plan.Attribution.Model = "" },
			runner:     &fakeRunner{},
			wantReason: "model",
		},
		{
			name: "budget exhausted",
			mutatePlan: func(plan *Plan) {
				plan.Budget.Cost = 0.15
			},
			runner:     &fakeRunner{observations: []Observation{passingObservation("one"), passingObservation("two"), passingObservation("three")}},
			wantReason: "budget",
		},
		{
			name: "invalid runtime metrics",
			runner: &fakeRunner{observations: []Observation{
				func() Observation {
					observation := passingObservation("one")
					observation.CostUSD = -1
					return observation
				}(),
				passingObservation("two"),
				passingObservation("three"),
			}},
			wantReason: "invalid runtime metrics",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := validPlan(t)
			if tt.mutatePlan != nil {
				tt.mutatePlan(&plan)
			}
			report := Harness{Runner: tt.runner}.Run(context.Background(), plan)
			if report.Status != quality.OutcomeInconclusive {
				t.Fatalf("Run() status = %q, want inconclusive", report.Status)
			}
			if !strings.Contains(report.Reason, tt.wantReason) {
				t.Fatalf("Run() reason = %q, want %q", report.Reason, tt.wantReason)
			}
		})
	}
}

func TestHarnessRedactsOriginalRuntimeFailure(t *testing.T) {
	plan := validPlan(t)
	runner := &fakeRunner{err: errors.New("runtime rejected env:RUNTIME_TOKEN token-value")}

	report := Harness{Runner: runner}.Run(context.Background(), plan)

	if report.Status != quality.OutcomeInconclusive {
		t.Fatalf("Run() status = %q, want inconclusive", report.Status)
	}
	if strings.Contains(report.Reason, plan.Authorization.CredentialValue) || strings.Contains(report.Reason, plan.Authorization.CredentialRef) || !strings.Contains(report.Reason, Redacted) {
		t.Fatalf("Run() reason was not redacted: %q", report.Reason)
	}
}

func validPlan(t *testing.T) Plan {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "testdata", "runtime", "qualification-plan.json"))
	if err != nil {
		t.Fatal(err)
	}
	var plan Plan
	if err := json.Unmarshal(data, &plan); err != nil {
		t.Fatal(err)
	}
	plan.Authorization = Authorization{
		ExplicitOptIn:   true,
		CredentialRef:   "env:RUNTIME_TOKEN",
		CredentialValue: "token-value",
	}
	return plan
}

func passingObservation(evidence string) Observation {
	return Observation{
		Eligible:         true,
		ContractClean:    true,
		EvidenceComplete: true,
		CostUSD:          0.10,
		Latency:          2 * time.Second,
		Tokens:           100,
		ToolCalls:        2,
		Retries:          1,
		Evidence:         []string{evidence},
	}
}

type fakeRunner struct {
	observations []Observation
	err          error
	calls        int
}

func (runner *fakeRunner) Run(_ context.Context, request Request) (Observation, error) {
	runner.calls++
	if request.Authorization.CredentialValue == "" {
		return Observation{}, errors.New("missing credential")
	}
	if runner.err != nil {
		return Observation{}, runner.err
	}
	if runner.calls <= len(runner.observations) {
		return runner.observations[runner.calls-1], nil
	}
	return passingObservation("default evidence"), nil
}
