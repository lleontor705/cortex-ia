package conformance

import (
	"strings"
	"testing"
)

func TestObserveExternalRootBlockedProducesAccountableEvidence(t *testing.T) {
	protected := []byte("kiro-user-settings")
	observed, err := observeExternalRootBlocked("kiro", "portable-sequential", "go test ./internal/components/sdd/conformance -run TestNormalizedSemanticGolden/kiro", 1, "adapter config root escapes home", protected)
	if err != nil {
		t.Fatal(err)
	}
	if observed.Disposition != DispositionRejected {
		t.Fatalf("disposition = %q, want pre-mutation blocked/rejected", observed.Disposition)
	}
	if observed.ReasonID != "kiro/external-root/blocked" {
		t.Fatalf("reason = %q, want stable external-root reason", observed.ReasonID)
	}
	if observed.Command == "" || observed.ExitCode == 0 || observed.ProtectedRootDigest == "" {
		t.Fatalf("incomplete command/exit/protected-root evidence: %+v", observed)
	}
	if observed.Mutation != "none" {
		t.Fatalf("mutation = %q, want none", observed.Mutation)
	}
	if len(observed.Report.Records) != len(canonicalPhases) {
		t.Fatalf("records = %d, want nine", len(observed.Report.Records))
	}
	if err := ValidateEvidence(observed.Report); err != nil {
		t.Fatalf("ValidateEvidence() error = %v", err)
	}
	for _, record := range observed.Report.Records {
		if record.Accountability != AccountabilityPreMutation || record.Disposition != DispositionRejected {
			t.Fatalf("record is not pre-mutation blocked evidence: %+v", record)
		}
		if !strings.Contains(record.ReasonID, "kiro/external-root/blocked") {
			t.Fatalf("record reason = %q", record.ReasonID)
		}
	}
}
