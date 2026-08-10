package context7

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/agents/opencode"
)

func TestInjectContext7_OpenCode(t *testing.T) {
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

func TestTemplates_Context7Name(t *testing.T) {
	tmpl := Templates()
	if tmpl.Name != "context7" {
		t.Errorf("Name = %s", tmpl.Name)
	}
	if tmpl.OpenCodeOverlayJSON == nil {
		t.Error("expected OpenCode remote overlay")
	}
}
