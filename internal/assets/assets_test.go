package assets

import (
	"strings"
	"testing"
)

func TestReadSkill(t *testing.T) {
	content, err := Read("skills/bootstrap/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	if len(content) == 0 {
		t.Error("expected non-empty bootstrap skill")
	}
}

func TestReadConvention(t *testing.T) {
	content, err := Read("skills/_shared/cortex-convention.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(content, "Cortex Convention") {
		t.Error("expected cortex convention content")
	}
}

func TestReadOrchestrator(t *testing.T) {
	content, err := Read("generic/sdd-orchestrator.md")
	if err != nil {
		t.Fatal(err)
	}
	if len(content) == 0 {
		t.Error("expected non-empty orchestrator")
	}
}

func TestOrchestratorDoesNotRequireTextualModelTags(t *testing.T) {
	for _, path := range []string{
		"generic/sdd-orchestrator.md",
		"generic/sdd-orchestrator-single.md",
	} {
		content, err := Read(path)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(content, "MODEL:") {
			t.Errorf("%s should not inject textual MODEL tags into prompts", path)
		}
	}
}

func TestPortableTeamLeadSkillIsNotEmbedded(t *testing.T) {
	if _, err := Read("skills/team-lead/SKILL.md"); err == nil {
		t.Fatal("portable team-lead scheduling skill must not be embedded")
	}
}

func TestReadOrchestratorSingle(t *testing.T) {
	content, err := Read("generic/sdd-orchestrator-single.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(content, "Single-Agent") {
		t.Error("expected single-agent orchestrator")
	}
}

func TestReadCortexProtocol(t *testing.T) {
	content, err := Read("generic/cortex-protocol.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(content, "Persistent Memory") {
		t.Error("expected cortex protocol content")
	}
}

func TestReadCommands(t *testing.T) {
	entries, err := ListDir("opencode/commands")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 10 {
		t.Errorf("expected exactly 10 commands, got %d", len(entries))
	}
}

func TestNewChangeDispatchesInitialPhasesAsDirectChildren(t *testing.T) {
	content, err := Read("opencode/commands/new-change.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range []string{"task", "bootstrap", "investigate", "draft-proposal", "blocked"} {
		if !strings.Contains(content, marker) {
			t.Errorf("new-change command missing %q", marker)
		}
	}
	if !(strings.Index(content, "bootstrap") < strings.Index(content, "investigate") && strings.Index(content, "investigate") < strings.Index(content, "draft-proposal")) {
		t.Error("new-change must dispatch bootstrap, investigate, then draft-proposal")
	}
	for _, forbidden := range []string{"skill/bootstrap", "skill/investigate", "skill/draft-proposal", "Read canonical proposal skill", "Draft change proposal"} {
		if strings.Contains(content, forbidden) {
			t.Errorf("new-change command must not perform or load child phase work: found %q", forbidden)
		}
	}
}

func TestListAllSkills(t *testing.T) {
	entries, err := ListDir("skills")
	if err != nil {
		t.Fatal(err)
	}
	// 19 skill dirs + 1 _shared = at least 20
	if len(entries) < 20 {
		t.Errorf("expected at least 20 skill entries, got %d", len(entries))
	}
}

func TestReadNonExistent(t *testing.T) {
	_, err := Read("nonexistent/file.md")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestAudit_AllEmbeddedSkillsAndSubagentsIntegrity(t *testing.T) {
	entries, err := ListDir("skills")
	if err != nil {
		t.Fatalf("ListDir(skills) error = %v", err)
	}

	validSkillsCount := 0
	for _, entry := range entries {
		name := entry.Name()
		if name == "_shared" || strings.HasSuffix(name, ".json") {
			continue
		}
		skillPath := "skills/" + name + "/SKILL.md"
		content, err := Read(skillPath)
		if err != nil {
			t.Errorf("Skill %q missing SKILL.md: %v", name, err)
			continue
		}
		if len(content) == 0 {
			t.Errorf("Skill %q has empty SKILL.md", name)
			continue
		}
		if !strings.HasPrefix(content, "---\n") && !strings.HasPrefix(content, "---\r\n") {
			t.Errorf("Skill %q SKILL.md missing frontmatter opening delimiter '---'", name)
		}
		if !strings.Contains(content, "name:") {
			t.Errorf("Skill %q SKILL.md missing 'name:' field in frontmatter", name)
		}
		if !strings.Contains(content, "description:") {
			t.Errorf("Skill %q SKILL.md missing 'description:' field in frontmatter", name)
		}
		validSkillsCount++
	}

	if validSkillsCount < 20 {
		t.Errorf("Audit found only %d valid skills, expected >= 20", validSkillsCount)
	}
	t.Logf("Audit passed: verified %d skills with valid frontmatter & non-empty content", validSkillsCount)
}
