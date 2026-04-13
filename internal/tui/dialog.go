package tui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/lleontor705/cortex-ia/internal/tui/styles"
)

// DialogType identifies the kind of confirmation dialog.
type DialogType int

const (
	DialogNone DialogType = iota
	DialogRestoreConfirm
	DialogDeleteConfirm
	DialogProfileDelete
)

// Dialog holds the state for a modal confirmation dialog.
type Dialog struct {
	Type    DialogType
	Title   string
	Message string
	Warning string
}

var dialogBoxStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(styles.Warning).
	Padding(1, 3).
	Width(50)

var dialogTitleStyle = lipgloss.NewStyle().
	Bold(true).
	Foreground(styles.Primary).
	MarginBottom(1)

// renderDialog renders the dialog overlay centered on the screen.
func renderDialog(d Dialog, width, height int) string {
	var content string

	content = dialogTitleStyle.Render(d.Title) + "\n\n"
	content += d.Message + "\n"

	if d.Warning != "" {
		content += "\n" + lipgloss.NewStyle().Foreground(styles.Warning).Render(d.Warning)
	}

	content += "\n\n" + lipgloss.NewStyle().Foreground(styles.Muted).Render("y/enter confirm • n/esc cancel")

	box := dialogBoxStyle.Render(content)

	if width > 0 && height > 0 {
		return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)
	}
	return box
}
