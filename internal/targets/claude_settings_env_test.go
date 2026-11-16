package targets

import (
	"os"
	"path/filepath"
	"testing"
)

// The Claude settings env marker must be installed, preserve user keys, and be
// removed on uninstall.
func TestClaudeSettingsAgentShellMarkerLifecycle(t *testing.T) {
	home := t.TempDir()
	settings := filepath.Join(home, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settings), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(settings, []byte(`{"env": {"USER_KEY": "keep"}, "theme": "dark"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := InstallClaudeWith(home, ApplyOptions{}); err != nil {
		t.Fatalf("InstallClaudeWith: %v", err)
	}
	m := readJSONMap(t, settings)
	env, ok := m["env"].(map[string]any)
	if !ok || env["CORTEX_IA_AGENT_SHELL"] != "1" || env["USER_KEY"] != "keep" {
		t.Fatalf("after install env = %v, want marker plus user key", m["env"])
	}

	if _, err := UninstallClaude(home, false); err != nil {
		t.Fatalf("UninstallClaude: %v", err)
	}
	m = readJSONMap(t, settings)
	if m["theme"] != "dark" {
		t.Errorf("unrelated key lost: %v", m)
	}
	env, _ = m["env"].(map[string]any)
	if _, present := env["CORTEX_IA_AGENT_SHELL"]; present {
		t.Errorf("marker survived uninstall: %v", env)
	}
	if env["USER_KEY"] != "keep" {
		t.Errorf("user env key lost: %v", env)
	}
}
