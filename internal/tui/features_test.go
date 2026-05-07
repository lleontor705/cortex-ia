package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lleontor705/cortex-ia/internal/agents"
	"github.com/lleontor705/cortex-ia/internal/backup"
	"github.com/lleontor705/cortex-ia/internal/model"
	"github.com/lleontor705/cortex-ia/internal/state"
)

// ---------------------------------------------------------------------------
// Review: Preset toggle (Issue C fix)
// ---------------------------------------------------------------------------

func TestReview_SpaceTogglesPreset(t *testing.T) {
	m := New(nil, "/tmp", "test")
	m.Screen = ScreenReview
	m.Preset = model.PresetFull
	m.Cursor = 0 // reviewCursorPreset

	m = updateModel(t, m, " ")
	if m.Preset != model.PresetMinimal {
		t.Errorf("Preset = %q, want %q (Minimal) after toggle", m.Preset, model.PresetMinimal)
	}

	m = updateModel(t, m, " ")
	if m.Preset != model.PresetFull {
		t.Errorf("Preset = %q, want %q (Full) after second toggle", m.Preset, model.PresetFull)
	}
}

func TestReview_PresetToggleReResolvesComponents(t *testing.T) {
	m := New(nil, "/tmp", "test")
	m.Screen = ScreenReview
	m.Preset = model.PresetFull
	m.Cursor = 0

	m = updateModel(t, m, " ")
	if m.Resolved == nil {
		t.Error("Resolved should be populated after preset toggle")
	}
}

// ---------------------------------------------------------------------------
// Review: navigation covers 4 positions (Preset, SDD, TDD, Confirm)
// ---------------------------------------------------------------------------

func TestReview_NavigationBoundaries(t *testing.T) {
	m := New(nil, "/tmp", "test")
	m.Screen = ScreenReview
	m.Cursor = 0

	// Down 3 times: 0 → 1 → 2 → 3
	m = updateModel(t, m, "down")
	m = updateModel(t, m, "down")
	m = updateModel(t, m, "down")
	if m.Cursor != reviewCursorConfirm {
		t.Errorf("Cursor = %d, want %d (confirm)", m.Cursor, reviewCursorConfirm)
	}

	// Down again: should clamp at Confirm
	m = updateModel(t, m, "down")
	if m.Cursor != reviewCursorConfirm {
		t.Errorf("Cursor after extra down = %d, want %d (clamped)", m.Cursor, reviewCursorConfirm)
	}
}

// ---------------------------------------------------------------------------
// Detection: Quick Install (f key)
// ---------------------------------------------------------------------------

func TestDetection_FKeyEnablesQuickInstall(t *testing.T) {
	m := New(nil, "/tmp", "test")
	m.Screen = ScreenDetection

	m = updateModel(t, m, "f")
	if !m.QuickInstall {
		t.Error("QuickInstall should be true after pressing f")
	}
	if m.Screen != ScreenAgents {
		t.Errorf("Screen = %v, want ScreenAgents", m.Screen)
	}
}

func TestDetection_EnterDisablesQuickInstall(t *testing.T) {
	m := New(nil, "/tmp", "test")
	m.Screen = ScreenDetection
	m.QuickInstall = true

	m = updateModel(t, m, "enter")
	if m.QuickInstall {
		t.Error("QuickInstall should be false after Enter (customize path)")
	}
}

// ---------------------------------------------------------------------------
// Agents: Quick Install skips to Review
// ---------------------------------------------------------------------------

func TestAgents_QuickInstallSkipsToReview(t *testing.T) {
	m := New(nil, t.TempDir(), "test")
	m.Screen = ScreenAgents
	m.QuickInstall = true
	m.Agents = []AgentItem{
		{ID: model.AgentClaudeCode, Selected: true},
	}

	m = updateModel(t, m, "enter")
	if m.Screen != ScreenReview {
		t.Errorf("Screen = %v, want ScreenReview (quick install skips to Review)", m.Screen)
	}
	// Quick install should populate defaults
	if m.Persona != model.PersonaProfessional {
		t.Errorf("Persona = %v, want PersonaProfessional", m.Persona)
	}
	if m.ModelPreset != model.ModelPresetBalanced {
		t.Errorf("ModelPreset = %v, want Balanced", m.ModelPreset)
	}
}

