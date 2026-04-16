package tui

import (
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/backup"
	"github.com/lleontor705/cortex-ia/internal/model"
)

// Smoke tests — ensure view functions render non-empty output without panicking.
// Rendering correctness is covered by the screens/ package tests.

func newRenderTestModel(t *testing.T) Model {
	t.Helper()
	m := New(nil, t.TempDir(), "1.0.0")
	m.Width = 120
	m.Height = 40
	return m
}

func TestView_Welcome(t *testing.T) {
	m := newRenderTestModel(t)
	m.Screen = ScreenWelcome
	if m.viewWelcome() == "" {
		t.Error("viewWelcome returned empty")
	}
}

func TestView_Welcome_FirstRun(t *testing.T) {
	m := newRenderTestModel(t)
	m.Screen = ScreenWelcome
	m.FirstRun = true
	out := m.viewWelcome()
	if !strings.Contains(out, "Welcome") && !strings.Contains(out, "Install") {
		t.Errorf("first run welcome should contain hint, got %q", out)
	}
}

func TestView_Welcome_WithUpdateBadge(t *testing.T) {
	m := newRenderTestModel(t)
	m.Screen = ScreenWelcome
	m.UpdateCheckDone = true
	// Note: this exercises the badge code path
	if m.viewWelcome() == "" {
		t.Error("viewWelcome returned empty")
	}
}

func TestView_Detection(t *testing.T) {
	m := newRenderTestModel(t)
	m.Screen = ScreenDetection
	// Populate SysInfo manually since RunDetection needs a real registry
	m.SysInfo = &SysInfoCache{
		OS: "linux", Arch: "amd64", PkgMgr: "apt", Shell: "bash",
		NodeVer: "v20", GitVer: "2.40", GoVer: "1.22",
		Npx: true, Cortex: false, DetectedAgents: 2,
	}
	if m.viewDetection() == "" {
		t.Error("viewDetection returned empty")
	}
}

func TestView_Detection_NoSysInfo(t *testing.T) {
	m := newRenderTestModel(t)
	m.Screen = ScreenDetection
	m.SysInfo = nil
	out := m.viewDetection()
	if !strings.Contains(out, "Detecting") {
		t.Errorf("expected 'Detecting' placeholder, got %q", out)
	}
}

func TestView_Agents(t *testing.T) {
	m := newRenderTestModel(t)
	m.Screen = ScreenAgents
	m.Agents = []AgentItem{
		{ID: model.AgentClaudeCode, Name: "claude-code", Selected: true},
	}
	if m.viewAgents() == "" {
		t.Error("viewAgents returned empty")
	}
}

func TestView_Persona(t *testing.T) {
	m := newRenderTestModel(t)
	m.Screen = ScreenPersona
	if m.viewPersona() == "" {
		t.Error("viewPersona returned empty")
	}
}

func TestView_ClaudeModelPicker(t *testing.T) {
	m := newRenderTestModel(t)
	m.Screen = ScreenClaudeModelPicker
	if m.viewClaudeModelPicker() == "" {
		t.Error("viewClaudeModelPicker returned empty")
	}
}

func TestView_SkillPicker_Empty(t *testing.T) {
	m := newRenderTestModel(t)
	m.Screen = ScreenSkillPicker
	m.AvailableSkills = nil
	if m.viewSkillPicker() == "" {
		t.Error("viewSkillPicker returned empty")
	}
}

func TestView_SkillPicker_WithSkills(t *testing.T) {
	m := newRenderTestModel(t)
	m.Screen = ScreenSkillPicker
	m.AvailableSkills = []SkillItem{
		{Name: "skill-a", Description: "First skill", Selected: true},
	}
	out := m.viewSkillPicker()
	if !strings.Contains(out, "skill-a") {
		t.Error("expected skill name in output")
	}
}

