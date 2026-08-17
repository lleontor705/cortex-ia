package sdd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"syscall"

	"github.com/lleontor705/cortex-ia/internal/agents"
	"github.com/lleontor705/cortex-ia/internal/components/filemerge"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/capability"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/compiler"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/manifest"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/prompt"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/registry"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/renderers"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/resolution"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/skillcore"
	"github.com/lleontor705/cortex-ia/internal/model"
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
	// CustomSkills carries the registry-verified custom skill records with
	// their canonical bytes. The compiler projection deliberately strips raw
	// content, so the content-bearing records ride this bridge; every record
	// must agree with the composed overlay (fail closed on disagreement).
	CustomSkills []skillcore.Skill
	// SkillLayout is the adapter-declared destination authority for custom
	// skills (agents.SkillLayoutProvider). It is required exactly when the
	// composition carries a custom skill overlay; an overlay without a
	// declared layout is unrepresentable, never a guessed destination.
	SkillLayout agents.SkillLayoutProvider
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
	composition, err := renderersComposition(input.Compilation.Composition, input.CustomSkills)
	if err != nil {
		return CompiledInjectionBundle{}, fmt.Errorf("wire composed custom skills: %w", err)
	}
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
		Composition:             composition,
		NativeSkillPreload:      input.Compilation.Composition.Adapter.NativeSkillPreload,
		NativeSkillOnDemand:     input.Compilation.Composition.Adapter.NativeSkillOnDemand,
		NativeModelField:        input.Compilation.Composition.Adapter.NativeModelField,
		NativeWorktreeIsolation: input.Compilation.Composition.Adapter.NativeWorktreeIsolation,
		SkillLayout:             input.SkillLayout,
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