func TestAgents_CustomizeGoesToPersona(t *testing.T) {
	m := New(nil, "/tmp", "test")
	m.Screen = ScreenAgents
	m.QuickInstall = false
	m.Agents = []AgentItem{
		{ID: model.AgentClaudeCode, Selected: true},
	}

	m = updateModel(t, m, "enter")
	if m.Screen != ScreenPersona {
		t.Errorf("Screen = %v, want ScreenPersona", m.Screen)
	}
}

// ---------------------------------------------------------------------------
// Agents: Validation error on empty selection
// ---------------------------------------------------------------------------

func TestAgents_EnterWithNoSelectionShowsError(t *testing.T) {
	m := New(nil, "/tmp", "test")
	m.Screen = ScreenAgents
	m.Agents = []AgentItem{{ID: model.AgentClaudeCode, Selected: false}}

	m = updateModel(t, m, "enter")
	if m.Screen != ScreenAgents {
		t.Errorf("Screen = %v, want ScreenAgents (should stay)", m.Screen)
	}
	if m.ValidationErr == "" {
		t.Error("ValidationErr should be set when no agents selected")
	}
}

func TestAgents_InputClearsValidationErr(t *testing.T) {
	m := New(nil, "/tmp", "test")
	m.Screen = ScreenAgents
	m.ValidationErr = "previous error"
	m.Agents = []AgentItem{{ID: model.AgentClaudeCode, Selected: true}}

	m = updateModel(t, m, "up")
	if m.ValidationErr != "" {
		t.Errorf("ValidationErr should be cleared on input, got %q", m.ValidationErr)
	}
}

// ---------------------------------------------------------------------------
// Persona: auto-skip when only 1 persona
// ---------------------------------------------------------------------------

func TestPersona_AutoSkipsWhenOnlyOne(t *testing.T) {
	m := New(nil, "/tmp", "test")
	m.Screen = ScreenPersona
	m.Personas = []model.PersonaID{model.PersonaProfessional}

	// Any message should trigger auto-skip
	m = updateModel(t, m, "up")
	if m.Screen != ScreenClaudeModelPicker {
		t.Errorf("Screen = %v, want ScreenClaudeModelPicker (auto-skip)", m.Screen)
	}
}

// ---------------------------------------------------------------------------
// Complete: undo restores backup
// ---------------------------------------------------------------------------

func TestComplete_U_TriggersRestore(t *testing.T) {
	restored := false
	m := New(nil, "/tmp", "test")
	m.Screen = ScreenComplete
	m.Result.BackupID = "bk-123"
	m.RestoreFn = func(_ backup.Manifest) error {
		restored = true
		return nil
	}
	m.ListBackupsFn = func() ([]backup.Manifest, []string) {
		return []backup.Manifest{{ID: "bk-123"}}, nil
	}

	result, cmd := m.Update(keyMsg("u"))
	rm := result.(Model)
	if !rm.OperationRunning {
		t.Error("OperationRunning should be true after u")
	}
	if cmd == nil {
		t.Fatal("expected non-nil cmd for restore")
	}
	msg := cmd()
	if !restored {
		t.Error("RestoreFn should have been called")
	}
	if _, ok := msg.(BackupRestoreMsg); !ok {
		t.Errorf("expected BackupRestoreMsg, got %T", msg)
	}
}

func TestComplete_U_NoBackupNoOp(t *testing.T) {
	m := New(nil, "/tmp", "test")
	m.Screen = ScreenComplete
	m.Result.BackupID = "" // no backup

	m = updateModel(t, m, "u")
	if m.OperationRunning {
		t.Error("OperationRunning should remain false when no backup")
	}
}

// ---------------------------------------------------------------------------
// Welcome: new menu options
// ---------------------------------------------------------------------------

func TestWelcome_MaintenanceOption(t *testing.T) {
	reg := agents.NewRegistry()
	m := New(reg, t.TempDir(), "1.0.0")
	m.Screen = ScreenWelcome
	m.Cursor = int(WelcomeMaintenance)

	m = updateModel(t, m, "enter")
	if m.Screen != ScreenMaintenance {
		t.Errorf("Screen = %v, want ScreenMaintenance", m.Screen)
	}
}

