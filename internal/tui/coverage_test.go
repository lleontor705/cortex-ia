package tui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/model"
)

// ---------------------------------------------------------------------------
// KeyMap ShortHelp/FullHelp coverage
// ---------------------------------------------------------------------------

func TestKeyMaps_ShortHelpAndFullHelp(t *testing.T) {
	cases := []struct {
		name string
		km   interface {
			ShortHelp() []interface{ Keys() []string }
		}
	}{}
	_ = cases // not used; direct tests below

	// CheckboxKeyMap
	ck := CheckboxKeyMap{}
	if len(ck.ShortHelp()) == 0 {
		t.Error("CheckboxKeyMap ShortHelp empty")
	}

	// BackupKeyMap
	bk := BackupKeyMap{}
	if bk.ShortHelp() == nil {
		t.Error("BackupKeyMap ShortHelp nil")
	}
	if bk.FullHelp() == nil {
		t.Error("BackupKeyMap FullHelp nil")
	}

	// InputKeyMap
	ik := InputKeyMap{}
	if ik.ShortHelp() == nil {
		t.Error("InputKeyMap ShortHelp nil")
	}
	if ik.FullHelp() == nil {
		t.Error("InputKeyMap FullHelp nil")
	}

	// WelcomeKeyMap
	wk := WelcomeKeyMap{}
	if wk.ShortHelp() == nil {
		t.Error("WelcomeKeyMap ShortHelp nil")
	}
	if wk.FullHelp() == nil {
		t.Error("WelcomeKeyMap FullHelp nil")
	}

	// NavigateKeyMap
	nk := NavigateKeyMap{}
	if nk.ShortHelp() == nil {
		t.Error("NavigateKeyMap ShortHelp nil")
	}
}

// ---------------------------------------------------------------------------
// Init
// ---------------------------------------------------------------------------

func TestModel_Init_ReturnsBatch(t *testing.T) {
	m := New(nil, "/tmp", "test")
	cmd := m.Init()
	if cmd == nil {
		t.Error("Init should return non-nil cmd batch")
	}
}

// ---------------------------------------------------------------------------
// ModelConfig navigation
// ---------------------------------------------------------------------------

func TestModelConfig_TabSwitch(t *testing.T) {
	m := New(nil, "/tmp", "test")
	m.Screen = ScreenModelConfig
	m.ModelConfigTab = ModelConfigTabClaude

	m = updateModel(t, m, "tab")
	if m.ModelConfigTab != ModelConfigTabOpenCode {
		t.Errorf("expected OpenCode tab after Tab, got %v", m.ModelConfigTab)
	}
}

func TestModelConfig_Esc(t *testing.T) {
	m := New(nil, "/tmp", "test")
	m.Screen = ScreenModelConfig

	m = updateModel(t, m, "esc")
	if m.Screen != ScreenWelcome {
		t.Errorf("Screen = %v, want ScreenWelcome", m.Screen)
	}
}

func TestModelConfigClaude_EnterAppliesPreset(t *testing.T) {
	m := New(nil, "/tmp", "test")
	m.Screen = ScreenModelConfig
	m.ModelConfigTab = ModelConfigTabClaude
	m.ClaudeModelCursor = 0 // Opus → Performance

	result, cmd := m.Update(keyMsg("enter"))
	rm := result.(Model)
	if rm.ModelPreset != model.ModelPresetPerformance {
		t.Errorf("ModelPreset = %v, want Performance", rm.ModelPreset)
	}
	if cmd == nil {
		t.Error("expected toast dismiss cmd")
	}
}

func TestModelConfigClaude_UpDown(t *testing.T) {
	m := New(nil, "/tmp", "test")
	m.Screen = ScreenModelConfig
	m.ModelConfigTab = ModelConfigTabClaude
	m.ClaudeModelCursor = 0

	m = updateModel(t, m, "down")
	if m.ClaudeModelCursor != 1 {
		t.Errorf("cursor = %d, want 1", m.ClaudeModelCursor)
	}
	m = updateModel(t, m, "up")
	if m.ClaudeModelCursor != 0 {
		t.Errorf("cursor = %d, want 0", m.ClaudeModelCursor)
	}
}

// ---------------------------------------------------------------------------
// OpenCode Models
// ---------------------------------------------------------------------------

func TestAssignOCModel(t *testing.T) {
	m := New(nil, "/tmp", "test")
	m.OCSelectedAgent = "create"
	m.assignOCModel("anthropic/claude-sonnet-4")

	got, ok := m.OpenCodeAssignments["create"]
	if !ok {
		t.Fatal("assignment not set")
	}
	if got.Provider != "anthropic" {
		t.Errorf("Provider = %q, want anthropic", got.Provider)
	}
	if got.Model != "claude-sonnet-4" {
		t.Errorf("Model = %q, want claude-sonnet-4", got.Model)
	}
}

func TestAssignOCModel_InvalidFormat(t *testing.T) {
	m := New(nil, "/tmp", "test")
	m.OCSelectedAgent = "create"
	m.assignOCModel("no-slash-model")

	if _, ok := m.OpenCodeAssignments["create"]; ok {
		t.Error("assignment should not be set for invalid format")
	}
}