func renderersComposition(input prompt.CompositionResult, customSkills []skillcore.Skill) (renderers.Composition, error) {
	bindings := make([]renderers.SkillBinding, len(input.SkillBindings))
	for i, binding := range input.SkillBindings {
		mode := renderers.SkillModeFallbackRead
		switch binding.Mode {
		case prompt.SkillModeNativeOnDemand:
			mode = renderers.SkillModeNativeOnDemand
		case prompt.SkillModeNativePreload:
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
	overlay, err := composedCustomSkills(input.CustomSkills, customSkills)
	if err != nil {
		return renderers.Composition{}, err
	}
	return renderers.Composition{RootIndex: input.RootIndex, Modules: slices.Clone(input.Modules), SkillBindings: bindings, SharedContract: input.SharedContract, ProfileOverlay: input.ProfileOverlay, QualityTemplate: input.QualityTemplate, QualityPlan: input.QualityPlan, ModelRoutes: routes, OperationalAssets: assets, CustomSkills: overlay, Metadata: slices.Clone(input.Metadata)}, nil
}

// composedCustomSkills cross-checks the composed overlay layer (design WU-12)
// against the typed content-bearing records from the registry and returns the
// records the renderer lowers through the adapter-declared layout (WU-14/15).
// The check is fail-closed: a composed entry without a typed record, a digest
// disagreement, a non-custom origin, or a typed record absent from the
// composition is an inconsistent input and never a silently dropped skill.
func composedCustomSkills(composed []prompt.ComposedCustomSkill, typed []skillcore.Skill) ([]skillcore.Skill, error) {
	if len(composed) == 0 && len(typed) == 0 {
		return nil, nil
	}
	byID := make(map[model.SkillID]skillcore.Skill, len(typed))
	for _, skill := range typed {
		if _, duplicate := byID[skill.ID]; duplicate {
			return nil, fmt.Errorf("typed custom skill %q is declared twice", skill.ID)
		}
		byID[skill.ID] = skill
	}
	overlay := make([]skillcore.Skill, 0, len(composed))
	for _, entry := range composed {
		skill, ok := byID[model.SkillID(entry.ID)]
		if !ok {
			return nil, fmt.Errorf("composed custom skill %q has no content-bearing typed record", entry.ID)
		}
		if skill.Origin != skillcore.OriginCustom {
			return nil, fmt.Errorf("typed custom skill %q has non-custom origin %d", skill.ID, uint8(skill.Origin))
		}
		if skill.ContentSHA256 != entry.ContentSHA256 {
			return nil, fmt.Errorf("composed custom skill %q digest %s disagrees with the typed record digest %s", entry.ID, entry.ContentSHA256, skill.ContentSHA256)
		}
		delete(byID, skill.ID)
		record := skill
		record.Content = bytes.Clone(skill.Content)
		overlay = append(overlay, record)
	}
	if leftovers := sortedSkillIDs(byID); len(leftovers) != 0 {
		return nil, fmt.Errorf("typed custom skills absent from the composed overlay: %s", strings.Join(leftovers, ", "))
	}
	return overlay, nil
}

func sortedSkillIDs(skills map[model.SkillID]skillcore.Skill) []string {
	ids := make([]string, 0, len(skills))
	for id := range skills {
		ids = append(ids, string(id))
	}
	slices.Sort(ids)
	return ids
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

// ---------------------------------------------------------------------------
// Global install plan (pure, pre-write)
// ---------------------------------------------------------------------------

// Diagnostic rules of the bundle planning and apply stages. Rules are stable
// identifiers; thresholds and envelope definitions are not copied here.
const (
	RulePlanHomeRequired         = "bundle.plan.home_required"
	RulePlanReceiptUnsealed      = "bundle.plan.receipt_unsealed"
	RulePlanPathInvalid          = "bundle.plan.path_invalid"
	RulePlanDestinationCollision = "bundle.plan.destination_collision"
	RulePlanTargetNotRegular     = "bundle.plan.target_not_regular"
	RulePlanTargetUnreadable     = "bundle.plan.target_unreadable"
	RulePlanUnsafeAncestor       = "bundle.plan.unsafe_ancestor"
	RuleApplyHomeRequired        = "bundle.apply.home_required"
	RuleApplySnapshotFailed      = "bundle.apply.snapshot_failed"
	RuleApplyUnsafeAncestor      = "bundle.apply.unsafe_ancestor"
	RuleApplyWriteFailed         = "bundle.apply.write_failed"
	RuleVerifyFailed             = "bundle.verify.failed"
	RuleRollbackResidual         = "bundle.rollback.residual"
)

// BundleOperation is one apply-ready planned managed mutation ("bundle
// operation"): a write of planned bytes (Delete=false) or the retirement of a
// prior-managed output (Delete=true). Ops carry immutable planned bytes and
// the observed pre-plan digest, so apply never re-reads unverified sources
// (source-TOCTOU mitigation for design risk "Source TOCTOU between preflight
// and apply").
type BundleOperation struct {
	// Path is the home-relative, slash-separated target.
	Path string
	// SemanticID is the managed asset identity from the compiled bundle.
	SemanticID ir.SemanticID
	// Content is the planned bytes for writes; nil for deletes.
	Content []byte
	// Mode is the planned permission bits for writes.
	Mode fs.FileMode
	// Delete marks a stale prior-managed retirement.
	Delete bool
	// Existed reports whether the target existed when planned.
	Existed bool
	// BeforeSHA256 is the digest of the on-disk bytes when planned; empty
	// when the target was absent.
	BeforeSHA256 string
}

// SharedBundlePlan is the shared half of the global plan ("bundle plan"):
// writes of outputs desired identically by more than one adapter bundle
// (executed exactly once) and the stale-managed retirements, which are
// managed-output lifecycle decisions rather than adapter representation.
type SharedBundlePlan struct {
	Writes  []BundleOperation
	Deletes []BundleOperation
}

// AdapterPlan is one adapter's lowered operations. Ops are limited to that
// adapter's bundle outputs and are disjoint from every other plan entry
// (enforced by BuildGlobalInstallPlan), so apply chains may run per adapter.
type AdapterPlan struct {
	Agent model.AgentID
	Ops   []BundleOperation
}

// GlobalInstallPlan is the pure, pre-write result of planning every compiled
// bundle against the home directory: the shared plan, per-adapter writes, the
// rollback inventory, and the canonical receipt with the planned host
// outputs. Building it performs reads only — no directory creation, backup,
// or target mutation happens before it returns (design D7).
type GlobalInstallPlan struct {
	Shared        SharedBundlePlan
	Adapters      []AdapterPlan
	RollbackPaths []string
	Receipt       registry.Receipt
}

// GlobalInstallPlanRequest is the complete input to pure planning. Bundles are
// the compiled per-target bundles; PriorManagedPaths lists the outputs managed
// by the previous successful install (the prior canonical receipt's host
// outputs); Receipt is the sealed canonical receipt of this effective input.
type GlobalInstallPlanRequest struct {
	HomeDir           string
	Bundles           []CompiledInjectionBundle
	PriorManagedPaths []string
	Receipt           registry.Receipt
}

// BuildGlobalInstallPlan computes the exact convergence operations for the
// requested bundles without writing anything. A converged state yields no
// operations; a prior-managed output that is no longer desired yields a delete
// operation (prior-managed diff); destinations are collision-checked across
// bundles and against un-managed on-disk content. The returned receipt is the
// input receipt re-sealed with the planned host outputs. Non-empty
// diagnostics are pure pre-write rejections with a deterministic primary
// cause.
func BuildGlobalInstallPlan(request GlobalInstallPlanRequest) (GlobalInstallPlan, registry.Diagnostics) {
	if strings.TrimSpace(request.HomeDir) == "" {
		return GlobalInstallPlan{}, registry.SortDiagnostics(registry.Diagnostics{planDiagnostic(registry.ErrorInvalid, RulePlanHomeRequired, nil,
			"provide the home directory the bundles are planned against")})
	}
	if err := registry.ValidateReceipt(request.Receipt); err != nil {
		return GlobalInstallPlan{}, registry.SortDiagnostics(registry.Diagnostics{planDiagnostic(registry.ErrorInvalid, RulePlanReceiptUnsealed, err,
			"plan from the canonical receipt sealed by registry.Resolve for the same effective input")})
	}

	type desiredOutput struct {
		asset      renderers.Asset
		declarants int
	}
	desired := make(map[string]desiredOutput)
	desiredOrder := make([]string, 0)
	declare := func(asset renderers.Asset) (desiredOutput, bool) {
		if previous, exists := desired[asset.Path]; exists {
			if bytes.Equal(previous.asset.Content, asset.Content) && previous.asset.Mode.Perm() == asset.Mode.Perm() {
				desired[asset.Path] = desiredOutput{asset: previous.asset, declarants: previous.declarants + 1}
				return desired[asset.Path], true
			}
			return desiredOutput{}, false
		}
		desired[asset.Path] = desiredOutput{asset: asset, declarants: 1}
		desiredOrder = append(desiredOrder, asset.Path)
		return desired[asset.Path], true
	}

	diags := registry.Diagnostics{}
	for _, bundle := range request.Bundles {
		for _, asset := range bundle.Bundle.Assets {
			if !validBundleAssetPath(asset.Path) {
				diags = append(diags, registry.Diagnostic{
					Class: registry.ErrorInvalid, Stage: registry.StagePlan, Rule: RulePlanPathInvalid,
					Cause:           fmt.Errorf("bundle asset %q", asset.Path),
					SafeRemediation: "emit clean slash-separated home-relative bundle asset paths",
				})
				continue
			}
			if _, ok := declare(asset); !ok {
				diags = append(diags, registry.Diagnostic{
					Class: registry.ErrorCollision, Stage: registry.StagePlan, Rule: RulePlanDestinationCollision,
					Cause:           fmt.Errorf("destination %q is desired with differing content or mode", asset.Path),
					SafeRemediation: "render bundles so every destination carries exactly one desired output",
				})
			}
		}
	}

	managed := make(map[string]struct{}, len(request.PriorManagedPaths))
	for _, managedPath := range request.PriorManagedPaths {
		if !validBundleAssetPath(managedPath) {
			diags = append(diags, registry.Diagnostic{
				Class: registry.ErrorInvalid, Stage: registry.StagePlan, Rule: RulePlanPathInvalid,
				Cause:           fmt.Errorf("prior-managed path %q", managedPath),
				SafeRemediation: "record prior-managed outputs as clean home-relative paths from the committed receipt",
			})
			continue
		}
		managed[managedPath] = struct{}{}
	}

	if len(diags) != 0 {
		return GlobalInstallPlan{}, registry.SortDiagnostics(diags)
	}

	adapterIndex := make(map[model.AgentID]int)
	sharedWrites := make([]BundleOperation, 0)
	adapterWrites := make(map[model.AgentID][]BundleOperation, len(request.Bundles))
	snapshot := func(assetPath string) (bundleTargetState, bool) {
		state, err := readBundleTarget(request.HomeDir, assetPath)
		if err != nil {
			var unsafe *unsafeAncestorError
			if errors.As(err, &unsafe) {
				diags = append(diags, planDiagnostic(registry.ErrorInvalid, RulePlanUnsafeAncestor, err,
					"remove the symlinked or reparse-point ancestor beneath the home directory so managed outputs stay inside it"))
				return bundleTargetState{}, false
			}
			diags = append(diags, planDiagnostic(registry.ErrorInvalid, RulePlanTargetUnreadable, fmt.Errorf("target %q: %w", assetPath, err),
				"make planned targets readable regular files or remove them"))
			return bundleTargetState{}, false
		}
		if state.existed && !state.regular {
			diags = append(diags, planDiagnostic(registry.ErrorInvalid, RulePlanTargetNotRegular, fmt.Errorf("target %q", assetPath),
				"planned destinations and prior-managed outputs must be regular files"))
			return bundleTargetState{}, false
		}
		return state, true
	}

	for _, assetPath := range desiredOrder {
		output := desired[assetPath]
		state, ok := snapshot(assetPath)
		if !ok {
			continue
		}
		converged := state.existed && bytes.Equal(state.content, output.asset.Content)
		if !converged && state.existed {
			if _, isManaged := managed[assetPath]; !isManaged {
				diags = append(diags, planDiagnostic(registry.ErrorCollision, RulePlanDestinationCollision,
					fmt.Errorf("destination %q holds differing un-managed content", assetPath),
					"remove the conflicting file or restore it from the managed receipt before installing"))
				continue
			}
		}
		if converged {
			continue
		}
		op := BundleOperation{
			Path: assetPath, SemanticID: output.asset.SemanticID,
			Content: bytes.Clone(output.asset.Content), Mode: output.asset.Mode.Perm(),
			Existed: state.existed,
		}
		if state.existed {
			op.BeforeSHA256 = ir.FingerprintContent(state.content)
		}
		if output.declarants > 1 {
			sharedWrites = append(sharedWrites, op)
			continue
		}
		agent := agentIDForBundleOutput(request.Bundles, assetPath)
		adapterWrites[agent] = append(adapterWrites[agent], op)
	}

	staleDeletes := make([]BundleOperation, 0)
	managedOrder := make([]string, 0, len(managed))
	for managedPath := range managed {
		managedOrder = append(managedOrder, managedPath)
	}
	slices.Sort(managedOrder)
	for _, managedPath := range managedOrder {
		if _, retained := desired[managedPath]; retained {
			continue
		}
		state, ok := snapshot(managedPath)
		if !ok {
			continue
		}
		if !state.existed {
			continue // already converged: the managed output is gone
		}
		staleDeletes = append(staleDeletes, BundleOperation{
			Path: managedPath, Delete: true, Existed: true, BeforeSHA256: ir.FingerprintContent(state.content),
		})
	}

	if len(diags) != 0 {
		return GlobalInstallPlan{}, registry.SortDiagnostics(diags)
	}

	adapters := make([]AdapterPlan, 0, len(adapterWrites))
	for _, bundle := range request.Bundles {
		agent := model.AgentID(bundle.Target)
		if _, placed := adapterIndex[agent]; placed {
			continue
		}
		adapterIndex[agent] = len(adapters)
		ops := adapterWrites[agent]
		sortBundleOperations(ops)
		adapters = append(adapters, AdapterPlan{Agent: agent, Ops: ops})
	}
	sortBundleOperations(sharedWrites)
	sortBundleOperations(staleDeletes)

	rollbackPaths := make([]string, 0, len(sharedWrites)+len(staleDeletes))
	for _, ops := range [][]BundleOperation{sharedWrites, staleDeletes} {
		for _, op := range ops {
			rollbackPaths = append(rollbackPaths, op.Path)
		}
	}
	for _, adapter := range adapters {
		for _, op := range adapter.Ops {
			rollbackPaths = append(rollbackPaths, op.Path)
		}
	}
	slices.Sort(rollbackPaths)
	rollbackPaths = slices.Compact(rollbackPaths)

	hostOutputs := slices.Clone(desiredOrder)
	slices.Sort(hostOutputs)
	receipt := registry.SealReceipt(request.Receipt)
	receipt.HostOutputs = hostOutputs
	receipt = registry.SealReceipt(receipt)

	return GlobalInstallPlan{
		Shared:        SharedBundlePlan{Writes: sharedWrites, Deletes: staleDeletes},
		Adapters:      adapters,
		RollbackPaths: rollbackPaths,
		Receipt:       receipt,
	}, nil
}

func planDiagnostic(class registry.ErrorClass, rule string, cause error, remediation string) registry.Diagnostic {
	return registry.Diagnostic{
		Class: class, Stage: registry.StagePlan, Rule: rule,
		Cause: cause, SafeRemediation: remediation,
	}
}

func agentIDForBundleOutput(bundles []CompiledInjectionBundle, assetPath string) model.AgentID {
	for _, bundle := range bundles {
		for _, asset := range bundle.Bundle.Assets {
			if asset.Path == assetPath {
				return model.AgentID(bundle.Target)
			}
		}
	}
	return model.AgentID("")
}

func sortBundleOperations(ops []BundleOperation) {
	slices.SortFunc(ops, func(left, right BundleOperation) int { return strings.Compare(left.Path, right.Path) })
}

func validBundleAssetPath(assetPath string) bool {
	return assetPath != "" && fs.ValidPath(assetPath) && path.Clean(assetPath) == assetPath && !strings.ContainsAny(assetPath, `\:`)
}

// unsafeAncestorError reports a managed target whose path crosses a symlink
// or reparse-point ancestor beneath homeDir: path resolution through it can
// leave the home directory, so no managed observation or mutation may cross
// one. Windows junctions are reparse points os.Lstat reports without
// fs.ModeDir (and without fs.ModeSymlink on current Go), so the reparse
// irregular bit is rejected alongside symlinks.
type unsafeAncestorError struct {
	assetPath string
	ancestor  string
}

func (e *unsafeAncestorError) Error() string {
	return fmt.Sprintf("managed output %q traverses symlink or reparse ancestor %q beneath the home directory", e.assetPath, e.ancestor)
}

// guardBundleTarget rejects targets whose intermediate components beneath
// homeDir are symlinks or reparse points: path resolution through them can
// leave the home directory, so no managed observation or mutation may cross
// one. Non-directory ancestors that are plain files dead-end resolution
// inside homeDir (ENOTDIR) and are allowed through; the failing operation
// reports them organically. Absent ancestors cannot traverse either: the
// apply stage creates real directories for them. homeDir itself is the
// containment root and is not inspected; symlinks above it are the
// caller's authority.
func guardBundleTarget(homeDir, assetPath string) error {
	if assetPath == "" {
		return nil
	}
	if !fs.ValidPath(assetPath) || path.Clean(assetPath) != assetPath {
		return &unsafeAncestorError{assetPath: assetPath, ancestor: assetPath}
	}
	components := strings.Split(assetPath, "/")
	for index := 1; index < len(components); index++ {
		ancestor := strings.Join(components[:index], "/")
		info, err := os.Lstat(filepath.Join(homeDir, filepath.FromSlash(ancestor)))
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR) {
				return nil
			}
			return fmt.Errorf("inspect ancestor %q: %w", ancestor, err)
		}
		if mode := info.Mode(); mode&fs.ModeSymlink != 0 || mode&fs.ModeIrregular != 0 {
			return &unsafeAncestorError{assetPath: assetPath, ancestor: ancestor}
		}
	}
	return nil
}

// bundleTargetState is the plan-time observation of one target path.
type bundleTargetState struct {
	content []byte
	mode    fs.FileMode
	existed bool
	regular bool
}

// readBundleTarget reads a target path without creating anything. A path
// beneath a non-directory parent cannot exist as a regular file, so both
// ErrNotExist and ENOTDIR are reported as absent and the apply fails closed
// if the write is genuinely blocked. The observation refuses to resolve
// through symlinked or reparse-point ancestors beneath homeDir.
func readBundleTarget(homeDir, assetPath string) (bundleTargetState, error) {
	if err := guardBundleTarget(homeDir, assetPath); err != nil {
		return bundleTargetState{}, err
	}
	fullPath := filepath.Join(homeDir, filepath.FromSlash(assetPath))
	info, err := os.Lstat(fullPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR) {
			return bundleTargetState{}, nil
		}
		return bundleTargetState{}, fmt.Errorf("inspect target: %w", err)
	}
	if !info.Mode().IsRegular() {
		return bundleTargetState{existed: true, regular: false, mode: info.Mode().Perm()}, nil
	}
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return bundleTargetState{}, fmt.Errorf("read target: %w", err)
	}
	return bundleTargetState{content: content, mode: info.Mode().Perm(), existed: true, regular: true}, nil
}

