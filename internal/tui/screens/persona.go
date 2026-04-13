package screens

import (
	"strings"

	"github.com/lleontor705/cortex-ia/internal/model"
	"github.com/lleontor705/cortex-ia/internal/tui/styles"
)

// PersonaData holds the data needed to render the persona selection screen.
type PersonaData struct {
	Personas []model.PersonaID
	Cursor   int
	Selected model.PersonaID
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
		label := string(p) + " — " + descs[p]
		isSelected := p == data.Selected
		isFocused := i == data.Cursor
		sb.WriteString(RenderRadio(label, isSelected, isFocused))
	}

	return sb.String()
}
