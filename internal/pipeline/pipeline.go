package pipeline

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/lleontor705/cortex-ia/internal/agents"
	"github.com/lleontor705/cortex-ia/internal/backup"
	"github.com/lleontor705/cortex-ia/internal/catalog"
	"github.com/lleontor705/cortex-ia/internal/components/context7"
	"github.com/lleontor705/cortex-ia/internal/components/conventions"
	cortexcomp "github.com/lleontor705/cortex-ia/internal/components/cortex"
	forgespeccomp "github.com/lleontor705/cortex-ia/internal/components/forgespec"
	"github.com/lleontor705/cortex-ia/internal/components/mailbox"
	"github.com/lleontor705/cortex-ia/internal/components/persona"
	"github.com/lleontor705/cortex-ia/internal/components/sdd"
	sddinstall "github.com/lleontor705/cortex-ia/internal/components/sdd/install"
	skillscomp "github.com/lleontor705/cortex-ia/internal/components/skills"
	"github.com/lleontor705/cortex-ia/internal/model"
	"github.com/lleontor705/cortex-ia/internal/modelroute"
	"github.com/lleontor705/cortex-ia/internal/opencode"
	"github.com/lleontor705/cortex-ia/internal/state"
	"github.com/lleontor705/cortex-ia/internal/verify"
)

// validBackupID matches safe backup IDs (alphanumeric, hyphens, underscores).
var validBackupID = regexp.MustCompile(`^[a-zA-Z0-9\-_]+$`)

// ProgressFunc is called by the pipeline to report step-level progress.
// Implementations must be safe for concurrent use.
type ProgressFunc func(stepID string, status string, err error)

// InstallResult describes the outcome of a full installation.
type InstallResult struct {
	BackupID            string
	FilesChanged        []string
	ComponentsDone      []model.ComponentID
	Errors              []string
	WorkflowFingerprint string
	WorkflowPlan        sddinstall.Plan
	WorkflowDoctor      verify.DoctorReport
	WorkflowReceipt     sddinstall.Receipt
	WorkflowCutover     forgespeccomp.ForgeSpecResolution
	WorkflowRollback    bool
}

// Repair reapplies the previously installed configuration from lock/state metadata.
func Repair(homeDir string, registry *agents.Registry, version string, dryRun bool) (InstallResult, error) {
	s, err := state.Load(homeDir)
	if err != nil {
		return InstallResult{}, err
	}
	lock, err := state.LoadLock(homeDir)
	if err != nil {
		return InstallResult{}, err
	}

	selection, err := selectionFromMetadata(s, lock)
	if err != nil {
		return InstallResult{}, err
	}
	selection.DryRun = dryRun

	return Install(homeDir, registry, selection, version, dryRun)
}

// Rollback restores managed files from a previous backup manifest.
func Rollback(homeDir, backupID string) (backup.Manifest, error) {
	if backupID == "" {
		s, err := state.Load(homeDir)
		if err != nil {
			return backup.Manifest{}, err
		}
		lock, err := state.LoadLock(homeDir)
		if err != nil {
			return backup.Manifest{}, err
		}
		backupID = firstNonEmptyString(lock.LastBackupID, s.LastBackupID)
	}

	if backupID == "" {
		return backup.Manifest{}, fmt.Errorf("no backup available for rollback")
	}
	if !validBackupID.MatchString(backupID) {
		return backup.Manifest{}, fmt.Errorf("invalid backup ID format: %q", backupID)
	}

	manifestPath := filepath.Join(homeDir, ".cortex-ia", "backups", backupID, backup.ManifestFilename)
	manifest, err := backup.ReadManifest(manifestPath)
	if err != nil {
		return backup.Manifest{}, err
	}

	restore := backup.RestoreService{}
	if err := restore.Restore(manifest); err != nil {
		return backup.Manifest{}, err
	}

	return manifest, nil
}

