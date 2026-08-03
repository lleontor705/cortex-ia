package state

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/model"
	"github.com/lleontor705/cortex-ia/internal/modelroute"
)

const profilesFile = "profiles.json"

const profilesBackupFile = "profiles.json.legacy.bak"
const profilesBackupDigestFile = "profiles.json.legacy.bak.sha256"

// ProfilesBackupPath returns the immutable source-byte backup created during
// a legacy profile migration.
func ProfilesBackupPath(homeDir string) string {
	return filepath.Join(homeDir, stateDir, profilesBackupFile)
}

// ProfilesBackupDigestPath returns the digest sidecar for the preserved
// legacy profile bytes.
func ProfilesBackupDigestPath(homeDir string) string {
	return filepath.Join(homeDir, stateDir, profilesBackupDigestFile)
}

type profilesEnvelope struct {
	Version  int             `json:"version"`
	Profiles []model.Profile `json:"profiles"`
}

type legacyProfile struct {
	Name             string            `json:"name"`
	ModelAssignments map[string]string `json:"model_assignments,omitempty"`
}

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
	var envelope profilesEnvelope
	if err := json.Unmarshal(data, &envelope); err == nil && envelope.Version == 2 {
		return envelope.Profiles, nil
	}
	var legacy []legacyProfile
	if err := json.Unmarshal(data, &legacy); err != nil {
		return nil, fmt.Errorf("unmarshal profiles: %w", err)
	}
	profiles := make([]model.Profile, 0, len(legacy))
	for _, old := range legacy {
		p := model.Profile{Name: old.Name, Routes: map[string]modelroute.RouteRequest{}, ConfiguredAssignments: map[string]model.OpenCodeModelAssignment{}}
		for phase, value := range old.ModelAssignments {
			value = strings.TrimSpace(value)
			if route, parseErr := modelroute.NewRouteID(value); parseErr == nil {
				p.Routes[phase] = modelroute.RouteRequest{RouteID: route}
				continue
			}
			parts := strings.SplitN(value, "/", 2)
			if len(parts) == 2 && parts[0] != "" && parts[1] != "" {
				p.ConfiguredAssignments[phase] = model.OpenCodeModelAssignment{Provider: parts[0], Model: parts[1]}
				continue
			}
			return nil, fmt.Errorf("unmapped legacy profile assignment %q for %s: fail closed", value, phase)
		}
		profiles = append(profiles, p)
	}
	if err := os.WriteFile(ProfilesBackupPath(homeDir), data, 0o644); err != nil {
		return nil, fmt.Errorf("backup legacy profiles: %w", err)
	}
	digest := sha256.Sum256(data)
	if err := os.WriteFile(ProfilesBackupDigestPath(homeDir), []byte(fmt.Sprintf("%x\n", digest)), 0o644); err != nil {
		return nil, fmt.Errorf("write legacy profile digest: %w", err)
	}
	return profiles, nil
}

// SaveProfiles writes profiles to disk, creating the directory if needed.
func SaveProfiles(homeDir string, profiles []model.Profile) error {
	if err := EnsureDir(homeDir); err != nil {
		return err
	}
	for _, profile := range profiles {
		if len(profile.ModelAssignments) != 0 {
			return fmt.Errorf("profile %q contains legacy model assignments; migrate at ingress", profile.Name)
		}
	}
	data, err := json.MarshalIndent(profilesEnvelope{Version: 2, Profiles: profiles}, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal profiles: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(ProfilesPath(homeDir), data, 0o644); err != nil {
		return fmt.Errorf("write profiles file: %w", err)
	}
	return nil
}
