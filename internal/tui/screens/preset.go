package screens

import (
	"strings"

	"github.com/lleontor705/cortex-ia/internal/model"
	"github.com/lleontor705/cortex-ia/internal/tui/styles"
)

// PresetData holds the data needed to render the preset selection screen.
type PresetData struct {
	Presets  []model.PresetID
	Cursor   int
	Selected model.PresetID
}

// RenderPreset renders the preset selection screen.
func RenderPreset(data PresetData) string {
	var sb strings.Builder

	sb.WriteString(styles.Title.Render("Select Preset"))
	sb.WriteString("\n\n")

	descs := map[model.PresetID]string{
		model.PresetFull:    "All 8 components — full ecosystem",
		model.PresetMinimal: "Cortex + ForgeSpec + Context7 + SDD",
	}

	for i, p := range data.Presets {
		label := string(p) + " — " + descs[p]
		isSelected := p == data.Selected
		isFocused := i == data.Cursor
		sb.WriteString(RenderRadio(label, isSelected, isFocused))
	}

	return sb.String()
}
