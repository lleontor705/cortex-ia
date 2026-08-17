package prompt

import (
	"errors"
	"path"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/quality"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/registry"
	"github.com/lleontor705/cortex-ia/internal/model"
)

// composerTestContract mirrors a minimal valid adapter prompt contract with
// the same shape production qualification produces: safe roots and a
// traversal-rejecting path expander.
func composerTestContract() AdapterPromptContract {
	return AdapterPromptContract{
		Target:      "opencode",
		RootPath:    ".config/opencode",
		AgentPath:   func(id ir.SemanticID) string { return path.Join(".config/opencode/agent", string(id)) },
		SkillRoot:   "skills",
		CommandRoot: "commands",
		ExpandPath: func(root, relative string) (string, error) {
			if strings.Contains(relative, "..") {
				return "", errors.New("relative path contains a traversal segment")
			}
			return path.Join(root, relative), nil
		},
	}
}

// composerBaselineInput builds the minimal composition-ready baseline input:
// a typed catalog with the mandatory root index, shared contract, quality
// template, and the portable-flat profile overlay, plus one canonical
// workflow role so skill bindings are exercised.
func composerBaselineInput(t *testing.T) CompositionInput {
	t.Helper()
	specFor := func(id ir.SemanticID, class ir.AssetClass, source string, maxTokens int) ir.AssetSpec {
		content := []byte("# " + string(id) + "\n")
		return ir.AssetSpec{
			ID: id, Class: class, SourcePath: source, Required: true,
			MaxTokens: maxTokens, SHA256: ir.FingerprintContent(content),
		}
	}
	overlay := specFor("asset/profile-overlay/portable-flat", ir.AssetProfileOverlay, "profile-overlay/portable-flat.md", 800)
	overlay.Profiles = []ir.SemanticID{"portable-flat"}
	return CompositionInput{
		Workflow: ir.WorkflowIR{Roles: []ir.Role{{ID: "role/bootstrap", Objective: "ground the change"}}},
		Catalog: ir.AssetCatalog{
			SchemaVersion: ir.AssetCatalogSchema.Current,
			Assets: []ir.AssetSpec{
				specFor("asset/root-index", ir.AssetRootIndex, "AGENTS.md", 1500),
				specFor("asset/shared-contract", ir.AssetSharedContract, "_shared/sdd-phase-contract.md", 1000),
				specFor("asset/quality-template", ir.AssetQualityTemplate, "generated/quality-plan-template.md", 800),
				overlay,
			},
		},
		Adapter: composerTestContract(),
		Profile: quality.ProfilePlan{ProfileID: "portable-flat"},
	}
}

// composerCustomSkill builds one registry-normalized custom skill record
// exactly as the registry merge delivers it: canonical UTF-8/LF content with
// the normalization-stage digest and custom origin.
func composerCustomSkill(t *testing.T, id, body string) registry.Skill {
	t.Helper()
	canonical, digest, err := registry.NormalizeContent([]byte(body))
	if err != nil {
		t.Fatalf("normalize custom skill content: %v", err)
	}
	return registry.Skill{
		ID: model.SkillID(id), Content: canonical, ContentSHA256: digest, Origin: registry.OriginCustom,
	}
}

// composerOverlaySpec returns the effective-catalog asset spec the additive
// overlay produces for one accepted custom skill.
func composerOverlaySpec(skill registry.Skill) ir.AssetSpec {
	return ir.AssetSpec{
		ID:         ir.SemanticID("asset/skill/" + string(skill.ID)),
		Class:      ir.AssetSkill,
		SourcePath: "skills/" + string(skill.ID) + "/SKILL.md",
		Required:   true,
		MaxTokens:  3500,
		SHA256:     skill.ContentSHA256,
	}
}

