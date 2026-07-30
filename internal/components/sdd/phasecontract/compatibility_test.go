package phasecontract

import "testing"

func TestCompatibilityAliasesLowerOnlyAtBoundary(t *testing.T) {
	for alias, want := range map[string]PhaseID{
		"bootstrap": PhaseInit, "investigate": PhaseExplore, "draft-proposal": PhasePropose,
		"write-specs": PhaseSpec, "architect": PhaseDesign, "decompose": PhaseTasks,
		"implement": PhaseApply, "validate": PhaseVerify, "finalize": PhaseArchive,
	} {
		got, evidence, err := DecodeCompatibilityAlias(alias, CompatibilityVersion)
		if err != nil {
			t.Fatalf("DecodeCompatibilityAlias(%q): %v", alias, err)
		}
		if got != want || evidence.Alias != alias || evidence.Version != CompatibilityVersion {
			t.Errorf("alias %q -> phase %q, evidence %+v; want %q and versioned evidence", alias, got, evidence, want)
		}
	}
}

func TestCompatibilityAliasRejectsUnknownVersionAndName(t *testing.T) {
	if _, _, err := DecodeCompatibilityAlias("validate", "0.0.0"); err == nil {
		t.Fatal("unknown compatibility version accepted")
	}
	if _, _, err := DecodeCompatibilityAlias("unknown", CompatibilityVersion); err == nil {
		t.Fatal("unknown compatibility alias accepted")
	}
}
