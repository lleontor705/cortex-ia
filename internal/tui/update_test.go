package tui

import (
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lleontor705/cortex-ia/internal/agents"
	"github.com/lleontor705/cortex-ia/internal/model"
	"github.com/lleontor705/cortex-ia/internal/pipeline"
)

// ctrlCMsg returns a ctrl+c key message (not covered by the shared keyMsg helper).
func ctrlCMsg() tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyCtrlC}
}

// newTestModel creates a minimal Model suitable for testing.
func newTestModel() Model {
	return New(nil, "/tmp", "1.0.0")
}

// ---------------------------------------------------------------------------
// Global handlers
// ---------------------------------------------------------------------------

func TestUpdate_CtrlC_Quits(t *testing.T) {
	m := newTestModel()
	result, cmd := m.Update(ctrlCMsg())
	rm := result.(Model)
	if !rm.Quitting {
		t.Error("Quitting should be true after ctrl+c")
	}
	if cmd == nil {
		t.Error("cmd should be non-nil (tea.Quit)")
	}
}

func TestUpdate_Q_QuitsWhenNotInstalling(t *testing.T) {
	m := newTestModel()
	m.Screen = ScreenWelcome
	result, cmd := m.Update(keyMsg("q"))
	rm := result.(Model)
	if !rm.Quitting {
		t.Error("Quitting should be true when q pressed on welcome screen")
	}
	if cmd == nil {
		t.Error("cmd should be non-nil (tea.Quit)")
	}
}

func TestUpdate_Q_DoesNotQuitDuringInstall(t *testing.T) {
	m := newTestModel()
	m.Screen = ScreenInstalling
	result, _ := m.Update(keyMsg("q"))
	rm := result.(Model)
	if rm.Quitting {
		t.Error("Quitting should be false when q pressed during installing")
	}
}

func TestUpdate_WindowSizeMsg(t *testing.T) {
	m := newTestModel()
	result, cmd := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	rm := result.(Model)
	if rm.Width != 120 {
		t.Errorf("Width = %d, want 120", rm.Width)
	}
	if rm.Height != 40 {
		t.Errorf("Height = %d, want 40", rm.Height)
	}
	if cmd != nil {
		t.Error("cmd should be nil for WindowSizeMsg")
	}
}

// ---------------------------------------------------------------------------
// Message handlers
// ---------------------------------------------------------------------------

func TestUpdate_PipelineDoneMsg(t *testing.T) {
	m := newTestModel()
	m.Screen = ScreenInstalling
	m.PipelineRunning = true

	msg := PipelineDoneMsg{
		Result: pipeline.InstallResult{
			BackupID:       "bk-123",
			ComponentsDone: []model.ComponentID{model.ComponentCortex},
		},
	}
	result, _ := m.Update(msg)
	rm := result.(Model)

	if rm.Result.BackupID != "bk-123" {
		t.Errorf("Result.BackupID = %q, want %q", rm.Result.BackupID, "bk-123")
	}
	if rm.Screen != ScreenComplete {
		t.Errorf("Screen = %v, want ScreenComplete", rm.Screen)
	}
	if rm.PipelineRunning {
		t.Error("PipelineRunning should be false after PipelineDoneMsg")
	}
}

func TestUpdate_StepProgressMsg_UpdatesProgress(t *testing.T) {
	m := newTestModel()
	m.Screen = ScreenInstalling
	m.PipelineRunning = true
	m.Progress = NewProgressState([]string{"agent/comp-a", "agent/comp-b"})
	ch := make(chan StepProgressMsg, 10)
	m.progressCh = ch

	// Send a running status
	msg := StepProgressMsg{StepID: "agent/comp-a", Status: ProgressStatusRunning}
	result, cmd := m.Update(msg)
	rm := result.(Model)
	if rm.Progress.Items[0].Status != ProgressStatusRunning {
		t.Errorf("Items[0].Status = %q, want %q", rm.Progress.Items[0].Status, ProgressStatusRunning)
	}
	// Should re-schedule listener since pipeline is still running
	if cmd == nil {
		t.Error("cmd should be non-nil (listenProgress) while PipelineRunning")
	}

	// Send a succeeded status
	msg = StepProgressMsg{StepID: "agent/comp-a", Status: ProgressStatusSucceeded}
	result, _ = rm.Update(msg)
	rm = result.(Model)
	if rm.Progress.Items[0].Status != ProgressStatusSucceeded {
		t.Errorf("Items[0].Status = %q, want %q", rm.Progress.Items[0].Status, ProgressStatusSucceeded)
	}
}

