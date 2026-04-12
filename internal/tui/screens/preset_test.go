package screens

import (
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/model"
)

func TestRenderPreset_ShowsPresetNames(t *testing.T) {
	data := PresetData{
		Presets: []model.PresetID{model.PresetFull, model.PresetMinimal},
		Cursor:  0,
	}
	output := RenderPreset(data)
	if !strings.Contains(output, "full") {
		t.Error("expected 'full' in output")
	}
	if !strings.Contains(output, "minimal") {
		t.Error("expected 'minimal' in output")
	}
}

func TestRenderPreset_ShowsDescriptions(t *testing.T) {
	data := PresetData{
		Presets: []model.PresetID{model.PresetFull, model.PresetMinimal},
		Cursor:  0,
	}
	output := RenderPreset(data)
	if !strings.Contains(output, "All 8 components") {
		t.Error("expected 'All 8 components' in output")
	}
	if !strings.Contains(output, "Cortex + ForgeSpec + Context7 + SDD") {
		t.Error("expected minimal preset description in output")
	}
}
