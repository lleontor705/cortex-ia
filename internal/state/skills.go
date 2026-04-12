package state

import (
	"os"
	"path/filepath"
)

// CommunitySkillsDir returns the path to community skills.
func CommunitySkillsDir(homeDir string) string {
	return filepath.Join(homeDir, ".cortex-ia", "skills-community")
}

// ListCommunitySkills returns the names of installed community skills.
// A directory is considered a skill if it contains a SKILL.md file.
func ListCommunitySkills(homeDir string) []string {
	dir := CommunitySkillsDir(homeDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var skills []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		skillFile := filepath.Join(dir, entry.Name(), "SKILL.md")
		if _, err := os.Stat(skillFile); err == nil {
			skills = append(skills, entry.Name())
		}
	}
	return skills
}