// Install runs the full installation pipeline using a 2-stage orchestrator:
// Stage 1 (Prepare): validate agents + create backup (stops on error, rolls back)
// Stage 2 (Apply): inject components per agent + save state (continues on error)
func Install(homeDir string, registry *agents.Registry, selection model.Selection, version string, dryRun bool, onProgress ...ProgressFunc) (InstallResult, error) {
	var progress ProgressFunc
	if len(onProgress) > 0 {
		progress = onProgress[0]
	}

	result := InstallResult{}
	if err := selection.ValidateCurrent(); err != nil {
		return result, err
	}

	// Resolve profile if specified.
	if selection.ProfileName != "" && selection.ModelAssignments == nil {
		profiles, err := state.LoadProfiles(homeDir)
		if err == nil {
			for _, p := range profiles {
				if p.Name == selection.ProfileName {
					selection.ModelAssignments = p.ModelAssignments
					if len(selection.ModelAssignments) == 0 && len(p.ConfiguredAssignments) > 0 {
						selection.ModelAssignments = make(model.ModelAssignments, len(p.ConfiguredAssignments))
						for phase, assignment := range p.ConfiguredAssignments {
							selection.ModelAssignments[phase] = assignment.FormatOpenCodeModel()
						}
					}
					break
				}
			}
		}
	}

	// 1. Resolve components with dependencies.
	components := selection.Components
	if len(components) == 0 {
		components = catalog.ComponentsForPreset(selection.Preset)
	}
	resolved := catalog.ResolveDeps(components)
	var preparedWorkflow PreparedWorkflowInstall
	if slicesContainsComponent(resolved, model.ComponentSDD) {
		adapters := make([]agents.Adapter, 0, len(selection.Agents))
		for _, agentID := range selection.Agents {
			adapter, getErr := registry.Get(agentID)
			if getErr != nil {
				return result, getErr
			}
			adapters = append(adapters, adapter)
		}
		routeInput, routeErr := explicitWorkflowRoutes(homeDir, selection)
		if routeErr != nil {
			return result, routeErr
		}
		if selection.ProfileName == "" && !dryRun {
			selection.ProfileName = "active"
			if err := persistActiveWorkflowProfile(homeDir, selection); err != nil {
				return result, fmt.Errorf("persist explicit workflow route profile: %w", err)
			}
		}
		var prepareErr error
		preparedWorkflow, prepareErr = PrepareWorkflow(context.Background(), WorkflowRequest{
			HomeDir: homeDir, Adapters: adapters, GeneratorVersion: version, RouteResolution: routeInput,
		})
		if prepareErr != nil {
			return result, prepareErr
		}
		result.WorkflowFingerprint = preparedWorkflow.Fingerprint
		result.WorkflowPlan = preparedWorkflow.Plan
		result.WorkflowDoctor = preparedWorkflow.Doctor
		result.WorkflowCutover = preparedWorkflow.Cutover
	}

	if dryRun {
		if progress == nil {
			fmt.Println("=== DRY RUN ===")
			fmt.Printf("Agents: %v\n", selection.Agents)
			fmt.Printf("Preset: %s\n", selection.Preset)
			fmt.Printf("Components (resolved): %v\n", resolved)
			fmt.Println("No changes will be made.")
		}
		result.ComponentsDone = resolved
		return result, nil
	}

	// 2. Ensure ~/.cortex-ia/ base directory exists before any component runs.
	if err := state.EnsureDir(homeDir); err != nil {
		return result, fmt.Errorf("ensure cortex-ia directory: %w", err)
	}

	// 3. Build prepare steps.
	bkStep := &backupStep{
		homeDir: homeDir, registry: registry,
		agentIDs: selection.Agents, resolved: resolved, version: version,
		progress: progress,
	}
	prepareSteps := []Step{
		&validateStep{registry: registry, agentIDs: selection.Agents},
		bkStep,
	}

	// 4. Build apply steps: one sequential chain per agent, agents run in parallel.
	componentSet := make(map[model.ComponentID]bool)
	for _, c := range resolved {
		componentSet[c] = true
	}

	var allComponentSteps []*componentStep
	var workflowOnce sync.Once
	var workflowFiles []string
	var workflowErr error
	applyWorkflow := func() ([]string, error) {
		workflowOnce.Do(func() {
			receipt, applyErr := preparedWorkflow.Apply()
			result.WorkflowReceipt = receipt
			result.WorkflowRollback = receipt.RestoreAvailable && receipt.BackupVerified
			workflowFiles = append(workflowFiles, receipt.Applied...)
			workflowErr = applyErr
		})
		return workflowFiles, workflowErr
	}

	// Build one sequential step chain per agent. Each chain applies
	// components in dependency order for that agent.
	var agentChains [][]Step
	for _, agentID := range selection.Agents {
		adapter, err := registry.Get(agentID)
		if err != nil {
			continue // validateStep already catches this
		}

		if progress == nil {
			fmt.Printf("\nConfiguring %s...\n", agentID)
		}
		var chain []Step
		for _, inj := range buildInjectors(homeDir, adapter, selection, applyWorkflow, componentSet[model.ComponentSDD]) {
			if !componentSet[inj.id] {
				continue
			}
			cs := &componentStep{
				homeDir: homeDir, adapter: adapter,
				componentID: inj.id, injectorFn: inj.fn,
				progress: progress,
			}
			chain = append(chain, cs)
			allComponentSteps = append(allComponentSteps, cs)
		}
		if len(chain) > 0 {
			agentChains = append(agentChains, chain)
		}
	}

	// 5. Run 2-stage: prepare sequentially, then agents in parallel.
	// Within each agent, components run sequentially (same config files).
	// Different agents run in parallel (different config dirs).
	prepResult := RunStage(prepareSteps)
	if prepResult.Error != nil {
		result.BackupID = bkStep.BackupID
		result.ComponentsDone = resolved
		return result, prepResult.Error
	}

	// 5a. Mark installation as in-progress (after backup succeeds).
	// If the process crashes or components fail, the marker stays so
	// that "cortex-ia doctor" can detect the incomplete install.
	statusStep := &installStatusStep{homeDir: homeDir, backupID: bkStep.BackupID}
	if err := statusStep.Run(); err != nil {
		// Non-fatal: warn but continue — the install itself is more important.
		result.Errors = append(result.Errors, fmt.Sprintf("install status marker: %v", err))
	}

	applyResult := RunParallelChains(agentChains)

	// 6. Inject persona for each agent (non-component injection).
	if selection.Persona != "" {
		for _, agentID := range selection.Agents {
			adapter, err := registry.Get(agentID)
			if err != nil {
				continue
			}
			pResult, pErr := persona.Inject(homeDir, adapter, selection.Persona)
			if pErr != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("persona/%s: %v", agentID, pErr))
				continue
			}
			result.FilesChanged = append(result.FilesChanged, pResult.Files...)
		}
	}

	// 7a. Auto-apply model assignments to the effective OpenCode config so per-agent model
	// routing lands without requiring a separate `profiles apply` call.
	// Only runs when (a) model assignments exist, (b) OpenCode is in the
	// selected agents, and (c) Apply succeeded.
	if applyResult.Error == nil && len(selection.ModelAssignments) > 0 {
		hasOpenCode := false
		for _, id := range selection.Agents {
			if id == model.AgentOpenCode {
				hasOpenCode = true
				break
			}
		}
		if hasOpenCode {
			configName := filepath.Base(opencode.GlobalConfigPath(homeDir))
			configured := make(model.OpenCodeModelAssignments, len(selection.ModelAssignments))
			for phase, value := range selection.ModelAssignments {
				provider, modelID, ok := strings.Cut(string(value), "/")
				if ok && provider != "" && modelID != "" {
					configured[phase] = model.OpenCodeModelAssignment{Provider: provider, Model: modelID}
				}
			}
			ocAssignments := sdd.ProfileToOpenCodeAssignments(model.Profile{
				Name:                  firstNonEmptyString(selection.ProfileName, "active"),
				ConfiguredAssignments: configured,
			})
			if len(ocAssignments) > 0 {
				receipt, applyErr := opencode.ApplyToOpenCodeConfig(homeDir, ocAssignments)
				if applyErr != nil {
					result.BackupID = bkStep.BackupID
					result.ComponentsDone = resolved
					for _, cs := range allComponentSteps {
						result.FilesChanged = append(result.FilesChanged, cs.Files...)
					}
					result.Errors = append(result.Errors, fmt.Sprintf("apply model assignments to %s: %v", configName, applyErr))
					return result, fmt.Errorf("apply model assignments to %s: %w", configName, applyErr)
				}
				result.FilesChanged = append(result.FilesChanged, receipt.ConfigPath)
			}
		}
	}

	// 7b. Translate results.
	result.BackupID = bkStep.BackupID
	result.ComponentsDone = resolved
	for _, cs := range allComponentSteps {
		result.FilesChanged = append(result.FilesChanged, cs.Files...)
	}

	if applyResult.Error != nil {
		// Leave install-status as "in-progress" so doctor can detect the failure.
		if applyResult.Failed != "" {
			result.Errors = append(result.Errors, applyResult.Failed)
		}
		return result, fmt.Errorf("installation completed with errors")
	}

	// 8. Save state (after successful apply).
	s := state.State{
		InstalledAgents: selection.Agents,
		Preset:          selection.Preset,
		Components:      resolved,
		LastInstall:     time.Now(),
		LastBackupID:    result.BackupID,
		Version:         version,
		LastProfile:     selection.ProfileName,
		StrictTDD:       selection.StrictTDD,
	}
	if err := state.Save(homeDir, s); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("save state: %v", err))
	}

	lock := state.Lockfile{
		InstalledAgents: selection.Agents,
		Preset:          selection.Preset,
		Components:      resolved,
		Files:           dedupeStrings(result.FilesChanged),
		GeneratedAt:     time.Now(),
		LastBackupID:    result.BackupID,
		Version:         version,
	}
	if err := state.SaveLock(homeDir, lock); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("save lock: %v", err))
	}

	// 9. Clear the in-progress marker — installation succeeded.
	if err := state.ClearInstallStatus(homeDir); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("clear install status: %v", err))
	}

	if len(result.Errors) > 0 {
		return result, fmt.Errorf("installation completed with %d warning(s)", len(result.Errors))
	}

	return result, nil
}

