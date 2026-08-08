package permissions

import (
	"encoding/json"
	"os"
	"path/filepath"
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
	settingsPath := adapter.SettingsPath(tmpDir)
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(settingsPath, []byte(`{"permissions":{"bash":"allow","file":"allow"},"share":"disabled"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := Inject(tmpDir, adapter)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed {
		t.Error("expected Changed=true")
	}

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}

	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatal(err)
	}

	if _, exists := settings["permissions"]; exists {
		t.Fatal("deprecated permissions key must not be emitted")
	}
	perms, ok := settings["permission"].(map[string]any)
	if !ok {
		t.Fatal("expected permission key")
	}
	if _, exists := perms["file"]; exists {
		t.Fatal("deprecated file permission must not be emitted")
	}
	bash, ok := perms["bash"].(map[string]any)
	if !ok {
		t.Fatal("expected ordered bash permission rules for OpenCode")
	}
	if bash["*"] != "ask" || bash["rm -rf /"] != "deny" || bash["sudo rm -rf *"] != "deny" {
		t.Errorf("bash permission rules = %#v", bash)
	}
	read, ok := perms["read"].(map[string]any)
	if !ok {
		t.Fatal("expected read permission rules for OpenCode")
	}
	if read["*"] != "allow" || read[".env"] != "deny" || read[".env.*"] != "deny" || read[".env.example"] != "allow" {
		t.Errorf("read permission rules = %#v", read)
	}
	serialized := string(data)
	for _, ordered := range [][2]string{
		{`"*":"ask"`, `"rm -rf /":"deny"`},
		{`".env.*":"deny"`, `".env.example":"allow"`},
	} {
		if broad, specific := strings.Index(serialized, ordered[0]), strings.Index(serialized, ordered[1]); broad < 0 || specific < 0 || broad >= specific {
			t.Errorf("permission rule order %q before %q not preserved:\n%s", ordered[0], ordered[1], serialized)
		}
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
