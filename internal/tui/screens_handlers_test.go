package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lleontor705/cortex-ia/internal/backup"
	"github.com/lleontor705/cortex-ia/internal/model"
)

// keyMsg creates a tea.KeyMsg for testing.
func keyMsg(key string) tea.KeyMsg {
	switch key {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	default:
		if len(key) == 1 {
			return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
		}
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	}
}

// updateModel sends a key message through the model's Update method and returns
// the resulting model. It requires the model's Screen to already be set so the
// router dispatches to the correct handler.
func updateModel(t *testing.T, m Model, key string) Model {
	t.Helper()
	result, _ := m.Update(keyMsg(key))
	return result.(Model)
}

// ---------------------------------------------------------------------------
// Install flow — Claude Model Picker
// ---------------------------------------------------------------------------

func TestClaudeModelPicker_UpDown(t *testing.T) {
	m := New(nil, "/tmp", "test")
	m.Screen = ScreenClaudeModelPicker
	m.ClaudeModelCursor = 0

	m = updateModel(t, m, "down")
	if m.ClaudeModelCursor != 1 {
		t.Errorf("after down: cursor = %d, want 1", m.ClaudeModelCursor)
	}

	m = updateModel(t, m, "down")
	if m.ClaudeModelCursor != 2 {
		t.Errorf("after second down: cursor = %d, want 2", m.ClaudeModelCursor)
	}

	// Should stay at 2 (max index for 3 models)
	m = updateModel(t, m, "down")
	if m.ClaudeModelCursor != 2 {
		t.Errorf("after third down: cursor = %d, want 2 (clamped)", m.ClaudeModelCursor)
	}

	m = updateModel(t, m, "up")
	if m.ClaudeModelCursor != 1 {
		t.Errorf("after up: cursor = %d, want 1", m.ClaudeModelCursor)
	}

	// Move to 0, then try going below 0
	m = updateModel(t, m, "up")
	m = updateModel(t, m, "up")
	if m.ClaudeModelCursor != 0 {
		t.Errorf("after multiple ups: cursor = %d, want 0 (clamped)", m.ClaudeModelCursor)
	}
}

func TestClaudeModelPicker_Enter(t *testing.T) {
	m := New(nil, "/tmp", "test")
	m.Screen = ScreenClaudeModelPicker

	m = updateModel(t, m, "enter")
	if m.Screen != ScreenSkillPicker {
		t.Errorf("Screen = %v, want ScreenSkillPicker", m.Screen)
	}
}

func TestClaudeModelPicker_Enter_MapsPresetFromCursor(t *testing.T) {
	tests := []struct {
		cursor int
		preset model.ModelPreset
	}{
		{0, model.ModelPresetPerformance},
		{1, model.ModelPresetBalanced},
		{2, model.ModelPresetEconomy},
	}
	for _, tt := range tests {
		m := New(nil, "/tmp", "test")
		m.Screen = ScreenClaudeModelPicker
		m.ClaudeModelCursor = tt.cursor

		m = updateModel(t, m, "enter")
		if m.ModelPreset != tt.preset {
			t.Errorf("cursor=%d: ModelPreset = %q, want %q", tt.cursor, m.ModelPreset, tt.preset)
		}
		if m.ModelAssignments == nil {
			t.Errorf("cursor=%d: ModelAssignments should not be nil", tt.cursor)
		}
	}
}

func TestClaudeModelPicker_Esc(t *testing.T) {
	m := New(nil, "/tmp", "test")
	m.Screen = ScreenClaudeModelPicker

	m = updateModel(t, m, "esc")
	if m.Screen != ScreenPersona {
		t.Errorf("Screen = %v, want ScreenPersona", m.Screen)
	}
}

// ---------------------------------------------------------------------------
// Install flow — Review (with SDD/TDD toggles)
// ---------------------------------------------------------------------------