// ---------------------------------------------------------------------------
// Transactional apply, verification, and canonical receipt commit
// ---------------------------------------------------------------------------

// GlobalApplyResult reports one converged apply. Receipt is the committed
// canonical receipt; it is zero on failure, and a failed apply never leaves a
// committed receipt behind (no false baseline receipt).
type GlobalApplyResult struct {
	Changed     bool
	Applied     []string
	Deleted     []string
	Receipt     registry.Receipt
	ReceiptPath string
}

// committedRegistryReceiptRelPath is the home-relative, slash-separated
// location of the canonical receipt; the apply guards its ancestors like
// every other managed output.
const committedRegistryReceiptRelPath = ".cortex-ia/sdd-registry-receipt.json"

// CommittedRegistryReceiptPath returns the canonical receipt location for a
// home directory.
func CommittedRegistryReceiptPath(homeDir string) string {
	return filepath.Join(homeDir, filepath.FromSlash(committedRegistryReceiptRelPath))
}

// ApplyGlobalInstallPlan applies the exact planned operations transactionally
// (design D10): every rollback path is snapshotted before mutation; any write,
// verification, or receipt-commit failure restores the snapshot; restoration
// is itself verified, and an incomplete restoration is reported as a rollback
// diagnostic rather than a success. The canonical receipt is committed only
// after the desired writes and deletes and the residual verification succeed.
// Operations execute sequentially (adapter writes, shared writes, stale
// deletes); per-adapter chain scheduling belongs to the caller.
func ApplyGlobalInstallPlan(homeDir string, plan GlobalInstallPlan) (GlobalApplyResult, error) {
	if strings.TrimSpace(homeDir) == "" {
		return GlobalApplyResult{}, &registry.InstallError{
			Primary: registry.Diagnostic{Class: registry.ErrorInvalid, Stage: registry.StageApply, Rule: RuleApplyHomeRequired,
				SafeRemediation: "provide the home directory the plan was built against"},
			All: []registry.Diagnostic{{Class: registry.ErrorInvalid, Stage: registry.StageApply, Rule: RuleApplyHomeRequired}},
		}
	}
	snapshot, err := snapshotRollbackPaths(homeDir, plan.RollbackPaths)
	if err != nil {
		return GlobalApplyResult{}, &registry.InstallError{
			Primary: applySnapshotDiagnostic(err),
			All:     []registry.Diagnostic{applySnapshotDiagnostic(err)},
		}
	}

	applied := make([]string, 0)
	deleted := make([]string, 0)
	writeOps := slices.Clone(plan.Shared.Writes)
	for _, adapter := range plan.Adapters {
		writeOps = append(writeOps, adapter.Ops...)
	}
	if err := executeBundleOperations(homeDir, writeOps, &applied); err != nil {
		return applyFailure(homeDir, snapshot, applyOperationDiagnostic(err,
			"free the reported target and rerun install; the pre-apply snapshot was restored"))
	}
	if err := executeBundleOperations(homeDir, plan.Shared.Deletes, &deleted); err != nil {
		return applyFailure(homeDir, snapshot, applyOperationDiagnostic(err,
			"free the reported stale output and rerun install; the pre-apply snapshot was restored"))
	}

	if err := verifyAppliedPlan(homeDir, plan); err != nil {
		return applyFailure(homeDir, snapshot, registry.Diagnostic{
			Class: registry.ErrorWrite, Stage: registry.StageVerify, Rule: RuleVerifyFailed, Cause: err,
			SafeRemediation: "inspect the reported output; the pre-apply snapshot was restored",
		})
	}

	receiptPath := CommittedRegistryReceiptPath(homeDir)
	if err := guardBundleTarget(homeDir, committedRegistryReceiptRelPath); err != nil {
		return applyFailure(homeDir, snapshot, applyOperationDiagnostic(fmt.Errorf("commit canonical receipt: %w", err), ""))
	}
	if _, err := filemerge.WriteFileAtomic(receiptPath, registry.CanonicalReceiptJSON(plan.Receipt), 0o644); err != nil {
		return applyFailure(homeDir, snapshot, registry.Diagnostic{
			Class: registry.ErrorWrite, Stage: registry.StageApply, Rule: RuleApplyWriteFailed,
			Cause:           fmt.Errorf("commit canonical receipt: %w", err),
			SafeRemediation: "make the receipt location writable and rerun install; the pre-apply snapshot was restored",
		})
	}

	return GlobalApplyResult{
		Changed:     len(applied)+len(deleted) != 0,
		Applied:     applied,
		Deleted:     deleted,
		Receipt:     plan.Receipt,
		ReceiptPath: receiptPath,
	}, nil
}

