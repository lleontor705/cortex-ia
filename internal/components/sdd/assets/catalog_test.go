package assets

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"reflect"
	"testing"

	embedded "github.com/lleontor705/cortex-ia/internal/assets"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
	"github.com/lleontor705/cortex-ia/internal/model"
)

// assertCatalogEqual proves byte-for-byte equality of two materialized
// catalogs: the receipt-visible catalog document, the content bytes, the
// generated references, and the generator identity fields must all match.
func assertCatalogEqual(t *testing.T, got, want MaterializedCatalog) {
	t.Helper()
	gotDocument, gotErr := json.Marshal(got.Catalog)
	wantDocument, wantErr := json.Marshal(want.Catalog)
	if gotErr != nil || wantErr != nil {
		t.Fatalf("marshal catalog documents: got error %v, want error %v", gotErr, wantErr)
	}
	if !bytes.Equal(gotDocument, wantDocument) {
		t.Errorf("catalog documents differ:\ngot:  %s\nwant: %s", gotDocument, wantDocument)
	}
	if got.Fingerprint() != want.Fingerprint() {
		t.Errorf("catalog fingerprints differ: got %s, want %s", got.Fingerprint(), want.Fingerprint())
	}
	if !reflect.DeepEqual(got.Contents, want.Contents) {
		t.Errorf("catalog contents differ: got %d entries, want %d entries", len(got.Contents), len(want.Contents))
	}
	if !reflect.DeepEqual(got.Generated, want.Generated) {
		t.Error("generated references differ between catalogs")
	}
	if got.GeneratorVersion != want.GeneratorVersion {
		t.Errorf("generator versions differ: got %q, want %q", got.GeneratorVersion, want.GeneratorVersion)
	}
	if got.SourceFingerprint != want.SourceFingerprint {
		t.Errorf("source fingerprints differ: got %q, want %q", got.SourceFingerprint, want.SourceFingerprint)
	}
}

// TestCatalog_NoOverlayBaseline verifies REQ-BASE-001 (SC-BASE-H, SC-BASE-E;
// AC-BASE-1, AC-BASE-2) and design invariant 0: without a custom overlay, and
// with an explicitly empty overlay, the effective catalog is byte-for-byte the
// embedded baseline catalog.
func TestCatalog_NoOverlayBaseline(t *testing.T) {
	baseline, err := BuildOperationalCatalog()
	if err != nil {
		t.Fatalf("build embedded baseline catalog: %v", err)
	}
	if baseline.Count("asset/skill/bootstrap") != 1 {
		t.Fatalf("embedded baseline sanity check failed: expected exactly one bootstrap skill")
	}

	noOverlay, err := BuildEffectiveCatalog(nil)
	if err != nil {
		t.Fatalf("build effective catalog without overlay: %v", err)
	}
	assertCatalogEqual(t, noOverlay, baseline)

	emptyOverlay, err := BuildEffectiveCatalog([]CustomSkill{})
	if err != nil {
		t.Fatalf("build effective catalog with empty overlay: %v", err)
	}
	assertCatalogEqual(t, emptyOverlay, baseline)
}

// TestCatalog_AddValidCustom verifies REQ-REG-001 (SC-REG1-H; AC-REG-1): a
// valid custom skill with a new ID appears exactly once alongside the complete
// embedded baseline, with identity and content preserved.
func TestCatalog_AddValidCustom(t *testing.T) {
	baseline, err := BuildOperationalCatalog()
	if err != nil {
		t.Fatalf("build embedded baseline catalog: %v", err)
	}
	content := []byte("# custom-review\n\nExtra review pass for registry slices.\n")

	effective, err := BuildEffectiveCatalog([]CustomSkill{{ID: model.SkillID("custom-review"), Content: content}})
	if err != nil {
		t.Fatalf("build effective catalog with one valid custom skill: %v", err)
	}

	if got := effective.Count("asset/skill/custom-review"); got != 1 {
		t.Errorf("custom skill appears %d times, want exactly 1", got)
	}
	if len(effective.Catalog.Assets) != len(baseline.Catalog.Assets)+1 {
		t.Errorf("effective catalog has %d assets, want baseline %d plus the one addition", len(effective.Catalog.Assets), len(baseline.Catalog.Assets))
	}
	for _, spec := range baseline.Catalog.Assets {
		if got := effective.Count(string(spec.ID)); got != 1 {
			t.Errorf("baseline asset %q appears %d times in effective catalog, want 1", spec.ID, got)
		}
	}
	if !bytes.Equal(effective.Contents["asset/skill/custom-review"], content) {
		t.Errorf("custom skill content not preserved: got %q, want %q", effective.Contents["asset/skill/custom-review"], content)
	}
	for _, spec := range effective.Catalog.Assets {
		if spec.ID != "asset/skill/custom-review" {
			continue
		}
		if spec.Class != ir.AssetSkill {
			t.Errorf("custom skill class = %q, want %q", spec.Class, ir.AssetSkill)
		}
		if !spec.Required {
			t.Error("custom skill must be a required asset so it is installed")
		}
		if spec.SourcePath != "skills/custom-review/SKILL.md" {
			t.Errorf("custom skill source path = %q, want %q", spec.SourcePath, "skills/custom-review/SKILL.md")
		}
		if spec.SHA256 != ir.FingerprintContent(content) {
			t.Errorf("custom skill SHA256 = %q, want %q", spec.SHA256, ir.FingerprintContent(content))
		}
		if spec.MaxTokens <= 0 {
			t.Errorf("custom skill MaxTokens = %d, want a positive budget", spec.MaxTokens)
		}
	}
	for i := 1; i < len(effective.Catalog.Assets); i++ {
		if effective.Catalog.Assets[i-1].ID >= effective.Catalog.Assets[i].ID {
			t.Errorf("effective catalog is not sorted by semantic ID: %q before %q", effective.Catalog.Assets[i-1].ID, effective.Catalog.Assets[i].ID)
		}
	}
	if effective.Fingerprint() == baseline.Fingerprint() {
		t.Error("adding a custom skill must change the catalog fingerprint")
	}
}

