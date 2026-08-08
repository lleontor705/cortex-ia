package renderers

import (
	"testing"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
)

func TestAdapterAssetMapMapsSemanticAssetsOnce(t *testing.T) {
	m, err := AdapterAssetMapFor("opencode")
	if err != nil {
		t.Fatal(err)
	}
	got, err := m.Map(ir.SemanticID("asset/skill/implement"), ir.AssetSkill, "skills/implement/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	if got.Scope != ir.ScopeWorkflowRoot || got.Relative != ".config/opencode/skills/implement/SKILL.md" {
		t.Fatalf("destination = %+v", got)
	}
	if _, err := m.Map(ir.SemanticID("asset/skill/implement"), ir.AssetSkill, "skills/implement/SKILL.md"); err == nil {
		t.Fatal("duplicate semantic asset unexpectedly mapped")
	}
}

func TestAdapterAssetMapKeepsRoleStubsOutsideNativeAgentDiscovery(t *testing.T) {
	m, err := AdapterAssetMapFor("opencode")
	if err != nil {
		t.Fatal(err)
	}
	got, err := m.Map("asset/role/implement/binding", ir.AssetRoleStub, ".config/opencode/roles/implement.md")
	if err != nil {
		t.Fatal(err)
	}
	if got.Relative != ".config/opencode/.cortex-ia/roles/implement.md" {
		t.Fatalf("role stub destination = %q", got.Relative)
	}
}

func TestAdapterAssetMapRejectsUnsafeAndDuplicateDestinations(t *testing.T) {
	m, err := AdapterAssetMapFor("vscode")
	if err != nil {
		t.Fatal(err)
	}
	for _, relative := range []string{"../escape.md", ".copilot/skills/x.md", "internal/x.md"} {
		if _, err := m.Map(ir.SemanticID("asset/"+relative), ir.AssetSkill, relative); err == nil {
			t.Errorf("Map(%q) unexpectedly succeeded", relative)
		}
	}
	if _, err := m.Map("asset/one", ir.AssetSkill, "skills/shared.md"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Map("asset/two", ir.AssetSkill, "skills/shared.md"); err == nil {
		t.Fatal("duplicate destination unexpectedly succeeded")
	}
}

func TestAdapterAssetMapRejectsUnsupportedTarget(t *testing.T) {
	if _, err := AdapterAssetMapFor("unsupported"); err == nil {
		t.Fatal("unsupported target unexpectedly has an asset map")
	}
}
