package tui

import (
	"slices"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lleontor705/cortex-ia/internal/backup"
	"github.com/lleontor705/cortex-ia/internal/model"
	"github.com/lleontor705/cortex-ia/internal/pipeline"
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
	if isRetiredScreen(m.Screen) {
		t.Skip("retired TUI screen is intentionally unreachable")
	}
	result, _ := m.Update(keyMsg(key))
	return result.(Model)
}

// ---------------------------------------------------------------------------
// Install flow — SDD Mode
// ---------------------------------------------------------------------------

func TestSDDMode_Toggle(t *testing.T) {
	m := New(nil, "/tmp", "test")
	m.Screen = ScreenSDDMode
	m.SDDEnabled = true

	m = updateModel(t, m, "down")
	if m.SDDEnabled {
		t.Error("SDDEnabled should be false after toggle")
	}

	m = updateModel(t, m, "up")
	if !m.SDDEnabled {
		t.Error("SDDEnabled should be true after second toggle")
	}
}

func TestSDDMode_Enter(t *testing.T) {
	m := New(nil, "/tmp", "test")
	m.Screen = ScreenSDDMode

	m = updateModel(t, m, "enter")
	if m.Screen != ScreenStrictTDD {
		t.Errorf("Screen = %v, want ScreenStrictTDD", m.Screen)
	}
}

// ---------------------------------------------------------------------------
// Install flow — Strict TDD
// ---------------------------------------------------------------------------

func TestStrictTDD_Toggle(t *testing.T) {
	m := New(nil, "/tmp", "test")
	m.Screen = ScreenStrictTDD
	m.StrictTDDEnabled = false

	m = updateModel(t, m, "down")
	if !m.StrictTDDEnabled {
		t.Error("StrictTDDEnabled should be true after toggle")
	}

	m = updateModel(t, m, "up")
	if m.StrictTDDEnabled {
		t.Error("StrictTDDEnabled should be false after second toggle")
	}
}

func TestStrictTDD_Enter(t *testing.T) {
	m := New(nil, "/tmp", "test")
	m.Screen = ScreenStrictTDD

	m = updateModel(t, m, "enter")
	if m.Screen != ScreenDependencyTree {
		t.Errorf("Screen = %v, want ScreenDependencyTree", m.Screen)
	}
}

// ---------------------------------------------------------------------------
// Install flow — Dependency Tree
// ---------------------------------------------------------------------------

