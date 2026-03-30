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
