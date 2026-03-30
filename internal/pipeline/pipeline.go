package pipeline

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/lleontor705/cortex-ia/internal/agents"
	"github.com/lleontor705/cortex-ia/internal/backup"
	"github.com/lleontor705/cortex-ia/internal/catalog"
	"github.com/lleontor705/cortex-ia/internal/components/context7"
	cortexcomp "github.com/lleontor705/cortex-ia/internal/components/cortex"
	"github.com/lleontor705/cortex-ia/internal/components/conventions"
	forgespeccomp "github.com/lleontor705/cortex-ia/internal/components/forgespec"
	"github.com/lleontor705/cortex-ia/internal/components/mailbox"
	"github.com/lleontor705/cortex-ia/internal/components/orchestrator"
	"github.com/lleontor705/cortex-ia/internal/components/sdd"
	"github.com/lleontor705/cortex-ia/internal/model"
	"github.com/lleontor705/cortex-ia/internal/state"
)

// InstallResult describes the outcome of a full installation.
type InstallResult struct {
	BackupID       string
	FilesChanged   []string
	ComponentsDone []model.ComponentID
	Errors         []string
}

// Install runs the full installation pipeline for the given selection.
func Install(homeDir string, registry *agents.Registry, selection model.Selection, version string, dryRun bool) (InstallResult, error) {
	result := InstallResult{}

	// 1. Resolve components with dependencies.
	components := selection.Components
	if len(components) == 0 {
		components = catalog.ComponentsForPreset(selection.Preset)
	}
	resolved := catalog.ResolveDeps(components)

	if dryRun {
		fmt.Println("=== DRY RUN ===")
		fmt.Printf("Agents: %v\n", selection.Agents)
		fmt.Printf("Preset: %s\n", selection.Preset)
		fmt.Printf("Components (resolved): %v\n", resolved)
		fmt.Println("No changes will be made.")
		result.ComponentsDone = resolved
		return result, nil
	}

	// 2. Create backup.
	backupDir := filepath.Join(homeDir, ".cortex-ia", "backups", time.Now().Format("20060102-150405"))
	snap := backup.NewSnapshotter()

	backupPaths := collectBackupPaths(homeDir, registry, selection.Agents, resolved)
	manifest, err := snap.Create(backupDir, backupPaths)
	if err != nil {
		return result, fmt.Errorf("backup: %w", err)
	}
	manifest.Source = backup.BackupSourceInstall
	manifest.CreatedByVersion = version
	_ = backup.WriteManifest(filepath.Join(backupDir, backup.ManifestFilename), manifest)
	result.BackupID = manifest.ID
	fmt.Printf("Backup created: %s (%d files)\n", manifest.ID, manifest.FileCount)

	// 3. Apply components per agent.
	componentSet := make(map[model.ComponentID]bool)
	for _, c := range resolved {
		componentSet[c] = true
	}

	for _, agentID := range selection.Agents {
		adapter, err := registry.Get(agentID)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", agentID, err))
			continue
		}

		fmt.Printf("\nConfiguring %s...\n", agentID)
		files, errs := applyComponents(homeDir, adapter, componentSet)
		result.FilesChanged = append(result.FilesChanged, files...)
		result.Errors = append(result.Errors, errs...)
	}

	result.ComponentsDone = resolved

	// 4. Save state.
	s := state.State{
		InstalledAgents: selection.Agents,
		Preset:          selection.Preset,
		Components:      resolved,
		LastInstall:     time.Now(),
		LastBackupID:    result.BackupID,
		Version:         version,
	}
	if err := state.Save(homeDir, s); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("save state: %v", err))
	}

	return result, nil
}

func applyComponents(homeDir string, adapter agents.Adapter, components map[model.ComponentID]bool) ([]string, []string) {
	var files []string
	var errs []string

	type injector struct {
		id model.ComponentID
		fn func() ([]string, error)
	}

	injectors := []injector{
		{model.ComponentCortex, func() ([]string, error) {
			r, err := cortexcomp.Inject(homeDir, adapter)
			return r.Files, err
		}},
		{model.ComponentCLIOrch, func() ([]string, error) {
			r, err := orchestrator.Inject(homeDir, adapter)
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
			r, err := sdd.Inject(homeDir, adapter)
			return r.Files, err
		}},
	}

	for _, inj := range injectors {
		if !components[inj.id] {
			continue
		}
		f, err := inj.fn()
		if err != nil {
			errs = append(errs, fmt.Sprintf("  %s/%s: %v", adapter.Agent(), inj.id, err))
			continue
		}
		files = append(files, f...)
		fmt.Printf("  [+] %s\n", inj.id)
	}

	return files, errs
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
		for _, name := range []string{"cortex", "cli-orchestrator", "agent-mailbox", "forgespec", "context7"} {
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
