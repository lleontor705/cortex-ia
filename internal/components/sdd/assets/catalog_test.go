package assets

import (
	"slices"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
)

func TestOperationalCatalogIsCompleteDeterministicAndFingerprinted(t *testing.T) {
	a, err := BuildOperationalCatalog()
	if err != nil {
		t.Fatal(err)
	}
	b, err := BuildOperationalCatalog()
	if err != nil {
		t.Fatal(err)
	}
	if a.Fingerprint() != b.Fingerprint() {
		t.Fatalf("catalog fingerprint changed: %s != %s", a.Fingerprint(), b.Fingerprint())
	}
	if len(a.Catalog.Assets) < 36 {
		t.Fatalf("catalog assets = %d, want at least 36 retained assets", len(a.Catalog.Assets))
	}
	ids := make([]string, len(a.Catalog.Assets))
	for i, spec := range a.Catalog.Assets {
		ids[i] = string(spec.ID)
		if len(a.Contents[spec.ID]) == 0 {
			t.Fatalf("asset %q has no materialized content", spec.ID)
		}
		if spec.SHA256 != fingerprint(a.Contents[spec.ID]) {
			t.Fatalf("asset %q fingerprint mismatch", spec.ID)
		}
	}
	if !slices.IsSorted(ids) {
		t.Fatalf("catalog IDs are not sorted: %v", ids)
	}
	for _, id := range []string{"asset/root-index", "asset/shared-contract", "asset/skill/bootstrap", "asset/skill/finalize", "asset/role/bootstrap", "asset/role/archive", "asset/profile-overlay/portable-sequential", "asset/manifest/model-routes"} {
		if _, ok := a.Contents[ir.SemanticID(id)]; !ok {
			t.Fatalf("required retained asset %q missing", id)
		}
	}
}

func fingerprint(content []byte) string { return ir.FingerprintContent(content) }
