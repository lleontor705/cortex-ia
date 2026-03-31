package theme

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/agents/claude"
	"github.com/lleontor705/cortex-ia/internal/agents/opencode"
	"github.com/lleontor705/cortex-ia/internal/agents/windsurf"
)

func TestInject_Claude(t *testing.T) {
	tmpDir := t.TempDir()
	adapter := claude.NewAdapter()

	result, err := Inject(tmpDir, adapter, ThemeCortex)
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

	themeSection, ok := settings["theme"].(map[string]any)
	if !ok {
		t.Fatal("expected theme section")
	}
	if themeSection["name"] != "cortex" {
		t.Errorf("theme name = %v, want cortex", themeSection["name"])
	}
}

func TestInject_OpenCode(t *testing.T) {
	tmpDir := t.TempDir()
	adapter := opencode.NewAdapter()

	result, err := Inject(tmpDir, adapter, ThemeCortex)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed {
		t.Error("expected Changed=true")
	}
}

func TestInject_NoSettingsPath(t *testing.T) {
	tmpDir := t.TempDir()
	adapter := windsurf.NewAdapter()

	// Windsurf returns empty SettingsPath → should skip.
	result, err := Inject(tmpDir, adapter, ThemeCortex)
	if err != nil {
		t.Fatal(err)
	}
	if result.Changed {
		t.Error("expected no change for agent without settings")
	}
}

func TestInject_DefaultTheme(t *testing.T) {
	tmpDir := t.TempDir()
	adapter := claude.NewAdapter()

	// Empty theme → defaults to cortex.
	result, err := Inject(tmpDir, adapter, "")
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed {
		t.Error("expected Changed=true with default theme")
	}
}

func TestInject_InvalidTheme(t *testing.T) {
	tmpDir := t.TempDir()
	adapter := claude.NewAdapter()

	_, err := Inject(tmpDir, adapter, "nonexistent")
	if err == nil {
		t.Fatal("expected error for invalid theme")
	}
}

func TestInject_Idempotent(t *testing.T) {
	tmpDir := t.TempDir()
	adapter := claude.NewAdapter()

	if _, err := Inject(tmpDir, adapter, ThemeCortex); err != nil {
		t.Fatal(err)
	}
	second, err := Inject(tmpDir, adapter, ThemeCortex)
	if err != nil {
		t.Fatal(err)
	}
	if second.Changed {
		t.Error("expected idempotent")
	}
}

func TestInject_MergesWithExisting(t *testing.T) {
	tmpDir := t.TempDir()
	adapter := claude.NewAdapter()

	// Pre-write some settings.
	settingsPath := adapter.SettingsPath(tmpDir)
	os.MkdirAll(settingsPath[:len(settingsPath)-len("settings.json")-1], 0o755)
	os.WriteFile(settingsPath, []byte(`{"existing": "value"}`), 0o644)

	if _, err := Inject(tmpDir, adapter, ThemeCortex); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(settingsPath)
	var settings map[string]any
	json.Unmarshal(data, &settings)

	if settings["existing"] != "value" {
		t.Error("expected existing settings to be preserved")
	}
	if _, ok := settings["theme"]; !ok {
		t.Error("expected theme to be injected")
	}
}
