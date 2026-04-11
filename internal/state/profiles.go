package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/lleontor705/cortex-ia/internal/model"
)

const profilesFile = "profiles.json"

// ProfilesPath returns the path to the profiles file.
func ProfilesPath(homeDir string) string {
	return filepath.Join(homeDir, stateDir, profilesFile)
}

// LoadProfiles reads saved profiles from disk.
// Returns nil, nil when the file does not exist yet.
func LoadProfiles(homeDir string) ([]model.Profile, error) {
	data, err := os.ReadFile(ProfilesPath(homeDir))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read profiles file: %w", err)
	}
	var profiles []model.Profile
	if err := json.Unmarshal(data, &profiles); err != nil {
		return nil, fmt.Errorf("unmarshal profiles: %w", err)
	}
	return profiles, nil
}

// SaveProfiles writes profiles to disk, creating the directory if needed.
func SaveProfiles(homeDir string, profiles []model.Profile) error {
	if err := EnsureDir(homeDir); err != nil {
		return err
	}
	data, err := json.MarshalIndent(profiles, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal profiles: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(ProfilesPath(homeDir), data, 0o644); err != nil {
		return fmt.Errorf("write profiles file: %w", err)
	}
	return nil
}
