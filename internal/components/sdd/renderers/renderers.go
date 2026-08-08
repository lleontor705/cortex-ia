// Package renderers defines the runtime-renderer boundary and validates its
// output before a bundle can reach persistent installation code.
package renderers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"slices"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/manifest"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/quality"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/resolution"
	"github.com/lleontor705/cortex-ia/internal/modelroute"
)

// TargetID identifies one runtime adapter without coupling the domain to its
// concrete implementation.
type TargetID string

// AssetKind identifies a syntax or managed asset type qualified by a target's
// resolved capability profile.
type AssetKind string

const (
	AssetInstruction AssetKind = "instruction"
	AssetRule        AssetKind = "rule"
	AssetSkill       AssetKind = "skill"
	AssetCommand     AssetKind = "command"
	AssetAgent       AssetKind = "agent"
	AssetPermission  AssetKind = "permission"
	AssetHook        AssetKind = "hook"
	AssetMCP         AssetKind = "mcp"
	AssetModel       AssetKind = "model"
	AssetSchema      AssetKind = "schema"
	AssetFixture     AssetKind = "fixture"
)

// ExtensionDeclaration is a manifest-visible target extension. ID must be
// namespaced by the resolved target, for example "claude/hooks".
type ExtensionDeclaration struct {
	ID       ir.SemanticID `json:"id"`
	Optional bool          `json:"optional"`
}

// ResolvedWorkflow is the complete immutable input to a target renderer.
// AllowedAssetKinds and AllowedPermissions are authoritative output bounds.
type ResolvedWorkflow struct {
	Workflow              ir.WorkflowIR           `json:"workflow"`
	Target                TargetID                `json:"target"`
	Profile               string                  `json:"profile"`
	GenerationFingerprint string                  `json:"generation_fingerprint"`
	Capabilities          []resolution.Resolution `json:"capabilities,omitempty"`
	AllowedAssetKinds     []AssetKind             `json:"allowed_asset_kinds"`
	AllowedPermissions    []string                `json:"allowed_permissions"`
	Extensions            []ExtensionDeclaration  `json:"extensions,omitempty"`
	QualificationEvidence []manifest.Evidence     `json:"qualification_evidence,omitempty"`
	Metadata              json.RawMessage         `json:"metadata,omitempty"`
	// Composition carries the prompt-layer composition result that renderers
	// lower into adapter-specific assets. It contains the root index path,
	// module paths, shared contract path, profile overlay path, quality
	// template path, and SkillBindings for all workflow roles.
	Composition Composition `json:"composition,omitempty"`
	// NativeSkillPreload controls whether the adapter can preload skills
	// (native-preload) or must read them as a mandatory first action
	// (fallback-read). Determined by adapter qualification evidence.
	NativeSkillPreload bool `json:"native_skill_preload,omitempty"`
	// NativeSkillOnDemand controls whether the adapter discovers installed
	// skills and invokes them through a native skill tool when required.
	NativeSkillOnDemand bool `json:"native_skill_on_demand,omitempty"`
	// NativeModelField controls whether the adapter supports a native model
	// field in its agent/role configuration.
	NativeModelField bool `json:"native_model_field,omitempty"`
	// NativeWorktreeIsolation controls whether the adapter supports
	// runtime-enforced worktree isolation for direct child tasks.
	NativeWorktreeIsolation bool `json:"native_worktree_isolation,omitempty"`
}

// SkillLoadMode returns native-preload only when the adapter has qualified
// native preload; otherwise the first action must read the installed skill
// (fallback-read). This implements REQ-INST-003's preload-vs-fallback rule at
// the renderer boundary.
func (r ResolvedWorkflow) SkillLoadMode() SkillLoadMode {
	if r.NativeSkillOnDemand {
		return SkillModeNativeOnDemand
	}
	if r.NativeSkillPreload {
		return SkillModeNativePreload
	}
	return SkillModeFallbackRead
}

