package screens

import (
	"strings"
	"testing"
)

func TestRenderAgents_ShowsAgentNames(t *testing.T) {
	data := AgentsData{
		Agents: []AgentData{
			{Name: "claude-code", Binary: "claude", Selected: false},
			{Name: "gemini-cli", Binary: "gemini", Selected: false},
		},
		Cursor: 0,
	}
	output := RenderAgents(data)
	for _, a := range data.Agents {
		if !strings.Contains(output, a.Name) {
			t.Errorf("expected agent name %q in output", a.Name)
		}
	}
}

func TestRenderAgents_CursorPosition(t *testing.T) {
	data := AgentsData{
		Agents: []AgentData{
			{Name: "agent-alpha", Selected: false},
			{Name: "agent-beta", Selected: false},
			{Name: "agent-gamma", Selected: false},
		},
		Cursor: 1,
	}
	output := RenderAgents(data)
	lines := strings.Split(output, "\n")
	found := false
	for _, line := range lines {
		if strings.Contains(line, "agent-beta") && strings.Contains(line, "> ") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected cursor indicator on agent-beta")
	}
}

func TestRenderAgents_SelectedMarker(t *testing.T) {
	data := AgentsData{
		Agents: []AgentData{
			{Name: "agent-one", Selected: true},
			{Name: "agent-two", Selected: false},
		},
		Cursor: 0,
	}
	output := RenderAgents(data)
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "agent-one") {
			if !strings.Contains(line, "\u25cf") { // filled circle ●
				t.Error("expected filled marker for selected agent")
			}
			return
		}
	}
	t.Error("agent-one not found in output")
}
