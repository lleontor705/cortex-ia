package assets

import (
	"regexp"
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
)

var catalogForbiddenAliasPattern = regexp.MustCompile(`(?i)\b(?:` + strings.Join([]string{
	string([]byte{'s', 'o', 'n', 'n', 'e', 't'}),
	string([]byte{'o', 'p', 'u', 's'}),
	string([]byte{'h', 'a', 'i', 'k', 'u'}),
}, "|") + `)\b`)

func TestOperationalCatalogIncludesGeneratedReferencesExactlyOnce(t *testing.T) {
	catalog, err := BuildOperationalCatalog()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"asset/contract/phase-envelope",
		"asset/contract/status-vocabulary",
		"asset/contract/phase-schema",
		"asset/contract/policy",
		"asset/root/policy",
	}
	for _, id := range want {
		if got := catalog.Count(id); got != 1 {
			t.Fatalf("generated asset %q count = %d, want exactly once", id, got)
		}
	}
	if catalog.GeneratorVersion == "" || catalog.SourceFingerprint == "" {
		t.Fatalf("catalog metadata is incomplete: version=%q fingerprint=%q", catalog.GeneratorVersion, catalog.SourceFingerprint)
	}
	for _, spec := range catalog.Catalog.Assets {
		if strings.Contains(spec.SourcePath, "sdd-phase-common.md") {
			t.Fatalf("compatibility pointer was cataloged: %q", spec.SourcePath)
		}
		if strings.HasPrefix(string(spec.ID), "phase/") && (strings.Contains(string(spec.ID), "bootstrap") || strings.Contains(string(spec.ID), "investigate") || strings.Contains(string(spec.ID), "validate")) {
			t.Fatalf("alias leaked into stored phase ID: %q", spec.ID)
		}
	}
}

func TestOperationalCatalogHasOneRootAuthority(t *testing.T) {
	catalog, err := BuildOperationalCatalog()
	if err != nil {
		t.Fatal(err)
	}

	var roots []ir.AssetSpec
	for _, asset := range catalog.Catalog.Assets {
		if asset.Class == ir.AssetRootIndex {
			roots = append(roots, asset)
		}
	}
	if len(roots) != 1 {
		t.Fatalf("root authorities = %d, want exactly one: %#v", len(roots), roots)
	}
	for _, asset := range catalog.Catalog.Assets {
		if asset.ID == "asset/root/policy" && asset.Class == ir.AssetRootIndex {
			t.Fatal("generated root policy must remain a supplemental root module, not root-index authority")
		}
	}
}

func TestOperationalCatalogGeneratedAssetsHaveUniqueBoundedProviderNeutralContent(t *testing.T) {
	catalog, err := BuildOperationalCatalog()
	if err != nil {
		t.Fatal(err)
	}
	seen := make(map[string]struct{}, len(catalog.Catalog.Assets))
	for _, spec := range catalog.Catalog.Assets {
		if _, ok := seen[string(spec.ID)]; ok {
			t.Fatalf("duplicate generated catalog semantic ID %q", spec.ID)
		}
		seen[string(spec.ID)] = struct{}{}
		content := catalog.Contents[spec.ID]
		if catalogForbiddenAliasPattern.Match(content) {
			t.Fatalf("generated catalog asset %q contains an active provider-tier alias", spec.ID)
		}
		if spec.MaxTokens > 0 && estimateCatalogTokens(content) > spec.MaxTokens {
			t.Fatalf("generated catalog asset %q exceeds budget %d", spec.ID, spec.MaxTokens)
		}
	}
}

func estimateCatalogTokens(content []byte) int { return (len([]rune(string(content))) + 2) / 3 }
