package qualification

import (
	"strings"
	"testing"
	"time"
)

func TestDecodeHistoricalProvenanceRejectsUnknownFields(t *testing.T) {
	_, err := DecodeHistoricalProvenance(strings.NewReader(`{"schema_version":"1.0.0","expected_count":0,"records":[],"current_green":null,"invented":true}`))
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("DecodeHistoricalProvenance() error = %v, want unknown field", err)
	}
}

func TestValidateHistoricalProvenanceRejectsDuplicatesInvalidCommandsAndCountMismatch(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*HistoricalProvenance)
		want   string
	}{
		{name: "duplicate task IDs", mutate: func(p *HistoricalProvenance) { p.Records = append(p.Records, p.Records[0]) }, want: "duplicate task ID"},
		{name: "duplicate source digests", mutate: func(p *HistoricalProvenance) { duplicate := p.Records[0]; duplicate.TaskID = "task-2"; p.Records = append(p.Records, duplicate) }, want: "duplicate source digest"},
		{name: "invalid four-dot command", mutate: func(p *HistoricalProvenance) { p.Records[0].Green.Command = "go test ./...." }, want: "invalid Go test command"},
		{name: "count mismatch", mutate: func(p *HistoricalProvenance) { p.ExpectedCount = 2 }, want: "record count"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provenance := validHistoricalProvenance()
			tt.mutate(&provenance)
			if err := provenance.Validate(); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Validate() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestValidateHistoricalProvenanceRejectsIncompleteProvidedEvidence(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*CycleEvidence)
		want   string
	}{
		{name: "missing timestamp", mutate: func(e *CycleEvidence) { e.StartedAt = time.Time{} }, want: "timestamp"},
		{name: "missing tree reference", mutate: func(e *CycleEvidence) { e.TreeRef = "" }, want: "tree_ref"},
		{name: "missing cwd", mutate: func(e *CycleEvidence) { e.CWD = "" }, want: "cwd"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provenance := validHistoricalProvenance()
			tt.mutate(provenance.Records[0].Red)
			if err := provenance.Validate(); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Validate() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestHistoricalGapDoesNotSynthesizeMissingREDAndCurrentGreenIsSeparate(t *testing.T) {
	provenance := validHistoricalProvenance()
	provenance.Records[0] = HistoricalRecord{
		TaskID: "task-1", SourceID: "forgespec:task-1", SourceDigest: "sha256:one",
		Classification: ProvenanceHistoricalGap, GapReason: "recorded note has no independently verifiable RED ordering or tree reference",
	}

	if err := provenance.Validate(); err != nil {
		t.Fatal(err)
	}
	if provenance.Records[0].Red != nil {
		t.Fatal("historical gap synthesized RED evidence")
	}
	if provenance.CurrentGreen == nil || provenance.CurrentGreen.Command != "go test ./..." {
		t.Fatalf("current GREEN = %#v, want separate valid command", provenance.CurrentGreen)
	}
}

func validHistoricalProvenance() HistoricalProvenance {
	start := time.Date(2026, 7, 27, 1, 0, 0, 0, time.UTC)
	evidence := func(command string, offset time.Duration, exit int) *CycleEvidence {
		return &CycleEvidence{
			Command: command, CWD: "D:/FuentesLuis/lleontor705/cortex-ia",
			StartedAt: start.Add(offset), FinishedAt: start.Add(offset + time.Second),
			TreeRef: "worktree:sha256:abc", ExitStatus: exit,
		}
	}
	return HistoricalProvenance{
		SchemaVersion: "1.0.0", ExpectedCount: 1,
		Records: []HistoricalRecord{{
			TaskID: "task-1", SourceID: "forgespec:task-1", SourceDigest: "sha256:one",
			Classification: ProvenanceProven, Red: evidence("go test ./internal/example -run TestThing -count=1", 0, 1),
			Green: evidence("go test ./internal/example -run TestThing -count=1", 2*time.Second, 0),
			Refactor: evidence("go test ./internal/example -run TestThing -count=1", 4*time.Second, 0),
		}},
		CurrentGreen: evidence("go test ./...", 6*time.Second, 0),
	}
}