func TestUpdate_StepProgressMsg_StopsListeningAfterPipelineDone(t *testing.T) {
	m := newTestModel()
	m.Screen = ScreenInstalling
	m.PipelineRunning = false // pipeline already done
	m.Progress = NewProgressState([]string{"agent/comp-a"})
	m.progressCh = nil

	msg := StepProgressMsg{StepID: "agent/comp-a", Status: ProgressStatusSucceeded}
	_, cmd := m.Update(msg)
	if cmd != nil {
		t.Error("cmd should be nil when PipelineRunning is false")
	}
}

func TestReview_Enter_InitializesProgressAndChannel(t *testing.T) {
	m := newTestModel()
	m.Screen = ScreenReview
	m.Agents = []AgentItem{
		{ID: model.AgentClaudeCode, Selected: true},
	}
	m.Resolved = []model.ComponentID{model.ComponentCortex, model.ComponentSDD}
	m.ExecuteFn = func(sel model.Selection, onProgress pipeline.ProgressFunc) pipeline.InstallResult {
		// Simulate calling onProgress
		if onProgress != nil {
			for _, c := range sel.Components {
				for _, a := range sel.Agents {
					stepID := fmt.Sprintf("%s/%s", a, c)
					onProgress(stepID, ProgressStatusRunning, nil)
					onProgress(stepID, ProgressStatusSucceeded, nil)
				}
			}
		}
		return pipeline.InstallResult{BackupID: "test-bk"}
	}

	result, cmd := m.Update(keyMsg("enter"))
	rm := result.(Model)
	if rm.Screen != ScreenInstalling {
		t.Errorf("Screen = %v, want ScreenInstalling", rm.Screen)
	}
	if !rm.PipelineRunning {
		t.Error("PipelineRunning should be true")
	}
	if len(rm.Progress.Items) != 2 {
		t.Errorf("Progress.Items len = %d, want 2", len(rm.Progress.Items))
	}
	if rm.progressCh == nil {
		t.Error("progressCh should not be nil")
	}
	if cmd == nil {
		t.Error("cmd should be non-nil (tea.Batch)")
	}
}

func TestUpdate_TickMsg_IncrementsSpinner(t *testing.T) {
	m := newTestModel()
	m.SpinnerFrame = 5
	result, _ := m.Update(TickMsg{})
	rm := result.(Model)
	if rm.SpinnerFrame != 6 {
		t.Errorf("SpinnerFrame = %d, want 6", rm.SpinnerFrame)
	}
}

func TestUpdate_TickMsg_ContinuesWhenRunning(t *testing.T) {
	m := newTestModel()
	m.PipelineRunning = true
	_, cmd := m.Update(TickMsg{})
	if cmd == nil {
		t.Error("cmd should be non-nil (tickCmd) when PipelineRunning is true")
	}
}

// ---------------------------------------------------------------------------
// Screen transitions (key events)
// ---------------------------------------------------------------------------

func TestUpdateWelcome_EnterInstall(t *testing.T) {
	reg := agents.NewRegistry()
	m := New(reg, "/tmp", "1.0.0")
	m.Screen = ScreenWelcome
	m.Cursor = 0 // WelcomeInstall

	result, _ := m.Update(keyMsg("enter"))
	rm := result.(Model)
	if rm.Screen != ScreenDetection {
		t.Errorf("Screen = %v, want ScreenDetection", rm.Screen)
	}
	if rm.SysInfo == nil {
		t.Error("SysInfo should be populated after RunDetection")
	}
}

func TestUpdateWelcome_EnterQuit(t *testing.T) {
	m := newTestModel()
	m.Screen = ScreenWelcome
	// WelcomeQuit is the last option (index 8)
	m.Cursor = len(welcomeOptions()) - 1

	result, cmd := m.Update(keyMsg("enter"))
	rm := result.(Model)
	if !rm.Quitting {
		t.Error("Quitting should be true after selecting Quit")
	}
	if cmd == nil {
		t.Error("cmd should be non-nil (tea.Quit)")
	}
}

func TestUpdateWelcome_UpDown(t *testing.T) {
	m := newTestModel()
	m.Screen = ScreenWelcome
	m.Cursor = 0

	// Move down
	result, _ := m.Update(keyMsg("down"))
	rm := result.(Model)
	if rm.Cursor != 1 {
		t.Errorf("Cursor after down = %d, want 1", rm.Cursor)
	}

	// Move down again
	result, _ = rm.Update(keyMsg("down"))
	rm = result.(Model)
	if rm.Cursor != 2 {
		t.Errorf("Cursor after second down = %d, want 2", rm.Cursor)
	}

	// Move up
	result, _ = rm.Update(keyMsg("up"))
	rm = result.(Model)
	if rm.Cursor != 1 {
		t.Errorf("Cursor after up = %d, want 1", rm.Cursor)
	}

	// Move up past 0 should stay at 0
	rm.Cursor = 0
	result, _ = rm.Update(keyMsg("up"))
	rm = result.(Model)
	if rm.Cursor != 0 {
		t.Errorf("Cursor should stay at 0, got %d", rm.Cursor)
	}

	// Move down past max should stay at max
	max := len(welcomeOptions()) - 1
	rm.Cursor = max
	result, _ = rm.Update(keyMsg("down"))
	rm = result.(Model)
	if rm.Cursor != max {
		t.Errorf("Cursor should stay at %d, got %d", max, rm.Cursor)
	}
}

