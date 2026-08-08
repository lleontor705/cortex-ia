package renderers

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
)

func TestAllRenderersConsumeComposition(t *testing.T) {
	seen := map[TargetID]bool{}
	for _, fixture := range rendererConformanceFixtures() {
		if fixture.Resolved.Profile != "portable-sequential" || seen[fixture.Target] {
			continue
		}
		seen[fixture.Target] = true
		fixture := fixture
		t.Run(string(fixture.Target), func(t *testing.T) {
			fixture.Resolved.Composition = testComposition(fixture.Resolved)
			bundle, err := Render(context.Background(), fixture.Renderer, fixture.Resolved)
			if err != nil {
				t.Fatalf("Render() error = %v", err)
			}
			manifest := decodeCompositionManifest(t, bundle)
			if manifest["root_index"] != fixture.Resolved.Composition.RootIndex {
				t.Fatalf("root index = %v, want %q", manifest["root_index"], fixture.Resolved.Composition.RootIndex)
			}
			if len(manifest["skill_bindings"].([]any)) != len(fixture.Resolved.Workflow.Roles) {
				t.Fatalf("skill binding count = %d, want %d", len(manifest["skill_bindings"].([]any)), len(fixture.Resolved.Workflow.Roles))
			}
			encoded, _ := json.Marshal(bundle)
			if strings.Contains(strings.ToLower(string(encoded)), "mailbox") || strings.Contains(strings.ToLower(string(encoded)), "team-lead") || strings.Contains(strings.ToLower(string(encoded)), "a2a_") {
				t.Fatal("composition output contains retired coordination surface")
			}
		})
	}
	if len(seen) != 4 {
		t.Fatalf("renderer coverage = %d, want 4", len(seen))
	}
}

func TestCompositionUsesQualifiedNativePreloadOrFallbackRead(t *testing.T) {
	fixture := rendererConformanceFixtures()[0]
	fixture.Resolved.Composition = testComposition(fixture.Resolved)
	fixture.Resolved.NativeSkillPreload = true
	for index := range fixture.Resolved.Composition.SkillBindings {
		fixture.Resolved.Composition.SkillBindings[index].Mode = SkillModeNativePreload
	}
	bundle, err := Render(context.Background(), fixture.Renderer, fixture.Resolved)
	if err != nil {
		t.Fatalf("qualified native Render() error = %v", err)
	}
	manifest := decodeCompositionManifest(t, bundle)
	for _, binding := range manifest["skill_bindings"].([]any) {
		if binding.(map[string]any)["mode"] != string(SkillModeNativePreload) {
			t.Fatalf("binding mode = %v, want native-preload", binding.(map[string]any)["mode"])
		}
	}
}

func TestCompositionRejectsUnqualifiedNativePreload(t *testing.T) {
	fixture := rendererConformanceFixtures()[0]
	fixture.Resolved.Composition = testComposition(fixture.Resolved)
	fixture.Resolved.NativeSkillPreload = true
	if _, err := Render(context.Background(), fixture.Renderer, fixture.Resolved); err == nil {
		t.Fatal("Render() succeeded with native preload and fallback bindings")
	}
}

func TestComposedCommonSkillsHaveOneRendererOwner(t *testing.T) {
	fixture := codexFixtureForComposition()
	fixture.Resolved.Composition = testComposition(fixture.Resolved)
	fixture.Resolved.Composition.OperationalAssets = []CompositionAsset{{
		ID: "asset/skill/implement", Class: ir.AssetSkill, Path: "skills/implement/SKILL.md", Content: []byte("implement"),
	}}
	bundle, err := Render(context.Background(), fixture.Renderer, fixture.Resolved)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	count := 0
	for _, asset := range bundle.Assets {
		if asset.Path == ".codex/skills/implement/SKILL.md" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("common skill ownership count = %d, want 1", count)
	}
}

func codexFixtureForComposition() conformanceFixture {
	for _, fixture := range rendererConformanceFixtures() {
		if fixture.Target == "codex" && fixture.Resolved.Profile == "portable-sequential" {
			return fixture
		}
	}
	panic("codex portable-sequential fixture missing")
}

func TestAdapterAssetMapsCoverAllTargetsAndProfiles(t *testing.T) {
	targets := []TargetID{"claude", "opencode", "vscode", "codex"}
	profiles := []string{"portable-sequential", "portable-flat", "native-advanced"}
	for _, target := range targets {
		assetMap, err := AdapterAssetMapFor(target)
		if err != nil {
			t.Fatalf("%s map: %v", target, err)
		}
		if assetMap.WorkflowRoot == "" || assetMap.SkillsRoot == "" || assetMap.AgentsRoot == "" || assetMap.RoleRoot == "" || assetMap.OverlayRoot == "" || assetMap.QualityRoot == "" || assetMap.ManifestRoot == "" || assetMap.ModelRoot == "" || assetMap.PermissionRoot == "" {
			t.Fatalf("%s map omitted a required typed root: %+v", target, assetMap)
		}
		for _, profile := range profiles {
			if err := assetMap.ValidateProfile(profile); err != nil {
				t.Fatalf("%s/%s map: %v", target, profile, err)
			}
		}
	}
}

func testComposition(resolved ResolvedWorkflow) Composition {
	bindings := make([]SkillBinding, 0, len(resolved.Workflow.Roles))
	roles := slices.Clone(resolved.Workflow.Roles)
	slices.SortFunc(roles, func(left, right ir.Role) int { return strings.Compare(string(left.ID), string(right.ID)) })
	for _, role := range roles {
		bindings = append(bindings, SkillBinding{Role: role.ID, Skill: canonicalCompositionSkills[role.ID], Mode: SkillModeFallbackRead, Path: "skills/" + string(role.ID) + "/SKILL.md", Hash: "hash-" + string(role.ID)})
	}
	return Composition{RootIndex: "sdd-root/index.md", Modules: []string{"sdd-root/routing.md", "sdd-root/contracts.md"}, SkillBindings: bindings, SharedContract: "skills/_shared/contract.md", ProfileOverlay: "profiles/portable-sequential.md", QualityTemplate: "quality/plan-template.json"}
}

func decodeCompositionManifest(t *testing.T, bundle Bundle) map[string]any {
	t.Helper()
	for _, asset := range bundle.Assets {
		if !strings.HasSuffix(asset.Path, "composition.json") {
			continue
		}
		var manifest map[string]any
		if err := json.Unmarshal(asset.Content, &manifest); err != nil {
			t.Fatalf("decode composition manifest: %v", err)
		}
		return manifest
	}
	t.Fatal("composition manifest missing")
	return nil
}