// TestCatalog_OverrideRejected verifies design invariant 1 (first half) and
// REQ-REG-002 (SC-REG2-F) at the catalog boundary: a custom skill whose ID
// matches an embedded skill ID is rejected before the catalog is returned and
// the embedded baseline stays byte-for-byte intact.
func TestCatalog_OverrideRejected(t *testing.T) {
	cases := []struct {
		name string
		id   model.SkillID
	}{
		{name: "CanonicalSkill", id: "bootstrap"},
		{name: "SharedSkill", id: "shared/cortex-convention"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			effective, err := BuildEffectiveCatalog([]CustomSkill{{ID: tc.id, Content: []byte("# hostile replacement\n")}})
			if err == nil {
				t.Fatalf("override of embedded skill %q was accepted", tc.id)
			}
			var overrideErr *OverrideError
			if !errors.As(err, &overrideErr) {
				t.Fatalf("error for embedded override %q is %T (%v), want *OverrideError", tc.id, err, err)
			}
			if overrideErr.SkillID != string(tc.id) {
				t.Errorf("OverrideError.SkillID = %q, want %q", overrideErr.SkillID, tc.id)
			}
			if len(effective.Catalog.Assets) != 0 {
				t.Errorf("rejected overlay must not return a partial catalog; got %d assets", len(effective.Catalog.Assets))
			}

			// The embedded baseline is preserved byte-for-byte.
			baseline, buildErr := BuildOperationalCatalog()
			if buildErr != nil {
				t.Fatalf("rebuild embedded baseline catalog: %v", buildErr)
			}
			embeddedBytes, readErr := fs.ReadFile(embedded.FS, "skills/bootstrap/SKILL.md")
			if readErr != nil {
				t.Fatalf("read embedded bootstrap skill: %v", readErr)
			}
			if got := baseline.Count("asset/skill/bootstrap"); got != 1 {
				t.Errorf("embedded bootstrap appears %d times after rejection, want 1", got)
			}
			if !bytes.Equal(baseline.Contents["asset/skill/bootstrap"], embeddedBytes) {
				t.Error("embedded bootstrap content changed after an override rejection")
			}
		})
	}
}

// TestCatalog_DuplicateCustomRejected verifies design invariant 1 (second
// half) and REQ-REG-002 (SC-REG2-E): two custom declarations sharing one ID
// fail even when their content is identical.
func TestCatalog_DuplicateCustomRejected(t *testing.T) {
	content := []byte("# dup-helper\nSame bytes declared twice.\n")
	effective, err := BuildEffectiveCatalog([]CustomSkill{
		{ID: model.SkillID("dup-helper"), Content: content},
		{ID: model.SkillID("dup-helper"), Content: content},
	})
	if err == nil {
		t.Fatal("duplicate custom skill ID was accepted")
	}
	var collisionErr *CollisionError
	if !errors.As(err, &collisionErr) {
		t.Fatalf("error for duplicate custom ID is %T (%v), want *CollisionError", err, err)
	}
	if collisionErr.SkillID != "dup-helper" {
		t.Errorf("CollisionError.SkillID = %q, want %q", collisionErr.SkillID, "dup-helper")
	}
	if len(effective.Catalog.Assets) != 0 {
		t.Errorf("rejected overlay must not return a partial catalog; got %d assets", len(effective.Catalog.Assets))
	}
}
