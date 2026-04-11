package screens

import (
	"fmt"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/model"
	"github.com/lleontor705/cortex-ia/internal/tui/styles"
)

// PersonaData holds the data needed to render the persona selection screen.
type PersonaData struct {
	Personas []model.PersonaID
	Cursor   int
}

// RenderPersona renders the persona selection screen.
func RenderPersona(data PersonaData) string {
	var sb strings.Builder

	sb.WriteString(styles.Title.Render("Select Persona"))
	sb.WriteString("\n\n")

	descs := map[model.PersonaID]string{
		model.PersonaProfessional: "Direct, concise, technical terminology",
		model.PersonaMentor:       "Teaching-oriented, explains trade-offs and patterns",
		model.PersonaMinimal:      "Code only, no explanations unless asked",
	}

	for i, p := range data.Personas {
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
