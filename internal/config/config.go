// Package config handles project-level .cortex-ia.yaml configuration.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/lleontor705/cortex-ia/internal/model"
	"gopkg.in/yaml.v3"
)

const FileName = ".cortex-ia.yaml"

// ProjectConfig represents a project-level configuration file.
type ProjectConfig struct {
	Preset             string   `yaml:"preset,omitempty"`
	Persona            string   `yaml:"persona,omitempty"`
	ModelPreset        string   `yaml:"model-preset,omitempty"`
	Agents             []string `yaml:"agents,omitempty"`
	DisabledComponents []string `yaml:"disabled-components,omitempty"`
	CustomSkills       []Skill  `yaml:"custom-skills,omitempty"`
}

// Skill describes a custom skill to load.
type Skill struct {
	Path string `yaml:"path"`
}

// FindProjectConfig walks up from startDir to find .cortex-ia.yaml.
// Returns the config and its directory, or nil if not found.
func FindProjectConfig(startDir string) (*ProjectConfig, string, error) {
	dir := startDir
	for {
		path := filepath.Join(dir, FileName)
		if _, err := os.Stat(path); err == nil {
			cfg, err := LoadFile(path)
			if err != nil {
				return nil, "", err
			}
			return cfg, dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break // reached root
		}
		dir = parent
	}
	return nil, "", nil
}

// LoadFile reads and parses a .cortex-ia.yaml file.
func LoadFile(path string) (*ProjectConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg ProjectConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return &cfg, nil
}

// WriteDefault creates a default .cortex-ia.yaml in the given directory.
func WriteDefault(dir string) (string, error) {
	path := filepath.Join(dir, FileName)
	content := `# cortex-ia project configuration
# See: https://github.com/lleontor705/cortex-ia

preset: full
persona: professional
model-preset: balanced

# Uncomment to restrict agents for this project:
# agents:
#   - claude-code
#   - opencode

# Uncomment to disable specific components:
# disabled-components:
#   - mailbox

# Add project-specific custom skills:
# custom-skills:
#   - path: ./skills/domain-validator
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("write config: %w", err)
	}
	return path, nil
}

// ApplyToSelection merges project config into a Selection.
func ApplyToSelection(cfg *ProjectConfig, sel *model.Selection) {
	if cfg == nil {
		return
	}

	if cfg.Preset != "" && sel.Preset == "" {
		sel.Preset = model.PresetID(cfg.Preset)
	}
	if cfg.Persona != "" && sel.Persona == "" {
		sel.Persona = model.PersonaID(cfg.Persona)
	}
	if cfg.ModelPreset != "" && sel.ModelAssignments == nil {
		sel.ModelAssignments = model.ModelsForPreset(model.ModelPreset(cfg.ModelPreset))
	}
	if len(cfg.Agents) > 0 && len(sel.Agents) == 0 {
		for _, a := range cfg.Agents {
			sel.Agents = append(sel.Agents, model.AgentID(a))
		}
	}
}
