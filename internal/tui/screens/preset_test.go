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

func TestRenderPreset_ShowsCurrentComponentCount(t *testing.T) {
	data := PresetData{
		Presets: []model.PresetID{model.PresetFull, model.PresetMinimal},
		Cursor:  0,
	}
	output := RenderPreset(data)
	if !strings.Contains(output, "All 7 components") {
		t.Errorf("expected current seven-component full preset description, got:\n%s", output)
	}
	if strings.Contains(output, "All 8 components") {
		t.Errorf("preset must not advertise the retired eight-component count\n%s", output)
	}
	if !strings.Contains(output, "Cortex + ForgeSpec + Context7 + SDD") {
		t.Error("expected minimal preset description in output")
	}
}
