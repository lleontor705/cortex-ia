package screens

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/lleontor705/cortex-ia/internal/tui/styles"
)

// AgentData represents an agent in the selection list.
type AgentData struct {
	Name     string
	Binary   string
	Selected bool
}

// AgentsData holds the data needed to render the agent selection screen.
type AgentsData struct {
	Agents []AgentData
	Cursor int
}

// RenderAgents renders the agent selection screen.
func RenderAgents(data AgentsData) string {
	var sb strings.Builder

	sb.WriteString(styles.Title.Render("Select Agents"))
	sb.WriteString("\n\n")

	for i, a := range data.Agents {
		cursor := "  "
		if i == data.Cursor {
			cursor = styles.Cursor.Render("> ")
		}

		check := "○"
		nameStyle := lipgloss.NewStyle()
		if a.Selected {
			check = styles.Selected.Render("●")
			nameStyle = styles.Selected
		}

		fmt.Fprintf(&sb, "%s%s %s", cursor, check, nameStyle.Render(a.Name))
		if a.Binary != "" {
			sb.WriteString(styles.Description.Render(fmt.Sprintf(" (%s)", a.Binary)))
		}
		sb.WriteString("\n")
	}

	sb.WriteString(styles.Help.Render("\n↑↓ navigate • Space toggle • a all • Enter confirm • Esc back"))
	return sb.String()
}