func persistActiveWorkflowProfile(homeDir string, selection model.Selection) error {
	assignments := make(model.OpenCodeModelAssignments, len(selection.ModelAssignments))
	for phase, value := range selection.ModelAssignments {
		provider, modelID, ok := strings.Cut(string(value), "/")
		if !ok || provider == "" || modelID == "" {
			continue
		}
		assignments[phase] = model.OpenCodeModelAssignment{Provider: provider, Model: modelID}
	}
	if len(assignments) == 0 {
		return fmt.Errorf("explicit provider/model assignments are required")
	}
	return state.SaveProfiles(homeDir, []model.Profile{{Name: "active", ConfiguredAssignments: assignments}})
}

// explicitWorkflowRoutes converts only caller/profile configuration into the
// resolver input. It never supplies a provider, model, or fallback itself.
func explicitWorkflowRoutes(homeDir string, selection model.Selection) (modelroute.ResolverInput, error) {
	requests := map[string]modelroute.RouteRequest{}
	assignments := map[string]model.OpenCodeModelAssignment{}
	if selection.ProfileName != "" {
		profiles, err := state.LoadProfiles(homeDir)
		if err != nil {
			return modelroute.ResolverInput{}, err
		}
		profile, found := sdd.FindProfile(profiles, selection.ProfileName)
		if !found {
			return modelroute.ResolverInput{}, fmt.Errorf("workflow profile %q is not configured", selection.ProfileName)
		}
		for phase, route := range profile.Routes {
			requests[canonicalRouteName(phase)] = route
		}
		for phase, assignment := range profile.ConfiguredAssignments {
			phase = canonicalRouteName(phase)
			assignments[phase] = assignment
			if _, exists := requests[phase]; !exists {
				route, routeErr := modelroute.NewRouteID("route/v1/" + phase)
				if routeErr != nil {
					return modelroute.ResolverInput{}, routeErr
				}
				requests[phase] = modelroute.RouteRequest{RouteID: route}
			}
		}
	}
	for phase, value := range selection.ModelAssignments {
		phase = canonicalRouteName(phase)
		text := string(value)
		if route, err := modelroute.NewRouteID(text); err == nil {
			requests[phase] = modelroute.RouteRequest{RouteID: route}
			continue
		}
		provider, modelID, ok := strings.Cut(text, "/")
		if !ok || provider == "" || modelID == "" {
			return modelroute.ResolverInput{}, fmt.Errorf("workflow assignment %q has no explicit route or provider/model configuration", phase)
		}
		assignments[phase] = model.OpenCodeModelAssignment{Provider: provider, Model: modelID}
		if _, exists := requests[phase]; !exists {
			semantic, err := modelroute.NewRouteID("route/v1/" + strings.TrimPrefix(strings.ToLower(phase), "sdd-"))
			if err != nil {
				return modelroute.ResolverInput{}, fmt.Errorf("derive semantic route for %q: %w", phase, err)
			}
			requests[phase] = modelroute.RouteRequest{RouteID: semantic}
		}
	}
	if len(requests) == 0 {
		return modelroute.ResolverInput{}, fmt.Errorf("explicit workflow ModelRoutes configuration is required")
	}
	providers := map[modelroute.ProviderID]modelroute.ProviderConfig{}
	now := time.Now().UTC()
	for phase, request := range requests {
		assignment, ok := assignments[phase]
		if !ok {
			continue
		}
		provider := modelroute.ProviderID(assignment.Provider)
		config := providers[provider]
		config.Provider = provider
		if config.Routes == nil {
			config.Routes = map[modelroute.RouteID]modelroute.RouteRef{}
		}
		config.Routes[request.RouteID] = modelroute.RouteRef{Provider: provider, Model: modelroute.ModelID(assignment.Model)}
		digest := sha256.Sum256([]byte(string(provider) + "/" + assignment.Model + "|" + string(request.RouteID)))
		config.Evidence = append(config.Evidence, modelroute.ResolutionEvidence{ID: fmt.Sprintf("user-config:%s", phase), Source: modelroute.SourceUserConfig, Provider: provider, Route: request.RouteID, ObservedAt: now, FreshUntil: now.Add(time.Hour), Digest: fmt.Sprintf("%x", digest), Qualified: true, ReasonID: "route.configured"})
		providers[provider] = config
	}
	providerConfigs := make([]modelroute.ProviderConfig, 0, len(providers))
	for _, config := range providers {
		providerConfigs = append(providerConfigs, config)
	}
	return modelroute.ResolverInput{Requests: requests, ProviderConfigs: providerConfigs, Now: now}, nil
}

