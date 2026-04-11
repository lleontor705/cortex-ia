package screens

import (
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/model"
)

func TestRenderReview_ShowsAgents(t *testing.T) {
	data := ReviewData{
		Agents:   []ReviewAgent{{Name: "claude-code"}, {Name: "gemini-cli"}},
		Preset:   model.PresetFull,
		Persona:  model.PersonaProfessional,
		Resolved: []model.ComponentID{model.ComponentCortex},
	}
	output := RenderReview(data)
	if !strings.Contains(output, "claude-code") {
		t.Error("expected 'claude-code' in output")
	}
	if !strings.Contains(output, "gemini-cli") {
		t.Error("expected 'gemini-cli' in output")
	}
}

func TestRenderReview_ShowsPresetAndPersona(t *testing.T) {
	data := ReviewData{
		Agents:   []ReviewAgent{{Name: "test-agent"}},
		Preset:   model.PresetMinimal,
		Persona:  model.PersonaMentor,
		Resolved: []model.ComponentID{model.ComponentCortex},
	}
	output := RenderReview(data)
	if !strings.Contains(output, string(model.PresetMinimal)) {
		t.Error("expected preset 'minimal' in output")
	}
	if !strings.Contains(output, string(model.PersonaMentor)) {
		t.Error("expected persona 'mentor' in output")
	}
}

func TestRenderReview_ShowsComponents(t *testing.T) {
	data := ReviewData{
		Agents:  []ReviewAgent{{Name: "test-agent"}},
		Preset:  model.PresetFull,
		Persona: model.PersonaProfessional,
		Resolved: []model.ComponentID{
			model.ComponentCortex,
			model.ComponentSDD,
			model.ComponentMailbox,
		},
	}
	output := RenderReview(data)
	for _, id := range data.Resolved {
		if !strings.Contains(output, string(id)) {
			t.Errorf("expected component %q in output", id)
		}
	}
}
