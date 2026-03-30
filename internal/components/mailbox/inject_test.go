package mailbox

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/agents/claude"
)

func TestInjectMailbox_ClaudeCode(t *testing.T) {
	tmpDir := t.TempDir()
	adapter := claude.NewAdapter()

	result, err := Inject(tmpDir, adapter)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed {
		t.Error("expected Changed=true")
	}

	path := filepath.Join(tmpDir, ".claude", "mcp", "agent-mailbox.json")
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
	if len(args) != 2 || args[1] != "agent-mailbox-mcp" {
		t.Errorf("args = %v", args)
	}
}