func canonicalRouteName(phase string) string {
	switch strings.TrimSpace(strings.ToLower(phase)) {
	case "sdd-init", "init", "bootstrap":
		return "bootstrap"
	case "orchestrator":
		return "orchestrator"
	case "sdd-explore", "explore", "investigate":
		return "investigate"
	case "sdd-propose", "propose", "draft-proposal":
		return "draft-proposal"
	case "sdd-spec", "spec", "write-specs":
		return "write-specs"
	case "sdd-design", "design", "architect":
		return "architect"
	case "sdd-tasks", "tasks", "decompose":
		return "decompose"
	case "sdd-apply", "apply", "implement":
		return "implement"
	case "sdd-verify", "verify", "validate":
		return "validate"
	case "sdd-archive", "archive", "finalize":
		return "finalize"
	default:
		return phase
	}
}

type injectorEntry struct {
	id model.ComponentID
	fn func() ([]string, error)
}

// buildInjectors returns the ordered list of component injectors for an agent.
func buildInjectors(homeDir string, adapter agents.Adapter, selection model.Selection, applyWorkflow func() ([]string, error), workflowOwnership ...bool) []injectorEntry {
	workflowOwnsInstructions := len(workflowOwnership) != 0 && workflowOwnership[0]
	entries := []injectorEntry{
		{model.ComponentCortex, func() ([]string, error) {
			r, err := cortexcomp.Inject(homeDir, adapter)
			return r.Files, err
		}},
		{model.ComponentForgeSpec, func() ([]string, error) {
			r, err := forgespeccomp.Inject(homeDir, adapter)
			return r.Files, err
		}},
		{model.ComponentMailbox, func() ([]string, error) {
			r, err := mailbox.Inject(homeDir, adapter)
			return r.Files, err
		}},
		{model.ComponentContext7, func() ([]string, error) {
			r, err := context7.Inject(homeDir, adapter)
			return r.Files, err
		}},
		{model.ComponentSDD, func() ([]string, error) {
			return applyWorkflow()
		}},
		{model.ComponentSkills, func() ([]string, error) {
			r, err := skillscomp.Inject(homeDir, adapter, selection.CommunitySkills)
			return r.Files, err
		}},
	}
	if !workflowOwnsInstructions {
		entries = append(entries, injectorEntry{model.ComponentConventions, func() ([]string, error) {
			r, err := conventions.Inject(homeDir, adapter)
			return r.Files, err
		}})
	}
	return entries
}

