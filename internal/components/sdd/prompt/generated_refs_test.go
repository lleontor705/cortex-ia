package prompt

import (
	"testing"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/assets"
)

func TestGeneratedReferencesExposeReceiptMetadataAndCanonicalIDs(t *testing.T) {
	catalog, err := assets.BuildOperationalCatalog()
	if err != nil {
		t.Fatal(err)
	}
	refs, err := GeneratedReferences(catalog)
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 5 {
		t.Fatalf("generated references = %d, want 5", len(refs))
	}
	for _, ref := range refs {
		if ref.Version == "" || ref.SourceFingerprint == "" || ref.SHA256 == "" {
			t.Fatalf("reference %q lost version/fingerprint metadata: %#v", ref.ID, ref)
		}
		if ref.ID == "asset/contract/policy" && ref.Phase != "" {
			t.Fatalf("generated reference stored alias/phase unexpectedly: %#v", ref)
		}
	}
}