func TestView_Review(t *testing.T) {
	m := newRenderTestModel(t)
	m.Screen = ScreenReview
	m.Agents = []AgentItem{
		{ID: model.AgentClaudeCode, Name: "claude-code", Selected: true},
	}
	m.Resolved = []model.ComponentID{model.ComponentCortex}
	m.ModelPreset = model.ModelPresetBalanced
	if m.viewReview() == "" {
		t.Error("viewReview returned empty")
	}
}

func TestView_Review_WithDryRun(t *testing.T) {
	m := newRenderTestModel(t)
	m.Screen = ScreenReview
	m.Resolved = []model.ComponentID{model.ComponentCortex}
	// Note: DryRunResult requires pipeline.InstallResult which we can't easily mock here
	out := m.viewReview()
	if !strings.Contains(out, "Preset:") {
		t.Error("review should show Preset toggle")
	}
}

func TestView_Installing_NoProgress(t *testing.T) {
	m := newRenderTestModel(t)
	m.Screen = ScreenInstalling
	// Empty progress state uses placeholder
	if m.viewInstalling() == "" {
		t.Error("viewInstalling returned empty")
	}
}

func TestView_Installing_WithProgress(t *testing.T) {
	m := newRenderTestModel(t)
	m.Screen = ScreenInstalling
	m.Progress = NewProgressState([]string{"agent/comp-a"})
	if m.viewInstalling() == "" {
		t.Error("viewInstalling returned empty")
	}
}

func TestView_Complete_Success(t *testing.T) {
	m := newRenderTestModel(t)
	m.Screen = ScreenComplete
	if m.viewComplete() == "" {
		t.Error("viewComplete returned empty")
	}
}

func TestView_Backups_Empty(t *testing.T) {
	m := newRenderTestModel(t)
	m.Screen = ScreenBackups
	m.Backups = nil
	out := m.viewBackups()
	if !strings.Contains(out, "No backups") {
		t.Errorf("expected 'No backups' message, got %q", out)
	}
}

func TestView_Backups_WithItems(t *testing.T) {
	m := newRenderTestModel(t)
	m.Screen = ScreenBackups
	m.Backups = []backup.Manifest{
		{ID: "bk-1", Description: "Test backup"},
	}
	out := m.viewBackups()
	if !strings.Contains(out, "bk-1") {
		t.Error("expected backup ID in output")
	}
}

func TestView_RenameBackup(t *testing.T) {
	m := newRenderTestModel(t)
	m.Screen = ScreenRenameBackup
	m.SelectedBackup = backup.Manifest{ID: "bk-1"}
	if m.viewRenameBackup() == "" {
		t.Error("viewRenameBackup returned empty")
	}
}

func TestView_Maintenance_UpgradeTab(t *testing.T) {
	m := newRenderTestModel(t)
	m.Screen = ScreenMaintenance
	m.MaintenanceTab = MaintenanceTabUpgrade
	if m.viewMaintenance() == "" {
		t.Error("viewMaintenance returned empty")
	}
}

func TestView_Maintenance_SyncTab(t *testing.T) {
	m := newRenderTestModel(t)
	m.Screen = ScreenMaintenance
	m.MaintenanceTab = MaintenanceTabSync
	if m.viewMaintenance() == "" {
		t.Error("viewMaintenance returned empty")
	}
}

func TestView_ProfileCreate(t *testing.T) {
	m := newRenderTestModel(t)
	m.Screen = ScreenProfileCreate
	if m.viewProfileCreate() == "" {
		t.Error("viewProfileCreate returned empty")
	}
}

func TestView_AgentBuilderEngine(t *testing.T) {
	m := newRenderTestModel(t)
	m.Screen = ScreenAgentBuilderEngine
	if m.viewAgentBuilderEngine() == "" {
		t.Error("viewAgentBuilderEngine returned empty")
	}
}

func TestView_AgentBuilderPrompt(t *testing.T) {
	m := newRenderTestModel(t)
	m.Screen = ScreenAgentBuilderPrompt
	out := m.viewAgentBuilderPrompt()
	if !strings.Contains(out, "Template") && !strings.Contains(out, "template") {
		t.Error("prompt screen should mention templates")
	}
}