// Renderer lowers resolved portable semantics into target-specific assets.
// Implementations must return an in-memory bundle and perform no mutation.
type Renderer interface {
	Target() TargetID
	Render(context.Context, ResolvedWorkflow) (Bundle, error)
}

// Asset is one deterministic managed file emitted by a renderer.
type Asset struct {
	Path        string          `json:"path"`
	SemanticID  ir.SemanticID   `json:"semantic_id"`
	Kind        AssetKind       `json:"kind"`
	Content     []byte          `json:"content"`
	Mode        fs.FileMode     `json:"mode"`
	Permissions []string        `json:"permissions,omitempty"`
	Extensions  []ir.SemanticID `json:"extensions,omitempty"`
	Metadata    json.RawMessage `json:"metadata,omitempty"`
}

// Bundle is ordered by relative path and semantic ID after validation.
type Bundle struct {
	Assets   []Asset         `json:"assets"`
	Metadata json.RawMessage `json:"metadata,omitempty"`
}

// SkillLoadMode records whether a skill is loaded via qualified native preload
// or by a mandatory first-action fallback read. This mirrors the prompt
// package's SkillLoadMode but is defined here to avoid a circular import.
type SkillLoadMode string

const (
	SkillModeNativePreload  SkillLoadMode = "native-preload"
	SkillModeNativeOnDemand SkillLoadMode = "native-on-demand"
	SkillModeFallbackRead   SkillLoadMode = "fallback-read"
)

// SkillBinding is the renderer-visible binding of exactly one role to one
// canonical skill with its load mode, installed path, and content hash. Every
// renderer must emit one SkillBinding per role so that adapters can preload
// or mandatory-read the correct skill file (REQ-INST-003).
type SkillBinding struct {
	Role        ir.SemanticID `json:"role"`
	Skill       ir.SemanticID `json:"skill"`
	Mode        SkillLoadMode `json:"mode"`
	Path        string        `json:"path"`
	Hash        string        `json:"hash"`
	FirstAction string        `json:"first_action,omitempty"`
}

// Composition is the renderer's view of the prompt composition result. It
// carries the fully expanded paths for every composed asset (root index,
// modules, shared contract, profile overlay, quality template) plus the
// SkillBindings for all workflow roles. Renderers lower this into adapter-specific
// destinations. It is defined in the renderers package (not the prompt package)
// to avoid a circular import: prompt already depends on renderers.TargetID.
type Composition struct {
	RootIndex         string              `json:"root_index"`
	Modules           []string            `json:"modules,omitempty"`
	SkillBindings     []SkillBinding      `json:"skill_bindings"`
	SharedContract    string              `json:"shared_contract"`
	ProfileOverlay    string              `json:"profile_overlay"`
	QualityTemplate   string              `json:"quality_template"`
	QualityPlan       quality.QualityPlan `json:"quality_plan"`
	ModelRoutes       []ModelRoute        `json:"model_routes,omitempty"`
	OperationalAssets []CompositionAsset  `json:"operational_assets,omitempty"`
	Metadata          json.RawMessage     `json:"metadata,omitempty"`
}

type CompositionAsset struct {
	ID          ir.SemanticID            `json:"id"`
	Class       ir.AssetClass            `json:"class"`
	Path        string                   `json:"path"`
	Content     []byte                   `json:"content"`
	LoadMode    SkillLoadMode            `json:"load_mode,omitempty"`
	Permissions []string                 `json:"permissions,omitempty"`
	Metadata    json.RawMessage          `json:"metadata,omitempty"`
	Route       modelroute.ResolvedRoute `json:"route,omitempty"`
}

// ModelRoute aliases the canonical provider-neutral route resolution. Renderers
// receive it fully resolved and never reconstruct route evidence.
type ModelRoute = modelroute.ResolvedRoute

