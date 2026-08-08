package pipeline

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/lleontor705/cortex-ia/internal/agents"
	forgespeccomp "github.com/lleontor705/cortex-ia/internal/components/forgespec"
	"github.com/lleontor705/cortex-ia/internal/components/mcpinject"
	"github.com/lleontor705/cortex-ia/internal/components/mcpprobe"
	"github.com/lleontor705/cortex-ia/internal/components/sdd"
	operationalassets "github.com/lleontor705/cortex-ia/internal/components/sdd/assets"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/canonical"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/capability"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/compiler"
	sddinstall "github.com/lleontor705/cortex-ia/internal/components/sdd/install"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/installroots"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/manifest"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/prompt"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/quality"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/renderers"
	sddresolution "github.com/lleontor705/cortex-ia/internal/components/sdd/resolution"
	"github.com/lleontor705/cortex-ia/internal/model"
	"github.com/lleontor705/cortex-ia/internal/modelroute"
	"github.com/lleontor705/cortex-ia/internal/state"
	"github.com/lleontor705/cortex-ia/internal/verify"
)

type WorkflowRequest struct {
	HomeDir               string
	Adapters              []agents.Adapter
	GeneratorVersion      string
	ForgeSpecEndpoint     string
	ForgeSpecRequirements forgespeccomp.WorkflowRequirements
	EvaluationTime        time.Time
	RequestedProfile      sdd.WorkflowProfile
	ExperimentalOptIns    []capability.CapabilityID
	ModelRoutes           prompt.ModelTable
	RouteResolution       modelroute.ResolverInput
}

type TargetBundle struct {
	Target      renderers.TargetID
	Profile     string
	Fingerprint string
	Bundle      renderers.Bundle
}

// PreparedWorkflowInstall is the immutable compiler-to-plan boundary shared by
// dry-run disclosure and apply. It contains no scheduler or runtime state.
type PreparedWorkflowInstall struct {
	Plan              sddinstall.Plan
	Bundles           []TargetBundle
	Fingerprint       string
	BundleFingerprint string
	Cutover           forgespeccomp.ForgeSpecResolution
	Doctor            verify.DoctorReport
	Retirements       []mcpinject.ConfigRetirement
	Metadata          WorkflowMetadata
	root              string
}

func resolveWorkflowRoutes(ctx context.Context, request WorkflowRequest) (prompt.ModelTable, map[string]modelroute.ResolvedRoute, error) {
	if len(request.RouteResolution.Requests) != 0 || request.RouteResolution.ActiveProfile != "" {
		resolved, _, err := modelroute.NewResolver().Resolve(ctx, request.RouteResolution)
		if err != nil {
			return prompt.ModelTable{}, nil, err
		}
		routes := make([]prompt.ModelRoute, 0, len(resolved))
		metadata := make(map[string]modelroute.ResolvedRoute, len(resolved))
		for name, decision := range resolved {
			role := ir.SemanticID(name)
			if !strings.HasPrefix(string(role), "role/") {
				role = ir.SemanticID("role/" + name)
			}
			decision.Role = role
			routes = append(routes, decision)
			metadata[string(role)] = decision
		}
		routes, metadata = completeTransverseRoutes(routes, metadata)
		return prompt.ModelTable{Routes: routes}, metadata, nil
	}
	if len(request.ModelRoutes.Routes) == 0 {
		return prompt.ModelTable{}, nil, fmt.Errorf("explicit model routes are required")
	}
	metadata := make(map[string]modelroute.ResolvedRoute)
	for _, route := range request.ModelRoutes.Routes {
		if route.PrimaryID != "" {
			metadata[string(route.Role)] = route
		}
	}
	routes, metadata := completeTransverseRoutes(request.ModelRoutes.Routes, metadata)
	return prompt.ModelTable{Routes: routes}, metadata, nil
}