func TestView_AgentBuilderSDD(t *testing.T) {
	m := newRenderTestModel(t)
	m.Screen = ScreenAgentBuilderSDD
	if m.viewAgentBuilderSDD() == "" {
		t.Error("viewAgentBuilderSDD returned empty")
	}
}

func TestView_AgentBuilderSDD_Expanded(t *testing.T) {
	m := newRenderTestModel(t)
	m.Screen = ScreenAgentBuilderSDD
	m.AgentBuilderSDDMode = sddModePhase // expanded
	out := m.viewAgentBuilderSDD()
	if !strings.Contains(out, "init") {
		t.Error("expanded SDD should show phase list")
	}
}

func TestView_AgentBuilderGenerating(t *testing.T) {
	m := newRenderTestModel(t)
	m.Screen = ScreenAgentBuilderGenerating
	if m.viewAgentBuilderGenerating() == "" {
		t.Error("viewAgentBuilderGenerating returned empty")
	}
}

func TestView_AgentBuilderPreview(t *testing.T) {
	m := newRenderTestModel(t)
	m.Screen = ScreenAgentBuilderPreview
	m.AgentBuilderEngine = model.AgentClaudeCode
	if m.viewAgentBuilderPreview() == "" {
		t.Error("viewAgentBuilderPreview returned empty")
	}
}

func TestView_AgentBuilderComplete(t *testing.T) {
	m := newRenderTestModel(t)
	m.Screen = ScreenAgentBuilderComplete
	m.AgentBuilderEngine = model.AgentClaudeCode
	if m.viewAgentBuilderComplete() == "" {
		t.Error("viewAgentBuilderComplete returned empty")
	}
}

func TestView_ModelConfig_ClaudeTab(t *testing.T) {
	m := newRenderTestModel(t)
	m.Screen = ScreenModelConfig
	m.ModelConfigTab = ModelConfigTabClaude
	if m.viewModelConfig() == "" {
		t.Error("viewModelConfig returned empty")
	}
}

func TestView_ModelConfig_OpenCodeTab(t *testing.T) {
	m := newRenderTestModel(t)
	m.Screen = ScreenModelConfig
	m.ModelConfigTab = ModelConfigTabOpenCode
	if m.viewModelConfig() == "" {
		t.Error("viewModelConfig returned empty")
	}
}

// ---------------------------------------------------------------------------
// renderTabBar helper
// ---------------------------------------------------------------------------

func TestRenderTabBar(t *testing.T) {
	out := renderTabBar([]string{"A", "B", "C"}, 1)
	if !strings.Contains(out, "A") || !strings.Contains(out, "B") || !strings.Contains(out, "C") {
		t.Errorf("tab bar missing labels: %q", out)
	}
}

func TestRenderTabBar_ActiveMarker(t *testing.T) {
	out := renderTabBar([]string{"One", "Two"}, 0)
	if !strings.Contains(out, "[One]") {
		t.Errorf("active tab should have brackets: %q", out)
	}
}

// ---------------------------------------------------------------------------
// resetAgentBuilder
// ---------------------------------------------------------------------------

func TestResetAgentBuilder(t *testing.T) {
	m := New(nil, "/tmp", "test")
	m.AgentBuilderEngine = model.AgentClaudeCode
	m.AgentBuilderTextArea.SetValue("something")
	m.AgentBuilderSDDMode = "full"
	m.AgentBuilderSDDPhase = "init"

	m.resetAgentBuilder()

	if m.AgentBuilderEngine != "" {
		t.Error("Engine should be reset")
	}
	if m.AgentBuilderTextArea.Value() != "" {
		t.Error("TextArea should be reset")
	}
	if m.AgentBuilderSDDMode != "" {
		t.Error("SDDMode should be reset")
	}
}
