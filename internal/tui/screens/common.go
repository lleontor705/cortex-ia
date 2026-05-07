package screens

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/lleontor705/cortex-ia/internal/tui/styles"
)

// MenuItem is a single row in a grouped menu.
type MenuItem struct {
	Label  string
	Hint   string
	Hotkey string
	Badge  string
}

// MenuGroup is a labelled cluster of related MenuItems.
type MenuGroup struct {
	Title string
	Items []MenuItem
}

// FlattenMenu returns the items in display order. The cursor index used by
// callers maps directly to this flat slice; group titles are presentational.
func FlattenMenu(groups []MenuGroup) []MenuItem {
	total := 0
	for _, g := range groups {
		total += len(g.Items)
	}
	out := make([]MenuItem, 0, total)
	for _, g := range groups {
		out = append(out, g.Items...)
	}
	return out
}

// FindItemByHotkey returns the global index of the menu item whose Hotkey
// matches key. Returns (-1, false) when no item declares that hotkey.
func FindItemByHotkey(groups []MenuGroup, key string) (int, bool) {
	idx := 0
	for _, g := range groups {
		for _, it := range g.Items {
			if it.Hotkey == key {
				return idx, true
			}
			idx++
		}
	}
	return -1, false
}

// RenderMenu draws a grouped menu with hotkeys, descriptions, and a focus cursor.
// `cursor` is the GLOBAL index across all groups (FlattenMenu order).
func RenderMenu(groups []MenuGroup, cursor int) string {
	var sb strings.Builder

	maxLabel := 0
	for _, g := range groups {
		for _, it := range g.Items {
			full := formatMenuLabel(it)
			if w := lipgloss.Width(full); w > maxLabel {
				maxLabel = w
			}
		}
	}
	padTo := maxLabel + 2

	idx := 0
	for gi, group := range groups {
		if gi > 0 {
			sb.WriteString("\n")
		}
		if group.Title != "" {
			sb.WriteString(styles.Subtitle.Render(group.Title))
			sb.WriteString("\n")
		}
		for _, it := range group.Items {
			focused := idx == cursor
			sb.WriteString(renderMenuRow(it, focused, padTo))
			sb.WriteString("\n")
			idx++
		}
	}
	return sb.String()
}

func formatMenuLabel(it MenuItem) string {
	var sb strings.Builder
	if it.Hotkey != "" {
		sb.WriteString("[")
		sb.WriteString(it.Hotkey)
		sb.WriteString("] ")
	}
	sb.WriteString(it.Label)
	if it.Badge != "" {
		sb.WriteString(" ")
		sb.WriteString(it.Badge)
	}
	return sb.String()
}

func renderMenuRow(it MenuItem, focused bool, padTo int) string {
	cursor := "  "
	labelStyle := lipgloss.NewStyle()
	hintStyle := styles.Description
	if focused {
		cursor = styles.Cursor.Render(styles.CursorPrefix)
		labelStyle = styles.Selected
	}

	label := formatMenuLabel(it)
	visualLabel := labelStyle.Render(label)

	gap := padTo - lipgloss.Width(label)
	if gap < 1 {
		gap = 1
	}

	if it.Hint == "" {
		return cursor + visualLabel
	}
	return cursor + visualLabel + strings.Repeat(" ", gap) + hintStyle.Render(it.Hint)
}

// RenderHelpBar renders a single-line keybinding hint for the bottom of any screen.
func RenderHelpBar(bindings ...string) string {
	if len(bindings) == 0 {
		return ""
	}
	return styles.Help.Render(strings.Join(bindings, "  ·  "))
}

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
