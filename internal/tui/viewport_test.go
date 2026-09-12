package tui

import (
	"strings"
	"testing"
)

// TestWizardHerdrViewport verifies that in a short terminal, navigating
// between options scrolls the viewport so the selected option is visible.
func TestWizardHerdrViewport(t *testing.T) {
	m := newModel(&fakeService{}, "/home/test", "vtest")
	m.width, m.height = 80, 14
	m = press(m, "enter") // Home -> Wizard Herdr
	if m.screen != screenWizardHerdr {
		t.Fatalf("expected screenWizardHerdr, got %v", m.screen)
	}

	// By default UseHerdr is false, so wizardCursor defaults to 1 (No, trabajar en terminal estándar).
	if m.wizardCursor != 1 {
		t.Fatalf("expected initial wizardCursor 1, got %d", m.wizardCursor)
	}
	view1 := m.View()
	if !strings.Contains(view1, "No, trabajar en terminal estándar") {
		t.Fatalf("short terminal wizard herdr view must show selected option 1:\n%s", view1)
	}
	if lines := viewLines(m); lines > m.height {
		t.Fatalf("view height %d exceeds terminal height %d", lines, m.height)
	}

	// Move cursor up to option 0 ("Sí, habilitar Herdr").
	m = press(m, "up")
	if m.wizardCursor != 0 {
		t.Fatalf("expected wizardCursor 0, got %d", m.wizardCursor)
	}
	view0 := m.View()
	if !strings.Contains(view0, "Sí, habilitar Herdr") {
		t.Fatalf("wizard herdr view missing option 0 after moving up:\n%s", view0)
	}
	if lines := viewLines(m); lines > m.height {
		t.Fatalf("view height %d exceeds terminal height %d", lines, m.height)
	}

	// Move cursor back down to option 1.
	m = press(m, "down")
	if m.wizardCursor != 1 {
		t.Fatalf("expected wizardCursor 1, got %d", m.wizardCursor)
	}
	view1Again := m.View()
	if !strings.Contains(view1Again, "No, trabajar en terminal estándar") {
		t.Fatalf("short terminal wizard herdr view must reveal option 1 on moving down:\n%s", view1Again)
	}
	if lines := viewLines(m); lines > m.height {
		t.Fatalf("view height %d exceeds terminal height %d", lines, m.height)
	}
}

// TestWizardDelegationViewport verifies that in a short terminal, navigating
// between options scrolls the viewport so the selected option is visible.
func TestWizardDelegationViewport(t *testing.T) {
	m := newModel(&fakeService{}, "/home/test", "vtest")
	m.width, m.height = 80, 13
	m = press(m, "enter") // Home -> Step 1 (Herdr)
	m = press(m, "enter") // Step 1 -> Step 2 (Delegation)
	if m.screen != screenWizardDelegation {
		t.Fatalf("expected screenWizardDelegation, got %v", m.screen)
	}

	// By default DelegationEnabled is false, so wizardCursor defaults to 1 (No, instalación estándar).
	if m.wizardCursor != 1 {
		t.Fatalf("expected initial wizardCursor 1, got %d", m.wizardCursor)
	}
	view1 := m.View()
	if !strings.Contains(view1, "No, instalación estándar") {
		t.Fatalf("short terminal wizard delegation view must show selected option 1:\n%s", view1)
	}
	if lines := viewLines(m); lines > m.height {
		t.Fatalf("view height %d exceeds terminal height %d", lines, m.height)
	}

	// Move cursor up to option 0 ("Sí, configurar delegación").
	m = press(m, "up")
	if m.wizardCursor != 0 {
		t.Fatalf("expected wizardCursor 0, got %d", m.wizardCursor)
	}
	view0 := m.View()
	if !strings.Contains(view0, "Sí, configurar delegación") {
		t.Fatalf("wizard delegation view missing option 0 after moving up:\n%s", view0)
	}
	if lines := viewLines(m); lines > m.height {
		t.Fatalf("view height %d exceeds terminal height %d", lines, m.height)
	}

	// Move cursor back down to option 1.
	m = press(m, "down")
	if m.wizardCursor != 1 {
		t.Fatalf("expected wizardCursor 1, got %d", m.wizardCursor)
	}
	view1Again := m.View()
	if !strings.Contains(view1Again, "No, instalación estándar") {
		t.Fatalf("short terminal wizard delegation view must reveal option 1 on moving down:\n%s", view1Again)
	}
	if lines := viewLines(m); lines > m.height {
		t.Fatalf("view height %d exceeds terminal height %d", lines, m.height)
	}
}