func completeTransverseRoutes(routes []prompt.ModelRoute, metadata map[string]modelroute.ResolvedRoute) ([]prompt.ModelRoute, map[string]modelroute.ResolvedRoute) {
	byRole := make(map[ir.SemanticID]prompt.ModelRoute, len(routes))
	for _, route := range routes {
		byRole[route.Role] = route
	}
	for _, inheritance := range []struct {
		role   ir.SemanticID
		parent ir.SemanticID
	}{
		{role: "role/orchestrator", parent: "role/bootstrap"},
		{role: "role/debate", parent: "role/orchestrator"},
		{role: "role/parallel-dispatch", parent: "role/orchestrator"},
	} {
		if _, exists := byRole[inheritance.role]; exists {
			continue
		}
		parent, exists := byRole[inheritance.parent]
		if !exists {
			continue
		}
		inherited := parent
		inherited.Role = inheritance.role
		routes = append(routes, inherited)
		byRole[inheritance.role] = inherited
		metadata[string(inheritance.role)] = inherited
	}
	return routes, metadata
}

func PrepareWorkflow(ctx context.Context, request WorkflowRequest) (PreparedWorkflowInstall, error) {
	if strings.TrimSpace(request.HomeDir) == "" {
		return PreparedWorkflowInstall{}, errors.New("workflow install home directory is required")
	}
	if len(request.Adapters) == 0 {
		return PreparedWorkflowInstall{}, errors.New("workflow install requires at least one target adapter")
	}
	modelRoutes, routeMetadata, err := resolveWorkflowRoutes(ctx, request)
	if err != nil {
		return PreparedWorkflowInstall{}, fmt.Errorf("resolve workflow model routes: %w", err)
	}
	evaluationTime := request.EvaluationTime
	resolutionTime := evaluationTime
	if evaluationTime.IsZero() {
		evaluationTime = time.Unix(0, 0).UTC()
		resolutionTime = time.Now().UTC()
	}
	snapshot, err := probeForgeSpec(ctx, request.ForgeSpecEndpoint)
	if err != nil {
		return PreparedWorkflowInstall{}, err
	}
	resolution := forgespeccomp.ResolveCapabilities(snapshot, request.ForgeSpecRequirements, resolutionTime)
	capabilityDigest, err := snapshot.Digest()
	if err != nil {
		return PreparedWorkflowInstall{}, fmt.Errorf("digest ForgeSpec capability snapshot: %w", err)
	}
	profileDecision := resolveProductionProfile(request.Adapters, request.RequestedProfile, request.ExperimentalOptIns, resolutionTime)
	if profileDecision.Disposition == ProfileDispositionBlocked {
		return PreparedWorkflowInstall{}, fmt.Errorf("resolve workflow profile: %s", profileDecision.ReasonID)
	}
	profile := string(profileDecision.Effective)
	profileDegradations := profileDecision.Degradations
	profileReason := profileDecision.ReasonID
	degradations := append(workflowDegradations(resolution), profileDegradations...)
	qualityPolicy := productionQualityPolicy()
	qualitySignals := quality.ChangeSignals{
		ChangeName: "prepare-workflow", Kind: quality.ChangeBehavior, ObservableBehavior: true,
		Risk: quality.RiskMedium, Reversibility: quality.ReversibilityDifficult,
		TrustBoundary: quality.TrustBoundaryInternal, DependencyBreadth: quality.DependencyCrossDomain,
		MigrationImpact: quality.MigrationNone,
	}
	qualityPlan, qualityTrace, err := quality.BuildPlan(quality.PipelineInput{
		Policy: qualityPolicy, Profile: quality.ProfilePlan{ProfileID: profile, Degradations: degradations}, Signals: qualitySignals,
		Evaluation: quality.EvaluationInput{Change: quality.ChangeContext{
			Kind: qualitySignals.Kind, ObservableBehavior: qualitySignals.ObservableBehavior, Risk: qualitySignals.Risk,
			Reversibility: qualitySignals.Reversibility, TrustBoundary: qualitySignals.TrustBoundary,
			DependencyBreadth: qualitySignals.DependencyBreadth, MigrationImpact: qualitySignals.MigrationImpact,
		}},
	})
	if err != nil {
		return PreparedWorkflowInstall{}, fmt.Errorf("build production quality plan: %w", err)
	}
	operationalCatalog, err := operationalassets.BuildOperationalCatalog()
	if err != nil {
		return PreparedWorkflowInstall{}, fmt.Errorf("materialize operational asset catalog: %w", err)
	}
	metadata := WorkflowMetadata{
		ContractFingerprint: capabilityDigest,
		ProfileRequested:    string(profileDecision.Requested),
		ProfileEffective:    string(profileDecision.Effective),
		QualityPlanID:       qualityTrace.PlanSHA256,
		ProfileReasonID:     profileReason,
		TrustEvidence:       []string{"forgespec://capabilities/" + capabilityDigest},
		Permissions:         []string{"workflow/read", "workflow/write-managed"},
		HumanGate:           fmt.Sprintf("approval-required:%t", request.ForgeSpecRequirements.RequireApproval),
		Observability:       "workflow.prepare/compile/materialize/install",
		Routes:              routeMetadata,
	}
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return PreparedWorkflowInstall{}, fmt.Errorf("marshal workflow metadata: %w", err)
	}

	workflowFactory := canonical.NewFactory()
	combined := renderers.Bundle{Assets: []renderers.Asset{}}
	bundles := make([]TargetBundle, 0, len(request.Adapters))
	fingerprints := make([]string, 0, len(request.Adapters))
	for _, adapter := range request.Adapters {
		target, err := workflowTarget(adapter.Agent())
		if err != nil {
			return PreparedWorkflowInstall{}, err
		}
		product, err := workflowFactory.Create(canonical.FactoryInput{
			Target: target, RuntimeVersion: ir.MustParseVersion("1.0.0"),
			ForgeSpecMode: manifest.CoordinationMode(resolution.Mode), CapabilitySnapshotSHA256: capabilityDigest,
		})
		if err != nil {
			return PreparedWorkflowInstall{}, err
		}
		workflowDocument, err := json.Marshal(product.Workflow)
		if err != nil {
			return PreparedWorkflowInstall{}, fmt.Errorf("marshal canonical workflow: %w", err)
		}
		facts := []capability.CapabilityFact{}
		if provider, ok := adapter.(agents.CapabilityProvider); ok {
			facts = append(facts, provider.CapabilityFacts()...)
		}
		if _, err := installroots.Resolve(string(target), request.HomeDir, adapter.GlobalConfigDir(request.HomeDir)); err != nil {
			return PreparedWorkflowInstall{}, fmt.Errorf("resolve typed install roots for %s: %w", target, err)
		}
		catalogDocument, err := json.Marshal(capability.Catalog{
			SchemaVersion: capability.CatalogSchema.Current,
			Version:       ir.MustParseVersion("1.0.0"),
			Facts:         facts,
		})
		if err != nil {
			return PreparedWorkflowInstall{}, fmt.Errorf("marshal capability catalog: %w", err)
		}
		adapterContract := workflowPromptContract(request.HomeDir, adapter, target)
		profilePlan := quality.ProfilePlan{ProfileID: profile, Degradations: degradations}
		commonAssets, commonDegradations, err := prompt.Materialize(prompt.MaterializerInput{
			Catalog: operationalCatalog.Catalog, Contents: operationalCatalog.Contents,
			Workflow: product.Workflow, Adapter: adapterContract, Profile: profile,
			Models: modelRoutes, AllowedPermissions: product.AllowedPermissions,
			Metadata: metadataJSON,
		})
		if err != nil {
			return PreparedWorkflowInstall{}, fmt.Errorf("materialize common workflow assets for %s: %w", target, err)
		}
		profilePlan.Degradations = append(profilePlan.Degradations, commonDegradations...)
		compiled, err := compiler.Compile(compiler.Input{
			WorkflowDocument:  workflowDocument,
			CatalogDocument:   catalogDocument,
			Target:            string(target),
			Profile:           profile,
			Configuration:     json.RawMessage(`{}`),
			CompilerVersion:   ir.MustParseVersion("1.0.0"),
			EvaluationTime:    evaluationTime,
			AssetCatalog:      operationalCatalog.Catalog,
			Adapter:           adapterContract,
			ProfilePlan:       profilePlan,
			QualityPolicy:     &qualityPolicy,
			QualitySignals:    &qualitySignals,
			Models:            modelRoutes,
			OperationalAssets: commonAssets,
			Metadata:          metadataJSON,
		})
		if err != nil {
			return PreparedWorkflowInstall{}, fmt.Errorf("compile canonical workflow for %s: %w", target, err)
		}
		compiled.Composition.OperationalAssets = commonAssets
		compiled.Composition.QualityPlan = qualityPlan
		compiled.Normalized.Composition.OperationalAssets = commonAssets
		compiled.Normalized.Composition.QualityPlan = qualityPlan
		capabilityResolutions := nativeResolutions(facts)
		bundle, err := sdd.CompileInjectionBundle(ctx, sdd.BundleCompilationInput{
			Compilation: compiled, Renderer: product.Renderer,
			AllowedAssetKinds: product.AllowedAssetKinds, AllowedPermissions: product.AllowedPermissions,
			ProfileOverride: profile, NativeCapabilities: capabilityIDs(facts), Capabilities: capabilityResolutions,
		})
		if err != nil {
			return PreparedWorkflowInstall{}, fmt.Errorf("compile workflow bundle for %s: %w", target, err)
		}
		configRoot := adapter.GlobalConfigDir(request.HomeDir)
		rebased, err := rebaseWorkflowBundle(request.HomeDir, configRoot, bundle.Bundle)
		if err != nil {
			return PreparedWorkflowInstall{}, fmt.Errorf("rebase workflow bundle for %s: %w", target, err)
		}
		combined.Assets = append(combined.Assets, rebased.Assets...)
		bundles = append(bundles, TargetBundle{Target: target, Profile: string(bundle.Profile), Fingerprint: bundle.Fingerprint, Bundle: rebased})
		fingerprints = append(fingerprints, string(target)+":"+bundle.Fingerprint)
	}
	slices.Sort(fingerprints)
	digest := sha256.Sum256([]byte(strings.Join(fingerprints, "\n")))
	fingerprint := hex.EncodeToString(digest[:])

	managed, err := loadManagedWorkflowAssets(request.HomeDir, combined)
	if err != nil {
		return PreparedWorkflowInstall{}, err
	}
	combined.Metadata = slices.Clone(metadataJSON)
	for index := range bundles {
		bundles[index].Bundle.Metadata = slices.Clone(metadataJSON)
	}
	plan, err := sddinstall.NewPlanner(request.HomeDir).Plan(sddinstall.PlanRequest{
		Bundle: combined, Managed: managed, Profile: profile, Degradations: degradations,
		ForgeSpecMode: string(resolution.Mode), CapabilitySnapshotSHA256: capabilityDigest,
		OwnershipMarkers: true, GeneratorVersion: request.GeneratorVersion, Metadata: metadataJSON,
	})
	if err != nil {
		return PreparedWorkflowInstall{}, fmt.Errorf("plan workflow install: %w", err)
	}
	// Agent Mailbox remains a current component; workflow cutover must not
	// remove a registration installed by the component pipeline.
	retirements := []mcpinject.ConfigRetirement{}
	doctor := productionWorkflowDoctor(profile, resolution, plan, bundles, retirements, request.HomeDir)
	return PreparedWorkflowInstall{
		Plan: plan, Bundles: bundles, Fingerprint: plan.Fingerprint, BundleFingerprint: fingerprint,
		Cutover: resolution, Doctor: doctor, Retirements: retirements, Metadata: metadata.Clone(), root: request.HomeDir,
	}, nil
}

