package prompt

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/quality"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/registry"
	"github.com/lleontor705/cortex-ia/internal/model"
)

// TokenReportEntry records the token estimate for one asset in the composition.
type TokenReportEntry struct {
	AssetID ir.SemanticID `json:"asset_id"`
	Class   ir.AssetClass `json:"class"`
	Tokens  int           `json:"tokens"`
	Limit   int           `json:"limit"`
}

// TokenReport aggregates per-asset token estimates for the composed bundle.
// Budget violations (Tokens > Limit) are returned as errors from Compose.
type TokenReport struct {
	Entries []TokenReportEntry `json:"entries"`
}

// CompositionResult is the complete adapter-valid output of prompt composition.
// It contains the fully expanded paths for every composed asset and the
// SkillBindings for all workflow roles. Renderers consume this to lower into
// adapter-specific destinations.
type CompositionResult struct {
	RootIndex         string                `json:"root_index"`
	Modules           []string              `json:"modules"`
	SkillBindings     []SkillBinding        `json:"skill_bindings"`
	SharedContract    string                `json:"shared_contract"`
	ProfileOverlay    string                `json:"profile_overlay"`
	QualityTemplate   string                `json:"quality_template"`
	QualityPlan       quality.QualityPlan   `json:"quality_plan"`
	Adapter           AdapterPromptContract `json:"-"`
	TokenReport       TokenReport           `json:"token_report"`
	OperationalAssets []MaterializedAsset   `json:"operational_assets,omitempty"`
	// CustomSkills is the composed custom skill overlay layer: exactly the
	// declared, catalog-represented additions in canonical ID order. It is
	// nil for baseline compositions without an overlay.
	CustomSkills []ComposedCustomSkill `json:"custom_skills,omitempty"`
	Metadata     json.RawMessage       `json:"metadata,omitempty"`
}

// ComposedCustomSkill is the composed projection of one registry-verified
// custom skill layered onto the embedded baseline: canonical identity, the
// registry-normalized content digest, and the expanded skill-root-relative
// destination. It deliberately carries no authority fields: custom skills
// are data assets only and can never grant agents, tools, permissions, or
// bindings.
type ComposedCustomSkill struct {
	ID            string `json:"id"`
	ContentSHA256 string `json:"content_sha256"`
	Path          string `json:"path"`
}

// EffectiveCompositionInput is the complete input to effective composition:
// the global composition input carrying the effective asset catalog (embedded
// baseline plus accepted custom additions), together with the
// registry-normalized custom skill overlay. The overlay may arrive in any
// declaration order; layering is keyed by the normalized identities, so
// equivalent inputs always compose identically.
type EffectiveCompositionInput struct {
	CompositionInput
	// CustomSkills carries the registry-normalized, provenance-verified
	// custom skill declarations. A nil or empty overlay composes the
	// byte-for-byte baseline.
	CustomSkills []registry.Skill
}

// Compose assembles the root index, selected modules, role stubs (via
// SkillBindings), shared contract, profile overlay, and QualityPlanTemplate
// into an adapter-valid composition result. It is the single owner of prompt
// layering: renderers lower only destinations and qualified native syntax.
//
// The function:
//  1. Validates the composition input (adapter contract + asset catalog).
//  2. Produces exactly one SkillBinding per canonical role.
//  3. Resolves the root index, modules, shared contract, profile overlay, and
//     quality template paths from the typed asset catalog.
//  4. Builds a token report for all budgeted assets.
//
// Any missing required asset, invalid binding, or budget violation returns an
// error so that no incomplete or over-limit composition can reach installation.
func Compose(input CompositionInput) (CompositionResult, error) {
	if err := input.Validate(); err != nil {
		return CompositionResult{}, fmt.Errorf("compose: %w", err)
	}

	contract := input.Adapter
	catalog := canonicalAssetOrder(input.Catalog)

	// Produce exactly one SkillBinding per workflow role.
	bindings := make([]SkillBinding, 0, len(input.Workflow.Roles))
	for _, role := range input.Workflow.Roles {
		binding, err := NewSkillBinding(role.ID, contract)
		if err != nil {
			return CompositionResult{}, fmt.Errorf("compose skill binding for %q: %w", role.ID, err)
		}
		bindings = append(bindings, binding)
	}

	// Resolve asset paths from the catalog by class.
	rootIndex, err := resolveSingleAsset(catalog, ir.AssetRootIndex, contract)
	if err != nil {
		return CompositionResult{}, fmt.Errorf("compose root index: %w", err)
	}
	sharedContract, err := resolveSingleAsset(catalog, ir.AssetSharedContract, contract)
	if err != nil {
		return CompositionResult{}, fmt.Errorf("compose shared contract: %w", err)
	}
	qualityTemplate, err := resolveSingleAsset(catalog, ir.AssetQualityTemplate, contract)
	if err != nil {
		return CompositionResult{}, fmt.Errorf("compose quality template: %w", err)
	}
	modules, err := resolveAssetPaths(catalog, ir.AssetRootModule, contract)
	if err != nil {
		return CompositionResult{}, fmt.Errorf("compose modules: %w", err)
	}

	// Resolve profile overlay — must match the active profile ID.
	profileOverlay, err := resolveProfileOverlay(catalog, input.Profile.ProfileID, contract)
	if err != nil {
		return CompositionResult{}, fmt.Errorf("compose profile overlay: %w", err)
	}

	// Build token report from all catalog assets.
	report := buildTokenReport(catalog)

	result := CompositionResult{
		RootIndex:       rootIndex,
		Modules:         modules,
		SkillBindings:   bindings,
		SharedContract:  sharedContract,
		ProfileOverlay:  profileOverlay,
		QualityTemplate: qualityTemplate,
		QualityPlan:     input.QualityPlan,
		Adapter:         contract,
		TokenReport:     report,
		Metadata:        slices.Clone(input.Metadata),
	}
	return result, nil
}