// withComposerOverlay attaches custom skill declarations to the input
// together with their effective-catalog asset specs, mirroring the additive
// layering the assets catalog produces for accepted custom skills.
func withComposerOverlay(t *testing.T, input CompositionInput, skills ...registry.Skill) CompositionInput {
	t.Helper()
	updated := input
	updated.Catalog.Assets = slices.Clone(input.Catalog.Assets)
	for _, skill := range skills {
		updated.Catalog.Assets = append(updated.Catalog.Assets, composerOverlaySpec(skill))
	}
	return updated
}

// requireEqualComposition asserts two compositions are identical on every
// comparable field. AdapterPromptContract carries function fields that
// reflect.DeepEqual can never match, so the adapter is compared on its
// non-function destination and capability fields.
func requireEqualComposition(t *testing.T, got, want CompositionResult) {
	t.Helper()
	if got.RootIndex != want.RootIndex {
		t.Errorf("RootIndex = %q, want %q", got.RootIndex, want.RootIndex)
	}
	if !reflect.DeepEqual(got.Modules, want.Modules) {
		t.Errorf("Modules = %v, want %v", got.Modules, want.Modules)
	}
	if !reflect.DeepEqual(got.SkillBindings, want.SkillBindings) {
		t.Errorf("SkillBindings = %v, want %v", got.SkillBindings, want.SkillBindings)
	}
	if got.SharedContract != want.SharedContract {
		t.Errorf("SharedContract = %q, want %q", got.SharedContract, want.SharedContract)
	}
	if got.ProfileOverlay != want.ProfileOverlay {
		t.Errorf("ProfileOverlay = %q, want %q", got.ProfileOverlay, want.ProfileOverlay)
	}
	if got.QualityTemplate != want.QualityTemplate {
		t.Errorf("QualityTemplate = %q, want %q", got.QualityTemplate, want.QualityTemplate)
	}
	if !reflect.DeepEqual(got.QualityPlan, want.QualityPlan) {
		t.Errorf("QualityPlan = %v, want %v", got.QualityPlan, want.QualityPlan)
	}
	if !reflect.DeepEqual(got.TokenReport, want.TokenReport) {
		t.Errorf("TokenReport = %v, want %v", got.TokenReport.Entries, want.TokenReport.Entries)
	}
	if !reflect.DeepEqual(got.OperationalAssets, want.OperationalAssets) {
		t.Errorf("OperationalAssets = %v, want %v", got.OperationalAssets, want.OperationalAssets)
	}
	if !reflect.DeepEqual(got.Metadata, want.Metadata) {
		t.Errorf("Metadata = %s, want %s", got.Metadata, want.Metadata)
	}
	if !reflect.DeepEqual(got.CustomSkills, want.CustomSkills) {
		t.Errorf("CustomSkills = %v, want %v", got.CustomSkills, want.CustomSkills)
	}
	if got.Adapter.Target != want.Adapter.Target {
		t.Errorf("Adapter.Target = %q, want %q", got.Adapter.Target, want.Adapter.Target)
	}
	if got.Adapter.RootPath != want.Adapter.RootPath {
		t.Errorf("Adapter.RootPath = %q, want %q", got.Adapter.RootPath, want.Adapter.RootPath)
	}
	if got.Adapter.SkillRoot != want.Adapter.SkillRoot {
		t.Errorf("Adapter.SkillRoot = %q, want %q", got.Adapter.SkillRoot, want.Adapter.SkillRoot)
	}
	if got.Adapter.CommandRoot != want.Adapter.CommandRoot {
		t.Errorf("Adapter.CommandRoot = %q, want %q", got.Adapter.CommandRoot, want.Adapter.CommandRoot)
	}
}

// requireZeroComposition asserts a rejected composition carries no partial
// output that a caller could mistake for a composed bundle.
func requireZeroComposition(t *testing.T, result CompositionResult) {
	t.Helper()
	if result.RootIndex != "" || result.SharedContract != "" || result.ProfileOverlay != "" || result.QualityTemplate != "" {
		t.Errorf("rejected composition leaked path fields: %+v", result)
	}
	if len(result.Modules) > 0 || len(result.SkillBindings) > 0 || len(result.CustomSkills) > 0 {
		t.Errorf("rejected composition leaked list fields: %+v", result)
	}
}

