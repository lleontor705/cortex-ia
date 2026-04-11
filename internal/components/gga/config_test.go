package gga

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/model"
)

func TestProviderForAgents_Claude(t *testing.T) {
	got := ProviderForAgents([]model.AgentID{model.AgentClaudeCode})
	if got != "claude" {
		t.Errorf("expected claude, got %s", got)
	}
}

func TestProviderForAgents_Gemini(t *testing.T) {
	got := ProviderForAgents([]model.AgentID{model.AgentGeminiCLI})
	if got != "gemini" {
		t.Errorf("expected gemini, got %s", got)
	}
}

func TestProviderForAgents_Default(t *testing.T) {
	got := ProviderForAgents(nil)
	if got != "claude" {
		t.Errorf("expected claude as default, got %s", got)
	}
}

func TestProviderForAgents_Priority(t *testing.T) {
	// When both claude and gemini are present, claude wins.
	got := ProviderForAgents([]model.AgentID{model.AgentGeminiCLI, model.AgentClaudeCode})
	if got != "claude" {
		t.Errorf("expected claude (priority), got %s", got)
	}
}

func TestProviderForAgents_OpenCode(t *testing.T) {
	got := ProviderForAgents([]model.AgentID{model.AgentOpenCode})
	if got != "opencode" {
		t.Errorf("expected opencode, got %s", got)
	}
}

func TestProviderForAgents_Codex(t *testing.T) {
	got := ProviderForAgents([]model.AgentID{model.AgentCodex})
	if got != "codex" {
		t.Errorf("expected codex, got %s", got)
	}
}

func TestBuildConfig_ContainsProvider(t *testing.T) {
	content := string(BuildConfig("claude"))
	if !strings.Contains(content, `PROVIDER="claude"`) {
		t.Error("config should contain PROVIDER=\"claude\"")
	}
}

func TestBuildConfig_ContainsFields(t *testing.T) {
	content := string(BuildConfig("gemini"))
	for _, field := range []string{"FILE_PATTERNS", "EXCLUDE_PATTERNS", "MAX_FILES", "TIMEOUT"} {
		if !strings.Contains(content, field) {
			t.Errorf("config should contain %s", field)
		}
	}
}

func TestConfigPath(t *testing.T) {
	got := ConfigPath("/home/user")
	want := filepath.Join("/home/user", ".config", "gga", "config")
	if got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestAgentsTemplatePath(t *testing.T) {
	got := AgentsTemplatePath("/home/user")
	want := filepath.Join("/home/user", ".config", "gga", "AGENTS.md")
	if got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestInject_WritesFiles(t *testing.T) {
	tmpDir := t.TempDir()
	result, err := Inject(tmpDir, []model.AgentID{model.AgentClaudeCode})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Files) != 2 {
		t.Errorf("expected 2 files, got %d", len(result.Files))
	}

	// Verify config file exists.
	configPath := ConfigPath(tmpDir)
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("config file was not created")
	}

	// Verify AGENTS.md exists.
	agentsPath := AgentsTemplatePath(tmpDir)
	if _, err := os.Stat(agentsPath); os.IsNotExist(err) {
		t.Error("AGENTS.md was not created")
	}
}

func TestInject_ConfigContent(t *testing.T) {
	tmpDir := t.TempDir()
	_, err := Inject(tmpDir, []model.AgentID{model.AgentGeminiCLI})
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(ConfigPath(tmpDir))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	if !strings.Contains(content, `PROVIDER="gemini"`) {
		t.Error("config should contain PROVIDER=\"gemini\"")
	}
	if !strings.Contains(content, "FILE_PATTERNS") {
		t.Error("config should contain FILE_PATTERNS")
	}
}

func TestInject_AgentsContent(t *testing.T) {
	tmpDir := t.TempDir()
	_, err := Inject(tmpDir, []model.AgentID{model.AgentClaudeCode})
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(AgentsTemplatePath(tmpDir))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	if !strings.Contains(content, "Code Review Rules") {
		t.Error("AGENTS.md should contain 'Code Review Rules'")
	}
	if !strings.Contains(content, "STATUS: PASSED") {
		t.Error("AGENTS.md should contain 'STATUS: PASSED'")
	}
}

func TestInject_Idempotent(t *testing.T) {
	tmpDir := t.TempDir()
	_, err := Inject(tmpDir, []model.AgentID{model.AgentClaudeCode})
	if err != nil {
		t.Fatal(err)
	}

	// Second inject should not report changes.
	result, err := Inject(tmpDir, []model.AgentID{model.AgentClaudeCode})
	if err != nil {
		t.Fatal(err)
	}
	if result.Changed {
		t.Error("second inject should not report changes")
	}
}