// applySnapshotDiagnostic classifies a pre-apply snapshot failure: a
// containment violation (symlink or reparse ancestor) is an invalid state,
// everything else is an unreadable-target write error.
func applySnapshotDiagnostic(err error) registry.Diagnostic {
	var unsafe *unsafeAncestorError
	if errors.As(err, &unsafe) {
		return registry.Diagnostic{
			Class: registry.ErrorInvalid, Stage: registry.StageApply, Rule: RuleApplyUnsafeAncestor, Cause: err,
			SafeRemediation: "remove the substituted symlink or reparse-point ancestor beneath the home directory and rerun install",
		}
	}
	return registry.Diagnostic{
		Class: registry.ErrorWrite, Stage: registry.StageApply, Rule: RuleApplySnapshotFailed, Cause: err,
		SafeRemediation: "make planned targets readable before applying",
	}
}

// applyOperationDiagnostic classifies an operation failure the same way.
func applyOperationDiagnostic(err error, fallbackRemediation string) registry.Diagnostic {
	var unsafe *unsafeAncestorError
	if errors.As(err, &unsafe) {
		return registry.Diagnostic{
			Class: registry.ErrorInvalid, Stage: registry.StageApply, Rule: RuleApplyUnsafeAncestor, Cause: err,
			SafeRemediation: "remove the substituted symlink or reparse-point ancestor beneath the home directory and rerun install",
		}
	}
	return registry.Diagnostic{
		Class: registry.ErrorWrite, Stage: registry.StageApply, Rule: RuleApplyWriteFailed, Cause: err,
		SafeRemediation: fallbackRemediation,
	}
}