func TestWelcome_ModelConfigOption(t *testing.T) {
	reg := agents.NewRegistry()
	m := New(reg, "/tmp", "1.0.0")
	m.Screen = ScreenWelcome
	m.Cursor = int(WelcomeModelConfig)

	m = updateModel(t, m, "enter")
	if m.Screen != ScreenModelConfig {
		t.Errorf("Screen = %v, want ScreenModelConfig", m.Screen)
	}
}

// ---------------------------------------------------------------------------
// Global shortcuts: ctrl+b, ctrl+m
// ---------------------------------------------------------------------------

func TestGlobalShortcut_CtrlB_JumpsToBackups(t *testing.T) {
	m := New(nil, "/tmp", "test")
	m.Screen = ScreenAgents
	m.ListBackupsFn = func() ([]backup.Manifest, []string) {
		return nil, nil
	}

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlB})
	rm := result.(Model)
	if rm.Screen != ScreenBackups {
		t.Errorf("Screen = %v, want ScreenBackups after ctrl+b", rm.Screen)
	}
}

// ---------------------------------------------------------------------------
// Maintenance: Upgrade+Sync chain (Issue A fix)
// ---------------------------------------------------------------------------

func TestMaintenanceUpgrade_SKeyStartsChain(t *testing.T) {
	m := New(nil, "/tmp", "test")
	m.Screen = ScreenMaintenance
	m.MaintenanceTab = MaintenanceTabUpgrade
	m.SyncFn = func(_ string) (int, error) { return 0, nil }

	result, cmd := m.Update(keyMsg("s"))
	rm := result.(Model)
	if !rm.UpgradeSyncChain {
		t.Error("UpgradeSyncChain should be true after pressing s")
	}
	if !rm.OperationRunning {
		t.Error("OperationRunning should be true")
	}
	if cmd == nil {
		t.Error("expected non-nil cmd for update check")
	}
}

func TestMaintenanceUpgrade_SKeyNoOpWhenNoSyncFn(t *testing.T) {
	m := New(nil, "/tmp", "test")
	m.Screen = ScreenMaintenance
	m.MaintenanceTab = MaintenanceTabUpgrade
	m.SyncFn = nil

	result, _ := m.Update(keyMsg("s"))
	rm := result.(Model)
	if rm.UpgradeSyncChain {
		t.Error("UpgradeSyncChain should be false when SyncFn not available")
	}
}

// ---------------------------------------------------------------------------
// Maintenance Sync: Profile delete by cursor (Issue B fix)
// ---------------------------------------------------------------------------

func TestMaintenanceSync_DeleteUsesCursor(t *testing.T) {
	m := New(nil, "/tmp", "test")
	m.Screen = ScreenMaintenance
	m.MaintenanceTab = MaintenanceTabSync
	m.Profiles = []model.Profile{
		{Name: "first"},
		{Name: "second"},
		{Name: "third"},
	}
	m.Cursor = 1 // pointing at "second"
	m.SelectedProfile = "first"

	m = updateModel(t, m, "d")
	if m.ProfileDeleteTarget != "second" {
		t.Errorf("ProfileDeleteTarget = %q, want %q", m.ProfileDeleteTarget, "second")
	}
	if m.ActiveDialog.Type != DialogProfileDelete {
		t.Error("expected DialogProfileDelete dialog")
	}
}

func TestMaintenanceSync_DeleteFallbackToSelected(t *testing.T) {
	m := New(nil, "/tmp", "test")
	m.Screen = ScreenMaintenance
	m.MaintenanceTab = MaintenanceTabSync
	m.Profiles = []model.Profile{} // empty
	m.Cursor = -1                   // invalid cursor
	m.SelectedProfile = "fallback"

	m = updateModel(t, m, "d")
	if m.ProfileDeleteTarget != "fallback" {
		t.Errorf("ProfileDeleteTarget = %q, want %q (fallback)", m.ProfileDeleteTarget, "fallback")
	}
}

// ---------------------------------------------------------------------------
// Model helpers: applyQuickDefaults
// ---------------------------------------------------------------------------

