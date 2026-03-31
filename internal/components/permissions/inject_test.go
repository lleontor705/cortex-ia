package permissions

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/agents/claude"
	"github.com/lleontor705/cortex-ia/internal/agents/codex"
	"github.com/lleontor705/cortex-ia/internal/agents/opencode"
)

func TestInject_Claude(t *testing.T) {
	tmpDir := t.TempDir()
	adapter := claude.NewAdapter()

	result, err := Inject(tmpDir, adapter)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed {
		t.Error("expected Changed=true")
	}

	data, err := os.ReadFile(adapter.SettingsPath(tmpDir))
	if err != nil {
		t.Fatal(err)
	}

	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatal(err)
	}

	perms, ok := settings["permissions"].(map[string]any)
	if !ok {
		t.Fatal("expected permissions key")
	}
	deny, ok := perms["deny"].([]any)
	if !ok || len(deny) == 0 {
		t.Error("expected deny list")
	}
}

func TestInject_OpenCode(t *testing.T) {
	tmpDir := t.TempDir()
	adapter := opencode.NewAdapter()

	result, err := Inject(tmpDir, adapter)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed {
		t.Error("expected Changed=true")
	}

	data, err := os.ReadFile(adapter.SettingsPath(tmpDir))
	if err != nil {
		t.Fatal(err)
	}

	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatal(err)
	}

	perms, ok := settings["permissions"].(map[string]any)
	if !ok {
		t.Fatal("expected permissions key")
	}
	if _, ok := perms["bash"]; !ok {
		t.Error("expected bash deny rules for OpenCode")
	}
}

func TestInject_Codex_PromptBased(t *testing.T) {
	tmpDir := t.TempDir()
	adapter := codex.NewAdapter()

	result, err := Inject(tmpDir, adapter)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed {
		t.Error("expected Changed=true")
	}

	promptFile := adapter.SystemPromptFile(tmpDir)
	data, err := os.ReadFile(promptFile)
	if err != nil {
		t.Fatal(err)
	}

	content := string(data)
	if !strings.Contains(content, "cortex-ia:cortex-permissions") {
		t.Error("expected permissions marker")
	}
	if !strings.Contains(content, ".env") {
		t.Error("expected .env in deny list")
	}
}

func TestInject_Idempotent(t *testing.T) {
	tmpDir := t.TempDir()
	adapter := claude.NewAdapter()

	if _, err := Inject(tmpDir, adapter); err != nil {
		t.Fatal(err)
	}
	second, err := Inject(tmpDir, adapter)
	if err != nil {
		t.Fatal(err)
	}
	if second.Changed {
		t.Error("expected idempotent")
	}
}
