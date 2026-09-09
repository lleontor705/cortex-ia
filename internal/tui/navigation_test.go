package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestHomeMenuEntries proves Home offers exactly the required actions.
func TestHomeMenuEntries(t *testing.T) {
	m := sized(newModel(&fakeService{}, "/home/test", "vtest"))
	view := m.View()
	for _, want := range []string{"Install / Sync", "Configure Delegation", "Manage MCPs", "Agent Studio (Create Sub-agent)", "Doctor / Recovery", "Uninstall", "Quit"} {
		if !strings.Contains(view, want) {
			t.Fatalf("home view missing entry %q:\n%s", want, view)
		}
	}
	if m.screen != screenHome || m.cursor != 0 {
		t.Fatalf("expected fresh model on home screen with cursor 0, got screen=%v cursor=%d", m.screen, m.cursor)
	}
}

// TestScreenSet pins the conceptual screen set.
func TestScreenSet(t *testing.T) {
	got := map[screen]bool{}
	for _, s := range []screen{screenHome, screenWizardHerdr, screenWizardDelegation, screenWizardRoles, screenReview, screenRunning, screenResult, screenMCP, screenWeb, screenAgentStudio, screenDelegation} {
		if got[s] {
			t.Fatalf("duplicate screen constant %v", s)
		}
		got[s] = true
	}
	if len(got) != 11 {
		t.Fatalf("expected exactly eleven screens, got %d", len(got))
	}
}