func TestApplyQuickDefaults(t *testing.T) {
	m := New(nil, t.TempDir(), "test")
	m.applyQuickDefaults()

	if m.Persona != model.PersonaProfessional {
		t.Errorf("Persona = %v, want PersonaProfessional", m.Persona)
	}
	if m.ModelPreset != model.ModelPresetBalanced {
		t.Errorf("ModelPreset = %v, want Balanced", m.ModelPreset)
	}
	if !m.SDDEnabled {
		t.Error("SDDEnabled should be true")
	}
	if m.StrictTDDEnabled {
		t.Error("StrictTDDEnabled should be false by default")
	}
	if m.Resolved == nil {
		t.Error("Resolved should be populated")
	}
}

// ---------------------------------------------------------------------------
// Model helpers: exportConfig
// ---------------------------------------------------------------------------

func TestExportConfig_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	m := New(nil, dir, "test")
	m.Agents = []AgentItem{
		{ID: model.AgentClaudeCode, Selected: true},
	}
	m.Preset = model.PresetMinimal
	m.Persona = model.PersonaMentor
	m.ModelPreset = model.ModelPresetPerformance
	m.SDDEnabled = true
	m.StrictTDDEnabled = true

	if err := m.exportConfig(); err != nil {
		t.Fatalf("exportConfig failed: %v", err)
	}

	// Load it back
	cfg, err := state.LoadExportConfig(dir)
	if err != nil {
		t.Fatalf("LoadExportConfig failed: %v", err)
	}
	if cfg.Preset != model.PresetMinimal {
		t.Errorf("Preset = %v, want Minimal", cfg.Preset)
	}
	if cfg.Persona != model.PersonaMentor {
		t.Errorf("Persona = %v, want Mentor", cfg.Persona)
	}
	if !cfg.StrictTDD {
		t.Error("StrictTDD should be true")
	}
}

// ---------------------------------------------------------------------------
// OpenCode: unsaved changes detection
// ---------------------------------------------------------------------------

func TestOCHasUnsavedChanges_NoSnapshot(t *testing.T) {
	m := New(nil, "/tmp", "test")
	m.OpenCodeAssignments = model.OpenCodeModelAssignments{
		"create": {Provider: "anthropic", Model: "sonnet"},
	}
	m.OCSavedAssignments = nil

	if !m.ocHasUnsavedChanges() {
		t.Error("should have unsaved changes when no snapshot exists")
	}
}

func TestOCHasUnsavedChanges_Matching(t *testing.T) {
	m := New(nil, "/tmp", "test")
	assignment := model.OpenCodeModelAssignment{Provider: "anthropic", Model: "sonnet"}
	m.OpenCodeAssignments = model.OpenCodeModelAssignments{"create": assignment}
	m.OCSavedAssignments = model.OpenCodeModelAssignments{"create": assignment}

	if m.ocHasUnsavedChanges() {
		t.Error("should not have unsaved changes when matching snapshot")
	}
}

func TestOCHasUnsavedChanges_Modified(t *testing.T) {
	m := New(nil, "/tmp", "test")
	m.OpenCodeAssignments = model.OpenCodeModelAssignments{
		"create": {Provider: "anthropic", Model: "opus"},
	}
	m.OCSavedAssignments = model.OpenCodeModelAssignments{
		"create": {Provider: "anthropic", Model: "sonnet"},
	}

	if !m.ocHasUnsavedChanges() {
		t.Error("should detect modified assignments")
	}
}

// ---------------------------------------------------------------------------
// Agent Builder: template selection
// ---------------------------------------------------------------------------

func TestAgentBuilderPrompt_TemplateKey(t *testing.T) {
	m := New(nil, "/tmp", "test")
	m.Screen = ScreenAgentBuilderPrompt

	m = updateModel(t, m, "1")
	if m.AgentBuilderTextArea.Value() == "" {
		t.Error("textarea should contain template after pressing 1")
	}
}

// ---------------------------------------------------------------------------
// Filter: Hint
// ---------------------------------------------------------------------------

func TestFilterHint_WhenInactive(t *testing.T) {
	f := NewFilterInput()
	if f.Hint() == "" {
		t.Error("inactive filter should show hint")
	}
}

func TestFilterHint_WhenActive(t *testing.T) {
	f := NewFilterInput()
	f.Activate()
	if f.Hint() != "" {
		t.Error("active filter should not show hint")
	}
}
