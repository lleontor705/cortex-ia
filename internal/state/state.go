package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/lleontor705/cortex-ia/internal/model"
)

const stateDir = ".cortex-ia"
const stateFile = "state.json"
const lockFile = "cortex-ia.lock"

// SharedSkillsDir returns the shared skills directory for all agents (~/.cortex-ia/skills/).
func SharedSkillsDir(homeDir string) string {
	return filepath.Join(homeDir, stateDir, "skills")
}

// SharedPromptsDir returns the shared prompts directory (~/.cortex-ia/prompts/).
func SharedPromptsDir(homeDir string) string {
	return filepath.Join(homeDir, stateDir, "prompts")
}

// State tracks what cortex-ia has installed.
type State struct {
	InstalledAgents []model.AgentID     `json:"installed_agents"`
	Preset          model.PresetID      `json:"preset,omitempty"`
	Components      []model.ComponentID `json:"components,omitempty"`
	LastInstall     time.Time           `json:"last_install"`
	LastBackupID    string              `json:"last_backup_id,omitempty"`
	Version         string              `json:"version,omitempty"`
	LastProfile     string              `json:"last_profile,omitempty"`
	StrictTDD       bool                `json:"strict_tdd,omitempty"`
}

// Lockfile captures the concrete installed artifact set for verification and repair.
type Lockfile struct {
	InstalledAgents []model.AgentID     `json:"installed_agents"`
	Preset          model.PresetID      `json:"preset,omitempty"`
	Components      []model.ComponentID `json:"components,omitempty"`
	Files           []string            `json:"files,omitempty"`
	GeneratedAt     time.Time           `json:"generated_at"`
	LastBackupID    string              `json:"last_backup_id,omitempty"`
	Version         string              `json:"version,omitempty"`
}

// BaseDir returns the path to the cortex-ia home directory (~/.cortex-ia/).
func BaseDir(homeDir string) string {
	return filepath.Join(homeDir, stateDir)
}

// EnsureDir creates the cortex-ia base directory and its core subdirectories
// (skills, prompts) if they do not already exist.
func EnsureDir(homeDir string) error {
	for _, dir := range []string{
		BaseDir(homeDir),
		SharedSkillsDir(homeDir),
		SharedPromptsDir(homeDir),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create directory %q: %w", dir, err)
		}
	}
	return nil
}

// StatePath returns the path to the state file.
func StatePath(homeDir string) string {
	return filepath.Join(homeDir, stateDir, stateFile)
}

// LockPath returns the path to the lock file.
func LockPath(homeDir string) string {
	return filepath.Join(homeDir, stateDir, lockFile)
}

// Load reads the state file. Returns empty state if not found.
func Load(homeDir string) (State, error) {
	path := StatePath(homeDir)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return State{}, nil
		}
		return State{}, fmt.Errorf("read state: %w", err)
	}

	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return State{}, fmt.Errorf("parse state: %w", err)
	}
	return s, nil
}

// Save writes the state file.
func Save(homeDir string, s State) error {
	path := StatePath(homeDir)
	dir := filepath.Dir(path)

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create state directory: %w", err)
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}
	data = append(data, '\n')

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write state: %w", err)
	}
	return nil
}

// LoadLock reads the lock file. Returns empty lock if not found.
func LoadLock(homeDir string) (Lockfile, error) {
	path := LockPath(homeDir)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Lockfile{}, nil
		}
		return Lockfile{}, fmt.Errorf("read lock: %w", err)
	}

	var lock Lockfile
	if err := json.Unmarshal(data, &lock); err != nil {
		return Lockfile{}, fmt.Errorf("parse lock: %w", err)
	}
	return lock, nil
}

// SaveLock writes the lock file.
func SaveLock(homeDir string, lock Lockfile) error {
	path := LockPath(homeDir)
	dir := filepath.Dir(path)

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create lock directory: %w", err)
	}

	data, err := json.MarshalIndent(lock, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal lock: %w", err)
	}
	data = append(data, '\n')

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write lock: %w", err)
	}
	return nil
}
