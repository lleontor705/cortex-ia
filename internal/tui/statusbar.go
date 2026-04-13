package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/lleontor705/cortex-ia/internal/tui/styles"
)

var statusBarStyle = lipgloss.NewStyle().
	Background(lipgloss.Color("#1E1E2E")).
	Foreground(lipgloss.Color("#CDD6F4")).
	Padding(0, 1)

var statusBarKeyStyle = lipgloss.NewStyle().
	Background(lipgloss.Color("#1E1E2E")).
	Foreground(styles.Secondary).
	Bold(true).
	Padding(0, 1)

// renderStatusBar returns a status bar with screen name, version, and context info.
func renderStatusBar(m Model) string {
	if m.Width == 0 {
		return ""
	}

	left := statusBarKeyStyle.Render(screenName(m.Screen))
	center := statusBarStyle.Render(fmt.Sprintf("cortex-ia v%s", m.Version))
	right := renderStatusContext(m)

	// Calculate padding to fill the width
	usedWidth := lipgloss.Width(left) + lipgloss.Width(center) + lipgloss.Width(right)
	gap := m.Width - usedWidth
	if gap < 0 {
		gap = 0
	}

	leftGap := gap / 2
	rightGap := gap - leftGap

	return left +
		statusBarStyle.Render(strings.Repeat(" ", leftGap)) +
		center +
		statusBarStyle.Render(strings.Repeat(" ", rightGap)) +
		right
}

// renderStatusContext returns contextual info for the right side of the status bar.
func renderStatusContext(m Model) string {
	switch m.Screen {
	case ScreenAgents:
		count := 0
		for _, a := range m.Agents {
			if a.Selected {
				count++
			}
		}
		info := fmt.Sprintf("%d/%d selected", count, len(m.Agents))
		if m.AgentFilter.Active || m.AgentFilter.Query() != "" {
			visible := len(m.visibleAgents())
			info += fmt.Sprintf(" (%d/%d shown)", visible, len(m.Agents))
		}
		return statusBarStyle.Render(info)

	case ScreenSkillPicker:
		count := 0
		for _, s := range m.AvailableSkills {
			if s.Selected {
				count++
			}
		}
		info := fmt.Sprintf("%d/%d skills", count, len(m.AvailableSkills))
		if m.SkillFilter.Active || m.SkillFilter.Query() != "" {
			visible := len(m.visibleSkills())
			info += fmt.Sprintf(" (%d/%d shown)", visible, len(m.AvailableSkills))
		}
		return statusBarStyle.Render(info)

	case ScreenBackups:
		return statusBarStyle.Render(fmt.Sprintf("%d backups", len(m.Backups)))

	case ScreenProfiles:
		return statusBarStyle.Render(fmt.Sprintf("%d profiles", len(m.Profiles)))

	case ScreenInstalling:
		return statusBarStyle.Render(fmt.Sprintf("%d%%", m.Progress.Percent()))

	case ScreenSync:
		if m.SelectedProfile != "" {
			return statusBarStyle.Render(fmt.Sprintf("profile: %s", m.SelectedProfile))
		}

	case ScreenAgentBuilderPreview:
		return statusBarStyle.Render(
			fmt.Sprintf("%d%%", int(m.AgentBuilderViewport.ScrollPercent()*100)),
		)
	}

	if m.PipelineRunning {
		return statusBarStyle.Render("installing...")
	}
	if m.OperationRunning {
		return statusBarStyle.Render("working...")
	}

	// Show active profile globally if set
	if m.SelectedProfile != "" {
		return statusBarStyle.Render(fmt.Sprintf("profile: %s", m.SelectedProfile))
	}

	return statusBarStyle.Render("? help")
}