// applyFailure restores the pre-apply snapshot and reports the failure. A
// complete restoration yields the original write/verify diagnostic with no
// rollback entry; an incomplete restoration adds a rollback diagnostic listing
// the residuals so no false success or false baseline receipt is possible.
func applyFailure(homeDir string, snapshot map[string]bundleTargetState, primary registry.Diagnostic) (GlobalApplyResult, error) {
	residuals := restoreSnapshot(homeDir, snapshot)
	installErr := &registry.InstallError{Primary: primary, All: []registry.Diagnostic{primary}}
	if len(residuals) != 0 {
		rollback := registry.Diagnostic{
			Class: registry.ErrorRollback, Stage: registry.StageRollback, Rule: RuleRollbackResidual,
			Cause:           fmt.Errorf("restoration did not converge; residual paths: %s", strings.Join(residuals, ", ")),
			SafeRemediation: "remove or fix the listed residual paths manually, then rerun install",
		}
		installErr.Rollback = &rollback
		installErr.All = append(installErr.All, rollback)
	}
	return GlobalApplyResult{}, installErr
}

func snapshotRollbackPaths(homeDir string, rollbackPaths []string) (map[string]bundleTargetState, error) {
	snapshot := make(map[string]bundleTargetState, len(rollbackPaths))
	for _, rollbackPath := range rollbackPaths {
		state, err := readBundleTarget(homeDir, rollbackPath)
		if err != nil {
			return nil, fmt.Errorf("snapshot %q: %w", rollbackPath, err)
		}
		snapshot[rollbackPath] = state
	}
	return snapshot, nil
}

