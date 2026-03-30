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

// State tracks what cortex-ia has installed.
type State struct {
	InstalledAgents []model.AgentID     `json:"installed_agents"`
	Preset          model.PresetID      `json:"preset,omitempty"`
	Components      []model.ComponentID `json:"components,omitempty"`
	LastInstall     time.Time           `json:"last_install"`
	LastBackupID    string              `json:"last_backup_id,omitempty"`
	Version         string              `json:"version,omitempty"`
}

// StatePath returns the path to the state file.
func StatePath(homeDir string) string {
	return filepath.Join(homeDir, stateDir, stateFile)
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