func TestReview_SpaceTogglesSDD(t *testing.T) {
	m := New(nil, "/tmp", "test")
	m.Screen = ScreenReview
	m.SDDEnabled = true
	m.Cursor = reviewCursorSDD

	m = updateModel(t, m, " ")
	if m.SDDEnabled {
		t.Error("SDDEnabled should be false after space toggle")
	}

	m = updateModel(t, m, " ")
	if !m.SDDEnabled {
		t.Error("SDDEnabled should be true after second toggle")
	}
}

func TestReview_SpaceTogglesTDD(t *testing.T) {
	m := New(nil, "/tmp", "test")
	m.Screen = ScreenReview
	m.StrictTDDEnabled = false
	m.Cursor = reviewCursorTDD

	m = updateModel(t, m, " ")
	if !m.StrictTDDEnabled {
		t.Error("StrictTDDEnabled should be true after toggle")
	}
}

// ---------------------------------------------------------------------------
// Install flow — Skill Picker
// ---------------------------------------------------------------------------

func TestSkillPicker_Toggle(t *testing.T) {
	m := New(nil, "/tmp", "test")
	m.Screen = ScreenSkillPicker
	m.AvailableSkills = []SkillItem{
		{Name: "skill-a", Selected: true},
		{Name: "skill-b", Selected: true},
	}
	m.SkillCursor = 0

	// Toggle first skill off.
	m = updateModel(t, m, " ")
	if m.AvailableSkills[0].Selected {
		t.Error("skill-a should be deselected after space")
	}
	if !m.AvailableSkills[1].Selected {
		t.Error("skill-b should still be selected")
	}

	// Toggle it back on.
	m = updateModel(t, m, " ")
	if !m.AvailableSkills[0].Selected {
		t.Error("skill-a should be selected after second space")
	}
}

func TestSkillPicker_SelectAll(t *testing.T) {
	m := New(nil, "/tmp", "test")
	m.Screen = ScreenSkillPicker
	m.AvailableSkills = []SkillItem{
		{Name: "skill-a", Selected: true},
		{Name: "skill-b", Selected: false},
	}

	// Not all selected, so 'a' should select all.
	m = updateModel(t, m, "a")
	for i, s := range m.AvailableSkills {
		if !s.Selected {
			t.Errorf("AvailableSkills[%d] should be selected after 'a'", i)
		}
	}

	// All selected, so 'a' should deselect all.
	m = updateModel(t, m, "a")
	for i, s := range m.AvailableSkills {
		if s.Selected {
			t.Errorf("AvailableSkills[%d] should be deselected after second 'a'", i)
		}
	}
}

func TestSkillPicker_Navigation(t *testing.T) {
	m := New(nil, "/tmp", "test")
	m.Screen = ScreenSkillPicker
	m.AvailableSkills = []SkillItem{
		{Name: "skill-a", Selected: true},
		{Name: "skill-b", Selected: true},
		{Name: "skill-c", Selected: true},
	}
	m.SkillCursor = 0

	m = updateModel(t, m, "down")
	if m.SkillCursor != 1 {
		t.Errorf("SkillCursor = %d, want 1", m.SkillCursor)
	}

	m = updateModel(t, m, "down")
	if m.SkillCursor != 2 {
		t.Errorf("SkillCursor = %d, want 2", m.SkillCursor)
	}

	// Clamp at end.
	m = updateModel(t, m, "down")
	if m.SkillCursor != 2 {
		t.Errorf("SkillCursor = %d, want 2 (clamped)", m.SkillCursor)
	}

	m = updateModel(t, m, "up")
	if m.SkillCursor != 1 {
		t.Errorf("SkillCursor = %d, want 1", m.SkillCursor)
	}

	// Clamp at start.
	m = updateModel(t, m, "up")
	m = updateModel(t, m, "up")
	if m.SkillCursor != 0 {
		t.Errorf("SkillCursor = %d, want 0 (clamped)", m.SkillCursor)
	}
}