// bundleOpFaultHook is the package's fault-injection seam for its own
// transactional tests: when non-nil it runs before each planned operation,
// after any earlier operation has already been applied. It mirrors the role
// of the install package's internal beforeMutation hook and is nil in
// production.
var bundleOpFaultHook func(op BundleOperation) error

func executeBundleOperations(homeDir string, ops []BundleOperation, touched *[]string) error {
	for _, op := range ops {
		if bundleOpFaultHook != nil {
			if err := bundleOpFaultHook(op); err != nil {
				return fmt.Errorf("managed output %q: %w", op.Path, err)
			}
		}
		// Re-checked per operation so an ancestor substituted after planning
		// (or after the snapshot) is rejected before os.Remove or the atomic
		// write ever resolves through it.
		if err := guardBundleTarget(homeDir, op.Path); err != nil {
			return fmt.Errorf("managed output %q: %w", op.Path, err)
		}
		fullPath := filepath.Join(homeDir, filepath.FromSlash(op.Path))
		if op.Delete {
			if err := os.Remove(fullPath); err != nil && !errors.Is(err, fs.ErrNotExist) {
				return fmt.Errorf("delete managed output %q: %w", op.Path, err)
			}
			*touched = append(*touched, op.Path)
			continue
		}
		mode := op.Mode.Perm()
		if mode == 0 {
			mode = 0o644
		}
		if _, err := filemerge.WriteFileAtomic(fullPath, op.Content, mode); err != nil {
			return fmt.Errorf("write managed output %q: %w", op.Path, err)
		}
		*touched = append(*touched, op.Path)
	}
	return nil
}

