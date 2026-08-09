package assets

import (
	"slices"
	"strings"
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
	for _, id := range []string{"asset/root-index", "asset/shared-contract", "asset/skill/shared/cortex-convention", "asset/skill/shared/cortex-advanced", "asset/skill/bootstrap", "asset/skill/finalize", "asset/role/bootstrap", "asset/role/archive", "asset/profile-overlay/portable-sequential", "asset/manifest/model-routes"} {
		if _, ok := a.Contents[ir.SemanticID(id)]; !ok {
			t.Fatalf("required retained asset %q missing", id)
		}
	}
	for _, id := range []ir.SemanticID{
		"asset/skill/orchestrator", "asset/skill/debate", "asset/skill/parallel-dispatch",
		"asset/role/orchestrator", "asset/role/debate", "asset/role/parallel-dispatch",
	} {
		if _, ok := a.Contents[id]; !ok {
			t.Fatalf("transverse retained asset %q missing", id)
		}
	}
	orchestrator := string(a.Contents["asset/skill/orchestrator"])
	root := string(a.Contents["asset/root-index"])
	if orchestrator == root {
		t.Fatal("orchestrator skill must have its own identity, not reuse root index bytes")
	}
	for _, marker := range []string{
		"---\nname: orchestrator\n",
		"description:",
		"# SDD Orchestrator — Operational Index",
	} {
		if !strings.Contains(orchestrator, marker) {
			t.Errorf("orchestrator skill missing identity marker %q", marker)
		}
	}
	if !strings.HasPrefix(orchestrator, "---\n") || strings.Count(orchestrator, "---\n") < 2 {
		t.Fatal("orchestrator skill must contain OpenCode YAML frontmatter")
	}
	for _, phase := range []string{"init", "explore", "propose", "spec", "design", "tasks", "apply", "verify", "archive"} {
		if !strings.Contains(orchestrator, "| "+phase+" |") {
			t.Errorf("orchestrator skill missing canonical phase %q", phase)
		}
	}
	for _, spec := range a.Catalog.Assets {
		if strings.Contains(string(a.Contents[spec.ID]), "mem_") {
			t.Fatalf("retained asset %q contains legacy Cortex namespace", spec.ID)
		}
	}
}

func fingerprint(content []byte) string { return ir.FingerprintContent(content) }
