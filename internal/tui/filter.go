package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
	"github.com/lleontor705/cortex-ia/internal/tui/styles"
)

// FilterInput wraps a textinput for list filtering.
type FilterInput struct {
	Input  textinput.Model
	Active bool
}

// NewFilterInput creates a new filter input.
func NewFilterInput() FilterInput {
	ti := textinput.New()
	ti.Placeholder = "Type to filter..."
	ti.Prompt = "/ "
	ti.PromptStyle = lipgloss.NewStyle().Foreground(styles.Secondary).Bold(true)
	ti.TextStyle = lipgloss.NewStyle().Foreground(styles.White)
	ti.PlaceholderStyle = lipgloss.NewStyle().Foreground(styles.Muted)
	ti.CharLimit = 40
	return FilterInput{Input: ti}
}

// Toggle activates or deactivates the filter.
func (f *FilterInput) Toggle() {
	f.Active = !f.Active
	if f.Active {
		f.Input.Focus()
	} else {
		f.Input.Blur()
		f.Input.SetValue("")
	}
}

// Activate turns on the filter.
func (f *FilterInput) Activate() {
	f.Active = true
	f.Input.Focus()
}

// Deactivate turns off the filter and clears it.
func (f *FilterInput) Deactivate() {
	f.Active = false
	f.Input.Blur()
	f.Input.SetValue("")
}

// Query returns the current filter query (lowercase).
func (f FilterInput) Query() string {
	return strings.ToLower(f.Input.Value())
}

// Matches returns true if the given text matches the filter query.
func (f FilterInput) Matches(text string) bool {
	q := f.Query()
	if q == "" {
		return true
	}
	return strings.Contains(strings.ToLower(text), q)
}

// View renders the filter input.
func (f FilterInput) View() string {
	if !f.Active {
		return ""
	}
	return f.Input.View() + "\n"
}

// Hint returns a dim hint when filter is available but not active.
func (f FilterInput) Hint() string {
	if f.Active || f.Query() != "" {
		return ""
	}
	return styles.Description.Render("  [/ filter]")
}
