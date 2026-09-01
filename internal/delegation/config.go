package delegation

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/lleontor705/cortex-ia/internal/components/filemerge"
)

const ConfigFilename = "cortex-delegation.json"

var supportedRoles = map[string]bool{
	"implement": true, "investigate": true, "planner": true, "reviewer": true,
}

// RoleConfig is deliberately declarative. Cortex owns the executable and
// argument vector so configuration cannot become an arbitrary shell surface.
type RoleConfig struct {
	Delegate bool   `json:"delegate"`
	CLI      string `json:"cli"` // native | agy
	Mode     string `json:"mode,omitempty"`
}

type HerdrSettings struct {
	AutoSplit      bool   `json:"auto_split"`
	SplitDirection string `json:"split_direction"`
	TimeoutSeconds int    `json:"timeout_seconds"`
}

type DelegationConfig struct {
	Version           string                `json:"version"`
	DelegationEnabled bool                  `json:"delegation_enabled"`
	UseHerdr          bool                  `json:"use_herdr"`
	HerdrSettings     HerdrSettings         `json:"herdr_settings"`
	Roles             map[string]RoleConfig `json:"roles"`
}

func NormalConfig() DelegationConfig {
	return DelegationConfig{
		Version:       "2.0.0",
		HerdrSettings: HerdrSettings{SplitDirection: "right", TimeoutSeconds: 300},
		Roles: map[string]RoleConfig{
			"implement": {CLI: "native"}, "investigate": {CLI: "native"},
			"reviewer": {CLI: "native"}, "planner": {CLI: "native"},
		},
	}
}

func DefaultDelegationConfig(useHerdr bool) DelegationConfig {
	cfg := NormalConfig()
	cfg.DelegationEnabled = true
	cfg.UseHerdr = useHerdr
	cfg.HerdrSettings.AutoSplit = useHerdr
	cfg.Roles["implement"] = RoleConfig{Delegate: true, CLI: "agy", Mode: "accept-edits"}
	cfg.Roles["investigate"] = RoleConfig{Delegate: true, CLI: "agy", Mode: "plan"}
	cfg.Roles["planner"] = RoleConfig{Delegate: true, CLI: "agy", Mode: "plan"}
	cfg.Roles["reviewer"] = RoleConfig{Delegate: true, CLI: "agy", Mode: "plan"}
	return cfg
}

func (c DelegationConfig) Validate() error {
	if c.Version != "2.0.0" {
		return fmt.Errorf("unsupported delegation config version %q", c.Version)
	}
	if c.HerdrSettings.SplitDirection != "right" && c.HerdrSettings.SplitDirection != "down" {
		return errors.New("herdr split_direction must be right or down")
	}
	if c.HerdrSettings.TimeoutSeconds < 1 || c.HerdrSettings.TimeoutSeconds > 3600 {
		return errors.New("delegation timeout_seconds must be between 1 and 3600")
	}
	for role, cfg := range c.Roles {
		if !supportedRoles[role] {
			return fmt.Errorf("unsupported delegation role %q", role)
		}
		if cfg.CLI != "native" && cfg.CLI != "agy" {
			return fmt.Errorf("role %q: cli must be native or agy", role)
		}
		if cfg.CLI == "agy" && cfg.Mode != "plan" && cfg.Mode != "accept-edits" {
			return fmt.Errorf("role %q: agy mode must be plan or accept-edits", role)
		}
	}
	return nil
}

func ResolveConfigPath(configRoot string) string { return filepath.Join(configRoot, ConfigFilename) }

func encodedConfig(cfg DelegationConfig) ([]byte, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return nil, err
	}
	return data, nil
}

func NeedsSave(configDir string, cfg DelegationConfig) (bool, error) {
	data, err := encodedConfig(cfg)
	if err != nil {
		return false, err
	}
	current, err := os.ReadFile(ResolveConfigPath(configDir))
	if errors.Is(err, os.ErrNotExist) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return !bytes.Equal(current, data), nil
}

func Save(configDir string, cfg DelegationConfig) error {
	data, err := encodedConfig(cfg)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return err
	}
	current, readErr := os.ReadFile(ResolveConfigPath(configDir))
	if readErr == nil && bytes.Equal(current, data) {
		return nil
	}
	if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
		return readErr
	}
	_, err = filemerge.WriteFileAtomic(ResolveConfigPath(configDir), data, 0o600)
	return err
}

func Load(configDir string) (DelegationConfig, error) {
	data, err := os.ReadFile(ResolveConfigPath(configDir))
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
	if cfg.Version == "1.0.0" {
		cfg.Version = "2.0.0"
		for role, roleCfg := range cfg.Roles {
			if roleCfg.CLI != "agy" {
				roleCfg.Delegate, roleCfg.CLI, roleCfg.Mode = false, "native", ""
			} else if role == "implement" {
				roleCfg.Mode = "accept-edits"
			} else {
				roleCfg.Mode = "plan"
			}
			cfg.Roles[role] = roleCfg
		}
	}
	if err := cfg.Validate(); err != nil {
		return NormalConfig(), err
	}
	return cfg, nil
}