// ComposeEffective composes the selected effective catalog: the embedded
// baseline layered with the declared custom skill overlay. The base
// composition is identical to the baseline composition; the overlay adds
// exactly one composed entry per declared skill and nothing else.
//
// Layering is fail-closed: every declared overlay skill must be represented
// exactly once in the effective catalog as a skill-class asset carrying the
// registry-normalized digest. A declared skill that is absent, mis-classed,
// digest-disagreeing, or declared twice is a composition error — the
// composer never silently falls back to the embedded-only (global-only)
// catalog when an overlay is present.
func ComposeEffective(input EffectiveCompositionInput) (CompositionResult, error) {
	result, err := Compose(input.CompositionInput)
	if err != nil {
		return CompositionResult{}, err
	}
	layer, err := composeCustomSkills(input.CustomSkills, input.Catalog, input.Adapter)
	if err != nil {
		return CompositionResult{}, fmt.Errorf("compose effective selection: %w", err)
	}
	result.CustomSkills = layer
	return result, nil
}

// canonicalAssetOrder returns the catalog with its assets in canonical
// semantic-ID order so every derived composition list (modules, token report
// entries) is identical for equivalent inputs regardless of the order the
// catalog was assembled in. The caller's slice is never mutated.
func canonicalAssetOrder(catalog ir.AssetCatalog) ir.AssetCatalog {
	ordered := catalog
	ordered.Assets = slices.Clone(catalog.Assets)
	slices.SortFunc(ordered.Assets, func(left, right ir.AssetSpec) int {
		return strings.Compare(string(left.ID), string(right.ID))
	})
	return ordered
}

// composeCustomSkills layers the declared overlay onto the composed result:
// each declared skill must be represented exactly once in the effective
// catalog, and the composed layer is ordered by normalized ID so declaration
// order never leaks into the composition.
func composeCustomSkills(overlay []registry.Skill, catalog ir.AssetCatalog, contract AdapterPromptContract) ([]ComposedCustomSkill, error) {
	if len(overlay) == 0 {
		return nil, nil
	}
	seen := make(map[model.SkillID]struct{}, len(overlay))
	layer := make([]ComposedCustomSkill, 0, len(overlay))
	for index, skill := range overlay {
		if _, duplicate := seen[skill.ID]; duplicate {
			return nil, fmt.Errorf("custom skill %q: duplicate overlay declaration", skill.ID)
		}
		seen[skill.ID] = struct{}{}
		if err := requireOverlayCatalogEntry(skill, catalog); err != nil {
			return nil, fmt.Errorf("custom skill %q (declaration %d): %w", skill.ID, index, err)
		}
		skillPath, err := contract.ExpandPath(contract.SkillRoot, string(skill.ID)+"/SKILL.md")
		if err != nil {
			return nil, fmt.Errorf("custom skill %q: expand skill path: %w", skill.ID, err)
		}
		layer = append(layer, ComposedCustomSkill{
			ID:            string(skill.ID),
			ContentSHA256: skill.ContentSHA256,
			Path:          skillPath,
		})
	}
	slices.SortFunc(layer, func(left, right ComposedCustomSkill) int {
		return strings.Compare(left.ID, right.ID)
	})
	return layer, nil
}

