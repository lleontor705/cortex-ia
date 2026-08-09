package install

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
)

func TestOwnershipStorePersistsStableSidecarAndInstallBase(t *testing.T) {
	root := t.TempDir()
	content := []byte("generated workflow\n")
	metadata, err := NewOwnership("agents/implement.md", "1.4.0", ir.SemanticID("asset/agent/implement"), content, content)
	if err != nil {
		t.Fatalf("NewOwnership() error = %v", err)
	}

	store := NewOwnershipStore(root)
	if err := store.Write(metadata, content); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	got, base, err := store.Read("agents/implement.md")
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if got != metadata {
		t.Fatalf("metadata = %+v, want %+v", got, metadata)
	}
	if string(base) != string(content) {
		t.Fatalf("base = %q, want %q", base, content)
	}
	if got.Owner != OwnerCortexIA || got.GeneratorVersion != "1.4.0" || got.SemanticID != "asset/agent/implement" {
		t.Fatalf("identity fields = %+v", got)
	}
	if got.BaseSHA256 != SHA256(content) || got.ContentSHA256 != SHA256(content) {
		t.Fatalf("hashes = base %q content %q", got.BaseSHA256, got.ContentSHA256)
	}

	wantSidecar := filepath.Join(root, "agents", "implement.md.cortex-ia.json")
	wantBase := filepath.Join(root, "agents", "implement.md.cortex-ia.base")
	for _, path := range []string{wantSidecar, wantBase} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected sidecar %q: %v", path, err)
		}
	}
}

func TestOwnershipStoreCentralizesOpenCodeEvidence(t *testing.T) {
	root := t.TempDir()
	content := []byte("generated command\n")
	metadata, err := NewOwnership(".config/opencode/commands/implement.md", "1.4.0", "asset/opencode/command/implement", content, content)
	if err != nil {
		t.Fatal(err)
	}
	store := NewOwnershipStore(root)
	if err := store.Write(metadata, content); err != nil {
		t.Fatal(err)
	}
	evidence, err := store.ReadEvidence(metadata.AssetPath)
	if err != nil {
		t.Fatal(err)
	}
	if evidence.Legacy {
		t.Fatal("new OpenCode ownership was reported as legacy")
	}
	if got, want := evidence.OwnershipPath, ".cortex-ia/opencode/ownership/commands/implement.md.cortex-ia.json"; got != want {
		t.Fatalf("ownership path = %q, want %q", got, want)
	}
	if got, want := evidence.BasePath, ".cortex-ia/opencode/ownership/commands/implement.md.cortex-ia.base"; got != want {
		t.Fatalf("base path = %q, want %q", got, want)
	}
	for _, path := range []string{evidence.OwnershipPath, evidence.BasePath} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(path))); err != nil {
			t.Fatalf("canonical evidence %q: %v", path, err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, ".config", "opencode", "commands", "implement.md.cortex-ia.json")); !os.IsNotExist(err) {
		t.Fatalf("new write created adjacent legacy ownership: %v", err)
	}
}

func TestOwnershipStoreFallsBackToAdjacentOpenCodeEvidenceButPrefersCanonical(t *testing.T) {
	root := t.TempDir()
	assetPath := ".config/opencode/agents/implement.md"
	legacyContent := []byte("legacy\n")
	writeLegacyOwnership(t, root, assetPath, "asset/opencode/agent/implement", legacyContent)
	store := NewOwnershipStore(root)
	evidence, err := store.ReadEvidence(assetPath)
	if err != nil {
		t.Fatal(err)
	}
	if !evidence.Legacy || string(evidence.Base) != string(legacyContent) {
		t.Fatalf("legacy evidence = %+v, base %q", evidence, evidence.Base)
	}
	if got, want := evidence.OwnershipPath, assetPath+sidecarSuffix; got != want {
		t.Fatalf("legacy ownership path = %q, want %q", got, want)
	}
	canonicalContent := []byte("canonical\n")
	canonical, err := NewOwnership(assetPath, "1.4.0", "asset/opencode/agent/implement", canonicalContent, canonicalContent)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Write(canonical, canonicalContent); err != nil {
		t.Fatal(err)
	}
	evidence, err = store.ReadEvidence(assetPath)
	if err != nil {
		t.Fatal(err)
	}
	if evidence.Legacy || string(evidence.Base) != string(canonicalContent) {
		t.Fatalf("canonical-first evidence = %+v, base %q", evidence, evidence.Base)
	}
}

func TestOwnershipStoreDoesNotMaskCorruptCanonicalEvidenceWithLegacyFallback(t *testing.T) {
	root := t.TempDir()
	assetPath := ".config/opencode/agents/implement.md"
	writeLegacyOwnership(t, root, assetPath, "asset/opencode/agent/implement", []byte("legacy\n"))
	writeTarget(t, root, ".cortex-ia/opencode/ownership/agents/implement.md.cortex-ia.json", []byte("not json\n"), 0o600)

	evidence, err := NewOwnershipStore(root).ReadEvidence(assetPath)
	if err == nil {
		t.Fatalf("ReadEvidence() = %+v, want corrupt canonical error", evidence)
	}
	if evidence.Legacy {
		t.Fatal("corrupt canonical evidence was masked by legacy fallback")
	}
}