func slicesContainsComponent(components []model.ComponentID, target model.ComponentID) bool {
	for _, component := range components {
		if component == target {
			return true
		}
	}
	return false
}

func collectBackupPaths(homeDir string, registry *agents.Registry, agentIDs []model.AgentID, components []model.ComponentID) []string {
	var paths []string
	componentSet := make(map[model.ComponentID]bool)
	for _, c := range components {
		componentSet[c] = true
	}

	for _, agentID := range agentIDs {
		adapter, err := registry.Get(agentID)
		if err != nil {
			continue
		}

		// System prompt file.
		if f := adapter.SystemPromptFile(homeDir); f != "" {
			paths = append(paths, f)
		}
		// Settings file.
		if f := adapter.SettingsPath(homeDir); f != "" {
			paths = append(paths, f)
		}
		// MCP config files.
		for _, name := range []string{"cortex", "forgespec", "context7"} {
			if f := adapter.MCPConfigPath(homeDir, name); f != "" {
				if _, err := os.Stat(f); err == nil {
					paths = append(paths, f)
				}
			}
		}
		// SDD files.
		if componentSet[model.ComponentSDD] {
			paths = append(paths, sdd.FilesToBackup(homeDir, adapter)...)
		}
	}

	return paths
}

func dedupeStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

