package mailbox

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/agents/opencode"
)

func TestInjectMailbox_OpenCode(t *testing.T) {
	tmpDir := t.TempDir()
	adapter := opencode.NewAdapter()

	result, err := Inject(tmpDir, adapter)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed {
		t.Error("expected Changed=true")
	}

	path := adapter.SettingsPath(tmpDir)
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var m map[string]any
	if err := json.Unmarshal(content, &m); err != nil {
		t.Fatal(err)
	}
}
