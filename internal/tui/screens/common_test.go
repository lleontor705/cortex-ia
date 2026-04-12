package screens

import (
	"strings"
	"testing"
)

func TestRenderOptions_HighlightsCursor(t *testing.T) {
	output := RenderOptions([]string{"Alpha", "Beta"}, 0)
	if !strings.Contains(output, "> ") {
		t.Error("expected cursor prefix '> ' on focused item")
	}
	if !strings.Contains(output, "Alpha") {
		t.Error("expected focused option label in output")
	}
}

func TestRenderOptions_NonCursor(t *testing.T) {
	output := RenderOptions([]string{"Alpha", "Beta"}, 0)
	// The second item (Beta) should NOT have the cursor prefix
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "Beta") && strings.Contains(line, "> ") {
			t.Error("non-cursor item should not have '> ' prefix")
		}
	}
}

func TestRenderCheckbox_CheckedFocused(t *testing.T) {
	output := RenderCheckbox("Option", true, true)
	if !strings.Contains(output, "[x]") {
		t.Error("expected [x] marker for checked checkbox")
	}
	if !strings.Contains(output, "> ") {
		t.Error("expected cursor prefix for focused checkbox")
	}
}

func TestRenderCheckbox_UncheckedUnfocused(t *testing.T) {
	output := RenderCheckbox("Option", false, false)
	if !strings.Contains(output, "[ ]") {
		t.Error("expected [ ] marker for unchecked checkbox")
	}
	if strings.Contains(output, "> ") {
		t.Error("expected no cursor prefix for unfocused checkbox")
	}
}

func TestRenderRadio_SelectedFocused(t *testing.T) {
	output := RenderRadio("Choice", true, true)
	if !strings.Contains(output, "(*)") {
		t.Error("expected (*) marker for selected radio")
	}
	if !strings.Contains(output, "> ") {
		t.Error("expected cursor prefix for focused radio")
	}
}

func TestRenderRadio_UnselectedUnfocused(t *testing.T) {
	output := RenderRadio("Choice", false, false)
	if !strings.Contains(output, "( )") {
		t.Error("expected ( ) marker for unselected radio")
	}
	if strings.Contains(output, "> ") {
		t.Error("expected no cursor prefix for unfocused radio")
	}
}
