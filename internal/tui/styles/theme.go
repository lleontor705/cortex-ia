package styles

import "github.com/charmbracelet/lipgloss"

var (
	// Colors
	Primary   = lipgloss.Color("#7C3AED") // Violet
	Secondary = lipgloss.Color("#06B6D4") // Cyan
	Success   = lipgloss.Color("#22C55E") // Green
	Warning   = lipgloss.Color("#F59E0B") // Amber
	Error     = lipgloss.Color("#EF4444") // Red
	Muted     = lipgloss.Color("#6B7280") // Gray
	White     = lipgloss.Color("#F9FAFB")

	// Styles
	Title = lipgloss.NewStyle().
		Bold(true).
		Foreground(Primary).
		MarginBottom(1)

	Subtitle = lipgloss.NewStyle().
			Foreground(Secondary).
			Bold(true)

	Description = lipgloss.NewStyle().
			Foreground(Muted)

	Selected = lipgloss.NewStyle().
			Foreground(Success).
			Bold(true)

	Cursor = lipgloss.NewStyle().
		Foreground(Primary).
		Bold(true)

	StatusOK = lipgloss.NewStyle().
			Foreground(Success)

	StatusFail = lipgloss.NewStyle().
			Foreground(Error)

	StatusWarn = lipgloss.NewStyle().
			Foreground(Warning)

	Help = lipgloss.NewStyle().
		Foreground(Muted).
		MarginTop(1)

	Box = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(Primary).
		Padding(1, 2)
)

const (
	Logo = `                __                      _
  _________  _____/ /____  _  __      (_)___ _
 / ___/ __ \/ ___/ __/ _ \| |/_/_____/ / __ ` + "`" + `/
/ /__/ /_/ / /  / /_/  __/>  </_____/ / /_/ /
\___/\____/_/   \__/\___/_/|_|    /_/\__,_/  `
)
