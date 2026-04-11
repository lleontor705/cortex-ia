package screens

import (
	"fmt"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/model"
	"github.com/lleontor705/cortex-ia/internal/tui/styles"
)

// PresetData holds the data needed to render the preset selection screen.
type PresetData struct {
	Presets []model.PresetID
	Cursor  int
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
		cursor := "  "
		if i == data.Cursor {
			cursor = styles.Cursor.Render("> ")
		}

		name := styles.Subtitle.Render(string(p))
		desc := styles.Description.Render(" — " + descs[p])
		fmt.Fprintf(&sb, "%s%s%s\n", cursor, name, desc)
	}

	sb.WriteString(styles.Help.Render("\n↑↓ navigate • Enter select • Esc back"))
	return sb.String()
}