// verifyAppliedPlan proves convergence: every planned host output exists, the
// bytes of every written output match the plan exactly, and every stale
// managed retirement is absent.
func verifyAppliedPlan(homeDir string, plan GlobalInstallPlan) error {
	for _, outputPath := range plan.Receipt.HostOutputs {
		state, err := readBundleTarget(homeDir, outputPath)
		if err != nil {
			return fmt.Errorf("verify output %q: %w", outputPath, err)
		}
		if !state.existed || !state.regular {
			return fmt.Errorf("expected managed output %q is absent or not a regular file", outputPath)
		}
	}
	writes := slices.Clone(plan.Shared.Writes)
	for _, adapter := range plan.Adapters {
		writes = append(writes, adapter.Ops...)
	}
	for _, op := range writes {
		state, err := readBundleTarget(homeDir, op.Path)
		if err != nil {
			return fmt.Errorf("verify written output %q: %w", op.Path, err)
		}
		if !state.existed || !bytes.Equal(state.content, op.Content) {
			return fmt.Errorf("written output %q does not match the planned bytes", op.Path)
		}
	}
	for _, op := range plan.Shared.Deletes {
		if err := guardBundleTarget(homeDir, op.Path); err != nil {
			return fmt.Errorf("verify stale output %q: %w", op.Path, err)
		}
		fullPath := filepath.Join(homeDir, filepath.FromSlash(op.Path))
		if _, err := os.Lstat(fullPath); !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("stale managed output %q is still present", op.Path)
		}
	}
	return nil
}

