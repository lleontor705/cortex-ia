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

// RenderAgents renders the agent selection screen with a counter header and
// a help bar listing the relevant shortcuts.
func RenderAgents(data AgentsData) string {
	var sb strings.Builder

	selected := 0
	for _, a := range data.Agents {
		if a.Selected {
			selected++
		}
	}
	header := fmt.Sprintf("Select Agents  %s",
		styles.Description.Render(fmt.Sprintf("(%d of %d selected)", selected, len(data.Agents))))
	sb.WriteString(styles.Title.Render(header))
	sb.WriteString("\n\n")

	if len(data.Agents) == 0 {
		sb.WriteString(styles.StatusWarn.Render("  ⚠ No agents detected. Install one and re-run `cortex-ia detect`."))
		sb.WriteString("\n\n")
		sb.WriteString(RenderHelpBar("esc back"))
		return sb.String()
	}

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

	if start > 0 {
		sb.WriteString(RenderScrollIndicator(total, start, end))
	}

	for i := start; i < end; i++ {
		a := data.Agents[i]
		cursor := "  "
		if i == data.Cursor {
			cursor = styles.Cursor.Render(styles.CursorPrefix)
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

	if end < total {
		sb.WriteString(RenderScrollIndicator(total, start, end))
	}

	sb.WriteString("\n")
	sb.WriteString(RenderHelpBar(
		"↑↓ navigate",
		"space toggle",
		"a select all",
		"n select none",
		"enter continue",
		"esc back",
	))

	return sb.String()
}
