package conformance

import (
	"strings"
	"testing"
	"time"
)

func TestAggregateQualityEvidenceRequiresEveryCommandAndAttribution(t *testing.T) {
	input := completeQualityGateInput()
	input.Commands = input.Commands[:len(input.Commands)-1]
	report, err := AggregateQualityEvidence(input)
	if err != nil {
		t.Fatal(err)
	}
	if report.Passed || !containsBlocker(report.Blockers, "missing mandatory command") {
		t.Fatalf("report = %#v, want failed missing-command gate", report)
	}
}

func TestAggregateQualityEvidenceAttributesIntroducedST1005ButSeparatesPreexistingST1023(t *testing.T) {
	input := completeQualityGateInput()
	input.StaticFindings = []StaticFinding{
		{Rule: "ST1005", Path: "internal/components/sdd/conformance/quality_gate.go", Line: 1, Introduced: true},
		{Rule: "ST1023", Path: "internal/components/sdd/quality/plan_test.go", Line: 1, Introduced: false},
	}
	report, err := AggregateQualityEvidence(input)
	if err != nil {
		t.Fatal(err)
	}
	if report.Passed || !containsBlocker(report.Blockers, "introduced static-analysis finding: ST1005") {
		t.Fatalf("report = %#v, want ST1005 blocker", report)
	}
	if containsBlocker(report.Blockers, "ST1023") {
		t.Fatalf("pre-existing ST1023 must be attributed separately: %#v", report.Blockers)
	}
	if len(report.PreexistingFindings) != 1 || report.PreexistingFindings[0].Rule != "ST1023" {
		t.Fatalf("pre-existing findings = %#v", report.PreexistingFindings)
	}
}

func TestAggregateQualityEvidenceIncludesMachineReadableRevisionEnvironmentAndHashes(t *testing.T) {
	report, err := AggregateQualityEvidence(completeQualityGateInput())
	if err != nil {
		t.Fatal(err)
	}
	if report.Fingerprint == "" || report.Revision == "" || len(report.Environment) == 0 {
		t.Fatalf("report lacks aggregate provenance: %#v", report)
	}
	for _, command := range report.Commands {
		if command.Command == "" || command.Revision == "" || command.OutputHash == "" || command.StartedAt.IsZero() || command.FinishedAt.IsZero() {
			t.Fatalf("command lacks provenance: %#v", command)
		}
	}
}

func TestAggregateQualityEvidenceRejectsStaleCommandRevisionAndClockWindow(t *testing.T) {
	input := completeQualityGateInput()
	input.Race.Revision = input.Revision
	input.Commands[0].Revision = "stale-revision"
	input.Commands[1].FinishedAt = input.Commands[1].StartedAt.Add(-time.Second)
	report, err := AggregateQualityEvidence(input)
	if err != nil {
		t.Fatal(err)
	}
	if report.Passed {
		t.Fatalf("stale command evidence passed: %#v", report)
	}
	if !containsBlocker(report.Blockers, "stale command revision") {
		t.Fatalf("missing stale-revision blocker: %#v", report.Blockers)
	}
	if !containsBlocker(report.Blockers, "invalid command time window") {
		t.Fatalf("missing command-time blocker: %#v", report.Blockers)
	}
}

func TestAggregateQualityEvidenceRejectsStaleRaceRevisionAndClockWindow(t *testing.T) {
	input := completeQualityGateInput()
	input.Race.Revision = input.Revision
	input.Race.Revision = "stale-revision"
	input.Race.FinishedAt = input.Race.StartedAt.Add(-time.Second)
	report, err := AggregateQualityEvidence(input)
	if err != nil {
		t.Fatal(err)
	}
	if report.Passed {
		t.Fatalf("stale race evidence passed: %#v", report)
	}
	if !containsBlocker(report.Blockers, "stale race revision") {
		t.Fatalf("missing stale-race blocker: %#v", report.Blockers)
	}
	if !containsBlocker(report.Blockers, "invalid race time window") {
		t.Fatalf("missing race-time blocker: %#v", report.Blockers)
	}
}

func completeQualityGateInput() QualityGateInput {
	now := time.Unix(100, 0).UTC()
	commands := make([]CommandEvidence, 0, len(MandatoryQualityCommands))
	for _, name := range MandatoryQualityCommands {
		commands = append(commands, CommandEvidence{Name: name, Command: name, Revision: "abc123", ExitCode: 0, StartedAt: now, FinishedAt: now.Add(time.Second), OutputHash: "sha256:output"})
	}
	return QualityGateInput{
		Revision: "abc123", Environment: map[string]string{"GOOS": "windows", "GOARCH": "amd64", "CGO_ENABLED": "0"},
		Commands: commands, Race: RaceCapabilityEvidence{GOOS: "linux", GOARCH: "amd64", CGOEnabled: true, Compiler: "gcc", Command: "CGO_ENABLED=1 go test -race -count=1 ./...", ExitCode: 0, StartedAt: now, FinishedAt: now.Add(time.Second), OutputHash: "sha256:race"},
		RaceMandatory: true,
	}
}

func containsBlocker(blockers []string, wanted string) bool {
	for _, blocker := range blockers {
		if strings.Contains(blocker, wanted) {
			return true
		}
	}
	return false
}
