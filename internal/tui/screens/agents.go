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
	Agents    []AgentData
	Cursor    int
	MaxHeight int // 0 means no limit
}

// RenderAgents renders the agent selection screen.
func RenderAgents(data AgentsData) string {
	var sb strings.Builder

	sb.WriteString(styles.Title.Render("Select Agents"))
	sb.WriteString("\n\n")

	// Calculate visible window
	total := len(data.Agents)
	maxVisible := total
	if data.MaxHeight > 0 && data.MaxHeight < total {
		maxVisible = data.MaxHeight
	}

	start := 0
	if data.Cursor >= maxVisible {
		start = data.Cursor - maxVisible + 1
	}
	end := start + maxVisible
	if end > total {
		end = total
		start = max(end-maxVisible, 0)
	}

	// Scroll indicator (above)
	if start > 0 {
		sb.WriteString(RenderScrollIndicator(total, start, end))
	}

	for i := start; i < end; i++ {
		a := data.Agents[i]
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

	// Scroll indicator (below)
	if end < total {
		sb.WriteString(RenderScrollIndicator(total, start, end))
	}

	return sb.String()
}
