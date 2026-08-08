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
	Profile            string   `yaml:"profile,omitempty"`
	ModelAssignment    any      `yaml:"model-assignment,omitempty"`
	PackageInstall     any      `yaml:"package-install,omitempty"`
	StrictTDD          bool     `yaml:"strict-tdd,omitempty"`
	Agents             []string `yaml:"agents,omitempty"`
	DisabledComponents []string `yaml:"disabled-components,omitempty"`
	CustomSkills       []Skill  `yaml:"custom-skills,omitempty"`

	retiredFields map[string]struct{}
}

// RetiredProjectFieldError reports configuration that must be migrated rather
// than translated into a supported installer selection.
type RetiredProjectFieldError struct {
	Field string
}

func (e *RetiredProjectFieldError) Error() string {
	return fmt.Sprintf("project configuration field %q is retired; remove it and use supported lifecycle configuration", e.Field)
}

// UnmarshalYAML retains retired field names only long enough to return an
// actionable migration error. They are never translated into selection data.
func (cfg *ProjectConfig) UnmarshalYAML(value *yaml.Node) error {
	type projectConfig ProjectConfig
	var decoded projectConfig
	if err := value.Decode(&decoded); err != nil {
		return err
	}
	*cfg = ProjectConfig(decoded)
	cfg.retiredFields = make(map[string]struct{})
	for i := 0; i+1 < len(value.Content); i += 2 {
		field := value.Content[i].Value
		switch field {
		case "profile", "model-preset", "model-assignment", "package-install":
			cfg.retiredFields[field] = struct{}{}
		}
	}
	return nil
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
	if err := cfg.validateCurrent(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// WriteDefault creates a default .cortex-ia.yaml in the given directory.
func WriteDefault(dir string) (string, error) {
	path := filepath.Join(dir, FileName)
	content := `# cortex-ia project configuration
# See: https://github.com/lleontor705/cortex-ia
# Full schema: docs/cortex-ia.yaml.example

preset: full           # full | minimal | custom
persona: professional  # professional | mentor | minimal

# Enforce TDD across SDD apply/verify phases.
# strict-tdd: false

# Restrict the agents this project supports. Omit to apply to every detected agent.
# agents:
#   - claude-code
#   - opencode

# Opt out of specific components for this project.
# disabled-components:
#   - skills

# Layer project-specific custom skills on top of the embedded set.
# custom-skills:
#   - path: ./skills/domain-validator
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("write config: %w", err)
	}
	return path, nil
}

// ApplyToSelection merges supported project configuration into a Selection.
// Retired configuration is rejected before it can change the selection.
func ApplyToSelection(cfg *ProjectConfig, sel *model.Selection) error {
	if cfg == nil {
		return nil
	}
	if err := cfg.validateCurrent(); err != nil {
		return err
	}

	if cfg.Preset != "" && sel.Preset == "" {
		sel.Preset = model.PresetID(cfg.Preset)
	}
	if cfg.Persona != "" && sel.Persona == "" {
		sel.Persona = model.PersonaID(cfg.Persona)
	}
	if len(cfg.Agents) > 0 && len(sel.Agents) == 0 {
		for _, a := range cfg.Agents {
			sel.Agents = append(sel.Agents, model.AgentID(a))
		}
	}
	if cfg.StrictTDD && !sel.StrictTDD {
		sel.StrictTDD = cfg.StrictTDD
	}
	return nil
}

func (cfg *ProjectConfig) validateCurrent() error {
	for _, field := range []string{"profile", "model-preset", "model-assignment", "package-install"} {
		if _, found := cfg.retiredFields[field]; found {
			return &RetiredProjectFieldError{Field: field}
		}
	}
	if cfg.Profile != "" {
		return &RetiredProjectFieldError{Field: "profile"}
	}
	if cfg.ModelPreset != "" {
		return &RetiredProjectFieldError{Field: "model-preset"}
	}
	if cfg.ModelAssignment != nil {
		return &RetiredProjectFieldError{Field: "model-assignment"}
	}
	if cfg.PackageInstall != nil {
		return &RetiredProjectFieldError{Field: "package-install"}
	}
	return nil
}
