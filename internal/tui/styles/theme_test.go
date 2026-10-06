package styles

import (
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

var ansiPattern = regexp.MustCompile("\x1b\\[[0-9;]*m")

func stripANSI(s string) string {
	return ansiPattern.ReplaceAllString(s, "")
}

func TestSpinnerChar_Cycles(t *testing.T) {
	for i := 0; i < 10; i++ {
		ch := SpinnerChar(i)
		if ch == "" {
			t.Errorf("SpinnerChar(%d) returned empty string", i)
		}
	}
}

func TestSpinnerChar_Wraps(t *testing.T) {
	if SpinnerChar(10) != SpinnerChar(0) {
		t.Errorf("SpinnerChar(10) = %q, want %q (should wrap)", SpinnerChar(10), SpinnerChar(0))
	}
	if SpinnerChar(20) != SpinnerChar(0) {
		t.Errorf("SpinnerChar(20) = %q, want %q (should wrap)", SpinnerChar(20), SpinnerChar(0))
	}
}

func TestLogo_NonEmpty(t *testing.T) {
	if Logo == "" {
		t.Error("Logo constant should not be empty")
	}
}

func TestShimmerLogo(t *testing.T) {
	rendered0 := ShimmerLogo(0)
	if rendered0 == "" {
		t.Error("ShimmerLogo(0) returned empty string")
	}
	rendered5 := ShimmerLogo(5)
	if rendered5 == "" {
		t.Error("ShimmerLogo(5) returned empty string")
	}
}

func TestCursorPrefix_NonEmpty(t *testing.T) {
	if CursorPrefix != "> " {
		t.Errorf("CursorPrefix = %q, want %q", CursorPrefix, "> ")
	}
}

func TestToggleTheme(t *testing.T) {
	// Start with dark
	ApplyTheme(ThemeDark)
	if ActiveTheme != ThemeDark {
		t.Errorf("initial theme should be dark, got %q", ActiveTheme)
	}

	ToggleTheme()
	if ActiveTheme != ThemeLight {
		t.Errorf("after toggle should be light, got %q", ActiveTheme)
	}

	ToggleTheme()
	if ActiveTheme != ThemeDark {
		t.Errorf("after second toggle should be dark, got %q", ActiveTheme)
	}
}

func TestApplyTheme_UpdatesColors(t *testing.T) {
	ApplyTheme(ThemeDark)
	darkPrimary := Primary

	ApplyTheme(ThemeLight)
	lightPrimary := Primary

	if string(darkPrimary) == string(lightPrimary) {
		t.Error("dark and light themes should have different primary colors")
	}

	// Restore dark theme
	ApplyTheme(ThemeDark)
}

func TestApplyTheme_UpdatesStyles(t *testing.T) {
	ApplyTheme(ThemeLight)
	// Styles should be rebuilt — just verify they exist
	view := Title.Render("test")
	if view == "" {
		t.Error("Title style should render after theme apply")
	}

	// Restore
	ApplyTheme(ThemeDark)
}

func TestLogo_FullWidthMatchesArt(t *testing.T) {
	widest := 0
	for _, line := range strings.Split(Logo, "\n") {
		if w := lipgloss.Width(line); w > widest {
			widest = w
		}
	}
	if widest != LogoFullWidth {
		t.Errorf("LogoFullWidth=%d but widest Logo row is %d", LogoFullWidth, widest)
	}
}

func TestLogoCompact_Dimensions(t *testing.T) {
	lines := strings.Split(LogoCompact, "\n")
	if len(lines) > 3 {
		t.Errorf("compact art must be at most 3 rows, got %d", len(lines))
	}
	for i, line := range lines {
		if w := lipgloss.Width(line); w > LogoCompactMaxWidth {
			t.Errorf("compact row %d width %d exceeds %d: %q", i, w, LogoCompactMaxWidth, line)
		}
	}
}

func TestHomeLogoArt_Selection(t *testing.T) {
	cases := []struct {
		width int
		want  string
	}{
		{LogoFullWidth + 10, Logo},
		{LogoFullWidth + 1, Logo},
		{LogoFullWidth, LogoCompact},
		{LogoMinWidth + 1, LogoCompact},
		{LogoMinWidth, LogoCompact},
		{LogoMinWidth - 1, ""},
		{0, ""},
	}
	for _, c := range cases {
		if got := HomeLogoArt(c.width); got != c.want {
			t.Errorf("HomeLogoArt(%d) = %q, want %q", c.width, got, c.want)
		}
	}
}

func TestShimmerLogoArt_RendersVariants(t *testing.T) {
	full := ShimmerLogoArt(Logo, 0)
	if !strings.Contains(stripANSI(full), "C O R T E X · I A") {
		t.Errorf("ShimmerLogoArt(Logo) lost the wordmark:\n%s", full)
	}

	compact := ShimmerLogoArt(LogoCompact, 3)
	if !strings.Contains(stripANSI(compact), "CORTEX·IA") {
		t.Errorf("ShimmerLogoArt(LogoCompact) lost the wordmark:\n%s", compact)
	}

	fullRows := strings.Split(full, "\n")
	artRows := strings.Split(Logo, "\n")
	if len(fullRows) != len(artRows) {
		t.Fatalf("styled full art has %d rows, want %d", len(fullRows), len(artRows))
	}
	for i := range artRows {
		if got, want := lipgloss.Width(fullRows[i]), lipgloss.Width(artRows[i]); got != want {
			t.Errorf("styling changed row %d width: got %d, want %d", i, got, want)
		}
	}
	widest := 0
	for _, line := range strings.Split(compact, "\n") {
		if w := lipgloss.Width(line); w > widest {
			widest = w
		}
	}
	if widest > LogoCompactMaxWidth {
		t.Errorf("styled compact art width %d exceeds %d", widest, LogoCompactMaxWidth)
	}

	if ShimmerLogo(4) != ShimmerLogoArt(Logo, 4) {
		t.Error("ShimmerLogo must delegate to ShimmerLogoArt(Logo, frame)")
	}
}
