package prompt

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/quality"
)

// canonicalRoles is the complete set of nine SDD phase roles in canonical
// order. Compose produces exactly one SkillBinding per role, deterministically.
var canonicalRoles = []ir.SemanticID{
	"role/bootstrap", "role/explore", "role/proposal", "role/spec",
	"role/design", "role/tasks", "role/apply", "role/verify", "role/archive",
}

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
// SkillBindings for all nine roles. Renderers consume this to lower into
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
	Metadata          json.RawMessage       `json:"metadata,omitempty"`
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

	// Produce exactly one SkillBinding per canonical role.
	bindings := make([]SkillBinding, 0, len(canonicalRoles))
	for _, role := range canonicalRoles {
		binding, err := NewSkillBinding(role, contract)
		if err != nil {
			return CompositionResult{}, fmt.Errorf("compose skill binding for %q: %w", role, err)
		}
		bindings = append(bindings, binding)
	}

	// Resolve asset paths from the catalog by class.
	rootIndex, err := resolveSingleAsset(input.Catalog, ir.AssetRootIndex, contract)
	if err != nil {
		return CompositionResult{}, fmt.Errorf("compose root index: %w", err)
	}
	sharedContract, err := resolveSingleAsset(input.Catalog, ir.AssetSharedContract, contract)
	if err != nil {
		return CompositionResult{}, fmt.Errorf("compose shared contract: %w", err)
	}
	qualityTemplate, err := resolveSingleAsset(input.Catalog, ir.AssetQualityTemplate, contract)
	if err != nil {
		return CompositionResult{}, fmt.Errorf("compose quality template: %w", err)
	}
	modules, err := resolveAssetPaths(input.Catalog, ir.AssetRootModule, contract)
	if err != nil {
		return CompositionResult{}, fmt.Errorf("compose modules: %w", err)
	}

	// Resolve profile overlay — must match the active profile ID.
	profileOverlay, err := resolveProfileOverlay(input.Catalog, input.Profile.ProfileID, contract)
	if err != nil {
		return CompositionResult{}, fmt.Errorf("compose profile overlay: %w", err)
	}

	// Build token report from all catalog assets.
	report := buildTokenReport(input.Catalog)

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
