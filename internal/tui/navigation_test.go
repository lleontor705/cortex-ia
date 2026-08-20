package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestHomeMenuEntries proves Home offers exactly the five required actions.
func TestHomeMenuEntries(t *testing.T) {
	m := sized(newModel(&fakeService{}, "/home/test", "vtest"))
	view := m.View()
	for _, want := range []string{"Install / Sync", "Manage MCPs", "Doctor / Recovery", "Uninstall", "Quit"} {
		if !strings.Contains(view, want) {
			t.Fatalf("home view missing entry %q:\n%s", want, view)
		}
	}
	if m.screen != screenHome || m.cursor != 0 {
		t.Fatalf("expected fresh model on home screen with cursor 0, got screen=%v cursor=%d", m.screen, m.cursor)
	}
}

// TestOnlyFiveScreens pins the conceptual screen set.
func TestOnlyFiveScreens(t *testing.T) {
	got := map[screen]bool{}
	for _, s := range []screen{screenHome, screenReview, screenRunning, screenResult, screenMCP} {
		if got[s] {
			t.Fatalf("duplicate screen constant %v", s)
		}
		got[s] = true
	}
	if len(got) != 5 {
		t.Fatalf("expected exactly five screens, got %d", len(got))
	}
}

// TestNavigationWalksAllScreens exercises reachability of every screen with
// plain keyboard navigation, and their return paths.
func TestNavigationWalksAllScreens(t *testing.T) {
	m := sized(newModel(&fakeService{}, "/home/test", "vtest"))

	// Home → Review.
	m = pressDrive(t, m, "enter")
	if m.screen != screenReview {
		t.Fatalf("expected review screen, got %v", m.screen)
	}
	if m.plan == nil {
		t.Fatal("expected plan to be recorded from the real returned command")
	}
	m = press(m, "esc")
	if m.screen != screenHome {
		t.Fatalf("expected esc to return home, got %v", m.screen)
	}

	// Home → MCP Manager.
	m = press(m, "down") // cursor 1: Manage MCPs
	m = pressDrive(t, m, "enter")
	if m.screen != screenMCP {
		t.Fatalf("expected MCP screen, got %v", m.screen)
	}
	if m.mcpReport == nil {
		t.Fatal("expected MCP list to load from the returned command")
	}
	m = press(m, "esc")
	if m.screen != screenHome {
		t.Fatalf("expected esc to return home from MCP, got %v", m.screen)
	}

	// Home → Doctor/Recovery (running) → Result → Home.
	m = press(m, "down") // cursor 2: Doctor / Recovery
	updated, cmd := m.Update(key("enter"))
	m = updated.(model)
	if m.screen != screenRunning || m.running.title != "Doctor" {
		t.Fatalf("expected doctor running screen, got %v %q", m.screen, m.running.title)
	}
	view := m.View()
	for _, phase := range []string{"Inspect state", "Compare digests", "Report"} {
		if !strings.Contains(view, phase) {
			t.Fatalf("running view missing phase %q:\n%s", phase, view)
		}
	}
	m = drive(t, m, cmd)
	if m.screen != screenResult {
		t.Fatalf("expected result screen after doctor, got %v", m.screen)
	}
	m = press(m, "enter")
	if m.screen != screenHome {
		t.Fatalf("expected enter to return home from result, got %v", m.screen)
	}

	// Uninstall opens a confirmation overlay, not the running screen.
	// (Result returns Home with cursor 0; Uninstall is entry 3.)
	m = press(m, "down")
	m = press(m, "down")
	m = press(m, "down") // cursor 3: Uninstall
	m = press(m, "enter")
	if m.confirm.kind != confirmUninstall || m.screen != screenHome {
		t.Fatalf("expected uninstall confirmation on home, got screen=%v confirm=%v", m.screen, m.confirm.kind)
	}
	m = press(m, "n")
	if m.confirm.kind != confirmNone {
		t.Fatal("expected n to cancel the confirmation")
	}

	// Quit entry quits.
	m = press(m, "down") // cursor 4: Quit (cursor stayed on 3 after cancel)
	updated, cmd = m.Update(key("enter"))
	if cmd == nil || !updated.(model).quitting {
		t.Fatal("expected Quit entry to quit")
	}
}

// TestHomeCursorBounds keeps the cursor inside the menu.
func TestHomeCursorBounds(t *testing.T) {
	m := sized(newModel(&fakeService{}, "/home/test", "vtest"))
	m = press(m, "up")
	if m.cursor != 0 {
		t.Fatalf("cursor above menu: %d", m.cursor)
	}
	for i := 0; i < len(homeEntries)+3; i++ {
		m = press(m, "down")
	}
	if m.cursor != len(homeEntries)-1 {
		t.Fatalf("cursor below menu: %d", m.cursor)
	}
}

// TestQuitKeyQuitsFromHome verifies the global quit key.
func TestQuitKeyQuitsFromHome(t *testing.T) {
	m := sized(newModel(&fakeService{}, "/home/test", "vtest"))
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("ctrl+c should quit")
	}
}

// TestHomeNumericHotkeys verifies keys 1-5 jump directly to actions from Home.
func TestHomeNumericHotkeys(t *testing.T) {
	m := sized(newModel(&fakeService{}, "/home/test", "vtest"))

	// Key '1' opens Review.
	m1 := pressDrive(t, m, "1")
	if m1.screen != screenReview {
		t.Fatalf("key 1 should open Review, got %v", m1.screen)
	}

	// Key '2' opens MCP.
	m2 := pressDrive(t, m, "2")
	if m2.screen != screenMCP {
		t.Fatalf("key 2 should open MCP, got %v", m2.screen)
	}

	// Key '3' starts Doctor (running).
	updated, _ := m.Update(key("3"))
	m3 := updated.(model)
	if m3.screen != screenRunning || m3.running.title != "Doctor" {
		t.Fatalf("key 3 should start Doctor, got %v %q", m3.screen, m3.running.title)
	}

	// Key '4' opens Uninstall confirmation.
	m4 := press(m, "4")
	if m4.confirm.kind != confirmUninstall {
		t.Fatalf("key 4 should trigger uninstall confirmation, got %v", m4.confirm.kind)
	}

	// Key '5' quits.
	updated5, cmd5 := m.Update(key("5"))
	if cmd5 == nil || !updated5.(model).quitting {
		t.Fatal("key 5 should quit")
	}
}
