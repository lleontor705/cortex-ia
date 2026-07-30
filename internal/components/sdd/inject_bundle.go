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
	selection, err := SelectCompiledWorkflowProfile(input.Compilation, input.NativeCapabilities, input.ExperimentalOptIns)
	if err != nil {
		return CompiledInjectionBundle{}, err
	}
	if input.Renderer == nil {
		return CompiledInjectionBundle{}, fmt.Errorf("compiled bundle renderer is required")
	}
	target := renderers.TargetID(input.Compilation.Normalized.Target)
	resolved := renderers.ResolvedWorkflow{
		Workflow:              input.Compilation.Normalized.Workflow,
		Target:                target,
		Profile:               string(selection.Profile),
		GenerationFingerprint: input.Compilation.Fingerprint,
		Capabilities:          slices.Clone(input.Capabilities),
		AllowedAssetKinds:     slices.Clone(input.AllowedAssetKinds),
		AllowedPermissions:    slices.Clone(input.AllowedPermissions),
		Extensions:            slices.Clone(input.Extensions),
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
