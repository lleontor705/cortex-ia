package screens

import (
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/model"
)

func TestRenderReview_ShowsAgents(t *testing.T) {
	data := ReviewData{
		Agents:   []ReviewAgent{{Name: "claude-code"}, {Name: "vscode-copilot"}},
		Preset:   model.PresetFull,
		Resolved: []model.ComponentID{model.ComponentCortex},
	}
	output := RenderReview(data)
	if !strings.Contains(output, "claude-code") {
		t.Error("expected 'claude-code' in output")
	}
	if !strings.Contains(output, "vscode-copilot") {
		t.Error("expected 'vscode-copilot' in output")
	}
}

func TestRenderReview_ShowsPreset(t *testing.T) {
	data := ReviewData{
		Agents:   []ReviewAgent{{Name: "test-agent"}},
		Preset:   model.PresetMinimal,
		Resolved: []model.ComponentID{model.ComponentCortex},
	}
	output := RenderReview(data)
	if !strings.Contains(output, string(model.PresetMinimal)) {
		t.Error("expected preset 'minimal' in output")
	}
}

func TestRenderReview_ShowsComponents(t *testing.T) {
	data := ReviewData{
		Agents: []ReviewAgent{{Name: "test-agent"}},
		Preset: model.PresetFull,
		Resolved: []model.ComponentID{
			model.ComponentCortex,
			model.ComponentForgeSpec,
			model.ComponentSDD,
			model.ComponentContext7,
		},
	}
	output := RenderReview(data)
	for _, id := range data.Resolved {
		if !strings.Contains(output, string(id)) {
			t.Errorf("expected component %q in output", id)
		}
	}
}