// requireOverlayCatalogEntry verifies one declared overlay skill is
// representable by the effective catalog: present exactly once, as a
// skill-class asset, with the registry-normalized digest. This is the
// fail-closed gate that prevents an implicit global-only composition when an
// overlay is present.
func requireOverlayCatalogEntry(skill registry.Skill, catalog ir.AssetCatalog) error {
	wantID := ir.SemanticID("asset/skill/" + string(skill.ID))
	matches := 0
	for _, spec := range catalog.Assets {
		if spec.ID != wantID {
			continue
		}
		matches++
		if spec.Class != ir.AssetSkill {
			return fmt.Errorf("effective catalog entry %q has class %s, want the skill asset class", spec.ID, spec.Class)
		}
		if spec.SHA256 != skill.ContentSHA256 {
			return fmt.Errorf("effective catalog digest for %q disagrees with the registry-normalized declaration digest", spec.ID)
		}
	}
	switch matches {
	case 1:
		return nil
	case 0:
		return fmt.Errorf("absent from the effective catalog; build the effective catalog from the same accepted declarations instead of composing the embedded-only catalog")
	default:
		return fmt.Errorf("appears %d times in the effective catalog, want exactly once", matches)
	}
}

// resolveSingleAsset finds the single asset of the given class and returns its
// expanded installed path. An error is returned if the class is missing or
// appears more than once.
func resolveSingleAsset(catalog ir.AssetCatalog, class ir.AssetClass, contract AdapterPromptContract) (string, error) {
	var found *ir.AssetSpec
	for i := range catalog.Assets {
		if catalog.Assets[i].Class == class {
			if found != nil {
				return "", fmt.Errorf("multiple %s assets in catalog", class)
			}
			found = &catalog.Assets[i]
		}
	}
	if found == nil {
		return "", fmt.Errorf("no %s asset in catalog", class)
	}
	return contract.ExpandPath(contract.SkillRoot, found.SourcePath)
}

// resolveAssetPaths finds all assets of the given class and returns their
// expanded installed paths in stable order.
func resolveAssetPaths(catalog ir.AssetCatalog, class ir.AssetClass, contract AdapterPromptContract) ([]string, error) {
	var paths []string
	for _, asset := range catalog.Assets {
		if asset.Class == class {
			p, err := contract.ExpandPath(contract.SkillRoot, asset.SourcePath)
			if err != nil {
				return nil, fmt.Errorf("asset %q: %w", asset.ID, err)
			}
			paths = append(paths, p)
		}
	}
	return paths, nil
}

// resolveProfileOverlay finds the profile overlay asset matching the active
// profile ID and returns its expanded installed path.
func resolveProfileOverlay(catalog ir.AssetCatalog, profileID string, contract AdapterPromptContract) (string, error) {
	for _, asset := range catalog.Assets {
		if asset.Class != ir.AssetProfileOverlay {
			continue
		}
		if slices.Contains(asset.Profiles, ir.SemanticID(profileID)) || profileMatchesProfiles(profileID, asset.Profiles) {
			return contract.ExpandPath(contract.SkillRoot, asset.SourcePath)
		}
	}
	return "", fmt.Errorf("no profile overlay matching profile %q in catalog", profileID)
}

// profileMatchesProfiles is a fallback for overlays that store profile IDs
// without a SemanticID prefix (e.g. "portable-sequential" vs "profile/portable-sequential").
func profileMatchesProfiles(profileID string, profiles []ir.SemanticID) bool {
	short := profileID
	if idx := strings.LastIndex(profileID, "/"); idx >= 0 {
		short = profileID[idx+1:]
	}
	for _, p := range profiles {
		ps := string(p)
		if ps == profileID || ps == short || strings.HasSuffix(ps, "/"+short) {
			return true
		}
	}
	return false
}

// buildTokenReport creates a token report entry for every asset in the catalog.
// The report records the estimated token count for content-backed assets and
// the class ceiling for budgeted assets.
func buildTokenReport(catalog ir.AssetCatalog) TokenReport {
	entries := make([]TokenReportEntry, 0, len(catalog.Assets))
	for _, asset := range catalog.Assets {
		limit := budgetCeilingForClass(asset.Class)
		entries = append(entries, TokenReportEntry{
			AssetID: asset.ID,
			Class:   asset.Class,
			Tokens:  0, // content-backed; actual estimation happens when content is loaded
			Limit:   limit,
		})
	}
	return TokenReport{Entries: entries}
}