func capabilityIDs(facts []capability.CapabilityFact) []capability.CapabilityID {
	ids := make([]capability.CapabilityID, 0, len(facts))
	for _, fact := range facts {
		ids = append(ids, fact.ID)
	}
	return ids
}

func nativeResolutions(facts []capability.CapabilityFact) []sddresolution.Resolution {
	result := make([]sddresolution.Resolution, 0, len(facts))
	for _, fact := range facts {
		result = append(result, sddresolution.Resolution{
			ID: fact.ID, State: sddresolution.StateNative,
			Evidence:  []sddresolution.EvidenceRef{sddresolution.EvidenceRef("evidence/" + string(fact.ID))},
			Guarantee: sddresolution.GuaranteeEnforced,
			Binding: sddresolution.Binding{
				ID: "binding/" + ir.SemanticID(fact.ID), Kind: sddresolution.BindingNative, CapabilityID: fact.ID,
				Evidence:    []sddresolution.EvidenceRef{sddresolution.EvidenceRef("evidence/" + string(fact.ID))},
				Enforcement: capability.EnforcementRuntime, Guarantee: sddresolution.GuaranteeEnforced,
			},
			Reason: "fresh adapter/runtime evidence",
		})
	}
	return result
}

func resolveProductionProfile(adapters []agents.Adapter, requested sdd.WorkflowProfile, optIns []capability.CapabilityID, now time.Time) ProfileDecision {
	facts := make([]capability.CapabilityFact, 0)
	for _, adapter := range adapters {
		if provider, ok := adapter.(agents.CapabilityProvider); ok {
			facts = append(facts, provider.CapabilityFacts()...)
		}
	}
	return ResolveProfileDecision(ProfileResolutionInput{Requested: requested, Facts: facts, ExperimentalOptIns: optIns, Now: now})
}

