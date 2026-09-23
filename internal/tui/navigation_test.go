package tui

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// The suite's shared Home-first helpers (openMCP, openReview,
// driveFromHomeToReview, initStudioTest, …) press Home keys immediately after
// newModel, so the navigation suite pins the boot surface to Home. The
// production stats boot (REQ-US-002) is exercised by TestStatsBoot, which
// resets bootScreen to productionBootScreen.
func init() { bootScreen = screenHome }

// TestHomeMenuEntries proves Home offers exactly the required actions.
func TestHomeMenuEntries(t *testing.T) {
	m := sized(newModel(&fakeService{}, "/home/test", "vtest"))
	view := m.View()
	for _, want := range []string{"Install / Sync", "Manage MCPs", "Agent Studio (Create Sub-agent)", "Estadísticas de uso", "Doctor / Recovery", "Uninstall", "Quit", "Configuración de modelos"} {
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
	for _, s := range []screen{screenHome, screenReview, screenRunning, screenResult, screenMCP, screenWeb, screenAgentStudio, screenStats} {
		if got[s] {
			t.Fatalf("duplicate screen constant %v", s)
		}
		got[s] = true
	}
	if len(got) != 8 {
		t.Fatalf("expected exactly eight screens, got %d", len(got))
	}
}

// TestNavigationWalksAllScreens exercises reachability of every screen with
// plain keyboard navigation, and their return paths.
func TestNavigationWalksAllScreens(t *testing.T) {
	m := sized(newModel(&fakeService{}, "/home/test", "vtest"))

	// Home → Review (draining planCmd).
	m = pressDrive(t, m, "enter")
	if m.screen != screenReview {
		t.Fatalf("expected review screen, got %v", m.screen)
	}
	if m.plan == nil {
		t.Fatal("expected plan to be recorded from the real returned command")
	}

	// Review → Back to Home.
	m = press(m, "b")
	if m.screen != screenHome {
		t.Fatalf("expected 'b' to return to home, got %v", m.screen)
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
	m = press(m, "down") // cursor 3: Agent Studio
	m = press(m, "enter")
	if m.screen != screenAgentStudio {
		t.Fatalf("expected screenAgentStudio, got %v", m.screen)
	}
	m = press(m, "esc")
	if m.screen != screenHome {
		t.Fatalf("expected esc to return home from agent studio, got %v", m.screen)
	}

	// Home → Usage Stats → Home (esc rests the cursor on the stats entry).
	m = press(m, "down") // cursor 4: Estadísticas de uso
	statsUpdated, _ := m.Update(key("enter"))
	m = statsUpdated.(model)
	if m.screen != screenStats {
		t.Fatalf("expected screenStats, got %v", m.screen)
	}
	m = press(m, "esc")
	if m.screen != screenHome || m.cursor != statsEntryIndex {
		t.Fatalf("expected esc to return home on the stats entry, got screen=%v cursor=%d", m.screen, m.cursor)
	}

	// Home → Doctor/Recovery (running) → Result → Home.
	m = press(m, "down") // cursor 5: Doctor / Recovery
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

// TestHomeNumericHotkeys verifies keys 1-9 jump directly to actions from Home.
func TestHomeNumericHotkeys(t *testing.T) {
	m := sized(newModel(&fakeService{}, "/home/test", "vtest"))

	// Key '1' opens Review directly.
	m1 := pressDrive(t, m, "1")
	if m1.screen != screenReview {
		t.Fatalf("key 1 should open Review directly, got %v", m1.screen)
	}

	// Key '2' opens MCP.
	m2 := pressDrive(t, m, "2")
	if m2.screen != screenMCP {
		t.Fatalf("key 2 should open MCP, got %v", m2.screen)
	}

	// Key '3' opens Web Console.
	m3 := press(m, "3")
	if m3.screen != screenWeb {
		t.Fatalf("key 3 should open Web Console, got %v", m3.screen)
	}

	// Key '4' opens Agent Studio.
	m4 := press(m, "4")
	if m4.screen != screenAgentStudio {
		t.Fatalf("key 4 should open Agent Studio, got %v", m4.screen)
	}

	// Key '5' opens the usage stats screen.
	m5 := press(m, "5")
	if m5.screen != screenStats {
		t.Fatalf("key 5 should open the usage stats screen, got %v", m5.screen)
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

	// Key '9' opens the models configuration screen.
	m9 := press(m, "9")
	if m9.screen != screenModels {
		t.Fatalf("key 9 should open the models screen, got %v", m9.screen)
	}
	if !strings.Contains(m9.View(), "Configuración de modelos") {
		t.Fatalf("expected key 9 to render the models frame, got:\n%s", m9.View())
	}
}

// TestWebReadinessEnablesBrowser proves that ready server state enables browser opening and reflects active state.
func TestWebReadinessEnablesBrowser(t *testing.T) {
	m := sized(newModel(&fakeService{}, "/home/test", "vtest"))
	m.screen = screenWeb

	// Simulate readiness message
	updated, _ := m.Update(webReadyMsg{url: "http://127.0.0.1:7331"})
	m = updated.(model)

	if !m.webReady || m.webURL != "http://127.0.0.1:7331" {
		t.Fatalf("expected webReady=true and valid URL, got ready=%v url=%q", m.webReady, m.webURL)
	}

	view := m.View()
	if !strings.Contains(view, "Servidor Activo") {
		t.Fatalf("expected view to reflect active server status, got:\n%s", view)
	}
	if !strings.Contains(view, "Presiona 'o' o 'Enter' para abrir") {
		t.Fatalf("expected action prompt to be available when ready, got:\n%s", view)
	}

	opened := ""
	oldOpener := browserOpener
	defer func() { browserOpener = oldOpener }()
	browserOpener = func(target string) {
		opened = target
	}

	m = press(m, "o")
	if opened != "http://127.0.0.1:7331" {
		t.Fatalf("expected browser to open http://127.0.0.1:7331, got %q", opened)
	}
}

// TestWebStartupFailureDisablesBrowser proves that store/listen/serve startup failure disables browser opening.
func TestWebStartupFailureDisablesBrowser(t *testing.T) {
	m := sized(newModel(&fakeService{}, "/home/test", "vtest"))
	m.screen = screenWeb

	// Simulate startup failure
	errFake := errors.New("listen tcp 127.0.0.1:7331: bind: address already in use")
	updated, _ := m.Update(webErrMsg{err: errFake})
	m = updated.(model)

	if m.webReady || m.webErr == nil {
		t.Fatalf("expected webReady=false and webErr set, got ready=%v err=%v", m.webReady, m.webErr)
	}

	view := m.View()
	if !strings.Contains(view, "Error de Inicio") {
		t.Fatalf("expected view to expose startup error, got:\n%s", view)
	}
	if !strings.Contains(view, "apertura de navegador deshabilitada") {
		t.Fatalf("expected view to indicate browser opening is disabled, got:\n%s", view)
	}

	opened := ""
	oldOpener := browserOpener
	defer func() { browserOpener = oldOpener }()
	browserOpener = func(target string) {
		opened = target
	}

	m = press(m, "enter")
	if opened != "" {
		t.Fatalf("expected browser opening to remain disabled on startup failure, opened: %q", opened)
	}
}

// TestWebBrowserGatingBeforeReadiness proves that browser opening is gated before readiness reports.
func TestWebBrowserGatingBeforeReadiness(t *testing.T) {
	m := sized(newModel(&fakeService{}, "/home/test", "vtest"))
	m.screen = screenWeb
	m.webStarting = true
	m.webReady = false

	view := m.View()
	if !strings.Contains(view, "Iniciando Servidor") {
		t.Fatalf("expected view to show starting state, got:\n%s", view)
	}
	if !strings.Contains(view, "apertura de navegador en espera") {
		t.Fatalf("expected view to show browser action on hold, got:\n%s", view)
	}

	opened := ""
	oldOpener := browserOpener
	defer func() { browserOpener = oldOpener }()
	browserOpener = func(target string) {
		opened = target
	}

	m = press(m, "enter")
	if opened != "" {
		t.Fatalf("expected browser opening to remain disabled before readiness, opened: %q", opened)
	}
}

// TestIndeterminateProgressNoFabricatedPhases proves that long operations render an indeterminate running indicator without fabricated completion percentages.
func TestIndeterminateProgressNoFabricatedPhases(t *testing.T) {
	m := sized(newModel(&fakeService{}, "/home/test", "vtest"))
	updated, _ := m.startRunning("Long Operation", []string{"Step One", "Step Two", "Step Three"}, nil)
	m = updated.(model)

	view := m.View()
	if !strings.Contains(view, "Operation in progress…") {
		t.Fatalf("expected indeterminate progress indicator in view, got:\n%s", view)
	}
	// Verify that synthetic percentage checkmarks and fake progress percentages do not appear
	if strings.Contains(view, "✓ Step One") {
		t.Fatalf("found fabricated phase completion checkmark in running view:\n%s", view)
	}
	if strings.Contains(view, "%") {
		t.Fatalf("found fabricated percentage completion in running view:\n%s", view)
	}
}
