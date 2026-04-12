package pipeline

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/lleontor705/cortex-ia/internal/agents"
	"github.com/lleontor705/cortex-ia/internal/backup"
	"github.com/lleontor705/cortex-ia/internal/catalog"
	"github.com/lleontor705/cortex-ia/internal/components/context7"
	cortexcomp "github.com/lleontor705/cortex-ia/internal/components/cortex"
	"github.com/lleontor705/cortex-ia/internal/components/conventions"
	forgespeccomp "github.com/lleontor705/cortex-ia/internal/components/forgespec"
	"github.com/lleontor705/cortex-ia/internal/components/mailbox"
	"github.com/lleontor705/cortex-ia/internal/components/persona"
	"github.com/lleontor705/cortex-ia/internal/components/sdd"
	ggacomp "github.com/lleontor705/cortex-ia/internal/components/gga"
	skillscomp "github.com/lleontor705/cortex-ia/internal/components/skills"
	"github.com/lleontor705/cortex-ia/internal/model"
	"github.com/lleontor705/cortex-ia/internal/state"
)

// validBackupID matches safe backup IDs (alphanumeric, hyphens, underscores).
var validBackupID = regexp.MustCompile(`^[a-zA-Z0-9\-_]+$`)

// ProgressFunc is called by the pipeline to report step-level progress.
// Implementations must be safe for concurrent use.
type ProgressFunc func(stepID string, status string, err error)

// InstallResult describes the outcome of a full installation.
type InstallResult struct {
	BackupID       string
	FilesChanged   []string
	ComponentsDone []model.ComponentID
	Errors         []string
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

	// Resolve profile if specified.
	if selection.ProfileName != "" && selection.ModelAssignments == nil {
		profiles, err := state.LoadProfiles(homeDir)
		if err == nil {
			for _, p := range profiles {
				if p.Name == selection.ProfileName {
					selection.ModelAssignments = p.ModelAssignments
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
		for _, inj := range buildInjectors(homeDir, adapter, selection) {
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

	// 7. Translate results.
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

type injectorEntry struct {
	id model.ComponentID
	fn func() ([]string, error)
}

// buildInjectors returns the ordered list of component injectors for an agent.
func buildInjectors(homeDir string, adapter agents.Adapter, selection model.Selection) []injectorEntry {
	return []injectorEntry{
		{model.ComponentCortex, func() ([]string, error) {
			r, err := cortexcomp.Inject(homeDir, adapter)
			return r.Files, err
		}},
		{model.ComponentMailbox, func() ([]string, error) {
			r, err := mailbox.Inject(homeDir, adapter)
			return r.Files, err
		}},
		{model.ComponentForgeSpec, func() ([]string, error) {
			r, err := forgespeccomp.Inject(homeDir, adapter)
			return r.Files, err
		}},
		{model.ComponentContext7, func() ([]string, error) {
			r, err := context7.Inject(homeDir, adapter)
			return r.Files, err
		}},
		{model.ComponentConventions, func() ([]string, error) {
			r, err := conventions.Inject(homeDir, adapter)
			return r.Files, err
		}},
		{model.ComponentSDD, func() ([]string, error) {
			r, err := sdd.Inject(homeDir, adapter, selection.ModelAssignments, selection.StrictTDD)
			return r.Files, err
		}},
		{model.ComponentSkills, func() ([]string, error) {
			r, err := skillscomp.Inject(homeDir, adapter, selection.CommunitySkills)
			return r.Files, err
		}},
		{model.ComponentGGA, func() ([]string, error) {
			r, err := ggacomp.Inject(homeDir, selection.Agents)
			return r.Files, err
		}},
	}
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
		for _, name := range []string{"cortex", "agent-mailbox", "forgespec", "context7"} {
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
		Agents:     dedupeAgents(lock.InstalledAgents, s.InstalledAgents),
		Preset:     firstNonEmptyPreset(lock.Preset, s.Preset, model.PresetFull),
		Components: dedupeComponents(lock.Components, s.Components),
	}

	if len(selection.Agents) == 0 {
		return model.Selection{}, fmt.Errorf("no cortex-ia installation metadata found")
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
