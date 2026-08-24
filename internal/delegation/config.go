package delegation

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// RoleConfig defines the delegation settings for an individual agent role / workflow phase.
type RoleConfig struct {
	Delegate bool     `json:"delegate"`
	CLI      string   `json:"cli"` // "agy" | "claude" | "native"
	Command  string   `json:"command"`
	Args     []string `json:"args"`
}

// HerdrSettings defines workspace and pane behavior for Herdr.
type HerdrSettings struct {
	AutoSplit      bool   `json:"auto_split"`
	SplitDirection string `json:"split_direction"`
	TimeoutSeconds int    `json:"timeout_seconds"`
}

// DelegationConfig represents the complete configuration saved in cortex-delegation.json.
type DelegationConfig struct {
	Version           string                `json:"version"`
	DelegationEnabled bool                  `json:"delegation_enabled"`
	UseHerdr          bool                  `json:"use_herdr"`
	HerdrSettings     HerdrSettings         `json:"herdr_settings"`
	Roles             map[string]RoleConfig `json:"roles"`
}

// NormalConfig returns a clean configuration without external delegation.
func NormalConfig() DelegationConfig {
	return DelegationConfig{
		Version:           "1.0.0",
		DelegationEnabled: false,
		UseHerdr:          false,
		HerdrSettings: HerdrSettings{
			AutoSplit:      false,
			SplitDirection: "right",
			TimeoutSeconds: 300,
		},
		Roles: map[string]RoleConfig{
			"implement":   {Delegate: false, CLI: "native"},
			"investigate": {Delegate: false, CLI: "native"},
			"reviewer":    {Delegate: false, CLI: "native"},
			"planner":     {Delegate: false, CLI: "native"},
		},
	}
}

// DefaultDelegationConfig returns a default delegation configuration with Herdr enabled.
func DefaultDelegationConfig(useHerdr bool) DelegationConfig {
	return DelegationConfig{
		Version:           "1.0.0",
		DelegationEnabled: true,
		UseHerdr:          useHerdr,
		HerdrSettings: HerdrSettings{
			AutoSplit:      true,
			SplitDirection: "right",
			TimeoutSeconds: 300,
		},
		Roles: map[string]RoleConfig{
			"implement":   {Delegate: true, CLI: "agy", Command: "agy", Args: []string{"--dangerously-skip-permissions", "-p"}},
			"investigate": {Delegate: false, CLI: "native"},
			"reviewer":    {Delegate: true, CLI: "claude", Command: "claude", Args: []string{"--dangerously-skip-permissions", "-p"}},
			"planner":     {Delegate: false, CLI: "native"},
		},
	}
}

// ConfigFilename is the canonical configuration file name.
const ConfigFilename = "cortex-delegation.json"

// ResolveConfigPath returns the absolute path to the delegation configuration file.
func ResolveConfigPath(configRoot string) string {
	return filepath.Join(configRoot, ConfigFilename)
}

// Save writes the delegation configuration to disk with formatted JSON.
func Save(configDir string, cfg DelegationConfig) error {
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}
	filePath := ResolveConfigPath(configDir)
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filePath, data, 0644)
}

// Load reads and parses the delegation configuration from disk.
// If the file does not exist, it returns a default NormalConfig without error.
func Load(configDir string) (DelegationConfig, error) {
	filePath := ResolveConfigPath(configDir)
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return NormalConfig(), nil
		}
		return NormalConfig(), err
	}
	var cfg DelegationConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return NormalConfig(), err
	}
	return cfg, nil
}
