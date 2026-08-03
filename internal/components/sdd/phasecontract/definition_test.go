package phasecontract

import "testing"

func TestCanonicalDefinitionsExposeExactlyNinePhases(t *testing.T) {
	got := CanonicalPhaseIDs()
	want := []PhaseID{PhaseInit, PhaseExplore, PhasePropose, PhaseSpec, PhaseDesign, PhaseTasks, PhaseApply, PhaseVerify, PhaseArchive}
	if len(got) != len(want) {
		t.Fatalf("canonical phase count = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("canonical phase[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestCanonicalTerminalVocabulariesAreDisjoint(t *testing.T) {
	for _, status := range []PhaseStatus{PhaseStatusSuccess, PhaseStatusPartial, PhaseStatusFailed, PhaseStatusBlocked} {
		if err := ValidatePhaseStatus(status); err != nil {
			t.Errorf("canonical status %q rejected: %v", status, err)
		}
	}
	for _, verdict := range []VerificationVerdict{VerdictPass, VerdictFail, VerdictBlocked, VerdictInconclusive} {
		if err := ValidateVerificationVerdict(verdict); err != nil {
			t.Errorf("canonical verdict %q rejected: %v", verdict, err)
		}
	}
	for _, value := range []string{"completed", "working", "done", "PASS"} {
		if ValidatePhaseStatus(PhaseStatus(value)) == nil {
			t.Errorf("noncanonical status %q accepted", value)
		}
	}
}