func TestSkillPicker_Enter(t *testing.T) {
	m := New(nil, "/tmp", "test")
	m.Screen = ScreenSkillPicker
	m.AvailableSkills = []SkillItem{
		{Name: "skill-a", Selected: true},
		{Name: "skill-b", Selected: false},
		{Name: "skill-c", Selected: true},
	}

	m = updateModel(t, m, "enter")
	if m.Screen != ScreenReview {
		t.Errorf("Screen = %v, want ScreenReview", m.Screen)
	}
	if len(m.SkillSelection) != 2 {
		t.Fatalf("len(SkillSelection) = %d, want 2", len(m.SkillSelection))
	}
	if m.SkillSelection[0] != "skill-a" {
		t.Errorf("SkillSelection[0] = %q, want %q", m.SkillSelection[0], "skill-a")
	}
	if m.SkillSelection[1] != "skill-c" {
		t.Errorf("SkillSelection[1] = %q, want %q", m.SkillSelection[1], "skill-c")
	}
}

func TestSkillPicker_EmptyList(t *testing.T) {
	m := New(nil, "/tmp", "test")
	m.Screen = ScreenSkillPicker
	m.AvailableSkills = nil

	m = updateModel(t, m, "enter")
	if m.Screen != ScreenReview {
		t.Errorf("Screen = %v, want ScreenReview", m.Screen)
	}
	if m.SkillSelection != nil {
		t.Errorf("SkillSelection = %v, want nil", m.SkillSelection)
	}
}

func TestSkillPicker_Esc(t *testing.T) {
	m := New(nil, "/tmp", "test")
	m.Screen = ScreenSkillPicker

	m = updateModel(t, m, "esc")
	if m.Screen != ScreenClaudeModelPicker {
		t.Errorf("Screen = %v, want ScreenClaudeModelPicker", m.Screen)
	}
}

// ---------------------------------------------------------------------------
// Backup handlers
// ---------------------------------------------------------------------------

func TestBackups_Navigation(t *testing.T) {
	m := New(nil, "/tmp", "test")
	m.Screen = ScreenBackups
	m.Backups = []backup.Manifest{
		{ID: "b1"},
		{ID: "b2"},
		{ID: "b3"},
	}
	m.Cursor = 0

	m = updateModel(t, m, "down")
	if m.Cursor != 1 {
		t.Errorf("cursor = %d, want 1", m.Cursor)
	}

	m = updateModel(t, m, "down")
	if m.Cursor != 2 {
		t.Errorf("cursor = %d, want 2", m.Cursor)
	}

	// Clamp at end
	m = updateModel(t, m, "down")
	if m.Cursor != 2 {
		t.Errorf("cursor = %d, want 2 (clamped)", m.Cursor)
	}

	m = updateModel(t, m, "up")
	if m.Cursor != 1 {
		t.Errorf("cursor = %d, want 1", m.Cursor)
	}
}

func TestBackups_Esc(t *testing.T) {
	m := New(nil, "/tmp", "test")
	m.Screen = ScreenBackups

	m = updateModel(t, m, "esc")
	if m.Screen != ScreenWelcome {
		t.Errorf("Screen = %v, want ScreenWelcome", m.Screen)
	}
}

func TestRestoreDialog_Yes(t *testing.T) {
	restored := false
	m := New(nil, "/tmp", "test")
	m.Screen = ScreenBackups
	m.SelectedBackup = backup.Manifest{ID: "b1"}
	m.ActiveDialog = Dialog{Type: DialogRestoreConfirm, Title: "Confirm", Message: "Restore?"}
	m.RestoreFn = func(manifest backup.Manifest) error {
		restored = true
		return nil
	}

	result, cmd := m.Update(keyMsg("y"))
	rm := result.(Model)
	if rm.ActiveDialog.Type != DialogNone {
		t.Error("dialog should be closed after confirm")
	}
	if !rm.OperationRunning {
		t.Error("OperationRunning should be true")
	}
	if cmd == nil {
		t.Fatal("expected non-nil cmd")
	}

	msg := cmd()
	if !restored {
		t.Error("RestoreFn should have been called")
	}
	if _, ok := msg.(BackupRestoreMsg); !ok {
		t.Errorf("expected BackupRestoreMsg, got %T", msg)
	}
}