func profileRank(profile sdd.WorkflowProfile) int {
	switch profile {
	case sdd.ProfilePortableSequential:
		return 0
	case sdd.ProfilePortableFlat:
		return 1
	case sdd.ProfileNativeAdvanced:
		return 2
	default:
		return -1
	}
}

func workflowPromptContract(homeDir string, adapter agents.Adapter, target renderers.TargetID) prompt.AdapterPromptContract {
	root := adapter.GlobalConfigDir(homeDir)
	if relative, err := filepath.Rel(homeDir, root); err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative) {
		root = filepath.ToSlash(relative)
	}
	expand := func(base, relative string) (string, error) {
		if strings.Contains(relative, "..") {
			return "", fmt.Errorf("unsafe composition path %q", relative)
		}
		return path.Join(base, filepath.ToSlash(relative)), nil
	}
	commands := adapter.CommandsDir(homeDir)
	if commands == "" {
		commands = path.Join(root, "commands")
	} else if relative, err := filepath.Rel(homeDir, commands); err == nil && !filepath.IsAbs(relative) {
		commands = filepath.ToSlash(relative)
	}
	return prompt.AdapterPromptContract{
		Target: target, RootPath: root, SkillRoot: path.Join(root, "skills"), CommandRoot: commands,
		AgentPath:             func(id ir.SemanticID) string { return path.Join(root, "agents", string(id)) },
		SupportsSlashCommands: adapter.SupportsSlashCommands(), NativeSkillOnDemand: target == "opencode",
		NativeModelField: target == "opencode", ExpandPath: expand,
	}
}

