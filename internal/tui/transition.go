package tui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lleontor705/cortex-ia/internal/tui/styles"
)

// TransitionFrameMsg advances the transition animation.
type TransitionFrameMsg struct{}

// Transition tracks a simple fade-in animation between screen changes.
type Transition struct {
	Active bool
	Frame  int
	Total  int
}

const transitionFrames = 3

// startTransition begins a transition animation.
func startTransition() (Transition, tea.Cmd) {
	t := Transition{Active: true, Frame: 0, Total: transitionFrames}
	return t, tea.Tick(40*time.Millisecond, func(_ time.Time) tea.Msg {
		return TransitionFrameMsg{}
	})
}

// advanceTransition moves the transition to the next frame.
func (t *Transition) advanceTransition() tea.Cmd {
	t.Frame++
	if t.Frame >= t.Total {
		t.Active = false
		return nil
	}
	return tea.Tick(40*time.Millisecond, func(_ time.Time) tea.Msg {
		return TransitionFrameMsg{}
	})
}

// applyTransition applies a fade-in effect to the content based on the current frame.
func applyTransition(content string, t Transition) string {
	if !t.Active {
		return content
	}

	lines := strings.Split(content, "\n")
	// Show progressively more lines based on frame
	visibleRatio := float64(t.Frame+1) / float64(t.Total)
	visibleLines := int(float64(len(lines)) * visibleRatio)
	if visibleLines < 1 {
		visibleLines = 1
	}
	if visibleLines > len(lines) {
		visibleLines = len(lines)
	}

	var result strings.Builder
	for i := 0; i < visibleLines; i++ {
		result.WriteString(lines[i])
		if i < visibleLines-1 {
			result.WriteString("\n")
		}
	}

	// Dim early frames
	if t.Frame == 0 {
		return lipgloss.NewStyle().Foreground(styles.Muted).Render(result.String())
	}
	return result.String()
}