func writeLegacyOwnership(t *testing.T, root, assetPath string, semanticID ir.SemanticID, content []byte) {
	t.Helper()
	metadata, err := NewOwnership(assetPath, "1.0.0", semanticID, content, content)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	writeTarget(t, root, assetPath+sidecarSuffix, append(encoded, '\n'), 0o600)
	writeTarget(t, root, assetPath+baseSuffix, content, 0o600)
}

func TestOwnershipMarkerRoundTripsIdentityAndHashes(t *testing.T) {
	content := []byte("managed body\n")
	metadata, err := NewRegionOwnership("rules/sdd.md", "2.0.1", "region/rule/sdd", content, content)
	if err != nil {
		t.Fatalf("NewOwnership() error = %v", err)
	}

	marker, err := FormatOwnershipMarker("<!-- ", " -->", metadata)
	if err != nil {
		t.Fatalf("FormatOwnershipMarker() error = %v", err)
	}
	if !strings.Contains(marker, MarkerPrefix) {
		t.Fatalf("marker %q does not contain %q", marker, MarkerPrefix)
	}
	got, err := ParseOwnershipMarker(marker)
	if err != nil {
		t.Fatalf("ParseOwnershipMarker() error = %v", err)
	}
	if got != metadata {
		t.Fatalf("parsed metadata = %+v, want %+v", got, metadata)
	}
	if got.Scope != OwnershipScopeRegion {
		t.Fatalf("marker scope = %q, want %q", got.Scope, OwnershipScopeRegion)
	}
}

func TestOwnershipStoreKeepsMultipleRegionSidecarsDistinct(t *testing.T) {
	root := t.TempDir()
	store := NewOwnershipStore(root)
	regions := []struct {
		id      ir.SemanticID
		content []byte
	}{
		{id: "region/rule/security", content: []byte("security\n")},
		{id: "region/rule/testing", content: []byte("testing\n")},
	}

	for _, region := range regions {
		metadata, err := NewRegionOwnership("AGENTS.md", "1.0.0", region.id, region.content, region.content)
		if err != nil {
			t.Fatalf("NewRegionOwnership(%q) error = %v", region.id, err)
		}
		if err := store.Write(metadata, region.content); err != nil {
			t.Fatalf("Write(%q) error = %v", region.id, err)
		}
	}

	for _, region := range regions {
		got, base, err := store.ReadRegion("AGENTS.md", region.id)
		if err != nil {
			t.Fatalf("ReadRegion(%q) error = %v", region.id, err)
		}
		if got.SemanticID != region.id || string(base) != string(region.content) {
			t.Fatalf("ReadRegion(%q) = %+v, %q", region.id, got, base)
		}
	}
}

func TestInspectOwnershipDistinguishesCustomizationCorruptionAndUnknown(t *testing.T) {
	generated := []byte("generated\n")
	metadata, err := NewOwnership("skills/apply.md", "1.0.0", "asset/skill/apply", generated, generated)
	if err != nil {
		t.Fatalf("NewOwnership() error = %v", err)
	}

	tests := []struct {
		name     string
		current  []byte
		metadata *Ownership
		base     []byte
		want     OwnershipState
	}{
		{name: "clean managed asset", current: generated, metadata: &metadata, base: generated, want: OwnershipClean},
		{name: "user customization", current: []byte("user edit\n"), metadata: &metadata, base: generated, want: OwnershipCustomized},
		{name: "corrupt recorded base", current: generated, metadata: &metadata, base: []byte("tampered base\n"), want: OwnershipCorrupt},
		{name: "unknown without trustworthy metadata", current: generated, want: OwnershipUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := InspectOwnership(tt.current, tt.metadata, tt.base)
			if got.State != tt.want {
				t.Fatalf("InspectOwnership() state = %q, want %q (reason: %s)", got.State, tt.want, got.Reason)
			}
		})
	}
}

func TestDestructiveReplacementRequiresDisclosedTakeoverForUnknownOwnership(t *testing.T) {
	unknown := Inspection{State: OwnershipUnknown, Reason: "ownership metadata is absent"}

	err := AuthorizeReplacement(unknown, ReplacementRequest{Destructive: true})
	if !errors.Is(err, ErrUnknownOwnership) {
		t.Fatalf("AuthorizeReplacement() error = %v, want ErrUnknownOwnership", err)
	}
	err = AuthorizeReplacement(unknown, ReplacementRequest{Destructive: true, Takeover: true})
	if !errors.Is(err, ErrTakeoverNotDisclosed) {
		t.Fatalf("undisclosed takeover error = %v, want ErrTakeoverNotDisclosed", err)
	}
	if err := AuthorizeReplacement(unknown, ReplacementRequest{Destructive: true, Takeover: true, TakeoverDisclosed: true}); err != nil {
		t.Fatalf("disclosed takeover error = %v", err)
	}
	if err := AuthorizeReplacement(unknown, ReplacementRequest{Destructive: false}); err != nil {
		t.Fatalf("non-destructive replacement error = %v", err)
	}
}
