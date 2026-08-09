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
			fixture.Resolved.Composition = testCompositionWithAssets(fixture.Resolved)
			fixture.Resolved.AllowedAssetKinds = appendMissingAssetKinds(fixture.Resolved.AllowedAssetKinds, AssetInstruction, AssetRule, AssetSkill)
			bundle, err := Render(context.Background(), fixture.Renderer, fixture.Resolved)
			if err != nil {
				t.Fatalf("Render() error = %v", err)
			}
			manifest := decodeCompositionManifest(t, bundle)
			if manifest["root_index"] == fixture.Resolved.Composition.RootIndex {
				t.Fatalf("root index was not lowered: %v", manifest["root_index"])
			}
			assertCompositionReferencesEmittedAssets(t, manifest, bundle)
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

func TestCompositionRejectsOrphanedReferences(t *testing.T) {
	fixture := codexFixtureForComposition()
	fixture.Resolved.Composition = testCompositionWithAssets(fixture.Resolved)
	fixture.Resolved.AllowedAssetKinds = appendMissingAssetKinds(fixture.Resolved.AllowedAssetKinds, AssetInstruction, AssetRule, AssetSkill)

	tests := []struct {
		name   string
		orphan func(*Composition)
	}{
		{name: "root index", orphan: func(composition *Composition) { composition.RootIndex = "missing/root.md" }},
		{name: "module", orphan: func(composition *Composition) { composition.Modules[0] = "missing/module.md" }},
		{name: "shared contract", orphan: func(composition *Composition) { composition.SharedContract = "missing/contract.md" }},
		{name: "profile overlay", orphan: func(composition *Composition) { composition.ProfileOverlay = "missing/overlay.md" }},
		{name: "quality template", orphan: func(composition *Composition) { composition.QualityTemplate = "missing/quality.md" }},
		{name: "skill binding", orphan: func(composition *Composition) { composition.SkillBindings[0].Path = "missing/SKILL.md" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resolved := fixture.Resolved
			resolved.Composition = cloneTestComposition(fixture.Resolved.Composition)
			test.orphan(&resolved.Composition)
			if _, err := Render(context.Background(), fixture.Renderer, resolved); err == nil || !strings.Contains(err.Error(), "orphaned") {
				t.Fatalf("Render() error = %v, want orphaned reference", err)
			}
		})
	}
}

func TestCompositionUsesQualifiedNativePreloadOrFallbackRead(t *testing.T) {
	fixture := rendererConformanceFixtures()[0]
	fixture.Resolved.Composition = testCompositionWithAssets(fixture.Resolved)
	fixture.Resolved.AllowedAssetKinds = appendMissingAssetKinds(fixture.Resolved.AllowedAssetKinds, AssetInstruction, AssetRule, AssetSkill)
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
	fixture.Resolved.Composition = testCompositionWithAssets(fixture.Resolved)
	fixture.Resolved.AllowedAssetKinds = appendMissingAssetKinds(fixture.Resolved.AllowedAssetKinds, AssetInstruction, AssetRule, AssetSkill)
	fixture.Resolved.NativeSkillPreload = true
	if _, err := Render(context.Background(), fixture.Renderer, fixture.Resolved); err == nil {
		t.Fatal("Render() succeeded with native preload and fallback bindings")
	}
}

func TestComposedCommonSkillsHaveOneRendererOwner(t *testing.T) {
	fixture := codexFixtureForComposition()
	fixture.Resolved.Composition = testCompositionWithAssets(fixture.Resolved)
	fixture.Resolved.AllowedAssetKinds = appendMissingAssetKinds(fixture.Resolved.AllowedAssetKinds, AssetInstruction, AssetRule, AssetSkill)
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
		skill := canonicalCompositionSkills[role.ID]
		bindings = append(bindings, SkillBinding{Role: role.ID, Skill: skill, Mode: SkillModeFallbackRead, Path: "skills/" + strings.TrimPrefix(string(skill), "skill/") + "/SKILL.md", Hash: "hash-" + string(role.ID)})
	}
	return Composition{RootIndex: "sdd-root/index.md", Modules: []string{"sdd-root/routing.md", "sdd-root/contracts.md"}, SkillBindings: bindings, SharedContract: "skills/_shared/contract.md", ProfileOverlay: "profiles/portable-sequential.md", QualityTemplate: "quality/plan-template.json"}
}

func testCompositionWithAssets(resolved ResolvedWorkflow) Composition {
	composition := testComposition(resolved)
	composition.OperationalAssets = []CompositionAsset{
		{ID: "asset/root-index", Class: ir.AssetRootIndex, Path: composition.RootIndex, Content: []byte("root")},
		{ID: "asset/root-module/routing", Class: ir.AssetRootModule, Path: composition.Modules[0], Content: []byte("routing")},
		{ID: "asset/root-module/contracts", Class: ir.AssetRootModule, Path: composition.Modules[1], Content: []byte("contracts")},
		{ID: "asset/shared-contract", Class: ir.AssetSharedContract, Path: composition.SharedContract, Content: []byte("shared")},
		{ID: "asset/profile-overlay", Class: ir.AssetProfileOverlay, Path: composition.ProfileOverlay, Content: []byte("overlay")},
		{ID: "asset/quality-template", Class: ir.AssetQualityTemplate, Path: composition.QualityTemplate, Content: []byte("quality")},
	}
	for _, binding := range composition.SkillBindings {
		composition.OperationalAssets = append(composition.OperationalAssets, CompositionAsset{ID: ir.SemanticID("asset/" + string(binding.Skill)), Class: ir.AssetSkill, Path: binding.Path, Content: []byte(binding.Skill)})
	}
	return composition
}

func cloneTestComposition(input Composition) Composition {
	result := input
	result.Modules = slices.Clone(input.Modules)
	result.SkillBindings = slices.Clone(input.SkillBindings)
	result.OperationalAssets = slices.Clone(input.OperationalAssets)
	return result
}

func appendMissingAssetKinds(kinds []AssetKind, required ...AssetKind) []AssetKind {
	result := slices.Clone(kinds)
	for _, kind := range required {
		if !slices.Contains(result, kind) {
			result = append(result, kind)
		}
	}
	return result
}

func assertCompositionReferencesEmittedAssets(t *testing.T, manifest map[string]any, bundle Bundle) {
	t.Helper()
	emitted := make(map[string]struct{}, len(bundle.Assets))
	for _, asset := range bundle.Assets {
		emitted[asset.Path] = struct{}{}
	}
	assertEmitted := func(field string, value any) {
		t.Helper()
		assetPath, ok := value.(string)
		if !ok {
			t.Fatalf("%s = %#v, want path string", field, value)
		}
		if _, ok := emitted[assetPath]; !ok {
			t.Fatalf("%s = %q, which is not an emitted asset", field, assetPath)
		}
	}
	for _, field := range []string{"root_index", "shared_contract", "profile_overlay", "quality_template"} {
		assertEmitted(field, manifest[field])
	}
	for _, module := range manifest["modules"].([]any) {
		assertEmitted("modules", module)
	}
	for _, rawBinding := range manifest["skill_bindings"].([]any) {
		binding := rawBinding.(map[string]any)
		assertEmitted("skill_bindings.path", binding["path"])
		if firstAction := binding["first_action"].(string); !strings.HasSuffix(firstAction, binding["path"].(string)) {
			t.Fatalf("first_action = %q, want lowered path %q", firstAction, binding["path"])
		}
	}
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