// compositionManifest is deliberately adapter-neutral; adapter renderers
// lower this one manifest while preserving references and qualified facts.
type compositionManifestData struct {
	RootIndex       string              `json:"root_index"`
	Modules         []string            `json:"modules"`
	SkillBindings   []SkillBinding      `json:"skill_bindings"`
	SharedContract  string              `json:"shared_contract"`
	ProfileOverlay  string              `json:"profile_overlay"`
	QualityTemplate string              `json:"quality_template"`
	QualityPlan     quality.QualityPlan `json:"quality_plan"`
	ModelRoutes     []ModelRoute        `json:"model_routes,omitempty"`
	NativePreload   bool                `json:"native_skill_preload"`
	NativeOnDemand  bool                `json:"native_skill_on_demand"`
	NativeModel     bool                `json:"native_model_field"`
	Worktree        bool                `json:"native_worktree_isolation"`
	Metadata        json.RawMessage     `json:"metadata,omitempty"`
}

var canonicalCompositionSkills = map[ir.SemanticID]ir.SemanticID{
	"role/orchestrator": "skill/orchestrator",
	"role/bootstrap":    "skill/bootstrap", "role/explore": "skill/investigate", "role/proposal": "skill/draft-proposal",
	"role/spec": "skill/write-specs", "role/design": "skill/architect", "role/tasks": "skill/decompose",
	"role/apply": "skill/implement", "role/verify": "skill/validate", "role/archive": "skill/finalize",
	"role/investigate": "skill/investigate", "role/draft-proposal": "skill/draft-proposal", "role/write-specs": "skill/write-specs",
	"role/architect": "skill/architect", "role/decompose": "skill/decompose", "role/implement": "skill/implement",
	"role/validate": "skill/validate", "role/finalize": "skill/finalize", "role/debate": "skill/debate",
	"role/parallel-dispatch": "skill/parallel-dispatch",
}

func compositionSkillBinding(composition Composition, role ir.SemanticID) (SkillBinding, bool) {
	aliases := map[ir.SemanticID]ir.SemanticID{
		"role/investigate": "role/explore", "role/draft-proposal": "role/proposal", "role/write-specs": "role/spec",
		"role/architect": "role/design", "role/decompose": "role/tasks", "role/implement": "role/apply",
		"role/validate": "role/verify", "role/finalize": "role/archive",
	}
	for _, binding := range composition.SkillBindings {
		if binding.Role == role || binding.Role == aliases[role] {
			return binding, true
		}
	}
	return SkillBinding{}, false
}

