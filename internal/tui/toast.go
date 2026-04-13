package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lleontor705/cortex-ia/internal/tui/styles"
)

// ToastMsg is sent when a toast should be displayed.
type ToastMsg struct {
	Text    string
	IsError bool
}

// ToastDismissMsg is sent when a toast should be dismissed.
type ToastDismissMsg struct{}

// Toast holds the state for a temporary notification.
type Toast struct {
	Text    string
	IsError bool
	Visible bool
}

var toastStyle = lipgloss.NewStyle().
	Padding(0, 2).
	Bold(true)

// renderToast renders the toast as a right-aligned overlay line.
func renderToast(t Toast, width int) string {
	if !t.Visible || t.Text == "" {
		return ""
	}
	style := toastStyle.Foreground(styles.Success)
	icon := "✓ "
	if t.IsError {
		style = toastStyle.Foreground(styles.Error)
		icon = "✗ "
	}
	rendered := style.Render(icon + t.Text)

	if width > 0 {
		return lipgloss.Place(width, 1, lipgloss.Right, lipgloss.Top, rendered)
	}
	return rendered
}

// dismissToastAfter returns a command that sends ToastDismissMsg after duration.
func dismissToastAfter(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(_ time.Time) tea.Msg {
		return ToastDismissMsg{}
	})
}
