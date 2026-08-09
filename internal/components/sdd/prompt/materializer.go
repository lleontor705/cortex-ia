package prompt

import (
	"encoding/json"
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
	"github.com/lleontor705/cortex-ia/internal/modelroute"
)

// MaterializedAsset is the adapter-neutral operational representation lowered
// once before renderer-specific syntax. Its semantic metadata is receipt-safe.
type MaterializedAsset struct {
	ID                ir.SemanticID            `json:"id"`
	Class             ir.AssetClass            `json:"class"`
	Path              string                   `json:"path"`
	Content           []byte                   `json:"content"`
	Scope             ir.AssetScope            `json:"scope"`
	LoadMode          SkillLoadMode            `json:"load_mode,omitempty"`
	Permissions       []string                 `json:"permissions,omitempty"`
	Primary           string                   `json:"primary,omitempty"`
	Fallback          string                   `json:"fallback,omitempty"`
	Degradation       string                   `json:"degradation,omitempty"`
	GeneratorVersion  string                   `json:"generator_version,omitempty"`
	SourceFingerprint string                   `json:"source_fingerprint,omitempty"`
	Metadata          json.RawMessage          `json:"metadata,omitempty"`
	Route             modelroute.ResolvedRoute `json:"route,omitempty"`
}

type MaterializerInput struct {
	Catalog             ir.AssetCatalog
	Contents            map[ir.SemanticID][]byte
	Workflow            ir.WorkflowIR
	Adapter             AdapterPromptContract
	Profile             string
	Models              ModelTable
	AllowedPermissions  []string
	GeneratedReferences []GeneratedReference
	Metadata            json.RawMessage
}

// TemplateContext is the typed path context available to operational assets.
// Templates are rendered by the materializer, not by renderers or installers,
// so path placeholders cannot drift between adapter implementations.
type TemplateContext struct {
	WorkflowRoot string
	SkillsRoot   string
}

func (input MaterializerInput) templateContext() TemplateContext {
	return TemplateContext{WorkflowRoot: input.Adapter.RootPath, SkillsRoot: input.Adapter.SkillRoot}
}

// RenderTypedTemplate resolves only the supported operational path tokens.
// Keeping token ownership here makes unresolved placeholders a compile-time
// error while avoiding renderer-specific string rewriting.
func RenderTypedTemplate(content []byte, context TemplateContext) ([]byte, error) {
	text := string(content)
	for _, token := range []struct {
		name  string
		value string
	}{
		{name: "{{SKILLS_DIR}}", value: context.SkillsRoot},
		{name: "{{HOME}}", value: context.WorkflowRoot},
	} {
		if token.value == "" {
			return nil, fmt.Errorf("template token %s has an empty typed path", token.name)
		}
		for {
			index := strings.Index(text, token.name)
			if index < 0 {
				break
			}
			text = text[:index] + token.value + text[index+len(token.name):]
		}
	}
	if strings.Contains(text, "{{SKILLS_DIR}}") || strings.Contains(text, "{{HOME}}") {
		return nil, fmt.Errorf("unresolved typed path placeholder")
	}
	return []byte(text), nil
}