func appendCompositionAsset(resolved ResolvedWorkflow, assets []Asset) ([]Asset, error) {
	assetMap, err := AdapterAssetMapFor(resolved.Target)
	if err != nil {
		return nil, err
	}
	if err := assetMap.ValidateProfile(resolved.Profile); err != nil {
		return nil, err
	}
	composition := resolved.Composition
	if composition.RootIndex == "" && len(composition.Modules) == 0 && len(composition.SkillBindings) == 0 && composition.SharedContract == "" && composition.ProfileOverlay == "" && composition.QualityTemplate == "" && !hasQualityPlan(composition.QualityPlan) {
		return assets, nil
	}
	for name, value := range map[string]string{"root_index": composition.RootIndex, "shared_contract": composition.SharedContract, "profile_overlay": composition.ProfileOverlay, "quality_template": composition.QualityTemplate} {
		if strings.TrimSpace(value) == "" || strings.Contains(value, "..") {
			return nil, fmt.Errorf("composition %s is missing or unsafe", name)
		}
	}
	modules := sortedUnique(composition.Modules)
	if len(modules) != len(composition.Modules) {
		return nil, fmt.Errorf("composition modules must be unique and ordered")
	}
	byRole := make(map[ir.SemanticID]SkillBinding, len(composition.SkillBindings))
	for _, binding := range composition.SkillBindings {
		if _, exists := byRole[binding.Role]; exists {
			return nil, fmt.Errorf("composition role %q has multiple skill bindings", binding.Role)
		}
		want, ok := canonicalCompositionSkills[binding.Role]
		if !ok || binding.Skill != want || strings.TrimSpace(binding.Path) == "" || strings.TrimSpace(binding.Hash) == "" {
			return nil, fmt.Errorf("composition role %q has invalid canonical skill binding", binding.Role)
		}
		if binding.Mode != resolved.SkillLoadMode() {
			return nil, fmt.Errorf("composition role %q uses %q, want qualified %q", binding.Role, binding.Mode, resolved.SkillLoadMode())
		}
		byRole[binding.Role] = binding
	}
	for _, role := range resolved.Workflow.Roles {
		lookup := role.ID
		if _, ok := byRole[lookup]; !ok {
			lookup = map[ir.SemanticID]ir.SemanticID{
				"role/investigate": "role/explore", "role/draft-proposal": "role/proposal", "role/write-specs": "role/spec",
				"role/architect": "role/design", "role/decompose": "role/tasks", "role/implement": "role/apply",
				"role/validate": "role/verify", "role/finalize": "role/archive",
			}[role.ID]
		}
		if _, ok := byRole[lookup]; !ok {
			return nil, fmt.Errorf("composition is missing skill binding for role %q", role.ID)
		}
	}
	bindings := slices.Clone(composition.SkillBindings)
	slices.SortFunc(bindings, func(left, right SkillBinding) int { return strings.Compare(string(left.Role), string(right.Role)) })
	for index := range bindings {
		switch bindings[index].Mode {
		case SkillModeNativeOnDemand:
			bindings[index].FirstAction = "skill:" + string(bindings[index].Skill)
		case SkillModeNativePreload:
			bindings[index].FirstAction = "preload:" + string(bindings[index].Skill)
		default:
			bindings[index].FirstAction = "read:" + bindings[index].Path
		}
	}
	routes := slices.Clone(composition.ModelRoutes)
	slices.SortFunc(routes, func(left, right ModelRoute) int { return strings.Compare(string(left.Role), string(right.Role)) })
	for _, route := range routes {
		if route.Role == "" || route.PrimaryID == "" || route.Primary.Provider == "" || route.Primary.Model == "" || len(route.Evidence) == 0 || (route.Requested.AllowFallback && route.Fallback == nil) {
			return nil, fmt.Errorf("composition model route for %q is not qualified", route.Role)
		}
		{
			if err := route.Requested.Validate(); err != nil {
				return nil, fmt.Errorf("composition model route for %q: %w", route.Role, err)
			}
		}
	}
	for _, asset := range composition.OperationalAssets {
		if strings.TrimSpace(asset.Path) == "" || len(asset.Content) == 0 || asset.ID == "" {
			return nil, fmt.Errorf("composition operational asset %q is incomplete", asset.ID)
		}
	}
	if hasQualityPlan(composition.QualityPlan) {
		if err := composition.QualityPlan.Validate(); err != nil {
			return nil, fmt.Errorf("composition quality plan: %w", err)
		}
	}
	data, err := json.Marshal(compositionManifestData{RootIndex: composition.RootIndex, Modules: modules, SkillBindings: bindings, SharedContract: composition.SharedContract, ProfileOverlay: composition.ProfileOverlay, QualityTemplate: composition.QualityTemplate, QualityPlan: composition.QualityPlan, ModelRoutes: routes, NativePreload: resolved.NativeSkillPreload, NativeOnDemand: resolved.NativeSkillOnDemand, NativeModel: resolved.NativeModelField, Worktree: resolved.NativeWorktreeIsolation, Metadata: slices.Clone(resolved.Metadata)})
	if err != nil {
		return nil, fmt.Errorf("marshal composition manifest: %w", err)
	}
	kind := AssetFixture
	if slices.Contains(resolved.AllowedAssetKinds, AssetSchema) {
		kind = AssetSchema
	} else if slices.Contains(resolved.AllowedAssetKinds, AssetInstruction) {
		kind = AssetInstruction
	}
	if !slices.Contains(resolved.AllowedAssetKinds, kind) {
		return nil, fmt.Errorf("resolved target %q cannot emit a composition manifest", resolved.Target)
	}
	for _, common := range composition.OperationalAssets {
		assetKind := AssetInstruction
		switch common.Class {
		case ir.AssetSkill:
			assetKind = AssetSkill
		case ir.AssetCommand:
			assetKind = AssetCommand
		case ir.AssetRoleStub:
			assetKind = AssetAgent
		case ir.AssetManifest:
			assetKind = AssetSchema
		case ir.AssetProfileOverlay:
			assetKind = AssetRule
		}
		if !slices.Contains(resolved.AllowedAssetKinds, assetKind) {
			continue
		}
		assets = append(assets, Asset{Path: common.Path, SemanticID: common.ID, Kind: assetKind, Content: bytes.Clone(common.Content), Mode: 0o644, Permissions: slices.Clone(common.Permissions), Metadata: slices.Clone(common.Metadata)})
	}
	assets = append(assets, Asset{Path: ".cortex-ia/composition.json", SemanticID: ir.SemanticID(string(resolved.Target) + "/composition"), Kind: kind, Content: append(data, '\n'), Mode: 0o644})
	// Composition is the sole semantic-to-destination lowering boundary. The
	// target renderers only emit adapter syntax; common assets are mapped once.
	for index := range assets {
		asset := &assets[index]
		if asset.SemanticID == ir.SemanticID(string(resolved.Target)+"/composition") {
			continue
		}
		common := false
		for _, candidate := range composition.OperationalAssets {
			if candidate.ID == asset.SemanticID {
				common = true
				break
			}
		}
		if !common {
			continue
		}
		mapped, mapErr := assetMap.Map(asset.SemanticID, compositionAssetClass(asset.Kind), asset.Path)
		if mapErr != nil {
			return nil, mapErr
		}
		asset.Path = mapped.Relative
	}
	return assets, nil
}