// TestComposer_OrderIndependence verifies REQ-REG-001 (SC-REG1-E) and
// REQ-DET-001 (SC-DET-H) at the composition boundary: custom skills declared
// in different input orders — in the overlay and in the effective catalog —
// produce identical compositions, because layering derives from the
// registry-normalized identities, never from arrival order.
func TestComposer_OrderIndependence(t *testing.T) {
	skills := []registry.Skill{
		composerCustomSkill(t, "custom-alpha", "# custom-alpha\n\nAlpha overlay body.\n"),
		composerCustomSkill(t, "custom-beta", "# custom-beta\n\nBeta overlay body.\n"),
		composerCustomSkill(t, "custom-gamma", "# custom-gamma\n\nGamma overlay body.\n"),
	}
	permutations := [][]int{
		{0, 1, 2},
		{2, 1, 0},
		{1, 2, 0},
	}
	permute := func(order []int) []registry.Skill {
		ordered := make([]registry.Skill, 0, len(order))
		for _, index := range order {
			ordered = append(ordered, skills[index])
		}
		return ordered
	}

	var reference CompositionResult
	for attempt, order := range permutations {
		ordered := permute(order)
		input := EffectiveCompositionInput{
			CompositionInput: withComposerOverlay(t, composerBaselineInput(t), ordered...),
			CustomSkills:     ordered,
		}
		result, err := ComposeEffective(input)
		if err != nil {
			t.Fatalf("compose effective catalog (order %v): %v", order, err)
		}
		if attempt == 0 {
			reference = result
			continue
		}
		requireEqualComposition(t, result, reference)
	}

	// The layered overlay is canonically ordered by normalized ID, carries
	// the registry-normalized digest, and expands to the skill-root-relative
	// destination.
	wantIDs := []string{"custom-alpha", "custom-beta", "custom-gamma"}
	if len(reference.CustomSkills) != len(wantIDs) {
		t.Fatalf("composed overlay has %d entries, want %d", len(reference.CustomSkills), len(wantIDs))
	}
	if !slices.IsSortedFunc(reference.CustomSkills, func(a, b ComposedCustomSkill) int {
		return strings.Compare(a.ID, b.ID)
	}) {
		t.Errorf("composed overlay is not sorted by ID: %v", reference.CustomSkills)
	}
	for index, entry := range reference.CustomSkills {
		if entry.ID != wantIDs[index] {
			t.Errorf("composed overlay[%d].ID = %q, want %q", index, entry.ID, wantIDs[index])
		}
		if wantPath := "skills/" + entry.ID + "/SKILL.md"; entry.Path != wantPath {
			t.Errorf("composed overlay[%d].Path = %q, want %q", index, entry.Path, wantPath)
		}
		if entry.ContentSHA256 != skills[index].ContentSHA256 {
			t.Errorf("composed overlay[%d].ContentSHA256 = %q, want the registry-normalized digest %q", index, entry.ContentSHA256, skills[index].ContentSHA256)
		}
	}
	for _, id := range wantIDs {
		found := false
		for _, entry := range reference.TokenReport.Entries {
			if string(entry.AssetID) == "asset/skill/"+id {
				found = true
				if entry.Class != ir.AssetSkill {
					t.Errorf("token report entry %q class = %q, want %q", entry.AssetID, entry.Class, ir.AssetSkill)
				}
			}
		}
		if !found {
			t.Errorf("token report is missing the composed overlay skill %q", id)
		}
	}

	// The baseline path is equally order-independent: shuffling the catalog
	// assets of an overlay-free input must not change the composition.
	t.Run("BaselineCatalogOrderIrrelevant", func(t *testing.T) {
		base := composerBaselineInput(t)
		forward, err := Compose(base)
		if err != nil {
			t.Fatalf("compose baseline: %v", err)
		}
		shuffled := base
		shuffled.Catalog.Assets = slices.Clone(base.Catalog.Assets)
		slices.Reverse(shuffled.Catalog.Assets)
		backward, err := Compose(shuffled)
		if err != nil {
			t.Fatalf("compose shuffled baseline: %v", err)
		}
		requireEqualComposition(t, backward, forward)
	})
}