func TestUpdateDetection_Enter(t *testing.T) {
	m := newTestModel()
	m.Screen = ScreenDetection

	result, _ := m.Update(keyMsg("enter"))
	rm := result.(Model)
	if rm.Screen != ScreenAgents {
		t.Errorf("Screen = %v, want ScreenAgents", rm.Screen)
	}
}

func TestUpdateDetection_Esc(t *testing.T) {
	m := newTestModel()
	m.Screen = ScreenDetection

	result, _ := m.Update(keyMsg("esc"))
	rm := result.(Model)
	if rm.Screen != ScreenWelcome {
		t.Errorf("Screen = %v, want ScreenWelcome", rm.Screen)
	}
}

func TestUpdateAgents_SpaceToggle(t *testing.T) {
	m := newTestModel()
	m.Screen = ScreenAgents
	m.Agents = []AgentItem{
		{ID: model.AgentClaudeCode, Selected: true},
		{ID: model.AgentGeminiCLI, Selected: false},
	}
	m.Cursor = 0

	// Toggle first agent off
	result, _ := m.Update(keyMsg(" "))
	rm := result.(Model)
	if rm.Agents[0].Selected {
		t.Error("Agent 0 should be deselected after space toggle")
	}

	// Toggle it back on
	result, _ = rm.Update(keyMsg(" "))
	rm = result.(Model)
	if !rm.Agents[0].Selected {
		t.Error("Agent 0 should be selected after second space toggle")
	}
}

func TestUpdateAgents_Enter(t *testing.T) {
	m := newTestModel()
	m.Screen = ScreenAgents
	m.Agents = []AgentItem{
		{ID: model.AgentClaudeCode, Selected: true},
	}

	result, _ := m.Update(keyMsg("enter"))
	rm := result.(Model)
	if rm.Screen != ScreenPersona {
		t.Errorf("Screen = %v, want ScreenPersona", rm.Screen)
	}
}

func TestUpdateAgents_EnterNoSelection(t *testing.T) {
	m := newTestModel()
	m.Screen = ScreenAgents
	m.Agents = []AgentItem{
		{ID: model.AgentClaudeCode, Selected: false},
	}

	result, _ := m.Update(keyMsg("enter"))
	rm := result.(Model)
	if rm.Screen != ScreenAgents {
		t.Errorf("Screen = %v, want ScreenAgents (should stay when no agents selected)", rm.Screen)
	}
}

func TestUpdatePersona_Enter(t *testing.T) {
	m := newTestModel()
	m.Screen = ScreenPersona
	m.Cursor = 1 // PersonaMentor (index 1 in default Personas)

	result, _ := m.Update(keyMsg("enter"))
	rm := result.(Model)
	if rm.Persona != model.PersonaMentor {
		t.Errorf("Persona = %v, want PersonaMentor", rm.Persona)
	}
	if rm.Screen != ScreenPreset {
		t.Errorf("Screen = %v, want ScreenPreset", rm.Screen)
	}
}

func TestUpdatePreset_Enter(t *testing.T) {
	m := newTestModel()
	m.Screen = ScreenPreset
	m.Cursor = 0 // PresetFull (index 0 in default Presets)

	result, _ := m.Update(keyMsg("enter"))
	rm := result.(Model)
	if rm.Preset != model.PresetFull {
		t.Errorf("Preset = %v, want PresetFull", rm.Preset)
	}
	if rm.Resolved == nil {
		t.Error("Resolved should be populated after preset selection")
	}
	if rm.Screen != ScreenClaudeModelPicker {
		t.Errorf("Screen = %v, want ScreenClaudeModelPicker", rm.Screen)
	}
}

func TestUpdateComplete_Enter(t *testing.T) {
	m := newTestModel()
	m.Screen = ScreenComplete

	result, cmd := m.Update(keyMsg("enter"))
	rm := result.(Model)
	if !rm.Quitting {
		t.Error("Quitting should be true after enter on complete screen")
	}
	if cmd == nil {
		t.Error("cmd should be non-nil (tea.Quit)")
	}
}