func TestDeleteDialog_Yes(t *testing.T) {
	deleted := false
	m := New(nil, "/tmp", "test")
	m.Screen = ScreenBackups
	m.SelectedBackup = backup.Manifest{ID: "b1"}
	m.ActiveDialog = Dialog{Type: DialogDeleteConfirm, Title: "Confirm", Message: "Delete?"}
	m.DeleteBackupFn = func(manifest backup.Manifest) error {
		deleted = true
		return nil
	}

	result, cmd := m.Update(keyMsg("y"))
	rm := result.(Model)
	if rm.ActiveDialog.Type != DialogNone {
		t.Error("dialog should be closed after confirm")
	}
	if cmd == nil {
		t.Fatal("expected non-nil cmd")
	}

	msg := cmd()
	if !deleted {
		t.Error("DeleteBackupFn should have been called")
	}
	if _, ok := msg.(BackupDeleteMsg); !ok {
		t.Errorf("expected BackupDeleteMsg, got %T", msg)
	}
}

func TestDialog_Esc_Dismisses(t *testing.T) {
	m := New(nil, "/tmp", "test")
	m.ActiveDialog = Dialog{Type: DialogRestoreConfirm, Title: "Test"}

	result, _ := m.Update(keyMsg("esc"))
	rm := result.(Model)
	if rm.ActiveDialog.Type != DialogNone {
		t.Error("dialog should be dismissed on esc")
	}
}

// ---------------------------------------------------------------------------
// Maintenance screen — Sync tab profile management
// ---------------------------------------------------------------------------

func TestMaintenance_SyncTab_CreateKey(t *testing.T) {
	m := New(nil, "/tmp", "test")
	m.Screen = ScreenMaintenance
	m.MaintenanceTab = MaintenanceTabSync

	m = updateModel(t, m, "c")
	if m.Screen != ScreenProfileCreate {
		t.Errorf("Screen = %v, want ScreenProfileCreate", m.Screen)
	}
}

func TestMaintenance_SyncTab_DeleteKey(t *testing.T) {
	m := New(nil, "/tmp", "test")
	m.Screen = ScreenMaintenance
	m.MaintenanceTab = MaintenanceTabSync
	m.Profiles = []model.Profile{{Name: "default"}}
	m.SelectedProfile = "default"

	m = updateModel(t, m, "d")
	if m.ActiveDialog.Type != DialogProfileDelete {
		t.Errorf("ActiveDialog.Type = %v, want DialogProfileDelete", m.ActiveDialog.Type)
	}
}

func TestProfileCreate_Enter(t *testing.T) {
	m := New(nil, t.TempDir(), "test")
	m.Screen = ScreenProfileCreate
	m.ProfileInput.SetValue("my-profile")
	m.Profiles = []model.Profile{}

	m = updateModel(t, m, "enter")
	if m.Screen != ScreenMaintenance {
		t.Errorf("Screen = %v, want ScreenMaintenance", m.Screen)
	}
	if len(m.Profiles) != 1 {
		t.Fatalf("len(Profiles) = %d, want 1", len(m.Profiles))
	}
	if m.Profiles[0].Name != "my-profile" {
		t.Errorf("Profile name = %q, want %q", m.Profiles[0].Name, "my-profile")
	}
}

func TestProfileCreate_Esc(t *testing.T) {
	m := New(nil, "/tmp", "test")
	m.Screen = ScreenProfileCreate
	m.ProfileInput.SetValue("something")

	m = updateModel(t, m, "esc")
	if m.Screen != ScreenMaintenance {
		t.Errorf("Screen = %v, want ScreenMaintenance", m.Screen)
	}
	if m.ProfileInput.Value() != "" {
		t.Errorf("ProfileInput.Value() = %q, want empty", m.ProfileInput.Value())
	}
}

