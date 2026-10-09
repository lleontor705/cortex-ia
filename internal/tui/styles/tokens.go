package styles

import "github.com/charmbracelet/lipgloss"

// NanInkTokens is the resolved Nan Ink palette for a single color mode.
//
// tokens.go is the single canonical source of Nan Ink colors: the TUI theme
// seam (ApplyTheme), the web console CSS, and the embedded OpenCode theme
// asset all mirror these values, and no other palette literal should exist.
//
// AccentBorder and AccentText are deliberately separate roles. AccentBorder is
// decoration-only (borders, rules, focus rings) and fails AA against the dark
// canvas, so it must never become a text foreground. AccentText is the AA-safe
// accent for text. Keeping them as distinct fields makes the decoration-only
// rule structural instead of a convention someone has to remember.
type NanInkTokens struct {
	BG           lipgloss.Color
	Surface      lipgloss.Color
	Line         lipgloss.Color
	Text         lipgloss.Color
	Body         lipgloss.Color
	Muted        lipgloss.Color
	AccentBorder lipgloss.Color
	AccentText   lipgloss.Color
	Selected     lipgloss.Color
	Success      lipgloss.Color
	Warning      lipgloss.Color
	Danger       lipgloss.Color
}

// NanInkDark is the operator-approved Nan Ink dark palette.
var NanInkDark = NanInkTokens{
	BG:           "#0B0B0C",
	Surface:      "#121214",
	Line:         "#3A3A3D",
	Text:         "#FFFFFF",
	Body:         "#C9C9CC",
	Muted:        "#9A9A9E",
	AccentBorder: "#7D39EB",
	AccentText:   "#9B6BF0",
	Selected:     "#1D1430",
	Success:      "#22C55E",
	Warning:      "#F59E0B",
	Danger:       "#EF4444",
}

// NanInkLight is the operator-approved Nan Ink "snow" light palette.
var NanInkLight = NanInkTokens{
	BG:           "#EFEEEB",
	Surface:      "#E3E1DC",
	Line:         "#D5D3CE",
	Text:         "#101010",
	Body:         "#2E2E2E",
	Muted:        "#6F6F6F",
	AccentBorder: "#7D39EB",
	AccentText:   "#7D39EB",
	Selected:     "#E7DCFB",
	Success:      "#166534",
	Warning:      "#92400E",
	Danger:       "#B91C1C",
}

// NanInkFor selects the token set for a mode, defaulting to dark.
func NanInkFor(id ThemeID) NanInkTokens {
	if id == ThemeLight {
		return NanInkLight
	}
	return NanInkDark
}
