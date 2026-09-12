package tui

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/delegation"
)

func TestDelegationScreenModelConfiguration(t *testing.T) {
	tempHome := t.TempDir()
	m := newModel(&fakeService{}, tempHome, "vtest")
	m.width, m.height = 100, 30

	// Navigate to Configure Delegation screen
	m = press(m, "down")  // cursor 1: Configure Delegation
	m = press(m, "enter") // Enter screenDelegation
	if m.screen != screenDelegation {
		t.Fatalf("expected screenDelegation, got %v", m.screen)
	}

	// Verify initial default model is displayed
	view := m.View()
	if !strings.Contains(view, "Modelo AGY Predeterminado") {
		t.Fatalf("expected view to contain 'Modelo AGY Predeterminado':\n%s", view)
	}
	if !strings.Contains(view, delegation.DefaultAGYModel) {
		t.Fatalf("expected view to contain default model %q:\n%s", delegation.DefaultAGYModel, view)
	}

	// Move to cursor 2: Modelo AGY Predeterminado
	m = press(m, "down") // cursor 1
	m = press(m, "down") // cursor 2
	if m.delegationCursor != 2 {
		t.Fatalf("expected delegationCursor 2, got %d", m.delegationCursor)
	}

	// Cycle model forward with "right"
	m = press(m, "right")
	nextExpected := delegation.NextModel(delegation.DefaultAGYModel, delegation.KnownAGYModels)
	if m.delegationCfg.DefaultModel != nextExpected {
		t.Fatalf("expected DefaultModel %q, got %q", nextExpected, m.delegationCfg.DefaultModel)
	}

	// Cycle model backward with "left"
	m = press(m, "left")
	if m.delegationCfg.DefaultModel != delegation.DefaultAGYModel {
		t.Fatalf("expected DefaultModel %q, got %q", delegation.DefaultAGYModel, m.delegationCfg.DefaultModel)
	}

	// Move to role "implement" (cursor 3)
	m = press(m, "down") // cursor 3
	if m.delegationCursor != 3 {
		t.Fatalf("expected delegationCursor 3, got %d", m.delegationCursor)
	}

	// Toggle role to AGY with space
	m = press(m, " ")
	r := m.delegationCfg.Roles["implement"]
	if !r.Delegate || r.CLI != "agy" {
		t.Fatalf("expected implement to be delegated to agy, got %+v", r)
	}
	if r.Model != delegation.DefaultAGYModel {
		t.Fatalf("expected implement model %q, got %q", delegation.DefaultAGYModel, r.Model)
	}

	// Cycle implement's model specifically with "right"
	m = press(m, "right")
	r = m.delegationCfg.Roles["implement"]
	if r.Model != nextExpected {
		t.Fatalf("expected implement model to cycle to %q, got %q", nextExpected, r.Model)
	}

	// Move to Save button (cursor 7)
	m = press(m, "down") // cursor 4
	m = press(m, "down") // cursor 5
	m = press(m, "down") // cursor 6
	m = press(m, "down") // cursor 7
	if m.delegationCursor != 7 {
		t.Fatalf("expected delegationCursor 7, got %d", m.delegationCursor)
	}

	// Press enter to save
	m = press(m, "enter")
	saveView := m.View()
	if !strings.Contains(saveView, "Configuración guardada") {
		t.Fatalf("expected save confirmation, got:\n%s", saveView)
	}

	// Load the saved configuration from disk and verify
	configDir := filepath.Join(tempHome, ".config", "opencode")
	loaded, err := delegation.Load(configDir)
	if err != nil {
		t.Fatalf("failed to load saved config: %v", err)
	}
	if loaded.DefaultModel != delegation.DefaultAGYModel {
		t.Fatalf("expected loaded DefaultModel %q, got %q", delegation.DefaultAGYModel, loaded.DefaultModel)
	}
	if loaded.Roles["implement"].Model != nextExpected {
		t.Fatalf("expected loaded implement model %q, got %q", nextExpected, loaded.Roles["implement"].Model)
	}
}

func TestWizardRolesModelCycling(t *testing.T) {
	tempHome := t.TempDir()
	m := newModel(&fakeService{}, tempHome, "vtest")
	m.width, m.height = 100, 30

	// Step 1: Herdr
	m = press(m, "enter")
	// Step 2: Delegation (select yes)
	m = press(m, "enter")
	m = press(m, "1")
	// Step 3: Roles
	m = press(m, "enter")
	if m.screen != screenWizardRoles {
		t.Fatalf("expected screenWizardRoles, got %v", m.screen)
	}

	// Toggle role 0 (implement) to AGY
	m = press(m, " ")
	r := m.delegationCfg.Roles["implement"]
	if !r.Delegate || r.CLI != "agy" {
		t.Fatalf("expected implement to be AGY in wizard, got %+v", r)
	}

	// Cycle model with right arrow
	m = press(m, "right")
	nextExpected := delegation.NextModel(delegation.DefaultAGYModel, delegation.KnownAGYModels)
	r = m.delegationCfg.Roles["implement"]
	if r.Model != nextExpected {
		t.Fatalf("expected implement model to cycle to %q in wizard, got %q", nextExpected, r.Model)
	}

	// Verify view shows the active model
	view := m.View()
	if !strings.Contains(view, nextExpected) {
		t.Fatalf("expected wizard view to display model %q:\n%s", nextExpected, view)
	}
}