// SelectionFromState reconstructs a Selection from persisted state/lock metadata.
func SelectionFromState(s state.State, lock state.Lockfile) (model.Selection, error) {
	return selectionFromMetadata(s, lock)
}

func selectionFromMetadata(s state.State, lock state.Lockfile) (model.Selection, error) {
	selection := model.Selection{
		Agents:      dedupeAgents(lock.InstalledAgents, s.InstalledAgents),
		Preset:      firstNonEmptyPreset(lock.Preset, s.Preset, model.PresetFull),
		Components:  dedupeComponents(lock.Components, s.Components),
		ProfileName: s.LastProfile,
	}

	if len(selection.Agents) == 0 {
		return model.Selection{}, fmt.Errorf("no cortex-ia installation metadata found")
	}
	if err := selection.ValidateCurrent(); err != nil {
		return model.Selection{}, fmt.Errorf("repair selection: %w", err)
	}

	return selection, nil
}

func dedupeAgents(groups ...[]model.AgentID) []model.AgentID {
	seen := make(map[model.AgentID]struct{})
	result := make([]model.AgentID, 0)
	for _, group := range groups {
		for _, value := range group {
			if value == "" {
				continue
			}
			if _, ok := seen[value]; ok {
				continue
			}
			seen[value] = struct{}{}
			result = append(result, value)
		}
	}
	return result
}

func dedupeComponents(groups ...[]model.ComponentID) []model.ComponentID {
	seen := make(map[model.ComponentID]struct{})
	result := make([]model.ComponentID, 0)
	for _, group := range groups {
		for _, value := range group {
			if value == "" {
				continue
			}
			if _, ok := seen[value]; ok {
				continue
			}
			seen[value] = struct{}{}
			result = append(result, value)
		}
	}
	return result
}

func firstNonEmptyPreset(values ...model.PresetID) model.PresetID {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return model.PresetFull
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
