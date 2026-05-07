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

func TestTeamLeadSkillUsesForgeSpecTaskBoardAPI(t *testing.T) {
	content, err := Read("skills/team-lead/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}

	for _, invalid := range []string{
		`tb_update(task_id: '{id}', status: 'completed'`,
		`tb_update(task_id: "{id}", status: "completed"`,
		`tb_update(task_id: '{id}', status: 'failed'`,
		`tb_update(task_id: "{id}", status: "failed"`,
		`tb_update(task_id: '{id}', status: 'pending'`,
		`tb_update(task_id: "{id}", status: "pending"`,
		`failed_reason`,
		`output:`,
		`tb_claim(task_id: "{id}", board_id: "{board_id}")`,
	} {
		if strings.Contains(content, invalid) {
			t.Errorf("team-lead skill contains invalid ForgeSpec task-board API fragment %q", invalid)
		}
	}

	for _, required := range []string{
		`tb_claim(task_id: "{id}", agent: "implement-{id}")`,
		`tb_update(task_id: '{id}', status: 'in_progress'`,
		`tb_update(task_id: '{id}', status: 'done'`,
		`tb_update(task_id: '{id}', status: 'blocked'`,
	} {
		if !strings.Contains(content, required) {
			t.Errorf("team-lead skill missing required ForgeSpec task-board API fragment %q", required)
		}
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
	if len(entries) < 10 {
		t.Errorf("expected at least 10 commands, got %d", len(entries))
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
