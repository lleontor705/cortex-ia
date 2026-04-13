package screens

import (
	"fmt"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/catalog"
	"github.com/lleontor705/cortex-ia/internal/model"
	"github.com/lleontor705/cortex-ia/internal/tui/styles"
)

// ReviewAgent represents an agent in the review summary.
type ReviewAgent struct {
	Name string
}

// ReviewData holds the data needed to render the review screen.
type ReviewData struct {
	Agents   []ReviewAgent
	Preset   model.PresetID
	Persona  model.PersonaID
	Resolved []model.ComponentID
}

// RenderReview renders the installation review screen.
func RenderReview(data ReviewData) string {
	var sb strings.Builder

	sb.WriteString(styles.Title.Render("Review Installation"))
	sb.WriteString("\n\n")

	sb.WriteString(styles.Subtitle.Render("Agents:"))
	sb.WriteString("\n")
	for _, a := range data.Agents {
		fmt.Fprintf(&sb, "  %s %s\n", styles.StatusOK.Render("●"), a.Name)
	}

	sb.WriteString("\n")
	sb.WriteString(styles.Subtitle.Render(fmt.Sprintf("Preset: %s", data.Preset)))
	sb.WriteString("  ")
	sb.WriteString(styles.Subtitle.Render(fmt.Sprintf("Persona: %s", data.Persona)))
	sb.WriteString("\n\n")

	sb.WriteString(styles.Subtitle.Render("Components (with dependencies):"))
	sb.WriteString("\n")
	cmap := catalog.ComponentMap()
	for _, id := range data.Resolved {
		if info, ok := cmap[id]; ok {
			fmt.Fprintf(&sb, "  %s %-18s %s\n",
				styles.StatusOK.Render("●"),
				styles.Selected.Render(string(id)),
				styles.Description.Render(info.Description))
		}
	}

	return sb.String()
}