func TestVisibleOCModels_NoFilter(t *testing.T) {
	m := New(nil, "/tmp", "test")
	m.OCFlatModels = []string{"a/b", "c/d", "e/f"}

	visible := m.visibleOCModels()
	if len(visible) != 3 {
		t.Errorf("expected 3 models, got %d", len(visible))
	}
}

func TestVisibleOCModels_WithFilter(t *testing.T) {
	m := New(nil, "/tmp", "test")
	m.OCFlatModels = []string{"anthropic/opus", "openai/gpt-4", "anthropic/sonnet"}
	m.OCModelFilter.Input.SetValue("anthropic")

	visible := m.visibleOCModels()
	if len(visible) != 2 {
		t.Errorf("expected 2 anthropic models, got %d", len(visible))
	}
}

func TestUpdateOpenCodeModelPicker_Navigation(t *testing.T) {
	m := New(nil, "/tmp", "test")
	m.Screen = ScreenOpenCodeModelPicker
	m.OCFlatModels = []string{"a/1", "a/2", "a/3"}
	m.OCModelCursor = 0

	m = updateModel(t, m, "down")
	if m.OCModelCursor != 1 {
		t.Errorf("cursor = %d, want 1", m.OCModelCursor)
	}

	m = updateModel(t, m, "up")
	if m.OCModelCursor != 0 {
		t.Errorf("cursor = %d, want 0", m.OCModelCursor)
	}
}

func TestUpdateOpenCodeModelPicker_Esc(t *testing.T) {
	m := New(nil, "/tmp", "test")
	m.Screen = ScreenOpenCodeModelPicker
	m.OCFlatModels = []string{"a/1"}

	m = updateModel(t, m, "esc")
	if m.Screen != ScreenModelConfig {
		t.Errorf("Screen = %v, want ScreenModelConfig", m.Screen)
	}
}

func TestUpdateOpenCodeModelPicker_EnterSelects(t *testing.T) {
	m := New(nil, "/tmp", "test")
	m.Screen = ScreenOpenCodeModelPicker
	m.OCFlatModels = []string{"anthropic/sonnet"}
	m.OCModelCursor = 0
	m.OCSelectedAgent = "create"

	m = updateModel(t, m, "enter")
	if m.Screen != ScreenModelConfig {
		t.Errorf("Screen = %v, want ScreenModelConfig", m.Screen)
	}
	if _, ok := m.OpenCodeAssignments["create"]; !ok {
		t.Error("assignment should be set after Enter")
	}
}

// ---------------------------------------------------------------------------
// View (full render)
// ---------------------------------------------------------------------------

func TestView_FullRender(t *testing.T) {
	m := New(nil, "/tmp", "1.0.0")
	m.Screen = ScreenWelcome
	m.Width = 80
	m.Height = 24

	out := m.View()
	if out == "" {
		t.Error("View should render non-empty output")
	}
}

func TestView_Quitting(t *testing.T) {
	m := New(nil, "/tmp", "test")
	m.Quitting = true

	out := m.View()
	if out != "" {
		t.Error("View should return empty when Quitting")
	}
}

// ---------------------------------------------------------------------------
// readSkillDescription
// ---------------------------------------------------------------------------

func TestReadSkillDescription(t *testing.T) {
	homeDir := t.TempDir()
	skillDir := filepath.Join(homeDir, ".cortex-ia", "skills-community", "test-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\nname: test\n---\n\n# Heading\n\nFirst real line of description.\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	desc := readSkillDescription(homeDir, "test-skill")
	if desc != "First real line of description." {
		t.Errorf("desc = %q, want %q", desc, "First real line of description.")
	}
}

func TestReadSkillDescription_MissingFile(t *testing.T) {
	desc := readSkillDescription(t.TempDir(), "nonexistent")
	if desc != "" {
		t.Errorf("missing file should return empty, got %q", desc)
	}
}

func TestReadSkillDescription_Truncates(t *testing.T) {
	homeDir := t.TempDir()
	skillDir := filepath.Join(homeDir, ".cortex-ia", "skills-community", "long-skill")
	_ = os.MkdirAll(skillDir, 0o755)
	longLine := "This is a very long description that definitely exceeds sixty characters for truncation test"
	content := longLine + "\n"
	_ = os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644)

	desc := readSkillDescription(homeDir, "long-skill")
	if len(desc) > 60 {
		t.Errorf("desc length = %d, want <= 60", len(desc))
	}
}

// ---------------------------------------------------------------------------
// dismissToastAfter
// ---------------------------------------------------------------------------

func TestDismissToastAfter(t *testing.T) {
	cmd := dismissToastAfter(1)
	if cmd == nil {
		t.Error("dismissToastAfter returned nil")
	}
}

// ---------------------------------------------------------------------------
// Update: global shortcuts paths
// ---------------------------------------------------------------------------

// Note: ctrl+m cannot be tested directly — in terminals it shares byte 0x0D with Enter.
// The implementation is still present but shadowed by Enter. Consider changing the shortcut.
