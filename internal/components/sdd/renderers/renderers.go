// Package renderers defines the runtime-renderer boundary and validates its
// output before a bundle can reach persistent installation code.
package renderers

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"slices"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/resolution"
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
}

// Bundle is ordered by relative path and semantic ID after validation.
type Bundle struct {
	Assets []Asset `json:"assets"`
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
	return Bundle{Assets: assets}, nil
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
