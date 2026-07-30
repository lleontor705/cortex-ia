package legacyoracle

import (
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/assets"
)

func TestBuildLegacyInventoryIsCompleteAndDeterministic(t *testing.T) {
	first, err := Build()
	if err != nil {
		t.Fatal(err)
	}
	if err := first.Validate(); err != nil {
		t.Fatal(err)
	}
	second, err := Build()
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Assets) != len(second.Assets) {
		t.Fatalf("inventory size changed: %d != %d", len(first.Assets), len(second.Assets))
	}
	for i := range first.Assets {
		if first.Assets[i] != second.Assets[i] {
			t.Fatalf("inventory is not deterministic at %d: %#v != %#v", i, first.Assets[i], second.Assets[i])
		}
	}
}

func TestEveryRetainedLegacyAssetExistsInEmbeddedSource(t *testing.T) {
	inventory, err := Build()
	if err != nil {
		t.Fatal(err)
	}
	for _, asset := range inventory.Assets {
		if asset.Classification == Retire || strings.HasSuffix(asset.SourcePath, ".go") {
			continue
		}
		if _, err := assets.Read(asset.SourcePath); err != nil {
			t.Errorf("legacy asset %q source %q is unavailable: %v", asset.ID, asset.SourcePath, err)
		}
	}
}

func TestRetirementReasonsAreExplicitAndTeamLeadIsNotRetained(t *testing.T) {
	inventory, err := Build()
	if err != nil {
		t.Fatal(err)
	}
	for _, asset := range inventory.Assets {
		if asset.Classification == Retire && strings.TrimSpace(asset.RetirementReason) == "" {
			t.Errorf("retired asset %q has no reason", asset.ID)
		}
		if strings.Contains(asset.ID, "team-lead") && asset.Classification != Retire {
			t.Errorf("team-lead asset %q was not retired", asset.ID)
		}
	}
}
