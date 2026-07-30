package forgespec

import "testing"

func TestLegacyHistoryEvidenceRequiresLegacyModeAndDirectV1Link(t *testing.T) {
	if err := ValidateHistoryBoundary("legacy", "direct-v1", "board-legacy", "board-direct", false); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, legacyMode, directMode string }{
		{"legacy mismatch", "direct-v1", "direct-v1"},
		{"direct mismatch", "legacy", "legacy"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateHistoryBoundary(tc.legacyMode, tc.directMode, "old", "new", false); err == nil {
				t.Fatal("mode mismatch accepted")
			}
		})
	}
	if err := ValidateHistoryBoundary("legacy", "direct-v1", "same", "same", false); err == nil {
		t.Fatal("same board accepted")
	}
}
