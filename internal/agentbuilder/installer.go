package agentbuilder

import (
	"fmt"
	"os"
	"path/filepath"
)

// Install writes the generated agent files to disk.
// The SKILL.md is placed at {homeDir}/.cortex-ia/skills-community/{skillName}/SKILL.md.
func Install(homeDir string, agent *GeneratedAgent) (InstallResult, error) {
	if agent == nil {
		return InstallResult{}, fmt.Errorf("agentbuilder: agent must not be nil")
	}
	if homeDir == "" {
		return InstallResult{}, fmt.Errorf("agentbuilder: homeDir must not be empty")
	}

	skillDir := filepath.Join(homeDir, ".cortex-ia", "skills-community", agent.SkillName)
	skillFile := filepath.Join(skillDir, "SKILL.md")

	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		return InstallResult{
			Err: fmt.Errorf("create directory %s: %w", skillDir, err),
		}, err
	}

	if err := os.WriteFile(skillFile, []byte(agent.SkillContent), 0o644); err != nil {
		return InstallResult{
			Err: fmt.Errorf("write %s: %w", skillFile, err),
		}, err
	}

	return InstallResult{
		FilesWritten: []string{skillFile},
	}, nil
}
