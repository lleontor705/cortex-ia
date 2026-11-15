package tui

import (
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/install"
	"github.com/lleontor705/cortex-ia/internal/pipeline"
	"github.com/lleontor705/cortex-ia/internal/state"
)

// The theme toggle consults the user instead of mutating their theme by
// default; these tests pin that opt-in contract at the TUI layer.

func TestReviewThemeToggleDefaultsToOptInConsult(t *testing.T) {
	fake := &fakeService{}
	m := openReview(t, fake)

	if m.opts.ApplyTheme {
		t.Fatal("apply cortex theme must start unselected (opt-in)")
	}
	view := m.View()
	if !strings.Contains(view, "[ ] apply cortex theme") {
		t.Fatalf("review must render the unselected theme row:\n%s", view)
	}
	if !strings.Contains(view, "opt-in; your theme is left untouched unless you select this") {
		t.Fatalf("theme row must state the opt-in consult wording:\n%s", view)
	}
}

func TestReviewThemeToggleFlipsApplyTheme(t *testing.T) {
	fake := &fakeService{}
	m := openReview(t, fake)

	m = press(m, "down") // context7
	m = press(m, "down") // apply cortex theme row
	m = pressDrive(t, m, "space")
	if !m.opts.ApplyTheme {
		t.Fatal("space on the theme row must select apply cortex theme")
	}
	if !strings.Contains(m.View(), "[x] apply cortex theme") {
		t.Fatalf("selected theme row must render checked:\n%s", m.View())
	}

	m = pressDrive(t, m, "space")
	if m.opts.ApplyTheme {
		t.Fatal("a second space must de-select apply cortex theme")
	}
	if !strings.Contains(m.View(), "[ ] apply cortex theme") {
		t.Fatalf("de-selected theme row must render unchecked:\n%s", m.View())
	}
}

func TestReviewThemeSelectionTravelsWithPlanAndConfirmedOptions(t *testing.T) {
	fake := &fakeService{
		planFn: func(install.Options) (*pipeline.Plan, error) {
			return &pipeline.Plan{Digest: "digest0001", MetadataPresence: state.PresenceAbsent}, nil
		},
	}
	m := openReview(t, fake)

	m = press(m, "down")
	m = press(m, "down")
	m = pressDrive(t, m, "space")

	planned := fake.planCalls[len(fake.planCalls)-1]
	if !planned.ApplyTheme {
		t.Fatalf("replan must carry the theme selection, got %+v", planned)
	}

	pressDrive(t, m, "enter")
	if len(fake.installCalls) != 1 {
		t.Fatalf("expected one confirmed install, got %d", len(fake.installCalls))
	}
	if !fake.installCalls[0].ApplyTheme {
		t.Fatalf("confirmed options must carry the theme selection, got %+v", fake.installCalls[0])
	}
}