func TestDependencyTree_Enter(t *testing.T) {
	m := New(nil, "/tmp", "test")
	m.Screen = ScreenDependencyTree

	m = updateModel(t, m, "enter")
	if m.Screen != ScreenSkillPicker {
		t.Errorf("Screen = %v, want ScreenSkillPicker", m.Screen)
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
	if m.Screen != ScreenDependencyTree {
		t.Errorf("Screen = %v, want ScreenDependencyTree", m.Screen)
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
// Operations handlers
// ---------------------------------------------------------------------------

func TestSync_Enter_CallsSyncFn(t *testing.T) {
	synced := false
	m := New(nil, "/tmp", "test")
	m.Screen = ScreenSync
	m.SyncFn = func(_ ...string) (int, error) {
		synced = true
		return 3, nil
	}

	result, cmd := m.Update(keyMsg("enter"))
	m = result.(Model)
	if !m.OperationRunning {
		t.Error("OperationRunning should be true")
	}
	if cmd == nil {
		t.Fatal("expected non-nil cmd")
	}

	// The command is a tea.Batch, so we cannot easily execute it here.
	// But we can verify the state was set correctly.
	_ = synced
}

func TestUpgrade_Esc(t *testing.T) {
	m := New(nil, "/tmp", "test")
	m.Screen = ScreenUpgrade

	m = updateModel(t, m, "esc")
	if m.Screen != ScreenWelcome {
		t.Errorf("Screen = %v, want ScreenWelcome", m.Screen)
	}
}

func TestUpgrade_RefreshKey(t *testing.T) {
	m := New(nil, "/tmp", "test")
	m.Screen = ScreenUpgrade
	m.Version = "dev" // "dev" makes Check() return immediately with UpToDate=true

	result, cmd := m.Update(keyMsg("r"))
	m = result.(Model)
	if !m.OperationRunning {
		t.Error("OperationRunning should be true after pressing r")
	}
	if m.UpdateCheckDone {
		t.Error("UpdateCheckDone should be false while check is running")
	}
	if cmd == nil {
		t.Error("expected non-nil cmd for update check")
	}
}

func TestUpgrade_RefreshKey_IgnoredWhenRunning(t *testing.T) {
	m := New(nil, "/tmp", "test")
	m.Screen = ScreenUpgrade
	m.OperationRunning = true

	result, cmd := m.Update(keyMsg("r"))
	m = result.(Model)
	if cmd != nil {
		t.Error("expected nil cmd when operation is already running")
	}
	_ = m
}

func TestUpgradeSync_EnterStartsChecking(t *testing.T) {
	m := New(nil, "/tmp", "test")
	m.Screen = ScreenUpgradeSync
	m.Version = "dev"

	result, cmd := m.Update(keyMsg("enter"))
	m = result.(Model)
	if !m.OperationRunning {
		t.Error("OperationRunning should be true after pressing enter")
	}
	if m.UpgradeSyncPhase != "checking" {
		t.Errorf("UpgradeSyncPhase = %q, want %q", m.UpgradeSyncPhase, "checking")
	}
	if cmd == nil {
		t.Error("expected non-nil cmd for update check")
	}
}

func TestUpgradeSync_EscIgnoredWhenRunning(t *testing.T) {
	m := New(nil, "/tmp", "test")
	m.Screen = ScreenUpgradeSync
	m.OperationRunning = true

	m = updateModel(t, m, "esc")
	if m.Screen != ScreenUpgradeSync {
		t.Errorf("Screen = %v, want ScreenUpgradeSync (should not navigate while running)", m.Screen)
	}
}

func TestUpgradeSync_EscWhenIdle(t *testing.T) {
	m := New(nil, "/tmp", "test")
	m.Screen = ScreenUpgradeSync
	m.UpgradeSyncPhase = "done"

	m = updateModel(t, m, "esc")
	if m.Screen != ScreenWelcome {
		t.Errorf("Screen = %v, want ScreenWelcome", m.Screen)
	}
	if m.UpgradeSyncPhase != "" {
		t.Errorf("UpgradeSyncPhase = %q, want empty", m.UpgradeSyncPhase)
	}
}

// ModelConfig screen was removed — Welcome goes directly to ClaudeModelPicker.
// See TestClaudeModelPicker_Enter_ModelConfigMode for the config mode test.

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
// Complete screen — any key quits
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

func TestRetiredTUISelectionsFailClosedBeforeSideEffects(t *testing.T) {
	called := false
	m := New(nil, t.TempDir(), "test")
	m.Screen = ScreenProfiles
	m.ExecuteFn = func(_ model.Selection, _ pipeline.ProgressFunc) pipeline.InstallResult {
		called = true
		return pipeline.InstallResult{}
	}

	result, cmd := m.Update(keyMsg("enter"))
	got := result.(Model)
	if cmd != nil {
		t.Error("retired selection returned a command")
	}
	if got.InstallErr == nil {
		t.Fatal("retired selection did not report a migration error")
	}
	if got.Screen != ScreenComplete {
		t.Errorf("Screen = %v, want ScreenComplete", got.Screen)
	}
	if called {
		t.Error("retired selection invoked installation")
	}
}

func TestRetiredWelcomeActionCannotBypassBoundary(t *testing.T) {
	m := New(nil, t.TempDir(), "test")
	result, cmd := m.dispatchWelcome(WelcomeOption(99))
	got := result.(Model)
	if cmd != nil {
		t.Error("retired welcome action returned a command")
	}
	if got.InstallErr == nil {
		t.Fatal("retired welcome action did not report a migration error")
	}
	if got.Screen != ScreenComplete {
		t.Errorf("Screen = %v, want ScreenComplete", got.Screen)
	}
}

func TestInjectedUnsupportedAgentFailsBeforeInstall(t *testing.T) {
	called := false
	m := New(nil, t.TempDir(), "test")
	m.Agents = []AgentItem{{ID: model.AgentID("gemini"), Selected: true}}
	m.ExecuteFn = func(_ model.Selection, _ pipeline.ProgressFunc) pipeline.InstallResult {
		called = true
		return pipeline.InstallResult{}
	}

	msg := m.runInstallWithProgress(make(chan StepProgressMsg))()
	done, ok := msg.(PipelineDoneMsg)
	if !ok {
		t.Fatalf("message = %T, want PipelineDoneMsg", msg)
	}
	if done.Err == nil {
		t.Fatal("unsupported agent did not return an error")
	}
	if called {
		t.Error("unsupported agent invoked installation")
	}
}

func TestSelectedSupportedAgentsAreExactlyCanonical(t *testing.T) {
	m := New(nil, t.TempDir(), "test")
	m.Agents = []AgentItem{
		{ID: model.AgentOpenCode, Selected: true},
	}
	got, err := m.selectedSupportedAgentIDs()
	if err != nil {
		t.Fatalf("selectedSupportedAgentIDs() error = %v", err)
	}
	want := []model.AgentID{
		model.AgentOpenCode,
	}
	if !slices.Equal(got, want) {
		t.Fatalf("selectedSupportedAgentIDs() = %v, want %v", got, want)
	}
}
