package tui

import (
	"testing"

	"github.com/lleontor705/cortex-ia/internal/model"
)

// ---------------------------------------------------------------------------
// Constructor & welcome menu
// ---------------------------------------------------------------------------

func TestNew_DefaultValues(t *testing.T) {
	m := New(nil, "/tmp", "1.0.0")

	if m.Screen != ScreenWelcome {
		t.Errorf("Screen = %v, want ScreenWelcome", m.Screen)
	}
	if m.Persona != model.PersonaProfessional {
		t.Errorf("Persona = %v, want PersonaProfessional", m.Persona)
	}
	if !m.SDDEnabled {
		t.Error("SDDEnabled should be true by default")
	}
	if m.Preset != "full" {
		t.Errorf("Preset = %v, want full", m.Preset)
	}
	if len(m.Personas) != 3 {
		t.Errorf("len(Personas) = %d, want 3", len(m.Personas))
	}
}

func TestWelcomeOptions_Count(t *testing.T) {
	opts := welcomeOptions()
	if len(opts) != 6 {
		t.Errorf("len(welcomeOptions) = %d, want 6", len(opts))
	}
}

func TestWelcomeOptions_Order(t *testing.T) {
	opts := welcomeOptions()
	if opts[0] != WelcomeInstall {
		t.Errorf("first option = %v, want WelcomeInstall", opts[0])
	}
	if opts[len(opts)-1] != WelcomeQuit {
		t.Errorf("last option = %v, want WelcomeQuit", opts[len(opts)-1])
	}
}

func TestWelcomeLabel_AllOptions(t *testing.T) {
	for _, opt := range welcomeOptions() {
		label := welcomeLabel(opt)
		if label == "" {
			t.Errorf("welcomeLabel(%v) returned empty string", opt)
		}
	}
}

func TestWelcomeLabel_Unknown(t *testing.T) {
	label := welcomeLabel(WelcomeOption(999))
	if label != "" {
		t.Errorf("welcomeLabel(999) = %q, want empty string", label)
	}
}

// ---------------------------------------------------------------------------
// Agent helpers
// ---------------------------------------------------------------------------

func TestSelectedAgentIDs_NoneSelected(t *testing.T) {
	m := New(nil, "/tmp", "1.0.0")
	m.Agents = []AgentItem{
		{ID: model.AgentClaudeCode, Selected: false},
		{ID: model.AgentGeminiCLI, Selected: false},
	}
	ids := m.SelectedAgentIDs()
	if ids != nil {
		t.Errorf("SelectedAgentIDs = %v, want nil", ids)
	}
}

func TestSelectedAgentIDs_SomeSelected(t *testing.T) {
	m := New(nil, "/tmp", "1.0.0")
	m.Agents = []AgentItem{
		{ID: model.AgentClaudeCode, Selected: true},
		{ID: model.AgentGeminiCLI, Selected: false},
		{ID: model.AgentCodex, Selected: true},
	}
	ids := m.SelectedAgentIDs()
	if len(ids) != 2 {
		t.Fatalf("len(SelectedAgentIDs) = %d, want 2", len(ids))
	}
	if ids[0] != model.AgentClaudeCode {
		t.Errorf("ids[0] = %v, want AgentClaudeCode", ids[0])
	}
	if ids[1] != model.AgentCodex {
		t.Errorf("ids[1] = %v, want AgentCodex", ids[1])
	}
}

func TestHasSelectedAgents_True(t *testing.T) {
	m := New(nil, "/tmp", "1.0.0")
	m.Agents = []AgentItem{
		{ID: model.AgentClaudeCode, Selected: false},
		{ID: model.AgentGeminiCLI, Selected: true},
	}
	if !m.HasSelectedAgents() {
		t.Error("HasSelectedAgents should be true when at least one agent is selected")
	}
}

func TestHasSelectedAgents_False(t *testing.T) {
	m := New(nil, "/tmp", "1.0.0")
	m.Agents = []AgentItem{
		{ID: model.AgentClaudeCode, Selected: false},
	}
	if m.HasSelectedAgents() {
		t.Error("HasSelectedAgents should be false when no agent is selected")
	}
}

func TestHasSelectedAgents_Empty(t *testing.T) {
	m := New(nil, "/tmp", "1.0.0")
	// Agents is nil by default from New()
	if m.HasSelectedAgents() {
		t.Error("HasSelectedAgents should be false when Agents is nil")
	}
}

// ---------------------------------------------------------------------------
// setScreen
// ---------------------------------------------------------------------------

func TestSetScreen_SavesPrevious(t *testing.T) {
	m := New(nil, "/tmp", "1.0.0")
	m.Screen = ScreenWelcome
	m.Cursor = 5

	m.setScreen(ScreenDetection)

	if m.PreviousScreen != ScreenWelcome {
		t.Errorf("PreviousScreen = %v, want ScreenWelcome", m.PreviousScreen)
	}
	if m.Screen != ScreenDetection {
		t.Errorf("Screen = %v, want ScreenDetection", m.Screen)
	}
	if m.Cursor != 0 {
		t.Errorf("Cursor = %d, want 0 after setScreen", m.Cursor)
	}
}
