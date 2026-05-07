package kimi

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/model"
	"github.com/lleontor705/cortex-ia/internal/system"
)

func TestAgentIdentity(t *testing.T) {
	a := NewAdapter()
	if a.Agent() != model.AgentKimi {
		t.Errorf("Agent() = %v, want %v", a.Agent(), model.AgentKimi)
	}
}

func TestPaths(t *testing.T) {
	a := NewAdapter()
	home := "/home/user"

	if got := a.GlobalConfigDir(home); got != filepath.Join(home, ".kimi") {
		t.Errorf("GlobalConfigDir = %q", got)
	}
	if got := a.SystemPromptFile(home); got != filepath.Join(home, ".kimi", "KIMI.md") {
		t.Errorf("SystemPromptFile = %q", got)
	}
	if got := a.SettingsPath(home); got != filepath.Join(home, ".kimi", "config.toml") {
		t.Errorf("SettingsPath = %q", got)
	}
	// Skills uses cross-agent shared path.
	if got := a.SkillsDir(home); got != filepath.Join(home, ".config", "agents", "skills") {
		t.Errorf("SkillsDir = %q (expected shared path)", got)
	}
	if got := a.MCPConfigPath(home, "cortex"); got != filepath.Join(home, ".kimi", "mcp.json") {
		t.Errorf("MCPConfigPath = %q", got)
	}
	if got := a.SubAgentsDir(home); got != filepath.Join(home, ".kimi", "agents") {
		t.Errorf("SubAgentsDir = %q", got)
	}
}

func TestInstallCommands(t *testing.T) {
	a := NewAdapter()
	cmds := a.InstallCommands(system.PlatformProfile{})
	if len(cmds) != 1 || cmds[0][0] != "uv" {
		t.Errorf("expected uv tool install, got %v", cmds)
	}
}

func TestDetect_DirMissing(t *testing.T) {
	a := &Adapter{
		lookPath:    func(string) (string, error) { return "/usr/local/bin/kimi", nil },
		statPath:    func(string) statResult { return statResult{err: os.ErrNotExist} },
		pathExists:  func(string) bool { return false },
		userHomeDir: func() (string, error) { return "/home/test", nil },
	}
	installed, _, _, configFound, err := a.Detect("/home/test")
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if !installed {
		t.Error("expected installed=true when binary present")
	}
	if configFound {
		t.Error("expected configFound=false when dir missing")
	}
}
