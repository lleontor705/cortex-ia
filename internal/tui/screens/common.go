package screens

import (
	"fmt"

	"github.com/lleontor705/cortex-ia/internal/tui/styles"
)

// RenderOptions renders a list of options with a cursor indicator.
func RenderOptions(options []string, cursor int) string {
	output := ""
	for idx, option := range options {
		if idx == cursor {
			output += styles.Selected.Render(styles.CursorPrefix+option) + "\n"
		} else {
			output += styles.Description.Render("  "+option) + "\n"
		}
	}
	return output
}

// RenderCheckbox renders a checkbox item with label, checked state and focus indicator.
func RenderCheckbox(label string, checked bool, focused bool) string {
	marker := "[ ]"
	markerStyle := styles.Description
	if checked {
		marker = "[x]"
		markerStyle = styles.StatusOK
	}

	prefix := "  "
	if focused {
		prefix = styles.CursorPrefix
		return styles.Selected.Render(prefix+markerStyle.Render(marker)+" "+label) + "\n"
	}
	return styles.Description.Render(prefix+markerStyle.Render(marker)+" "+label) + "\n"
}

// RenderRadio renders a radio button item with label, selection state and focus indicator.
func RenderRadio(label string, selected bool, focused bool) string {
	marker := "( )"
	markerStyle := styles.Description
	if selected {
		marker = "(*)"
		markerStyle = styles.Selected
	}

	prefix := "  "
	if focused {
		prefix = styles.CursorPrefix
		return styles.Selected.Render(prefix+markerStyle.Render(marker)+" "+label) + "\n"
	}
	return styles.Description.Render(prefix+markerStyle.Render(marker)+" "+label) + "\n"
}

// ProgressItem represents a single step in the installation progress display.
type ProgressItem struct {
	Label  string
	Status string
}

// InstallProgress holds the data needed to render the installation progress screen.
type InstallProgress struct {
	Percent     int
	CurrentStep string
	Items       []ProgressItem
	Logs        []string
	Done        bool
	Failed      bool
}

// ModelPickerState holds the state for the per-phase model assignment picker.
type ModelPickerState struct {
	Phases   []string
	Cursor   int
	Selected map[string]int // phase -> selected model index
}

// ClaudeModelPickerState holds the state for the Claude model tier picker.
type ClaudeModelPickerState struct {
	Options  []string
	Cursor   int
	Selected int
}

// RenderScrollIndicator renders scroll indicators for a list that may overflow.
// It shows "▲ N more above" and "▼ N more below" hints.
func RenderScrollIndicator(total, visibleStart, visibleEnd int) string {
	if total <= 0 || (visibleStart == 0 && visibleEnd >= total) {
		return ""
	}
	result := ""
	if visibleStart > 0 {
		result += styles.Description.Render(fmt.Sprintf("  ▲ %d more above\n", visibleStart))
	}
	if visibleEnd < total {
		result += styles.Description.Render(fmt.Sprintf("  ▼ %d more below\n", total-visibleEnd))
	}
	return result
}