// Materialize lowers required catalog entries into one deterministic common
// semantic set. Missing required bytes and duplicate IDs fail before mutation.
func Materialize(input MaterializerInput) ([]MaterializedAsset, []string, error) {
	if err := ValidatePhaseRoleBindings(CanonicalPhaseRoleBindings()); err != nil {
		return nil, nil, fmt.Errorf("materialize canonical phase-role bindings: %w", err)
	}
	if err := input.Catalog.Validate(); err != nil {
		return nil, nil, err
	}
	if err := input.Adapter.Validate(); err != nil {
		return nil, nil, err
	}
	assets := make([]MaterializedAsset, 0, len(input.Catalog.Assets)+len(input.Workflow.Roles))
	seen := map[ir.SemanticID]struct{}{}
	degradations := []string{}
	generatedMetadata := make(map[ir.SemanticID]GeneratedReference, len(input.GeneratedReferences))
	for _, reference := range input.GeneratedReferences {
		if _, exists := generatedMetadata[reference.ID]; exists {
			return nil, nil, fmt.Errorf("duplicate generated reference %q", reference.ID)
		}
		generatedMetadata[reference.ID] = reference
	}
	add := func(id ir.SemanticID, class ir.AssetClass, assetPath string, content []byte, mode SkillLoadMode, permissions []string, route ModelRoute, degradation string) error {
		if _, ok := seen[id]; ok {
			return fmt.Errorf("duplicate materialized semantic ID %q", id)
		}
		if len(content) == 0 {
			return fmt.Errorf("required materialized asset %q is empty", id)
		}
		var err error
		content, err = RenderTypedTemplate(content, input.templateContext())
		if err != nil {
			return err
		}
		seen[id] = struct{}{}
		var asset MaterializedAsset
		resolved := route
		if resolved.PrimaryID == "" && resolved.Role == "" {
			asset = MaterializedAsset{ID: id, Class: class, Path: assetPath, Content: slices.Clone(content), Scope: ir.ScopeWorkflowRoot, LoadMode: mode, Permissions: sortedIntersection(permissions, input.AllowedPermissions), Degradation: degradation, Metadata: slices.Clone(input.Metadata)}
		} else if resolved.PrimaryID != "" {
			asset = MaterializedAsset{ID: id, Class: class, Path: assetPath, Content: slices.Clone(content), Scope: ir.ScopeWorkflowRoot, LoadMode: mode, Permissions: sortedIntersection(permissions, input.AllowedPermissions), Primary: string(resolved.Primary.Model), Fallback: fallbackModelRef(resolved), Degradation: resolved.Degradation, Metadata: slices.Clone(input.Metadata), Route: resolved}
		} else {
			return fmt.Errorf("model route %q has incomplete resolution evidence", route.Role)
		}
		if reference, ok := generatedMetadata[id]; ok {
			asset.GeneratorVersion = reference.Version
			asset.SourceFingerprint = reference.SourceFingerprint
		}
		assets = append(assets, asset)
		return nil
	}
	for _, spec := range input.Catalog.Assets {
		if !spec.Required {
			continue
		}
		if spec.Class == ir.AssetCommand && !input.Adapter.SupportsSlashCommands {
			continue
		}
		if spec.Class == ir.AssetProfileOverlay && !profileMatchesProfiles(input.Profile, spec.Profiles) {
			continue
		}
		content := input.Contents[spec.ID]
		assetPath := spec.SourcePath
		if spec.Class == ir.AssetCommand {
			var err error
			assetPath, err = input.Adapter.ExpandPath(input.Adapter.CommandRoot, strings.TrimPrefix(string(spec.ID), "command/")+".md")
			if err != nil {
				return nil, nil, fmt.Errorf("expand command path for %q: %w", spec.ID, err)
			}
		}
		degradation := ""
		if spec.Class == ir.AssetCommand && !input.Adapter.SupportsSlashCommands {
			degradation = "unsupported native slash-command form"
		}
		if err := add(spec.ID, spec.Class, assetPath, content, input.Adapter.SkillLoadMode(), nil, ModelRoute{}, degradation); err != nil {
			return nil, nil, err
		}
	}
	if !input.Adapter.SupportsSlashCommands {
		degradations = append(degradations, "commands: unsupported native form")
	}
	for _, role := range input.Workflow.Roles {
		name := strings.TrimPrefix(string(role.ID), "role/")
		binding, err := NewSkillBinding(role.ID, input.Adapter)
		if err != nil {
			return nil, nil, err
		}
		if err := binding.Validate(); err != nil {
			return nil, nil, fmt.Errorf("validate skill binding for %q: %w", role.ID, err)
		}
		route, err := routeForRole(input.Models, role.ID)
		if err != nil {
			return nil, nil, err
		}
		fallback := fallbackModelRef(route)
		content := materializedRoleContent(role.ID, binding, string(route.Primary.Model), fallback)
		permissions := make([]string, 0, len(role.AllowedEffects))
		for _, effect := range role.AllowedEffects {
			permissions = append(permissions, string(effect))
		}
		if err := add(ir.SemanticID("asset/role/"+name+"/binding"), ir.AssetRoleStub, path.Join(input.Adapter.RootPath, "roles", name+".md"), content, input.Adapter.SkillLoadMode(), permissions, route, ""); err != nil {
			return nil, nil, err
		}
	}
	if len(input.Models.Routes) > 0 {
		for _, role := range canonicalRoles {
			route, err := input.Models.ModelFor(role)
			if err != nil || route.PrimaryID == "" || route.Primary.Provider == "" || route.Primary.Model == "" || len(route.Evidence) == 0 || (route.Requested.AllowFallback && route.Fallback == nil) {
				return nil, nil, fmt.Errorf("model route %q requires primary and fallback", role)
			}
		}
	}
	slices.SortFunc(assets, func(a, b MaterializedAsset) int { return strings.Compare(string(a.ID), string(b.ID)) })
	return assets, degradations, nil
}

func materializedRoleContent(role ir.SemanticID, binding SkillBinding, primary, fallback string) []byte {
	firstAction := fmt.Sprintf("First phase action: read the required fallback skill `%s` before any phase work.", binding.Path)
	if binding.Mode == SkillModeNativePreload {
		firstAction = fmt.Sprintf("First phase action: load native skill preload `%s` before any phase work.", binding.Path)
	}
	if primary == "" {
		primary = "(none)"
	}
	return []byte(fmt.Sprintf("# Role %s\n\nCanonical skill: `%s`\nLoad mode: `%s`\nModel: `%s` (fallback `%s`)\n\n%s\nContinue phase work only after the mapped skill is loaded.\n", role, binding.Path, binding.Mode, primary, fallback, firstAction))
}

func routeForRole(models ModelTable, role ir.SemanticID) (ModelRoute, error) {
	if len(models.Routes) == 0 {
		return ModelRoute{}, nil
	}
	if route, err := models.ModelFor(role); err == nil {
		if route.PrimaryID != "" {
			return route, nil
		}
	}
	return ModelRoute{}, fmt.Errorf("model route %q requires primary and fallback", role)
}

func fallbackModelRef(route modelroute.ResolvedRoute) string {
	if route.Fallback == nil {
		return ""
	}
	return string(route.Fallback.Model)
}

func sortedIntersection(requested, allowed []string) []string {
	set := make(map[string]struct{}, len(allowed))
	for _, value := range allowed {
		set[value] = struct{}{}
	}
	result := make([]string, 0, len(requested))
	for _, value := range requested {
		if _, ok := set[value]; ok {
			result = append(result, value)
		}
	}
	slices.Sort(result)
	return slices.Compact(result)
}
