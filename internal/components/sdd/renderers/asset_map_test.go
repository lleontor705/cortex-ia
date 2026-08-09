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
	if got.Relative != ".cortex-ia/opencode/roles/implement.md" {
		t.Fatalf("role stub destination = %q", got.Relative)
	}
}

func TestOpenCodeAssetMapSeparatesNativeAndInternalAssets(t *testing.T) {
	tests := []struct {
		name     string
		id       ir.SemanticID
		class    ir.AssetClass
		relative string
		want     string
	}{
		{name: "native skill", id: "asset/skill/implement", class: ir.AssetSkill, relative: "skills/implement/SKILL.md", want: ".config/opencode/skills/implement/SKILL.md"},
		{name: "shared skill contract", id: "asset/skill/shared/convention", class: ir.AssetSkill, relative: "skills/_shared/cortex-convention.md", want: ".cortex-ia/opencode/contracts/_shared/cortex-convention.md"},
		{name: "root module", id: "asset/root-module/contracts", class: ir.AssetRootModule, relative: "sdd-root/contracts.md", want: ".cortex-ia/opencode/root/contracts.md"},
		{name: "shared contract", id: "asset/shared-contract", class: ir.AssetSharedContract, relative: "skills/_shared/sdd-phase-contract.md", want: ".cortex-ia/opencode/contracts/_shared/sdd-phase-contract.md"},
		{name: "overlay", id: "asset/profile-overlay/flat", class: ir.AssetProfileOverlay, relative: "profiles/portable-flat.md", want: ".cortex-ia/opencode/overlays/portable-flat.md"},
		{name: "quality", id: "asset/quality-template", class: ir.AssetQualityTemplate, relative: "quality/plan-template.json", want: ".cortex-ia/opencode/quality/plan-template.json"},
		{name: "manifest", id: "asset/manifest/security", class: ir.AssetManifest, relative: "manifests/security.json", want: ".cortex-ia/opencode/manifests/security.json"},
		{name: "contract schema", id: "asset/contract/phase", class: ir.AssetContractSchema, relative: "contracts/phase.json", want: ".cortex-ia/opencode/contracts/phase.json"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			m, err := AdapterAssetMapFor("opencode")
			if err != nil {
				t.Fatal(err)
			}
			got, err := m.Map(test.id, test.class, test.relative)
			if err != nil {
				t.Fatal(err)
			}
			if got.Relative != test.want {
				t.Fatalf("destination = %q, want %q", got.Relative, test.want)
			}
		})
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
