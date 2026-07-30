package pipeline

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/lleontor705/cortex-ia/internal/agents"
	forgespeccomp "github.com/lleontor705/cortex-ia/internal/components/forgespec"
	"github.com/lleontor705/cortex-ia/internal/components/mcpinject"
	"github.com/lleontor705/cortex-ia/internal/components/mcpprobe"
	"github.com/lleontor705/cortex-ia/internal/components/sdd"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/canonical"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/capability"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/compiler"
	sddinstall "github.com/lleontor705/cortex-ia/internal/components/sdd/install"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/manifest"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/renderers"
	"github.com/lleontor705/cortex-ia/internal/model"
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
	root              string
}

func PrepareWorkflow(ctx context.Context, request WorkflowRequest) (PreparedWorkflowInstall, error) {
	if strings.TrimSpace(request.HomeDir) == "" {
		return PreparedWorkflowInstall{}, errors.New("workflow install home directory is required")
	}
	if len(request.Adapters) == 0 {
		return PreparedWorkflowInstall{}, errors.New("workflow install requires at least one target adapter")
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
	profile := string(sdd.ProfilePortableSequential)
	degradations := workflowDegradations(resolution)

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
		catalogDocument, err := json.Marshal(capability.Catalog{
			SchemaVersion: capability.CatalogSchema.Current,
			Version:       ir.MustParseVersion("1.0.0"),
			Facts:         []capability.CapabilityFact{},
		})
		if err != nil {
			return PreparedWorkflowInstall{}, fmt.Errorf("marshal capability catalog: %w", err)
		}
		compiled, err := compiler.Compile(compiler.Input{
			WorkflowDocument: workflowDocument,
			CatalogDocument:  catalogDocument,
			Target:           string(target),
			Profile:          profile,
			Configuration:    json.RawMessage(`{}`),
			CompilerVersion:  ir.MustParseVersion("1.0.0"),
			EvaluationTime:   evaluationTime,
		})
		if err != nil {
			return PreparedWorkflowInstall{}, fmt.Errorf("compile canonical workflow for %s: %w", target, err)
		}
		bundle, err := sdd.CompileInjectionBundle(ctx, sdd.BundleCompilationInput{
			Compilation: compiled, Renderer: product.Renderer,
			AllowedAssetKinds: product.AllowedAssetKinds, AllowedPermissions: product.AllowedPermissions,
		})
		if err != nil {
			return PreparedWorkflowInstall{}, fmt.Errorf("compile workflow bundle for %s: %w", target, err)
		}
		rebased, err := rebaseWorkflowBundle(request.HomeDir, adapter.GlobalConfigDir(request.HomeDir), bundle.Bundle)
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
	plan, err := sddinstall.NewPlanner(request.HomeDir).Plan(sddinstall.PlanRequest{
		Bundle: combined, Managed: managed, Profile: profile, Degradations: degradations,
		ForgeSpecMode: string(resolution.Mode), CapabilitySnapshotSHA256: capabilityDigest,
	})
	if err != nil {
		return PreparedWorkflowInstall{}, fmt.Errorf("plan workflow install: %w", err)
	}
	retirements, err := composeWorkflowRetirements(request.HomeDir, request.Adapters, &plan)
	if err != nil {
		return PreparedWorkflowInstall{}, err
	}
	doctor := productionWorkflowDoctor(profile, resolution, plan, bundles, retirements, request.HomeDir)
	return PreparedWorkflowInstall{
		Plan: plan, Bundles: bundles, Fingerprint: plan.Fingerprint, BundleFingerprint: fingerprint,
		Cutover: resolution, Doctor: doctor, Retirements: retirements, root: request.HomeDir,
	}, nil
}

func (prepared PreparedWorkflowInstall) Apply() (sddinstall.Receipt, error) {
	backupRoot := filepath.Join(prepared.root, ".cortex-ia", "backups")
	receipt, err := sddinstall.NewApplier(prepared.root, backupRoot).Apply(prepared.Plan)
	if receipt.ID == "" {
		return receipt, err
	}
	receipt.CapabilitySnapshot, _ = json.Marshal(prepared.Cutover.Snapshot)
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

func composeWorkflowRetirements(homeDir string, adapters []agents.Adapter, plan *sddinstall.Plan) ([]mcpinject.ConfigRetirement, error) {
	retirements := make([]mcpinject.ConfigRetirement, 0, len(adapters))
	for _, adapter := range adapters {
		path := legacyMailboxConfigPath(homeDir, adapter)
		var content []byte
		mode := os.FileMode(0o600)
		if path != "" {
			info, statErr := os.Stat(path)
			switch {
			case statErr == nil:
				mode = info.Mode().Perm()
				content, statErr = os.ReadFile(path)
				if statErr != nil {
					return nil, fmt.Errorf("read legacy registration target %q: %w", path, statErr)
				}
			case !errors.Is(statErr, os.ErrNotExist):
				return nil, fmt.Errorf("inspect legacy registration target %q: %w", path, statErr)
			}
		}
		if len(content) == 0 {
			retirements = append(retirements, mcpinject.ConfigRetirement{
				SemanticID: "retirement/agent-mailbox-registration",
				Selector:   mcpinject.ConfigSelector{Strategy: adapter.MCPStrategy(), Path: path},
				NoOpReason: "legacy metadata exists without a managed registration",
			})
			continue
		}
		var lock state.Lockfile
		if strings.Contains(string(content), "agent-mailbox") {
			var lockErr error
			lock, lockErr = state.LoadLock(homeDir)
			if lockErr != nil && !errors.Is(lockErr, os.ErrNotExist) {
				return nil, fmt.Errorf("load legacy workflow lock: %w", lockErr)
			}
		}
		retirement, retireErr := mcpinject.PlanRetirement(homeDir, adapter, string(content), legacyRetirementEvidence(lock, path, content))
		if retireErr != nil {
			return nil, fmt.Errorf("plan %s workflow retirement: %w", adapter.Agent(), retireErr)
		}
		retirements = append(retirements, retirement)
		if retirement.NoOpReason != "" || string(retirement.Before) == string(retirement.After) {
			continue
		}
		relative, relErr := filepath.Rel(homeDir, retirement.Selector.Path)
		if relErr != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
			return nil, fmt.Errorf("legacy retirement target %q escapes home", retirement.Selector.Path)
		}
		relative = filepath.ToSlash(relative)
		effect := sddinstall.Effect{
			Path: relative, SemanticID: retirement.SemanticID, BeforeSHA256: sddinstall.SHA256(retirement.Before),
			BeforeMode: mode, AfterMode: mode, Content: slices.Clone(retirement.After),
		}
		if retirement.Delete {
			plan.Deletes = append(plan.Deletes, effect)
		} else {
			effect.AfterSHA256 = sddinstall.SHA256(retirement.After)
			plan.Updates = append(plan.Updates, effect)
		}
		plan.Backup.Paths = append(plan.Backup.Paths, relative)
	}
	slices.SortFunc(plan.Updates, func(left, right sddinstall.Effect) int { return strings.Compare(left.Path, right.Path) })
	slices.SortFunc(plan.Deletes, func(left, right sddinstall.Effect) int { return strings.Compare(left.Path, right.Path) })
	slices.Sort(plan.Backup.Paths)
	plan.Backup.Paths = slices.Compact(plan.Backup.Paths)
	plan.Backup.Required = len(plan.Backup.Paths) != 0
	plan.ProtectedPaths = protectedMailboxPaths(homeDir)
	plan.Fingerprint = sddinstall.FingerprintPlan(*plan)
	return retirements, nil
}

func legacyMailboxConfigPath(homeDir string, adapter agents.Adapter) string {
	switch adapter.MCPStrategy() {
	case model.StrategySeparateMCPFiles, model.StrategyMCPConfigFile:
		return adapter.MCPConfigPath(homeDir, "agent-mailbox")
	case model.StrategyMergeIntoSettings, model.StrategyTOMLFile:
		return adapter.SettingsPath(homeDir)
	default:
		return ""
	}
}

func legacyRetirementEvidence(lock state.Lockfile, path string, content []byte) []mcpinject.RetirementEvidence {
	if len(content) == 0 || !slices.Contains(lock.Components, model.ComponentMailbox) || !lockOwnsPath(lock.Files, path) {
		return nil
	}
	digest := sddinstall.SHA256(content)
	return []mcpinject.RetirementEvidence{{
		ComponentID: model.ComponentMailbox, Source: "cortex-ia.lock", TemplateSHA256: digest,
		ObservedSHA256: digest, OwnershipSHA256: digest,
	}}
}

func lockOwnsPath(paths []string, target string) bool {
	for _, path := range paths {
		if filepath.Clean(path) == filepath.Clean(target) {
			return true
		}
	}
	return false
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
		asset.Path = filepath.ToSlash(filepath.Join(prefix, filepath.FromSlash(input.Path)))
		assets[index] = asset
	}
	return renderers.Bundle{Assets: assets}, nil
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
	case model.AgentGeminiCLI:
		return "gemini", nil
	case model.AgentVSCodeCopilot:
		return "vscode", nil
	case model.AgentKiroIDE:
		return "kiro", nil
	case model.AgentQwenCode:
		return "qwen", nil
	case model.AgentOpenCode, model.AgentCursor, model.AgentCodex, model.AgentAntigravity,
		model.AgentWindsurf, model.AgentKilocode, model.AgentKimi:
		return renderers.TargetID(agent), nil
	default:
		return "", fmt.Errorf("unsupported workflow target adapter %q", agent)
	}
}
