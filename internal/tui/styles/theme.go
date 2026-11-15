package styles

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ThemeID identifies a theme.
type ThemeID string

const (
	ThemeDark  ThemeID = "dark"
	ThemeLight ThemeID = "light"
)

// Theme holds the eight color roles the global styles are rebuilt from.
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

// themeFromTokens maps the canonical Nan Ink palette onto the eight TUI color
// roles. Nan Ink defines a single accent, so Secondary and White resolve to the
// neutral text roles; AccentBorder is decoration-only (it fails AA on the dark
// canvas) and is never promoted into a text foreground.
func themeFromTokens(t NanInkTokens) Theme {
	return Theme{
		Primary:   t.AccentText,
		Secondary: t.Body,
		Success:   t.Success,
		Warning:   t.Warning,
		Error:     t.Danger,
		Muted:     t.Muted,
		White:     t.Text,
		BgColor:   t.BG,
	}
}

// ActiveTheme tracks the currently active theme.
var ActiveTheme ThemeID = ThemeDark

// Current color roles and global styles — rebuilt by applyRoles.
var (
	Primary    lipgloss.Color
	Secondary  lipgloss.Color
	Success    lipgloss.Color
	Warning    lipgloss.Color
	Error      lipgloss.Color
	Muted      lipgloss.Color
	White      lipgloss.Color
	Line       lipgloss.Color
	SelectedBG lipgloss.Color

	Title          lipgloss.Style
	Subtitle       lipgloss.Style
	Description    lipgloss.Style
	Selected       lipgloss.Style
	Cursor         lipgloss.Style
	StatusOK       lipgloss.Style
	StatusFail     lipgloss.Style
	StatusWarn     lipgloss.Style
	Help           lipgloss.Style
	Box            lipgloss.Style
	Frame          lipgloss.Style
	Panel          lipgloss.Style
	ProgressFilled lipgloss.Style
	ProgressEmpty  lipgloss.Style
	Percent        lipgloss.Style
)

// applyRoles rebuilds every color role and global style from one Nan Ink token
// set. Box, Frame, and the global view frame carry the hairline normal border in
// the Line token; Panel is borderless because inactive panels are separated by
// gutters rather than chrome.
func applyRoles(t NanInkTokens) {
	roles := themeFromTokens(t)
	Primary = roles.Primary
	Secondary = roles.Secondary
	Success = roles.Success
	Warning = roles.Warning
	Error = roles.Error
	Muted = roles.Muted
	White = roles.White
	Line = t.Line
	SelectedBG = t.Selected

	Title = lipgloss.NewStyle().Bold(true).Foreground(Primary).MarginBottom(1)
	Subtitle = lipgloss.NewStyle().Foreground(Secondary).Bold(true)
	Description = lipgloss.NewStyle().Foreground(Muted)
	Selected = lipgloss.NewStyle().Foreground(White).Bold(true).Background(SelectedBG)
	Cursor = lipgloss.NewStyle().Foreground(Primary).Bold(true)
	StatusOK = lipgloss.NewStyle().Foreground(Success)
	StatusFail = lipgloss.NewStyle().Foreground(Error)
	StatusWarn = lipgloss.NewStyle().Foreground(Warning)
	Help = lipgloss.NewStyle().Foreground(Muted).MarginTop(1)
	Box = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(Line).Padding(1, 2)
	Frame = Box
	Panel = lipgloss.NewStyle().Padding(0, 1)
	ProgressFilled = lipgloss.NewStyle().Foreground(Success).Bold(true)
	ProgressEmpty = lipgloss.NewStyle().Foreground(Muted)
	Percent = lipgloss.NewStyle().Foreground(Secondary).Bold(true)
}

func init() {
	applyRoles(NanInkDark)
}

// CursorPrefix is the cursor indicator string used in selection lists.
const CursorPrefix = "> "

// SpinnerFrames contains the spinner animation characters.
var SpinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// SpinnerChar returns the spinner character for the given frame index.
func SpinnerChar(frame int) string {
	return SpinnerFrames[frame%len(SpinnerFrames)]
}

