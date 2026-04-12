package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const installStatusFile = "install-status.json"

// InstallStatus tracks whether an installation completed cleanly.
// If the process crashes or a component fails mid-way, the status
// remains "in-progress" so that doctor/repair can detect it.
type InstallStatus struct {
	Status    string `json:"status"`               // "in-progress" or "complete"
	StartedAt string `json:"started_at"`            // RFC3339 timestamp
	BackupID  string `json:"backup_id,omitempty"`   // backup created before this install
}

// InstallStatusPath returns the path to the install status file.
func InstallStatusPath(homeDir string) string {
	return filepath.Join(homeDir, stateDir, installStatusFile)
}

// LoadInstallStatus reads the install status file. Returns nil if the file
// does not exist (meaning no install is in progress and none was left dirty).
func LoadInstallStatus(homeDir string) (*InstallStatus, error) {
	path := InstallStatusPath(homeDir)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read install status: %w", err)
	}

	var status InstallStatus
	if err := json.Unmarshal(data, &status); err != nil {
		return nil, fmt.Errorf("parse install status: %w", err)
	}
	return &status, nil
}

// SaveInstallStatus writes the install status file.
func SaveInstallStatus(homeDir string, status InstallStatus) error {
	path := InstallStatusPath(homeDir)
	dir := filepath.Dir(path)

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create install status directory: %w", err)
	}

	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal install status: %w", err)
	}
	data = append(data, '\n')

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write install status: %w", err)
	}
	return nil
}

// ClearInstallStatus removes the install status file, indicating a clean state.
func ClearInstallStatus(homeDir string) error {
	path := InstallStatusPath(homeDir)
	err := os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove install status: %w", err)
	}
	return nil
}