// TestWizardRolesViewport verifies that in a short terminal, moving the cursor
// down to the Continue button scrolls the viewport so the button is visible.
func TestWizardRolesViewport(t *testing.T) {
	m := newModel(&fakeService{}, "/home/test", "vtest")
	m.width, m.height = 80, 12
	m = press(m, "enter") // Home -> Step 1 (Herdr)
	m = press(m, "enter") // Step 1 -> Step 2 (Delegation)
	m = press(m, "1")     // Select delegation
	m = press(m, "enter") // Step 2 -> Step 3 (Roles)
	if m.screen != screenWizardRoles {
		t.Fatalf("expected screenWizardRoles, got %v", m.screen)
	}

	// Unscrolled: first role ("implement") is visible.
	view0 := m.View()
	if !strings.Contains(view0, "implement") {
		t.Fatalf("wizard roles view missing implement when unscrolled:\n%s", view0)
	}
	if lines := viewLines(m); lines > m.height {
		t.Fatalf("view height %d exceeds terminal height %d", lines, m.height)
	}

	// Move down to the Continue button (cursor 4).
	for i := 0; i < 4; i++ {
		m = press(m, "down")
	}
	if m.wizardCursor != len(delegationRoles) {
		t.Fatalf("expected wizardCursor %d, got %d", len(delegationRoles), m.wizardCursor)
	}
	viewEnd := m.View()
	if !strings.Contains(viewEnd, "Continuar a la Revisión del Plan") {
		t.Fatalf("short terminal wizard roles view must reveal Continue button when focused:\n%s", viewEnd)
	}
	if lines := viewLines(m); lines > m.height {
		t.Fatalf("view height %d exceeds terminal height %d", lines, m.height)
	}

	// Move back up to role 0 ("implement").
	for i := 0; i < 4; i++ {
		m = press(m, "up")
	}
	if m.wizardCursor != 0 {
		t.Fatalf("expected wizardCursor 0, got %d", m.wizardCursor)
	}
	viewTop := m.View()
	if !strings.Contains(viewTop, "implement") {
		t.Fatalf("wizard roles view must reveal implement after scrolling up:\n%s", viewTop)
	}
	if lines := viewLines(m); lines > m.height {
		t.Fatalf("view height %d exceeds terminal height %d", lines, m.height)
	}
}

// TestDelegationScreenViewport verifies that in a short terminal, navigating
// to the bottom action buttons scrolls them into the visible viewport.
func TestDelegationScreenViewport(t *testing.T) {
	m := newModel(&fakeService{}, "/home/test", "vtest")
	m.width, m.height = 80, 14
	m = press(m, "down")  // cursor 1: Configure Delegation
	m = press(m, "enter") // Home -> Delegation Screen
	if m.screen != screenDelegation {
		t.Fatalf("expected screenDelegation, got %v", m.screen)
	}

	// Unscrolled: first item ("Multiplexor Herdr") is visible.
	view0 := m.View()
	if !strings.Contains(view0, "Multiplexor Herdr") {
		t.Fatalf("delegation screen missing Multiplexor Herdr when unscrolled:\n%s", view0)
	}
	if lines := viewLines(m); lines > m.height {
		t.Fatalf("view height %d exceeds terminal height %d", lines, m.height)
	}

	// Move cursor to item 7: [ Guardar Configuración ]
	for i := 0; i < 7; i++ {
		m = press(m, "down")
	}
	if m.delegationCursor != 7 {
		t.Fatalf("expected delegationCursor 7, got %d", m.delegationCursor)
	}
	viewSave := m.View()
	if !strings.Contains(viewSave, "Guardar Configuración") {
		t.Fatalf("short terminal delegation screen must reveal Guardar Configuración when focused:\n%s", viewSave)
	}
	if lines := viewLines(m); lines > m.height {
		t.Fatalf("view height %d exceeds terminal height %d", lines, m.height)
	}

	// Move cursor to item 8: [ Volver al Menú Principal ]
	m = press(m, "down")
	if m.delegationCursor != 8 {
		t.Fatalf("expected delegationCursor 8, got %d", m.delegationCursor)
	}
	viewBack := m.View()
	if !strings.Contains(viewBack, "Volver al Menú Principal") {
		t.Fatalf("short terminal delegation screen must reveal Volver al Menú Principal when focused:\n%s", viewBack)
	}
	if lines := viewLines(m); lines > m.height {
		t.Fatalf("view height %d exceeds terminal height %d", lines, m.height)
	}

	// Move cursor back to item 0: Multiplexor Herdr.
	for i := 0; i < 8; i++ {
		m = press(m, "up")
	}
	if m.delegationCursor != 0 {
		t.Fatalf("expected delegationCursor 0, got %d", m.delegationCursor)
	}
	viewTop := m.View()
	if !strings.Contains(viewTop, "Multiplexor Herdr") {
		t.Fatalf("delegation screen must reveal Multiplexor Herdr after scrolling back up:\n%s", viewTop)
	}
	if lines := viewLines(m); lines > m.height {
		t.Fatalf("view height %d exceeds terminal height %d", lines, m.height)
	}
}

// TestCursorOffsetCalculation exercises boundary cases of cursor-relative offset computation.
func TestCursorOffsetCalculation(t *testing.T) {
	// Zero/negative budget returns 0.
	if got := cursorOffset(5, 10, 0, 2, 1); got != 0 {
		t.Fatalf("expected 0 for budget 0, got %d", got)
	}

	// Content fits within content budget returns 0.
	// budget 20, top 2, bottom 1 -> content budget = 17 >= totalLines 10.
	if got := cursorOffset(8, 10, 20, 2, 1); got != 0 {
		t.Fatalf("expected 0 when content fits, got %d", got)
	}

	// Cursor within initial keep returns 0.
	// budget 8, top 2, bottom 1 -> content budget = 5, keep = 4.
	// cursorLine 2 < keep 4 -> offset = 0.
	if got := cursorOffset(2, 10, 8, 2, 1); got != 0 {
		t.Fatalf("expected 0 when cursorLine < keep, got %d", got)
	}

	// Cursor beyond keep scrolls.
	// budget 8, top 2, bottom 1 -> content budget = 5, keep = 4.
	// cursorLine 5 -> offset = 5 - 4 + 1 = 2.
	if got := cursorOffset(5, 10, 8, 2, 1); got != 2 {
		t.Fatalf("expected 2 when cursorLine is 5 with keep 4, got %d", got)
	}

	// Offset clamped to maxOffset (totalLines - keep = 10 - 4 = 6).
	// cursorLine 9 -> unconstrained 9 - 4 + 1 = 6.
	if got := cursorOffset(9, 10, 8, 2, 1); got != 6 {
		t.Fatalf("expected 6 (maxOffset) when cursorLine is at end, got %d", got)
	}
}