func hasQualityPlan(plan quality.QualityPlan) bool {
	return plan.PolicySHA256 != "" || plan.TemplateSHA256 != "" || plan.ChangeSignalsSHA256 != ""
}

func compositionAssetClass(kind AssetKind) ir.AssetClass {
	switch kind {
	case AssetSkill:
		return ir.AssetSkill
	case AssetCommand:
		return ir.AssetCommand
	case AssetAgent:
		return ir.AssetRoleStub
	case AssetRule:
		return ir.AssetProfileOverlay
	case AssetSchema:
		return ir.AssetManifest
	default:
		return ir.AssetRootModule
	}
}

const (
	ErrorInvalidResolvedWorkflow ir.SemanticID = "renderer/invalid-resolved-workflow"
	ErrorRendererTargetMismatch  ir.SemanticID = "renderer/target-mismatch"
	ErrorInvalidExtension        ir.SemanticID = "renderer/invalid-extension"
	ErrorInvalidAsset            ir.SemanticID = "renderer/invalid-asset"
	ErrorDuplicateAsset          ir.SemanticID = "renderer/duplicate-asset"
	ErrorUnsupportedAsset        ir.SemanticID = "renderer/unsupported-asset"
	ErrorUndeclaredExtension     ir.SemanticID = "renderer/undeclared-extension"
	ErrorUnresolvedVariable      ir.SemanticID = "renderer/unresolved-variable"
	ErrorPermissionWidening      ir.SemanticID = "renderer/permission-widening"
)

// ValidationError provides stable semantic identifiers for diagnostics and
// links a renderer failure to the source semantic asset whenever possible.
type ValidationError struct {
	ID         ir.SemanticID
	SemanticID ir.SemanticID
	Path       string
	Observed   string
	Expected   string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("renderer validation %s for %s at %s: observed %s; expected %s", e.ID, e.SemanticID, e.Path, e.Observed, e.Expected)
}

var unresolvedValuePattern = regexp.MustCompile(`\{\{[^{}]+\}\}|\$\{[^{}]+\}`)