func productionQualityPolicy() quality.QualityPolicy {
	return quality.QualityPolicy{
		Version:  "1.0.0",
		TDD:      quality.VerticalTDDPolicy{RequireWhenEligible: false},
		Property: quality.PropertyPolicy{Budget: quality.ActivityBudget{}},
		Fuzz:     quality.FuzzPolicy{Budget: quality.ActivityBudget{}},
		Mutation: quality.MutationPolicy{Mode: quality.MutationOff, Budget: quality.ActivityBudget{}},
	}
}

func (prepared PreparedWorkflowInstall) Apply() (sddinstall.Receipt, error) {
	backupRoot := filepath.Join(prepared.root, ".cortex-ia", "backups")
	receipt, err := sddinstall.NewApplier(prepared.root, backupRoot).Apply(prepared.Plan)
	if receipt.ID == "" {
		return receipt, err
	}
	receipt.CapabilitySnapshot, _ = json.Marshal(prepared.Cutover.Snapshot)
	if len(receipt.Metadata) == 0 {
		receipt.Metadata, _ = json.Marshal(prepared.Metadata)
	}
	receipt.PreDoctor, _ = json.Marshal(prepared.Doctor)
	receipt.PostDoctor, _ = json.Marshal(prepared.Doctor)
	for _, retirement := range prepared.Retirements {
		decision := "removed"
		if retirement.NoOpReason != "" {
			decision = retirement.NoOpReason
		}
		receipt.Retirements = append(receipt.Retirements, sddinstall.RetirementReceipt{
			SemanticID: retirement.SemanticID, Path: retirement.Selector.Path, Decision: decision,
		})
	}
	receipt = sddinstall.SealReceipt(receipt)
	if saveErr := state.NewWorkflowReceiptStore(prepared.root).Save(receipt); saveErr != nil {
		return receipt, errors.Join(err, fmt.Errorf("persist composed workflow receipt: %w", saveErr))
	}
	return receipt, err
}

