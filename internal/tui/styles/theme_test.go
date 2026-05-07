package styles

import "testing"

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
		t.Errorf("after first toggle should be light, got %q", ActiveTheme)
	}

	ToggleTheme()
	if ActiveTheme != ThemeHighContrast {
		t.Errorf("after second toggle should be high-contrast, got %q", ActiveTheme)
	}

	ToggleTheme()
	if ActiveTheme != ThemeDark {
		t.Errorf("after third toggle should be dark, got %q", ActiveTheme)
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
