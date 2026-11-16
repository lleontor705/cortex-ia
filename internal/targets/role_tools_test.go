package targets

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestClaudeRoleToolParity(t *testing.T) {
	home := t.TempDir()
	b, err := ClaudeBundle(home)
	if err != nil {
		t.Fatalf("ClaudeBundle: %v", err)
	}
	agent := func(role string) string {
		return string(findFile(t, b, filepath.Join(home, ".claude", "agents", role+".md")))
	}

	planner := agent("planner")
	if !strings.Contains(planner, "tools: Read, Grep, Glob, Write, Skill,") {
		t.Errorf("planner tools lack Write:\n%s", planner)
	}
	if !strings.Contains(planner, `matcher: "Write|Edit|MultiEdit|NotebookEdit|Bash|mcp__cortex_ia__.*"`) {
		t.Error("planner pre-tool matcher does not route Write to the hook")
	}
	if orch := agent("orchestrator"); !strings.Contains(orch, "Read, Grep, Glob, AskUserQuestion, Skill,") {
		t.Errorf("orchestrator tools lack AskUserQuestion:\n%s", orch)
	}
	if impl := agent("implement"); strings.Contains(strings.SplitN(impl, "\n---\n", 2)[0], "AskUserQuestion") {
		t.Error("implement must not receive AskUserQuestion")
	}
	for _, role := range []string{"orchestrator", "planner", "reviewer"} {
		if !strings.Contains(agent(role), "mcp__cortex__*") {
			t.Errorf("%s lacks the cortex memory MCP tools", role)
		}
	}
	if !strings.Contains(agent("orchestrator"), "`question`") || !strings.Contains(agent("orchestrator"), "`AskUserQuestion`") {
		t.Error("claude preamble does not map question to AskUserQuestion")
	}
}

func TestAGYRoleToolParity(t *testing.T) {
	home := t.TempDir()
	b, err := AGYBundle(home)
	if err != nil {
		t.Fatalf("AGYBundle: %v", err)
	}
	plugin := filepath.Join(home, ".gemini", "config", "plugins", "cortex-ia")
	agent := func(role string) string {
		return string(findFile(t, b, filepath.Join(plugin, "agents", role+".md")))
	}
	if planner := agent("planner"); !strings.Contains(planner, "  - write_to_file\n") {
		t.Errorf("planner lacks write_to_file:\n%s", planner)
	}
	for _, role := range []string{"orchestrator", "investigate", "reviewer"} {
		if strings.Contains(agent(role), "  - write_to_file\n") {
			t.Errorf("%s must not list write_to_file", role)
		}
	}
	if !strings.Contains(agent("orchestrator"), "  - call_mcp_tool\n") {
		t.Error("orchestrator lost call_mcp_tool")
	}
}
