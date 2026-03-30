package claude

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/model"
)

func TestAdapterIdentity(t *testing.T) {
	a := NewAdapter()
	if a.Agent() != model.AgentClaudeCode {
		t.Errorf("expected AgentClaudeCode, got %s", a.Agent())
	}
	if a.Tier() != model.TierFull {
		t.Errorf("expected TierFull, got %s", a.Tier())
	}
}

func TestAdapterPaths(t *testing.T) {
	a := NewAdapter()
	home := "/home/test"

	if got := a.GlobalConfigDir(home); got != filepath.Join(home, ".claude") {
		t.Errorf("GlobalConfigDir = %s", got)
	}
	if got := a.SystemPromptFile(home); got != filepath.Join(home, ".claude", "CLAUDE.md") {
		t.Errorf("SystemPromptFile = %s", got)
	}
	if got := a.SkillsDir(home); got != filepath.Join(home, ".claude", "skills") {
		t.Errorf("SkillsDir = %s", got)
	}
	if got := a.MCPConfigPath(home, "cortex"); got != filepath.Join(home, ".claude", "mcp", "cortex.json") {
		t.Errorf("MCPConfigPath = %s", got)
	}
}

func TestAdapterStrategies(t *testing.T) {
	a := NewAdapter()
	if a.SystemPromptStrategy() != model.StrategyMarkdownSections {
		t.Error("expected StrategyMarkdownSections")
	}
	if a.MCPStrategy() != model.StrategySeparateMCPFiles {
		t.Error("expected StrategySeparateMCPFiles")
	}
}

func TestAdapterCapabilities(t *testing.T) {
	a := NewAdapter()
	if !a.SupportsSkills() {
		t.Error("expected SupportsSkills=true")
	}
	if !a.SupportsMCP() {
		t.Error("expected SupportsMCP=true")
	}
	if !a.SupportsTaskDelegation() {
		t.Error("expected SupportsTaskDelegation=true")
	}
	if a.SupportsSubAgents() {
		t.Error("expected SupportsSubAgents=false")
	}
	if a.SupportsSlashCommands() {
		t.Error("expected SupportsSlashCommands=false")
	}
}

func TestDetectWithConfigDir(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".claude")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}

	a := &Adapter{
		lookPath: func(name string) (string, error) {
			return "/usr/local/bin/claude", nil
		},
	}

	installed, binaryPath, cfgPath, configFound, err := a.Detect(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	if !installed {
		t.Error("expected installed=true")
	}
	if binaryPath != "/usr/local/bin/claude" {
		t.Errorf("expected /usr/local/bin/claude, got %s", binaryPath)
	}
	if cfgPath != configDir {
		t.Errorf("expected %s, got %s", configDir, cfgPath)
	}
	if !configFound {
		t.Error("expected configFound=true")
	}
}

func TestDetectWithoutBinary(t *testing.T) {
	tmpDir := t.TempDir()

	a := &Adapter{
		lookPath: func(name string) (string, error) {
			return "", &os.PathError{Op: "lookpath", Path: name, Err: os.ErrNotExist}
		},
	}

	installed, _, _, _, err := a.Detect(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	if installed {
		t.Error("expected installed=false")
	}
}
