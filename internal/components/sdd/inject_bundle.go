package sdd

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/components/filemerge"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/capability"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/compiler"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/manifest"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/prompt"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/renderers"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/resolution"
)

// BundleCompilationInput is the only bridge from a normalized compiler result
// to SDD installation assets. It contains rendering bounds, not runtime
// scheduling or execution state.
type BundleCompilationInput struct {
	Compilation        compiler.Result
	Renderer           renderers.Renderer
	NativeCapabilities []capability.CapabilityID
	ExperimentalOptIns []capability.CapabilityID
	Capabilities       []resolution.Resolution
	AllowedAssetKinds  []renderers.AssetKind
	AllowedPermissions []string
	Extensions         []renderers.ExtensionDeclaration
	// ProfileOverride is the already-qualified profile selected by PrepareWorkflow.
	// When present it prevents a later renderer pass from silently falling back
	// to portable-sequential.
	ProfileOverride string
}

// CompiledInjectionBundle is an immutable, pre-install result. Profile and
// degradation are exposed here so callers can display them before mutation.
type CompiledInjectionBundle struct {
	Target       renderers.TargetID
	Profile      WorkflowProfile
	Degradations []string
	Fingerprint  string
	Bundle       renderers.Bundle
}

// CompileInjectionBundle selects a profile exclusively from the normalized
// compiler snapshot and renders one deterministic bundle. It performs no
// filesystem or external-service mutation.
func CompileInjectionBundle(ctx context.Context, input BundleCompilationInput) (CompiledInjectionBundle, error) {
	var selection ProfileSelection
	if input.ProfileOverride != "" {
		selection = ProfileSelection{Profile: WorkflowProfile(input.ProfileOverride), Degradations: []string{}}
	} else {
		var err error
		selection, err = SelectCompiledWorkflowProfile(input.Compilation, input.NativeCapabilities, input.ExperimentalOptIns)
		if err != nil {
			return CompiledInjectionBundle{}, err
		}
	}
	if input.Renderer == nil {
		return CompiledInjectionBundle{}, fmt.Errorf("compiled bundle renderer is required")
	}
	target := renderers.TargetID(input.Compilation.Normalized.Target)
	resolved := renderers.ResolvedWorkflow{
		Workflow:                input.Compilation.Normalized.Workflow,
		Target:                  target,
		Profile:                 string(selection.Profile),
		GenerationFingerprint:   input.Compilation.Fingerprint,
		Capabilities:            slices.Clone(input.Capabilities),
		AllowedAssetKinds:       slices.Clone(input.AllowedAssetKinds),
		AllowedPermissions:      slices.Clone(input.AllowedPermissions),
		Extensions:              slices.Clone(input.Extensions),
		QualificationEvidence:   qualificationEvidence(input.Compilation.Normalized.Catalog.Facts),
		Metadata:                slices.Clone(input.Compilation.Metadata),
		Composition:             renderersComposition(input.Compilation.Composition),
		NativeSkillPreload:      input.Compilation.Composition.Adapter.NativeSkillPreload,
		NativeModelField:        input.Compilation.Composition.Adapter.NativeModelField,
		NativeWorktreeIsolation: input.Compilation.Composition.Adapter.NativeWorktreeIsolation,
	}
	bundle, err := renderers.Render(ctx, input.Renderer, resolved)
	if err != nil {
		return CompiledInjectionBundle{}, fmt.Errorf("render compiled SDD bundle: %w", err)
	}
	degradations := append([]string(nil), selection.Degradations...)
	for _, degradation := range input.Compilation.Normalized.Degradations {
		degradations = append(degradations, formatCompilerDegradation(degradation))
	}
	slices.Sort(degradations)
	degradations = slices.Compact(degradations)
	return CompiledInjectionBundle{
		Target:       target,
		Profile:      selection.Profile,
		Degradations: degradations,
		Fingerprint:  input.Compilation.Fingerprint,
		Bundle:       bundle,
	}, nil
}

func qualificationEvidence(facts []capability.CapabilityFact) []manifest.Evidence {
	result := make([]manifest.Evidence, 0, len(facts))
	for _, fact := range facts {
		if fact.EvidenceRef == "" {
			continue
		}
		result = append(result, manifest.Evidence{
			ID: ir.SemanticID("evidence/" + string(fact.ID)), Class: fact.EvidenceClass, Reference: fact.EvidenceRef,
			Fresh: fact.FreshUntil.After(fact.ObservedAt), Experimental: fact.Experimental, Confidence: fact.Confidence,
		})
	}
	return result
}

func renderersComposition(input prompt.CompositionResult) renderers.Composition {
	bindings := make([]renderers.SkillBinding, len(input.SkillBindings))
	for i, binding := range input.SkillBindings {
		mode := renderers.SkillModeFallbackRead
		if binding.Mode == prompt.SkillModeNativePreload {
			mode = renderers.SkillModeNativePreload
		}
		bindings[i] = renderers.SkillBinding{Role: binding.Role, Skill: binding.Skill, Mode: mode, Path: binding.Path, Hash: binding.Hash}
	}
	assets := make([]renderers.CompositionAsset, len(input.OperationalAssets))
	for i, asset := range input.OperationalAssets {
		assets[i] = renderers.CompositionAsset{ID: asset.ID, Class: asset.Class, Path: asset.Path, Content: slices.Clone(asset.Content), LoadMode: renderers.SkillLoadMode(asset.LoadMode), Permissions: slices.Clone(asset.Permissions), Metadata: slices.Clone(asset.Metadata), Route: asset.Route}
	}
	routes := make([]renderers.ModelRoute, 0, len(input.OperationalAssets))
	for _, asset := range input.OperationalAssets {
		if asset.Primary == "" && asset.Fallback == "" {
			continue
		}
		asset.Route.Role = asset.ID
		routes = append(routes, asset.Route)
	}
	return renderers.Composition{RootIndex: input.RootIndex, Modules: slices.Clone(input.Modules), SkillBindings: bindings, SharedContract: input.SharedContract, ProfileOverlay: input.ProfileOverlay, QualityTemplate: input.QualityTemplate, QualityPlan: input.QualityPlan, ModelRoutes: routes, OperationalAssets: assets, Metadata: slices.Clone(input.Metadata)}
}

// InjectCompiledBundle applies only the assets in a previously compiled
// bundle. Reapplying identical bytes is a no-op through WriteFileAtomic.
func InjectCompiledBundle(homeDir string, compiled CompiledInjectionBundle) (InjectionResult, error) {
	files := make([]string, 0, len(compiled.Bundle.Assets))
	changed := false
	for _, asset := range compiled.Bundle.Assets {
		path := filepath.Join(homeDir, filepath.FromSlash(asset.Path))
		result, err := filemerge.WriteFileAtomic(path, asset.Content, asset.Mode)
		if err != nil {
			return InjectionResult{}, fmt.Errorf("write compiled SDD asset %q: %w", asset.Path, err)
		}
		changed = changed || result.Changed
		files = append(files, path)
	}
	return InjectionResult{
		Changed:      changed,
		Files:        files,
		Profile:      compiled.Profile,
		Degradations: slices.Clone(compiled.Degradations),
		Fingerprint:  compiled.Fingerprint,
	}, nil
}

func formatCompilerDegradation(degradation ir.Degradation) string {
	return strings.TrimSpace(string(degradation.SemanticID) + ": " + degradation.Reason)
}
