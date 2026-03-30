package context7

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/agents/claude"
)

func TestInjectContext7_ClaudeCode(t *testing.T) {
	tmpDir := t.TempDir()
	adapter := claude.NewAdapter()

	result, err := Inject(tmpDir, adapter)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed {
		t.Error("expected Changed=true")
	}

	path := filepath.Join(tmpDir, ".claude", "mcp", "context7.json")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var m map[string]any
	if err := json.Unmarshal(content, &m); err != nil {
		t.Fatal(err)
	}
	if m["command"] != "npx" {
		t.Errorf("command = %v, want npx", m["command"])
	}
	args := m["args"].([]any)
	if len(args) != 2 || args[1] != "@upstash/context7-mcp" {
		t.Errorf("args = %v", args)
	}
}

func TestTemplates_Context7Name(t *testing.T) {
	tmpl := Templates()
	if tmpl.Name != "context7" {
		t.Errorf("Name = %s", tmpl.Name)
	}
	if tmpl.OpenCodeOverlayJSON == nil {
		t.Error("expected OpenCode remote overlay")
	}
	if tmpl.AntigravityOverlayJSON == nil {
		t.Error("expected Antigravity overlay")
	}
}