func TestProfileDeleteDialog_Yes(t *testing.T) {
	m := New(nil, t.TempDir(), "test")
	m.Screen = ScreenMaintenance
	m.MaintenanceTab = MaintenanceTabSync
	m.Profiles = []model.Profile{{Name: "to-delete"}, {Name: "keep"}}
	m.SelectedProfile = "to-delete"
	m.ProfileDeleteTarget = "to-delete"
	m.ActiveDialog = Dialog{Type: DialogProfileDelete, Title: "Delete", Message: "Delete?"}

	result, _ := m.Update(keyMsg("y"))
	rm := result.(Model)
	if rm.ActiveDialog.Type != DialogNone {
		t.Error("dialog should be dismissed")
	}
	if len(rm.Profiles) != 1 {
		t.Fatalf("len(Profiles) = %d, want 1", len(rm.Profiles))
	}
	if rm.Profiles[0].Name != "keep" {
		t.Errorf("remaining profile = %q, want %q", rm.Profiles[0].Name, "keep")
	}
}

// ---------------------------------------------------------------------------
// Agent Builder handlers
// ---------------------------------------------------------------------------

func TestAgentBuilderEngine_Enter(t *testing.T) {
	m := New(nil, "/tmp", "test")
	m.Screen = ScreenAgentBuilderEngine
	m.Cursor = 0

	m = updateModel(t, m, "enter")
	if m.Screen != ScreenAgentBuilderPrompt {
		t.Errorf("Screen = %v, want ScreenAgentBuilderPrompt", m.Screen)
	}
	if m.AgentBuilderEngine != model.AgentClaudeCode {
		t.Errorf("AgentBuilderEngine = %v, want AgentClaudeCode", m.AgentBuilderEngine)
	}
}

func TestAgentBuilderEngine_Esc(t *testing.T) {
	m := New(nil, "/tmp", "test")
	m.Screen = ScreenAgentBuilderEngine

	m = updateModel(t, m, "esc")
	if m.Screen != ScreenWelcome {
		t.Errorf("Screen = %v, want ScreenWelcome", m.Screen)
	}
}

func TestAgentBuilderSDD_EnterFull(t *testing.T) {
	m := New(nil, "/tmp", "test")
	m.Screen = ScreenAgentBuilderSDD
	m.Cursor = 0 // Full SDD is first option

	result, cmd := m.Update(keyMsg("enter"))
	m = result.(Model)
	if m.Screen != ScreenAgentBuilderGenerating {
		t.Errorf("Screen = %v, want ScreenAgentBuilderGenerating", m.Screen)
	}
	if !m.OperationRunning {
		t.Error("OperationRunning should be true")
	}
	if cmd == nil {
		t.Error("expected non-nil cmd for generation")
	}
}

func TestAgentBuilderPreview_Enter(t *testing.T) {
	m := New(nil, "/tmp", "test")
	m.Screen = ScreenAgentBuilderPreview

	result, cmd := m.Update(keyMsg("enter"))
	m = result.(Model)
	if !m.OperationRunning {
		t.Error("OperationRunning should be true")
	}
	if cmd == nil {
		t.Error("expected non-nil cmd for install")
	}
}

func TestAgentBuilderComplete_Enter(t *testing.T) {
	m := New(nil, "/tmp", "test")
	m.Screen = ScreenAgentBuilderComplete

	m = updateModel(t, m, "enter")
	if m.Screen != ScreenWelcome {
		t.Errorf("Screen = %v, want ScreenWelcome", m.Screen)
	}
}

// ---------------------------------------------------------------------------
// Backup handlers — additional coverage
// ---------------------------------------------------------------------------

func TestBackups_RestoreKey(t *testing.T) {
	m := New(nil, "/tmp", "test")
	m.Screen = ScreenBackups
	m.Backups = []backup.Manifest{{ID: "b1"}, {ID: "b2"}}
	m.Cursor = 1

	m = updateModel(t, m, "r")
	if m.ActiveDialog.Type != DialogRestoreConfirm {
		t.Errorf("ActiveDialog.Type = %v, want DialogRestoreConfirm", m.ActiveDialog.Type)
	}
	if m.SelectedBackup.ID != "b2" {
		t.Errorf("SelectedBackup.ID = %q, want %q", m.SelectedBackup.ID, "b2")
	}
}