const (
	Logo = `     ⣠⣶⣿⣿⣿⣿⣶⣤⡀       ⢀⣤⣶⣿⣿⣿⣿⣶⣄
  ⢰⣿⣿⠟⠉   ⠹⣿⣿⣄⣠⣿⣿⠏   ⠈⠻⣿⣿⡆
  ⣾⣿⠃  ⢰⣶⣶⡄ ⠹⣿⣿⣿⣿⠏ ⢰⣶⣶⡄  ⠘⣿⣷
 ⢸⣿⡟   ⠸⣿⣿⣧  ⠹⣿⣿⠏  ⣼⣿⣿⠇   ⢻⣿⡇
  ⠹⣿⣧⡀   ⠉⠛⠿⣿⣿⣿⣿⣿⣿⠿⠛⠉   ⢀⣼⣿⠏
   ⠙⢿⣿⣦⣄    C O R T E X · I A   ⣠⣴⣿⡿⠋
     ⠉⠛⠿⠿⣿⣶⣤⣤⣤⣤⣤⣤⣶⣿⠿⠿⠛⠉`
)

// Responsive logo thresholds. Widths are measured in terminal columns and
// heights in rows, matching lipgloss.Width semantics for ANSI-styled art.
const (
	// LogoFullWidth is the rendered width of the widest Logo row.
	LogoFullWidth = 37
	// LogoMinWidth is the narrowest content width that still shows a mark.
	LogoMinWidth = 24
	// LogoCompactMaxWidth bounds every compact row so it can never wrap.
	LogoCompactMaxWidth = 20
	// LogoMinHeight is the shortest terminal that shows the subtitle line.
	LogoMinHeight = 16
	// LogoFullHeight is the shortest terminal where the full logo and the whole
	// menu fit without scrolling.
	LogoFullHeight = 24
)

// LogoCompact is a three-row braille mark for narrow or short terminals.
const LogoCompact = `  ⢀⣤⣶⣿⣶⣤⡀
  ⣿⣿⠿⠿⣿⣿ CORTEX·IA
  ⠙⠛⠛⠛⠛⠋`

// hexToRGB parses a hex string "#RRGGBB" into RGB integers.
func hexToRGB(hexStr string) (r, g, b int) {
	if strings.HasPrefix(hexStr, "#") && len(hexStr) == 7 {
		_, _ = fmt.Sscanf(hexStr[1:], "%02x%02x%02x", &r, &g, &b)
		return
	}
	return 155, 107, 240 // Nan Ink accent-text #9B6BF0
}

// HomeLogoArt selects the logo variant for the available content width. It
// returns the empty string when even the compact mark cannot fit, so callers
// can fall back to a plain header.
func HomeLogoArt(width int) string {
	switch {
	case width > LogoFullWidth:
		return Logo
	case width >= LogoMinWidth:
		return LogoCompact
	}
	return ""
}

// ShimmerLogoArt renders art with a wave-interpolated Primary→Secondary gradient
// that shifts with frame, preserving each row's display width.
func ShimmerLogoArt(art string, frame int) string {
	r1, g1, b1 := hexToRGB(string(Primary))
	r2, g2, b2 := hexToRGB(string(Secondary))

	lines := strings.Split(art, "\n")
	renderedLines := make([]string, 0, len(lines))

	for _, line := range lines {
		var sb strings.Builder
		for col, ch := range []rune(line) {
			if ch == ' ' {
				sb.WriteRune(' ')
				continue
			}
			// Compute wave phase based on column position and animation frame
			wave := float64(col)*0.16 - float64(frame)*0.22
			t := 0.5 + 0.5*math.Sin(wave)

			r := int(float64(r1)*(1.0-t) + float64(r2)*t)
			g := int(float64(g1)*(1.0-t) + float64(g2)*t)
			b := int(float64(b1)*(1.0-t) + float64(b2)*t)

			hex := fmt.Sprintf("#%02x%02x%02x", r, g, b)
			styled := lipgloss.NewStyle().Foreground(lipgloss.Color(hex)).Bold(true).Render(string(ch))
			sb.WriteString(styled)
		}
		renderedLines = append(renderedLines, sb.String())
	}
	return strings.Join(renderedLines, "\n")
}

// ShimmerLogo renders the full Logo with the animated color gradient.
func ShimmerLogo(frame int) string {
	return ShimmerLogoArt(Logo, frame)
}

// ToggleTheme switches between dark and light themes.
func ToggleTheme() {
	if ActiveTheme == ThemeDark {
		ApplyTheme(ThemeLight)
	} else {
		ApplyTheme(ThemeDark)
	}
}

// ApplyTheme rebuilds every color role and global style for the given mode.
func ApplyTheme(id ThemeID) {
	ActiveTheme = id
	applyRoles(NanInkFor(id))
}