// restoreSnapshot restores every snapshotted path to its pre-apply state and
// verifies the restoration, returning the paths that could not be converged.
// Files that existed are rewritten from the snapshot bytes; files that did
// not exist are removed.
func restoreSnapshot(homeDir string, snapshot map[string]bundleTargetState) []string {
	paths := make([]string, 0, len(snapshot))
	for rollbackPath := range snapshot {
		paths = append(paths, rollbackPath)
	}
	slices.Sort(paths)
	residuals := make([]string, 0)
	for _, rollbackPath := range paths {
		entry := snapshot[rollbackPath]
		// A restoration through a symlinked or reparse ancestor would mutate
		// outside the home directory; the path stays a reported residual so
		// no false success is possible.
		if err := guardBundleTarget(homeDir, rollbackPath); err != nil {
			residuals = append(residuals, rollbackPath)
			continue
		}
		fullPath := filepath.Join(homeDir, filepath.FromSlash(rollbackPath))
		if entry.existed && entry.regular {
			mode := entry.mode.Perm()
			if mode == 0 {
				mode = 0o644
			}
			if _, err := filemerge.WriteFileAtomic(fullPath, entry.content, mode); err != nil {
				residuals = append(residuals, rollbackPath)
				continue
			}
		} else if !entry.existed {
			if err := os.Remove(fullPath); err != nil && !errors.Is(err, fs.ErrNotExist) {
				residuals = append(residuals, rollbackPath)
				continue
			}
		}
		state, err := readBundleTarget(homeDir, rollbackPath)
		switch {
		case err != nil:
			residuals = append(residuals, rollbackPath)
		case entry.existed && entry.regular && (!state.existed || !bytes.Equal(state.content, entry.content)):
			residuals = append(residuals, rollbackPath)
		case !entry.existed && state.existed:
			residuals = append(residuals, rollbackPath)
		}
	}
	return residuals
}

// ---------------------------------------------------------------------------
// Committed canonical receipt
// ---------------------------------------------------------------------------

// committedRegistryReceipt mirrors the canonical receipt projection owned by
// the registry package (registry.CanonicalReceiptJSON). Only these stable
// fields exist on disk; volatile and execution-order data is never persisted.
type committedRegistryReceipt struct {
	BaselineDigest      string   `json:"baseline_digest"`
	EffectiveComponents []string `json:"effective_components"`
	EffectiveSkills     []struct {
		ContentSHA256 string `json:"content_sha256"`
		ID            string `json:"id"`
		Origin        string `json:"origin"`
	} `json:"effective_skills"`
	HostOutputs   []string `json:"host_outputs"`
	PolicyDigest  string   `json:"policy_digest"`
	SchemaVersion string   `json:"schema_version"`
}

// LoadCommittedRegistryReceipt loads and re-seals the canonical receipt
// committed by the last successful apply. The canonical projection excludes
// the fingerprint and raw skill bytes, so the receipt is re-sealed from the
// canonical fields; identical effective inputs always re-seal to the identical
// fingerprint. A missing receipt surfaces fs.ErrNotExist for the caller to
// treat as a first install.
func LoadCommittedRegistryReceipt(homeDir string) (registry.Receipt, error) {
	raw, err := os.ReadFile(CommittedRegistryReceiptPath(homeDir))
	if err != nil {
		return registry.Receipt{}, err
	}
	var committed committedRegistryReceipt
	if err := json.Unmarshal(raw, &committed); err != nil {
		return registry.Receipt{}, fmt.Errorf("decode committed registry receipt: %w", err)
	}
	receipt := registry.Receipt{
		SchemaVersion:  committed.SchemaVersion,
		PolicyDigest:   committed.PolicyDigest,
		BaselineDigest: committed.BaselineDigest,
		HostOutputs:    committed.HostOutputs,
	}
	receipt.EffectiveComponents = make([]model.ComponentID, 0, len(committed.EffectiveComponents))
	for _, id := range committed.EffectiveComponents {
		receipt.EffectiveComponents = append(receipt.EffectiveComponents, model.ComponentID(id))
	}
	receipt.EffectiveSkills.Ordered = make([]skillcore.Skill, 0, len(committed.EffectiveSkills))
	for _, skill := range committed.EffectiveSkills {
		origin := skillcore.OriginEmbedded
		if skill.Origin == "custom" {
			origin = skillcore.OriginCustom
		}
		receipt.EffectiveSkills.Ordered = append(receipt.EffectiveSkills.Ordered, skillcore.Skill{
			ID: model.SkillID(skill.ID), ContentSHA256: skill.ContentSHA256, Origin: origin,
		})
	}
	return registry.SealReceipt(receipt), nil
}
