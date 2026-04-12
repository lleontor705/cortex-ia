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

	Frame = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(Primary).
		Padding(1, 2)

	Panel = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(Muted).
		Padding(0, 1)

	ProgressFilled = lipgloss.NewStyle().
			Foreground(Success).
			Bold(true)

	ProgressEmpty = lipgloss.NewStyle().
			Foreground(Muted)

	Percent = lipgloss.NewStyle().
		Foreground(Secondary).
		Bold(true)
)

// CursorPrefix is the cursor indicator string used in selection lists.
const CursorPrefix = "> "

// SpinnerFrames contains the spinner animation characters.
var SpinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// SpinnerChar returns the spinner character for the given frame index.
func SpinnerChar(frame int) string {
	return SpinnerFrames[frame%len(SpinnerFrames)]
}

const (
	Logo = `                __                      _
  _________  _____/ /____  _  __      (_)___ _
 / ___/ __ \/ ___/ __/ _ \| |/_/_____/ / __ ` + "`" + `/
/ /__/ /_/ / /  / /_/  __/>  </_____/ / /_/ /
\___/\____/_/   \__/\___/_/|_|    /_/\__,_/  `
)