func TestBackups_DeleteKey(t *testing.T) {
	m := New(nil, "/tmp", "test")
	m.Screen = ScreenBackups
	m.Backups = []backup.Manifest{{ID: "b1"}}
	m.Cursor = 0

	m = updateModel(t, m, "d")
	if m.ActiveDialog.Type != DialogDeleteConfirm {
		t.Errorf("ActiveDialog.Type = %v, want DialogDeleteConfirm", m.ActiveDialog.Type)
	}
	if m.SelectedBackup.ID != "b1" {
		t.Errorf("SelectedBackup.ID = %q, want %q", m.SelectedBackup.ID, "b1")
	}
}

func TestBackups_RenameKey(t *testing.T) {
	m := New(nil, "/tmp", "test")
	m.Screen = ScreenBackups
	m.Backups = []backup.Manifest{{ID: "b1", Description: "old desc"}}
	m.Cursor = 0

	m = updateModel(t, m, "n")
	if m.Screen != ScreenRenameBackup {
		t.Errorf("Screen = %v, want ScreenRenameBackup", m.Screen)
	}
	if m.BackupRenameInput.Value() != "old desc" {
		t.Errorf("BackupRenameInput.Value() = %q, want %q", m.BackupRenameInput.Value(), "old desc")
	}
}

// ---------------------------------------------------------------------------
// Rename Backup handlers
// ---------------------------------------------------------------------------

func TestRenameBackup_Enter(t *testing.T) {
	renamed := false
	m := New(nil, "/tmp", "test")
	m.Screen = ScreenRenameBackup
	m.SelectedBackup = backup.Manifest{ID: "b1"}
	m.BackupRenameInput.SetValue("new description")
	m.RenameBackupFn = func(_ backup.Manifest, newDesc string) error {
		renamed = true
		return nil
	}
	m.ListBackupsFn = func() ([]backup.Manifest, []string) {
		return []backup.Manifest{{ID: "b1", Description: "new description"}}, nil
	}

	m = updateModel(t, m, "enter")
	if m.Screen != ScreenBackups {
		t.Errorf("Screen = %v, want ScreenBackups", m.Screen)
	}
	if !renamed {
		t.Error("RenameBackupFn should have been called")
	}
}

func TestRenameBackup_Esc(t *testing.T) {
	m := New(nil, "/tmp", "test")
	m.Screen = ScreenRenameBackup
	m.BackupRenameInput.SetValue("some text")

	m = updateModel(t, m, "esc")
	if m.Screen != ScreenBackups {
		t.Errorf("Screen = %v, want ScreenBackups", m.Screen)
	}
}

// ---------------------------------------------------------------------------
// Complete screen
// ---------------------------------------------------------------------------

func TestComplete_Q_Quits(t *testing.T) {
	m := New(nil, "/tmp", "test")
	m.Screen = ScreenComplete

	result, cmd := m.Update(keyMsg("q"))
	m = result.(Model)
	if !m.Quitting {
		t.Error("Quitting should be true after q")
	}
	if cmd == nil {
		t.Error("expected non-nil cmd (tea.Quit)")
	}
}

func TestComplete_Enter_ReturnsToWelcome(t *testing.T) {
	m := New(nil, "/tmp", "test")
	m.Screen = ScreenComplete

	m = updateModel(t, m, "enter")
	if m.Screen != ScreenWelcome {
		t.Errorf("Screen = %v, want ScreenWelcome", m.Screen)
	}
	if m.Quitting {
		t.Error("Quitting should be false — enter goes to menu")
	}
}

// ---------------------------------------------------------------------------
// Maintenance — Esc
// ---------------------------------------------------------------------------

func TestMaintenance_Esc(t *testing.T) {
	m := New(nil, "/tmp", "test")
	m.Screen = ScreenMaintenance

	m = updateModel(t, m, "esc")
	if m.Screen != ScreenWelcome {
		t.Errorf("Screen = %v, want ScreenWelcome", m.Screen)
	}
}