func protectedMailboxPaths(homeDir string) []string {
	root := filepath.Join(homeDir, ".agent-mailbox")
	return []string{
		filepath.Join(root, "mailbox.db"), filepath.Join(root, "mailbox.db-wal"), filepath.Join(root, "mailbox.db-shm"),
		filepath.Join(homeDir, "agent-mailbox-mcp"),
	}
}

func probeForgeSpec(ctx context.Context, endpoint string) (forgespeccomp.CapabilitySnapshot, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		endpoint = strings.TrimSpace(os.Getenv("CORTEX_IA_FORGESPEC_CAPABILITIES_URL"))
	}
	if endpoint != "" {
		return mcpprobe.NewClient(endpoint, nil).ProbeForgeSpec(ctx)
	}
	return forgespeccomp.CapabilitySnapshot{
		SchemaVersion: ir.MustParseVersion("1.0.0"), ServerVersion: ir.MustParseVersion("1.2.0"),
		ProbeStatus: forgespeccomp.ProbeUnavailable,
	}, nil
}

func workflowDegradations(resolution forgespeccomp.ForgeSpecResolution) []string {
	result := make([]string, 0, len(resolution.Degradations)+len(resolution.UnsupportedGuarantees))
	for _, item := range resolution.Degradations {
		result = append(result, fmt.Sprintf("%s: %s", item.CapabilityID, item.Reason))
	}
	for _, guarantee := range resolution.UnsupportedGuarantees {
		result = append(result, "unsupported guarantee: "+guarantee)
	}
	return result
}

func rebaseWorkflowBundle(homeDir, configDir string, bundle renderers.Bundle) (renderers.Bundle, error) {
	root, err := filepath.Abs(homeDir)
	if err != nil {
		return renderers.Bundle{}, err
	}
	config, err := filepath.Abs(configDir)
	if err != nil {
		return renderers.Bundle{}, err
	}
	prefix, err := filepath.Rel(root, config)
	if err != nil || prefix == ".." || strings.HasPrefix(prefix, ".."+string(filepath.Separator)) || filepath.IsAbs(prefix) {
		return renderers.Bundle{}, fmt.Errorf("adapter config root %q escapes home %q", configDir, homeDir)
	}
	assets := make([]renderers.Asset, len(bundle.Assets))
	for index, input := range bundle.Assets {
		asset := input
		assetPath := filepath.ToSlash(filepath.Clean(filepath.FromSlash(input.Path)))
		prefixSlash := filepath.ToSlash(filepath.Clean(prefix))
		if assetPath != prefixSlash && !strings.HasPrefix(assetPath, prefixSlash+"/") {
			assetPath = filepath.ToSlash(filepath.Join(prefix, filepath.FromSlash(assetPath)))
		}
		asset.Path = assetPath
		assets[index] = asset
	}
	return renderers.Bundle{Assets: assets, Metadata: slices.Clone(bundle.Metadata)}, nil
}

func loadManagedWorkflowAssets(root string, bundle renderers.Bundle) ([]sddinstall.ManagedAsset, error) {
	store := sddinstall.NewOwnershipStore(root)
	managed := make([]sddinstall.ManagedAsset, 0, len(bundle.Assets))
	for _, asset := range bundle.Assets {
		ownership, base, err := store.Read(asset.Path)
		if errors.Is(err, sddinstall.ErrOwnershipNotFound) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("read workflow ownership %q: %w", asset.Path, err)
		}
		managed = append(managed, sddinstall.ManagedAsset{Path: asset.Path, Ownership: ownership, Base: base, Mode: asset.Mode})
	}
	return managed, nil
}

func workflowTarget(agent model.AgentID) (renderers.TargetID, error) {
	switch agent {
	case model.AgentClaudeCode:
		return "claude", nil
	case model.AgentVSCodeCopilot:
		return "vscode", nil
	case model.AgentOpenCode, model.AgentCodex:
		return renderers.TargetID(agent), nil
	default:
		return "", fmt.Errorf("unsupported workflow target adapter %q", agent)
	}
}