// Render invokes a renderer only for its declared target and always validates
// and normalizes the returned bundle before exposing it to callers.
func Render(ctx context.Context, renderer Renderer, resolved ResolvedWorkflow) (Bundle, error) {
	if renderer == nil {
		return Bundle{}, validationError(ErrorInvalidResolvedWorkflow, "workflow/resolved", "$.renderer", "<nil>", "a renderer for the resolved target")
	}
	if renderer.Target() != resolved.Target {
		return Bundle{}, validationError(ErrorRendererTargetMismatch, "workflow/resolved", "$.target", string(renderer.Target()), string(resolved.Target))
	}
	bundle, err := renderer.Render(ctx, resolved)
	if err != nil {
		return Bundle{}, err
	}
	bundle.Metadata = slices.Clone(resolved.Metadata)
	return ValidateBundle(resolved, bundle)
}

// ValidateBundle enforces resolved target syntax, declared extensions,
// permission bounds, and stable asset representation. The result owns all
// slices and bytes and is independent of renderer input mutation.
func ValidateBundle(resolved ResolvedWorkflow, bundle Bundle) (Bundle, error) {
	allowedKinds, allowedPermissions, extensions, err := validateResolvedWorkflow(resolved)
	if err != nil {
		return Bundle{}, err
	}

	assets := make([]Asset, len(bundle.Assets))
	paths := make(map[string]ir.SemanticID, len(bundle.Assets))
	semanticIDs := make(map[ir.SemanticID]struct{}, len(bundle.Assets))
	for index, input := range bundle.Assets {
		assetPath := fmt.Sprintf("$.assets[%d]", index)
		if err := validateAssetIdentity(input, assetPath); err != nil {
			return Bundle{}, err
		}
		if previous, duplicate := paths[input.Path]; duplicate {
			return Bundle{}, validationError(ErrorDuplicateAsset, input.SemanticID, assetPath+".path", input.Path, "a unique relative path; already emitted by "+string(previous))
		}
		if _, duplicate := semanticIDs[input.SemanticID]; duplicate {
			return Bundle{}, validationError(ErrorDuplicateAsset, input.SemanticID, assetPath+".semantic_id", string(input.SemanticID), "a unique asset semantic ID")
		}
		paths[input.Path] = input.SemanticID
		semanticIDs[input.SemanticID] = struct{}{}
		if _, ok := allowedKinds[input.Kind]; !ok {
			return Bundle{}, validationError(ErrorUnsupportedAsset, input.SemanticID, assetPath+".kind", string(input.Kind), "an asset kind supported by the resolved target profile")
		}
		if match := unresolvedValuePattern.Find(input.Content); match != nil {
			return Bundle{}, validationError(ErrorUnresolvedVariable, input.SemanticID, assetPath+".content", string(match), "fully resolved target content")
		}

		permissions := sortedUnique(input.Permissions)
		for _, permission := range permissions {
			if _, ok := allowedPermissions[permission]; !ok {
				return Bundle{}, validationError(ErrorPermissionWidening, input.SemanticID, assetPath+".permissions", permission, "a permission present in the canonical resolved scope")
			}
		}
		assetExtensions := sortedUnique(input.Extensions)
		for _, extension := range assetExtensions {
			if _, ok := extensions[extension]; !ok {
				return Bundle{}, validationError(ErrorUndeclaredExtension, input.SemanticID, assetPath+".extensions", string(extension), "a target-namespaced extension declared in the resolved workflow")
			}
		}

		asset := input
		asset.Content = bytes.Clone(input.Content)
		asset.Permissions = permissions
		asset.Extensions = assetExtensions
		assets[index] = asset
	}

	slices.SortFunc(assets, func(left, right Asset) int {
		if difference := strings.Compare(left.Path, right.Path); difference != 0 {
			return difference
		}
		return strings.Compare(string(left.SemanticID), string(right.SemanticID))
	})
	return Bundle{Assets: assets, Metadata: slices.Clone(bundle.Metadata)}, nil
}