// TestComposer_BoundedOverlay verifies REQ-COMPAT-001 (SC-COMPAT-E): the
// overlay's effect on the composition is limited to the declared skills
// only, and a declared overlay entry that is not represented in the
// effective catalog fails closed instead of silently composing the
// embedded-only (global-only) catalog.
func TestComposer_BoundedOverlay(t *testing.T) {
	alpha := composerCustomSkill(t, "custom-alpha", "# custom-alpha\n\nAlpha overlay body.\n")
	beta := composerCustomSkill(t, "custom-beta", "# custom-beta\n\nBeta overlay body.\n")

	baseline, err := ComposeEffective(EffectiveCompositionInput{CompositionInput: composerBaselineInput(t)})
	if err != nil {
		t.Fatalf("compose baseline without overlay: %v", err)
	}
	if len(baseline.CustomSkills) != 0 {
		t.Fatalf("baseline composition carries %d overlay entries, want 0", len(baseline.CustomSkills))
	}

	t.Run("OverlayLimitedToDeclaredSkills", func(t *testing.T) {
		input := withComposerOverlay(t, composerBaselineInput(t), alpha, beta)
		// Declare the overlay in reverse order to prove the layer is
		// canonically ordered, not declaration-ordered.
		effective, err := ComposeEffective(EffectiveCompositionInput{
			CompositionInput: input,
			CustomSkills:     []registry.Skill{beta, alpha},
		})
		if err != nil {
			t.Fatalf("compose effective catalog with declared overlay: %v", err)
		}

		// The overlay layer contains exactly the two declared skills.
		want := []ComposedCustomSkill{
			{ID: "custom-alpha", ContentSHA256: alpha.ContentSHA256, Path: "skills/custom-alpha/SKILL.md"},
			{ID: "custom-beta", ContentSHA256: beta.ContentSHA256, Path: "skills/custom-beta/SKILL.md"},
		}
		if !reflect.DeepEqual(effective.CustomSkills, want) {
			t.Errorf("composed overlay = %+v, want %+v", effective.CustomSkills, want)
		}

		// Every non-overlay field is unchanged from the baseline composition.
		if effective.RootIndex != baseline.RootIndex {
			t.Errorf("RootIndex changed under overlay: %q != %q", effective.RootIndex, baseline.RootIndex)
		}
		if !reflect.DeepEqual(effective.Modules, baseline.Modules) {
			t.Errorf("Modules changed under overlay: %v != %v", effective.Modules, baseline.Modules)
		}
		if !reflect.DeepEqual(effective.SkillBindings, baseline.SkillBindings) {
			t.Errorf("SkillBindings changed under overlay: %v != %v", effective.SkillBindings, baseline.SkillBindings)
		}
		if effective.SharedContract != baseline.SharedContract {
			t.Errorf("SharedContract changed under overlay: %q != %q", effective.SharedContract, baseline.SharedContract)
		}
		if effective.ProfileOverlay != baseline.ProfileOverlay {
			t.Errorf("ProfileOverlay changed under overlay: %q != %q", effective.ProfileOverlay, baseline.ProfileOverlay)
		}
		if effective.QualityTemplate != baseline.QualityTemplate {
			t.Errorf("QualityTemplate changed under overlay: %q != %q", effective.QualityTemplate, baseline.QualityTemplate)
		}

		// The token report gains exactly the declared skills' entries, in the
		// canonical order; nothing else moves.
		expected := append(slices.Clone(baseline.TokenReport.Entries),
			TokenReportEntry{AssetID: "asset/skill/custom-alpha", Class: ir.AssetSkill},
			TokenReportEntry{AssetID: "asset/skill/custom-beta", Class: ir.AssetSkill},
		)
		slices.SortFunc(expected, func(a, b TokenReportEntry) int {
			return strings.Compare(string(a.AssetID), string(b.AssetID))
		})
		if !reflect.DeepEqual(effective.TokenReport.Entries, expected) {
			t.Errorf("token report = %v, want baseline plus exactly the declared overlay entries %v", effective.TokenReport.Entries, expected)
		}
	})

	t.Run("EmptyOverlayEqualsBaselineComposition", func(t *testing.T) {
		base := composerBaselineInput(t)
		viaCompose, err := Compose(base)
		if err != nil {
			t.Fatalf("compose baseline: %v", err)
		}
		viaEffective, err := ComposeEffective(EffectiveCompositionInput{CompositionInput: base})
		if err != nil {
			t.Fatalf("compose effective with empty overlay: %v", err)
		}
		requireEqualComposition(t, viaEffective, viaCompose)
		if viaEffective.CustomSkills != nil {
			t.Errorf("empty overlay produced overlay layer %v, want nil", viaEffective.CustomSkills)
		}
	})

	t.Run("DeclaredSkillAbsentFromCatalogFailsClosed", func(t *testing.T) {
		// The overlay is declared but the effective catalog was never built
		// with its entries: the composer must refuse instead of silently
		// composing the global-only catalog.
		input := composerBaselineInput(t)
		result, err := ComposeEffective(EffectiveCompositionInput{
			CompositionInput: input,
			CustomSkills:     []registry.Skill{alpha},
		})
		if err == nil {
			t.Fatalf("overlay absent from the effective catalog was composed: %+v", result.CustomSkills)
		}
		if !strings.Contains(err.Error(), "custom-alpha") {
			t.Errorf("rejection %q does not identify the missing skill ID", err)
		}
		requireZeroComposition(t, result)
	})

	t.Run("CatalogDigestDisagreementFailsClosed", func(t *testing.T) {
		input := withComposerOverlay(t, composerBaselineInput(t), alpha)
		for index, spec := range input.Catalog.Assets {
			if spec.ID == "asset/skill/custom-alpha" {
				input.Catalog.Assets[index].SHA256 = ir.FingerprintContent([]byte("# different content\n"))
			}
		}
		result, err := ComposeEffective(EffectiveCompositionInput{
			CompositionInput: input,
			CustomSkills:     []registry.Skill{alpha},
		})
		if err == nil {
			t.Fatalf("overlay with a disagreeing catalog digest was composed: %+v", result.CustomSkills)
		}
		if !strings.Contains(err.Error(), "custom-alpha") {
			t.Errorf("rejection %q does not identify the disagreeing skill ID", err)
		}
		requireZeroComposition(t, result)
	})

	t.Run("NonSkillClassCatalogEntryRejected", func(t *testing.T) {
		input := withComposerOverlay(t, composerBaselineInput(t), alpha)
		for index, spec := range input.Catalog.Assets {
			if spec.ID == "asset/skill/custom-alpha" {
				input.Catalog.Assets[index].Class = ir.AssetCommand
			}
		}
		result, err := ComposeEffective(EffectiveCompositionInput{
			CompositionInput: input,
			CustomSkills:     []registry.Skill{alpha},
		})
		if err == nil {
			t.Fatalf("overlay matched against a non-skill catalog entry: %+v", result.CustomSkills)
		}
		requireZeroComposition(t, result)
	})

	t.Run("DuplicateOverlayDeclarationRejected", func(t *testing.T) {
		input := withComposerOverlay(t, composerBaselineInput(t), alpha)
		result, err := ComposeEffective(EffectiveCompositionInput{
			CompositionInput: input,
			CustomSkills:     []registry.Skill{alpha, alpha},
		})
		if err == nil {
			t.Fatalf("duplicate overlay declaration was composed: %+v", result.CustomSkills)
		}
		if !strings.Contains(err.Error(), "custom-alpha") {
			t.Errorf("rejection %q does not identify the duplicated skill ID", err)
		}
		requireZeroComposition(t, result)
	})
}
