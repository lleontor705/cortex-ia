package kiro

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/model"
	"github.com/lleontor705/cortex-ia/internal/system"
)

func TestAgentIdentity(t *testing.T) {
	a := NewAdapter()
	if a.Agent() != model.AgentKiroIDE {
		t.Errorf("Agent() = %v, want %v", a.Agent(), model.AgentKiroIDE)
	}
}

func TestPaths(t *testing.T) {
	a := NewAdapter()
	home := "/home/user"

	if got := a.SystemPromptFile(home); got != filepath.Join(home, ".kiro", "steering", "cortex-ia.md") {
		t.Errorf("SystemPromptFile = %q", got)
	}
	if got := a.SkillsDir(home); got != filepath.Join(home, ".kiro", "skills") {
		t.Errorf("SkillsDir = %q", got)
	}
	if got := a.MCPConfigPath(home, "cortex"); got != filepath.Join(home, ".kiro", "settings", "mcp.json") {
		t.Errorf("MCPConfigPath = %q", got)
	}
	if got := a.SubAgentsDir(home); got != filepath.Join(home, ".kiro", "agents") {
		t.Errorf("SubAgentsDir = %q", got)
	}
}

func TestInstallCommandsReturnsNil(t *testing.T) {
	a := NewAdapter()
	if cmds := a.InstallCommands(system.PlatformProfile{}); cmds != nil {
		t.Errorf("InstallCommands = %v, want nil (kiro is not auto-installable)", cmds)
	}
	if a.SupportsAutoInstall() {
		t.Error("SupportsAutoInstall = true, want false")
	}
}

func TestKiroConfigDir_Platform(t *testing.T) {
	a := NewAdapter()
	got := a.GlobalConfigDir("/home/user")

	switch runtime.GOOS {
	case "darwin":
		if got != "/home/user/Library/Application Support/Kiro/User" {
			t.Errorf("darwin GlobalConfigDir = %q", got)
		}
	case "windows":
		// Windows result depends on APPDATA; just ensure non-empty.
		if got == "" {
			t.Error("windows GlobalConfigDir is empty")
		}
	default:
		// Linux respects XDG_CONFIG_HOME; default ~/.config/kiro/user.
		// Override for this assertion to keep it deterministic.
		_ = os.Unsetenv("XDG_CONFIG_HOME")
		got = a.GlobalConfigDir("/home/user")
		if got != "/home/user/.config/kiro/user" {
			t.Errorf("linux GlobalConfigDir = %q", got)
		}
	}
}

func TestDetect_BinaryNotFound(t *testing.T) {
	a := &Adapter{
		lookPath: func(string) (string, error) { return "", exec.ErrNotFound },
		statPath: os.Stat,
	}
	installed, _, _, _, err := a.Detect("/home/test")
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if installed {
		t.Error("expected installed=false when binary not found")
	}
}

func TestAgentNotInstallableError(t *testing.T) {
	err := AgentNotInstallableError{Agent: model.AgentKiroIDE}
	if err.Error() == "" {
		t.Error("expected non-empty error message")
	}
}
