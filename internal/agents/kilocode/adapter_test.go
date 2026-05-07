package kilocode

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/model"
)

func TestAgentIdentity(t *testing.T) {
	a := NewAdapter()
	if a.Agent() != model.AgentKilocode {
		t.Errorf("Agent() = %v, want %v", a.Agent(), model.AgentKilocode)
	}
}

func TestPaths(t *testing.T) {
	a := NewAdapter()
	home := "/home/user"
	want := filepath.Join(home, ".config", "kilo")

	if got := a.GlobalConfigDir(home); got != want {
		t.Errorf("GlobalConfigDir = %q, want %q", got, want)
	}
	if got := a.SystemPromptFile(home); got != filepath.Join(want, "AGENTS.md") {
		t.Errorf("SystemPromptFile = %q", got)
	}
	if got := a.MCPConfigPath(home, "cortex"); got != filepath.Join(want, "opencode.json") {
		t.Errorf("MCPConfigPath = %q", got)
	}
}

func TestDetect_DirMissing(t *testing.T) {
	a := &Adapter{
		lookPath: func(string) (string, error) { return "/usr/local/bin/kilo", nil },
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
