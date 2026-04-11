// Package skills installs community and built-in skill files into agent workspaces.
package skills

import (
	"fmt"
	"log"
	"path/filepath"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/agents"
	"github.com/lleontor705/cortex-ia/internal/assets"
	"github.com/lleontor705/cortex-ia/internal/components/filemerge"
	"github.com/lleontor705/cortex-ia/internal/model"
)

// isSDDSkill reports whether a skill ID belongs to the SDD suite.
// SDD skills are installed by the SDD component; the skills component skips
// them to prevent duplicate writes when both components are selected.
func isSDDSkill(id model.SkillID) bool {
	s := string(id)
	sddNames := map[string]bool{
		"bootstrap": true, "investigate": true, "draft-proposal": true,
		"write-specs": true, "architect": true, "decompose": true,
		"team-lead": true, "implement": true, "validate": true, "finalize": true,
		"debate": true, "debug": true, "execute-plan": true, "ideate": true,
		"monitor": true, "open-pr": true, "file-issue": true,
		"parallel-dispatch": true, "scan-registry": true,
	}
	return sddNames[s] || strings.HasPrefix(s, "sdd-")
}

// InjectionResult describes the outcome of skill injection.
type InjectionResult struct {
	Changed bool
	Files   []string
	Skipped []model.SkillID
}

// Inject writes the embedded SKILL.md files for each requested skill
// to the correct directory for the given agent adapter.
// SDD skills are skipped (handled by the SDD component).
func Inject(homeDir string, adapter agents.Adapter, skillIDs []model.SkillID) (InjectionResult, error) {
	if !adapter.SupportsSkills() {
		return InjectionResult{Skipped: skillIDs}, nil
	}

	skillDir := adapter.SkillsDir(homeDir)
	if skillDir == "" {
		return InjectionResult{Skipped: skillIDs}, nil
	}

	paths := make([]string, 0, len(skillIDs))
	skipped := make([]model.SkillID, 0)
	changed := false

	for _, id := range skillIDs {
		if isSDDSkill(id) {
			continue
		}

		assetPath := "skills/" + string(id) + "/SKILL.md"
		content, readErr := assets.Read(assetPath)
		if readErr != nil {
			log.Printf("skills: skipping %q — embedded asset not found: %v", id, readErr)
			skipped = append(skipped, id)
			continue
		}
		if len(content) == 0 {
			return InjectionResult{}, fmt.Errorf("skill %q: embedded asset exists but is empty", id)
		}

		path := filepath.Join(skillDir, string(id), "SKILL.md")
		wr, err := filemerge.WriteFileAtomic(path, []byte(content), 0o644)
		if err != nil {
			return InjectionResult{}, fmt.Errorf("skill %q: write failed: %w", id, err)
		}

		changed = changed || wr.Changed
		paths = append(paths, path)
	}

	return InjectionResult{Changed: changed, Files: paths, Skipped: skipped}, nil
}