// TestNavigationWalksAllScreens exercises reachability of every screen with
// plain keyboard navigation, and their return paths.
func TestNavigationWalksAllScreens(t *testing.T) {
	m := sized(newModel(&fakeService{}, "/home/test", "vtest"))

	// Home → Wizard Step 1 (Herdr).
	m = press(m, "enter")
	if m.screen != screenWizardHerdr {
		t.Fatalf("expected wizard step 1 (Herdr), got %v", m.screen)
	}
	herdrView := m.View()
	if !strings.Contains(herdrView, "Paso 1 de 3") {
		t.Fatalf("expected wizard step 1 view header, got:\n%s", herdrView)
	}

	// Step 1 → Step 2 (Delegation).
	m = press(m, "enter")
	if m.screen != screenWizardDelegation {
		t.Fatalf("expected wizard step 2 (Delegation), got %v", m.screen)
	}
	delView := m.View()
	if !strings.Contains(delView, "Paso 2 de 3") {
		t.Fatalf("expected wizard step 2 view header, got:\n%s", delView)
	}

	// Select "Sí, configurar delegación" (option 1 / cursor 0) → Step 3 (Roles).
	m = press(m, "1")
	m = press(m, "enter")
	if m.screen != screenWizardRoles {
		t.Fatalf("expected wizard step 3 (Roles), got %v", m.screen)
	}
	rolesView := m.View()
	if !strings.Contains(rolesView, "Paso 3 de 3") {
		t.Fatalf("expected wizard step 3 view header, got:\n%s", rolesView)
	}

	// Step 3 → Review.
	m = pressDrive(t, m, "enter")
	if m.screen != screenReview {
		t.Fatalf("expected review screen, got %v", m.screen)
	}
	if m.plan == nil {
		t.Fatal("expected plan to be recorded from the real returned command")
	}

	// Review → Back to Step 3.
	m = press(m, "b")
	if m.screen != screenWizardRoles {
		t.Fatalf("expected 'b' to return to wizard roles, got %v", m.screen)
	}

	// Step 3 → Back to Step 2.
	m = press(m, "b")
	if m.screen != screenWizardDelegation {
		t.Fatalf("expected 'b' to return to wizard delegation, got %v", m.screen)
	}

	// Step 2 → Back to Step 1.
	m = press(m, "b")
	if m.screen != screenWizardHerdr {
		t.Fatalf("expected 'b' to return to wizard herdr, got %v", m.screen)
	}

	// Step 1 → Home.
	m = press(m, "esc")
	if m.screen != screenHome {
		t.Fatalf("expected esc to return home, got %v", m.screen)
	}

	// Home → Configure Delegation → Home.
	m = press(m, "down") // cursor 1: Configure Delegation
	m = press(m, "enter")
	if m.screen != screenDelegation {
		t.Fatalf("expected screenDelegation, got %v", m.screen)
	}
	m = press(m, "esc")
	if m.screen != screenHome {
		t.Fatalf("expected esc to return home from delegation, got %v", m.screen)
	}

	// Home → MCP Manager.
	m = press(m, "down") // cursor was 1; now cursor 2: Manage MCPs
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

	// Home → Web Console → Home.
	m = press(m, "down") // cursor was 2 (MCP); now cursor 3: CortexIA Web Console
	m = press(m, "enter")
	if m.screen != screenWeb {
		t.Fatalf("expected screenWeb, got %v", m.screen)
	}
	m = press(m, "b")
	if m.screen != screenHome {
		t.Fatalf("expected b to return home from web, got %v", m.screen)
	}

	// Home → Agent Studio → Home.
	m = press(m, "down") // cursor was 3; now cursor 4: Agent Studio
	m = press(m, "enter")
	if m.screen != screenAgentStudio {
		t.Fatalf("expected screenAgentStudio, got %v", m.screen)
	}
	m = press(m, "esc")
	if m.screen != screenHome {
		t.Fatalf("expected esc to return home from agent studio, got %v", m.screen)
	}

	// Home → Doctor/Recovery (running) → Result → Home.
	m = press(m, "down") // cursor was 4; now cursor 5: Doctor / Recovery
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
	// (Result returns Home with cursor 0; Uninstall is entry 6.)
	m = press(m, "down")
	m = press(m, "down")
	m = press(m, "down")
	m = press(m, "down")
	m = press(m, "down")
	m = press(m, "down") // cursor 6: Uninstall
	m = press(m, "enter")
	if m.confirm.kind != confirmUninstall || m.screen != screenHome {
		t.Fatalf("expected uninstall confirmation on home, got screen=%v confirm=%v", m.screen, m.confirm.kind)
	}
	m = press(m, "n")
	if m.confirm.kind != confirmNone {
		t.Fatal("expected n to cancel the confirmation")
	}

	// Quit entry quits.
	m = press(m, "down") // cursor 7: Quit (cursor stayed on 6 after cancel)
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

// TestHomeNumericHotkeys verifies keys 1-8 jump directly to actions from Home.
func TestHomeNumericHotkeys(t *testing.T) {
	m := sized(newModel(&fakeService{}, "/home/test", "vtest"))

	// Key '1' opens Wizard Step 1 (Herdr).
	m1 := press(m, "1")
	if m1.screen != screenWizardHerdr {
		t.Fatalf("key 1 should open Wizard Step 1, got %v", m1.screen)
	}

	// Key '2' opens Configure Delegation.
	m2 := press(m, "2")
	if m2.screen != screenDelegation {
		t.Fatalf("key 2 should open Configure Delegation, got %v", m2.screen)
	}

	// Key '3' opens MCP.
	m3 := pressDrive(t, m, "3")
	if m3.screen != screenMCP {
		t.Fatalf("key 3 should open MCP, got %v", m3.screen)
	}

	// Key '4' opens Web Console.
	m4 := press(m, "4")
	if m4.screen != screenWeb {
		t.Fatalf("key 4 should open Web Console, got %v", m4.screen)
	}

	// Key '5' opens Agent Studio.
	m5 := press(m, "5")
	if m5.screen != screenAgentStudio {
		t.Fatalf("key 5 should open Agent Studio, got %v", m5.screen)
	}

	// Key '6' starts Doctor (running).
	updated, _ := m.Update(key("6"))
	m6 := updated.(model)
	if m6.screen != screenRunning || m6.running.title != "Doctor" {
		t.Fatalf("key 6 should start Doctor, got %v %q", m6.screen, m6.running.title)
	}

	// Key '7' opens Uninstall confirmation.
	m7 := press(m, "7")
	if m7.confirm.kind != confirmUninstall {
		t.Fatalf("key 7 should trigger uninstall confirmation, got %v", m7.confirm.kind)
	}

	// Key '8' quits.
	updated8, cmd8 := m.Update(key("8"))
	if cmd8 == nil || !updated8.(model).quitting {
		t.Fatal("key 8 should quit")
	}
}
