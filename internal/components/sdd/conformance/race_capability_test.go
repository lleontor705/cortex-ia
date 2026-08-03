package conformance

import (
	"testing"
	"time"
)

func TestEvaluateRaceCapabilityNeverPassesWithoutCGOCompiler(t *testing.T) {
	evidence := RaceCapabilityEvidence{GOOS: "windows", GOARCH: "amd64", CGOEnabled: false, Command: "CGO_ENABLED=1 go test -race -count=1 ./...", ExitCode: -1}
	decision := EvaluateRaceCapability(evidence, true)
	if decision.Status != RaceInconclusive || !decision.Blocking {
		t.Fatalf("decision = %#v, want blocking inconclusive", decision)
	}
}

func TestEvaluateRaceCapabilityPassRequiresSuccessfulQualifiedExecution(t *testing.T) {
	now := time.Unix(100, 0).UTC()
	evidence := RaceCapabilityEvidence{GOOS: "linux", GOARCH: "amd64", CGOEnabled: true, Compiler: "gcc", Command: "CGO_ENABLED=1 go test -race -count=1 ./...", Revision: "abc123", ExitCode: 0, StartedAt: now, FinishedAt: now.Add(time.Second), OutputHash: "sha256:race"}
	decision := EvaluateRaceCapability(evidence, true)
	if decision.Status != RacePass || decision.Blocking {
		t.Fatalf("decision = %#v, want nonblocking pass", decision)
	}
}

func TestEvaluateRaceCapabilityPassesForSuccessfulWindowsCGOExecution(t *testing.T) {
	now := time.Unix(100, 0).UTC()
	evidence := RaceCapabilityEvidence{GOOS: "windows", GOARCH: "amd64", CGOEnabled: true, Compiler: "gcc", Command: "CGO_ENABLED=1 go test -race -count=1 ./...", Revision: "abc123", ExitCode: 0, StartedAt: now, FinishedAt: now.Add(time.Second), OutputHash: "sha256:race"}
	decision := EvaluateRaceCapability(evidence, true)
	if decision.Status != RacePass || decision.Blocking {
		t.Fatalf("decision = %#v, want nonblocking pass", decision)
	}
}
