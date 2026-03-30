package cortex

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/agents/claude"
)

func TestInjectCortex_ClaudeCode(t *testing.T) {
	tmpDir := t.TempDir()
	adapter := claude.NewAdapter()

	result, err := Inject(tmpDir, adapter)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed {
		t.Error("expected Changed=true")
	}

	path := filepath.Join(tmpDir, ".claude", "mcp", "cortex.json")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var m map[string]any
	if err := json.Unmarshal(content, &m); err != nil {
		t.Fatal(err)
	}
	if m["command"] != "cortex" {
		t.Errorf("command = %v, want cortex", m["command"])
	}
	args := m["args"].([]any)
	if len(args) != 1 || args[0] != "mcp" {
		t.Errorf("args = %v, want [mcp]", args)
	}
}

func TestTemplates_CortexIsGoBinary(t *testing.T) {
	tmpl := Templates()
	if tmpl.Name != "cortex" {
		t.Errorf("Name = %s", tmpl.Name)
	}
	if tmpl.TOMLCommand != "cortex" {
		t.Errorf("TOMLCommand = %s, want cortex (Go binary, not npx)", tmpl.TOMLCommand)
	}
}