func validateResolvedWorkflow(resolved ResolvedWorkflow) (map[AssetKind]struct{}, map[string]struct{}, map[ir.SemanticID]struct{}, error) {
	target := strings.TrimSpace(string(resolved.Target))
	if target == "" || target != string(resolved.Target) || strings.ContainsAny(target, "/\\") || strings.ToLower(target) != target {
		return nil, nil, nil, validationError(ErrorInvalidResolvedWorkflow, "workflow/resolved", "$.target", string(resolved.Target), "a lower-case target namespace segment")
	}
	if strings.TrimSpace(resolved.Profile) == "" {
		return nil, nil, nil, validationError(ErrorInvalidResolvedWorkflow, "workflow/resolved", "$.profile", resolved.Profile, "a resolved profile")
	}

	allowedKinds := make(map[AssetKind]struct{}, len(resolved.AllowedAssetKinds))
	for index, kind := range resolved.AllowedAssetKinds {
		if strings.TrimSpace(string(kind)) == "" {
			return nil, nil, nil, validationError(ErrorInvalidResolvedWorkflow, "workflow/resolved", fmt.Sprintf("$.allowed_asset_kinds[%d]", index), string(kind), "a non-empty resolved asset kind")
		}
		allowedKinds[kind] = struct{}{}
	}
	allowedPermissions := make(map[string]struct{}, len(resolved.AllowedPermissions))
	for index, permission := range resolved.AllowedPermissions {
		if strings.TrimSpace(permission) == "" || permission != strings.TrimSpace(permission) {
			return nil, nil, nil, validationError(ErrorInvalidResolvedWorkflow, "workflow/resolved", fmt.Sprintf("$.allowed_permissions[%d]", index), permission, "a non-empty canonical permission")
		}
		allowedPermissions[permission] = struct{}{}
	}

	extensions := make(map[ir.SemanticID]struct{}, len(resolved.Extensions))
	for index, extension := range resolved.Extensions {
		extensionPath := fmt.Sprintf("$.extensions[%d].id", index)
		if err := ir.ValidateSemanticID(extension.ID); err != nil || !strings.HasPrefix(string(extension.ID), target+"/") {
			return nil, nil, nil, validationError(ErrorInvalidExtension, extension.ID, extensionPath, string(extension.ID), "a semantic ID namespaced by target "+target)
		}
		if _, duplicate := extensions[extension.ID]; duplicate {
			return nil, nil, nil, validationError(ErrorInvalidExtension, extension.ID, extensionPath, string(extension.ID), "one declaration per extension semantic ID")
		}
		extensions[extension.ID] = struct{}{}
	}
	return allowedKinds, allowedPermissions, extensions, nil
}

func validateAssetIdentity(asset Asset, assetPath string) error {
	if err := ir.ValidateSemanticID(asset.SemanticID); err != nil {
		return validationError(ErrorInvalidAsset, asset.SemanticID, assetPath+".semantic_id", string(asset.SemanticID), "a canonical namespaced semantic ID")
	}
	if asset.Path == "" || !fs.ValidPath(asset.Path) || path.Clean(asset.Path) != asset.Path || strings.ContainsAny(asset.Path, `\:`) {
		return validationError(ErrorInvalidAsset, asset.SemanticID, assetPath+".path", asset.Path, "a clean slash-separated relative asset path")
	}
	if asset.Mode == 0 || asset.Mode != asset.Mode.Perm() {
		return validationError(ErrorInvalidAsset, asset.SemanticID, assetPath+".mode", fmt.Sprintf("%#o", asset.Mode), "non-zero portable permission bits without file type or special mode bits")
	}
	return nil
}

func validationError(id, semanticID ir.SemanticID, path, observed, expected string) *ValidationError {
	return &ValidationError{ID: id, SemanticID: semanticID, Path: path, Observed: observed, Expected: expected}
}

func sortedUnique[T ~string](values []T) []T {
	if values == nil {
		return []T{}
	}
	result := slices.Clone(values)
	slices.Sort(result)
	return slices.Compact(result)
}
