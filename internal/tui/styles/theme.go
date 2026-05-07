package styles

import "github.com/charmbracelet/lipgloss"

// ThemeID identifies a theme.
type ThemeID string

const (
	ThemeDark         ThemeID = "dark"
	ThemeLight        ThemeID = "light"
	ThemeHighContrast ThemeID = "high-contrast"
)

// Theme holds all color values for a TUI theme.
type Theme struct {
	Primary   lipgloss.Color
	Secondary lipgloss.Color
	Success   lipgloss.Color
	Warning   lipgloss.Color
	Error     lipgloss.Color
	Muted     lipgloss.Color
	White     lipgloss.Color
	BgColor   lipgloss.Color
}

var darkTheme = Theme{
	Primary:   lipgloss.Color("#7C3AED"),
	Secondary: lipgloss.Color("#06B6D4"),
	Success:   lipgloss.Color("#22C55E"),
	Warning:   lipgloss.Color("#F59E0B"),
	Error:     lipgloss.Color("#EF4444"),
	Muted:     lipgloss.Color("#6B7280"),
	White:     lipgloss.Color("#F9FAFB"),
	BgColor:   lipgloss.Color("#1E1E2E"),
}

var lightTheme = Theme{
	Primary:   lipgloss.Color("#6D28D9"),
	Secondary: lipgloss.Color("#0891B2"),
	Success:   lipgloss.Color("#16A34A"),
	Warning:   lipgloss.Color("#D97706"),
	Error:     lipgloss.Color("#DC2626"),
	Muted:     lipgloss.Color("#6B7280"),
	White:     lipgloss.Color("#1F2937"),
	BgColor:   lipgloss.Color("#F9FAFB"),
}

var highContrastTheme = Theme{
	Primary:   lipgloss.Color("#FFFFFF"),
	Secondary: lipgloss.Color("#00FFFF"),
	Success:   lipgloss.Color("#00FF00"),
	Warning:   lipgloss.Color("#FFFF00"),
	Error:     lipgloss.Color("#FF0000"),
	Muted:     lipgloss.Color("#AAAAAA"),
	White:     lipgloss.Color("#FFFFFF"),
	BgColor:   lipgloss.Color("#000000"),
}

// ActiveTheme tracks the currently active theme.
var ActiveTheme ThemeID = ThemeDark

// Current color variables — updated by ApplyTheme.
var (
	Primary   = darkTheme.Primary
	Secondary = darkTheme.Secondary
	Success   = darkTheme.Success
	Warning   = darkTheme.Warning
	Error     = darkTheme.Error
	Muted     = darkTheme.Muted
	White     = darkTheme.White
)

// Styles — rebuilt by ApplyTheme.
var (
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

	// Frame is an alias for Box (kept for backwards compatibility).
	Frame = Box

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

// ToggleTheme cycles through dark → light → high-contrast themes.
func ToggleTheme() {
	switch ActiveTheme {
	case ThemeDark:
		ApplyTheme(ThemeLight)
	case ThemeLight:
		ApplyTheme(ThemeHighContrast)
	default:
		ApplyTheme(ThemeDark)
	}
}

// ApplyTheme sets all color and style variables to match the given theme.
func ApplyTheme(id ThemeID) {
	ActiveTheme = id

	var t Theme
	switch id {
	case ThemeLight:
		t = lightTheme
	case ThemeHighContrast:
		t = highContrastTheme
	default:
		t = darkTheme
	}

	Primary = t.Primary
	Secondary = t.Secondary
	Success = t.Success
	Warning = t.Warning
	Error = t.Error
	Muted = t.Muted
	White = t.White

	Title = lipgloss.NewStyle().Bold(true).Foreground(Primary).MarginBottom(1)
	Subtitle = lipgloss.NewStyle().Foreground(Secondary).Bold(true)
	Description = lipgloss.NewStyle().Foreground(Muted)
	Selected = lipgloss.NewStyle().Foreground(Success).Bold(true)
	Cursor = lipgloss.NewStyle().Foreground(Primary).Bold(true)
	StatusOK = lipgloss.NewStyle().Foreground(Success)
	StatusFail = lipgloss.NewStyle().Foreground(Error)
	StatusWarn = lipgloss.NewStyle().Foreground(Warning)
	Help = lipgloss.NewStyle().Foreground(Muted).MarginTop(1)
	Box = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(Primary).Padding(1, 2)
	Frame = Box
	Panel = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(Muted).Padding(0, 1)
	ProgressFilled = lipgloss.NewStyle().Foreground(Success).Bold(true)
	ProgressEmpty = lipgloss.NewStyle().Foreground(Muted)
	Percent = lipgloss.NewStyle().Foreground(Secondary).Bold(true)
}
