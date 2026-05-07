package qwen

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/model"
	"github.com/lleontor705/cortex-ia/internal/system"
)

func TestAgentIdentity(t *testing.T) {
	a := NewAdapter()
	if a.Agent() != model.AgentQwenCode {
		t.Errorf("Agent() = %v, want %v", a.Agent(), model.AgentQwenCode)
	}
}

func TestPaths(t *testing.T) {
	a := NewAdapter()
	home := "/home/user"

	if got := a.GlobalConfigDir(home); got != filepath.Join(home, ".qwen") {
		t.Errorf("GlobalConfigDir = %q", got)
	}
	if got := a.SystemPromptFile(home); got != filepath.Join(home, ".qwen", "QWEN.md") {
		t.Errorf("SystemPromptFile = %q", got)
	}
	if got := a.SettingsPath(home); got != filepath.Join(home, ".qwen", "settings.json") {
		t.Errorf("SettingsPath = %q", got)
	}
}

func TestInstallCommands(t *testing.T) {
	a := NewAdapter()
	cmds := a.InstallCommands(system.PlatformProfile{OS: "linux"})
	if len(cmds) != 1 || cmds[0][0] != "npm" {
		t.Errorf("expected npm install, got %v", cmds)
	}
}

func TestDetect_DirMissing(t *testing.T) {
	a := &Adapter{
		lookPath: func(string) (string, error) { return "/usr/local/bin/qwen", nil },
		statPath: func(string) statResult { return statResult{err: os.ErrNotExist} },
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
