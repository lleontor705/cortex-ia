package tui

import (
	"strings"
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

// ToastDismissMsg is sent when a toast should be dismissed (removes the oldest).
type ToastDismissMsg struct{}

// Toast holds the state for a temporary notification.
type Toast struct {
	Text    string
	IsError bool
	Visible bool
}

// ToastQueue manages up to maxToasts visible notifications.
type ToastQueue struct {
	Items []Toast
}

const maxToasts = 3

// Push adds a toast to the queue, evicting the oldest if full.
func (q *ToastQueue) Push(t Toast) {
	if len(q.Items) >= maxToasts {
		q.Items = q.Items[1:]
	}
	q.Items = append(q.Items, t)
}

// Dismiss removes the oldest toast.
func (q *ToastQueue) Dismiss() {
	if len(q.Items) > 0 {
		q.Items = q.Items[1:]
	}
}

// HasVisible returns true if there are any toasts to show.
func (q *ToastQueue) HasVisible() bool {
	return len(q.Items) > 0
}

var toastStyle = lipgloss.NewStyle().
	Padding(0, 2).
	Bold(true)

// renderToastQueue renders all active toasts stacked vertically.
func renderToastQueue(q ToastQueue, width int) string {
	if len(q.Items) == 0 {
		return ""
	}
	var lines []string
	for _, t := range q.Items {
		if t.Text == "" {
			continue
		}
		style := toastStyle.Foreground(styles.Success)
		icon := "✓ "
		if t.IsError {
			style = toastStyle.Foreground(styles.Error)
			icon = "✗ "
		}
		rendered := style.Render(icon + t.Text)
		if width > 0 {
			rendered = lipgloss.Place(width, 1, lipgloss.Right, lipgloss.Top, rendered)
		}
		lines = append(lines, rendered)
	}
	return strings.Join(lines, "\n")
}

// Legacy compat: renderToast for single toast (used by View)
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
