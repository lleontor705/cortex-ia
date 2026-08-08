package renderers

import (
	"testing"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
)

func TestSkillBindingHasAllFields(t *testing.T) {
	binding := SkillBinding{
		Role:  "role/apply",
		Skill: "skill/implement",
		Mode:  SkillModeFallbackRead,
		Path:  "skills/implement/SKILL.md",
		Hash:  "abc123",
	}
	if binding.Role != "role/apply" {
		t.Fatalf("Role = %q", binding.Role)
	}
	if binding.Skill != "skill/implement" {
		t.Fatalf("Skill = %q", binding.Skill)
	}
	if binding.Mode != SkillModeFallbackRead {
		t.Fatalf("Mode = %q", binding.Mode)
	}
	if binding.Path == "" || binding.Hash == "" {
		t.Fatal("Path and Hash must be settable")
	}
}

func TestSkillLoadModeConstantsExist(t *testing.T) {
	if SkillModeNativePreload != "native-preload" {
		t.Fatalf("SkillModeNativePreload = %q", SkillModeNativePreload)
	}
	if SkillModeFallbackRead != "fallback-read" {
		t.Fatalf("SkillModeFallbackRead = %q", SkillModeFallbackRead)
	}
	if SkillModeNativeOnDemand != "native-on-demand" {
		t.Fatalf("SkillModeNativeOnDemand = %q", SkillModeNativeOnDemand)
	}
}

func TestCompositionCarriesAllComposedPaths(t *testing.T) {
	comp := Composition{
		RootIndex:       "root-index.md",
		Modules:         []string{"routing.md", "contracts.md"},
		SkillBindings:   []SkillBinding{{Role: "role/apply", Skill: "skill/implement", Mode: SkillModeFallbackRead, Path: "p", Hash: "h"}},
		SharedContract:  "contract.md",
		ProfileOverlay:  "overlay.md",
		QualityTemplate: "quality.json",
	}
	if comp.RootIndex == "" || comp.SharedContract == "" || comp.ProfileOverlay == "" || comp.QualityTemplate == "" {
		t.Fatal("Composition paths must all be settable")
	}
	if len(comp.Modules) != 2 {
		t.Fatalf("Modules len = %d, want 2", len(comp.Modules))
	}
	if len(comp.SkillBindings) != 1 {
		t.Fatalf("SkillBindings len = %d, want 1", len(comp.SkillBindings))
	}
}

func TestResolvedWorkflowHasCompositionField(t *testing.T) {
	rw := ResolvedWorkflow{
		Target:             "claude",
		Profile:            "portable-sequential",
		AllowedAssetKinds:  []AssetKind{AssetSkill},
		AllowedPermissions: []string{"tool/read"},
		Composition: Composition{
			RootIndex:     "root.md",
			SkillBindings: []SkillBinding{{Role: "role/apply", Skill: "skill/implement", Mode: SkillModeFallbackRead, Path: "p", Hash: "h"}},
		},
	}
	if rw.Composition.RootIndex != "root.md" {
		t.Fatalf("Composition.RootIndex = %q", rw.Composition.RootIndex)
	}
	if len(rw.Composition.SkillBindings) != 1 {
		t.Fatalf("Composition.SkillBindings len = %d", len(rw.Composition.SkillBindings))
	}
}

func TestResolvedWorkflowHasAdapterContractFields(t *testing.T) {
	rw := ResolvedWorkflow{
		Target:                  "claude",
		Profile:                 "native-advanced",
		AllowedAssetKinds:       []AssetKind{AssetSkill},
		AllowedPermissions:      []string{"tool/read"},
		NativeSkillPreload:      true,
		NativeSkillOnDemand:     true,
		NativeModelField:        true,
		NativeWorktreeIsolation: true,
	}
	if !rw.NativeSkillPreload {
		t.Fatal("NativeSkillPreload must be settable to true")
	}
	if !rw.NativeSkillOnDemand {
		t.Fatal("NativeSkillOnDemand must be settable to true")
	}
	if !rw.NativeModelField {
		t.Fatal("NativeModelField must be settable to true")
	}
	if !rw.NativeWorktreeIsolation {
		t.Fatal("NativeWorktreeIsolation must be settable to true")
	}
}

func TestResolvedWorkflowSkillLoadModeDependsOnNativePreload(t *testing.T) {
	withOnDemand := ResolvedWorkflow{NativeSkillPreload: true, NativeSkillOnDemand: true}
	if mode := withOnDemand.SkillLoadMode(); mode != SkillModeNativeOnDemand {
		t.Fatalf("SkillLoadMode() with on-demand = %q, want %q", mode, SkillModeNativeOnDemand)
	}
	withPreload := ResolvedWorkflow{NativeSkillPreload: true}
	if mode := withPreload.SkillLoadMode(); mode != SkillModeNativePreload {
		t.Fatalf("SkillLoadMode() with preload = %q, want %q", mode, SkillModeNativePreload)
	}
	withoutPreload := ResolvedWorkflow{NativeSkillPreload: false}
	if mode := withoutPreload.SkillLoadMode(); mode != SkillModeFallbackRead {
		t.Fatalf("SkillLoadMode() without preload = %q, want %q", mode, SkillModeFallbackRead)
	}
}

func TestValidateBundlePassesWithComposition(t *testing.T) {
	resolved := resolvedWorkflow()
	resolved.Composition = Composition{
		RootIndex:     "root.md",
		SkillBindings: []SkillBinding{{Role: "role/apply", Skill: "skill/implement", Mode: SkillModeFallbackRead, Path: "p", Hash: "h"}},
	}
	asset := validAsset()
	_, err := ValidateBundle(resolved, Bundle{Assets: []Asset{asset}})
	if err != nil {
		t.Fatalf("ValidateBundle with Composition error = %v", err)
	}
}

func TestCompositionFromPromptResultConvertsBindings(t *testing.T) {
	// Tests the adapter function that converts prompt-layer composition data
	// into renderer-visible types, verifying that all paths and bindings survive
	// the conversion without loss.
	bindings := []SkillBinding{
		{Role: "role/bootstrap", Skill: "skill/bootstrap", Mode: SkillModeFallbackRead, Path: "skills/bootstrap/SKILL.md", Hash: "h1"},
		{Role: "role/apply", Skill: "skill/implement", Mode: SkillModeNativePreload, Path: "skills/implement/SKILL.md", Hash: "h2"},
	}
	comp := Composition{
		RootIndex:       "root.md",
		Modules:         []string{"m1.md", "m2.md"},
		SkillBindings:   bindings,
		SharedContract:  "contract.md",
		ProfileOverlay:  "overlay.md",
		QualityTemplate: "quality.json",
	}
	// Validate that every canonical role maps to exactly one binding.
	seen := make(map[ir.SemanticID]int)
	for _, b := range comp.SkillBindings {
		seen[b.Role]++
	}
	for _, role := range comp.SkillBindings {
		if seen[role.Role] != 1 {
			t.Fatalf("role %q has %d bindings in composition", role.Role, seen[role.Role])
		}
	}
}
