package uninstall

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/agents/claude"
	"github.com/lleontor705/cortex-ia/internal/components/conventions"
	"github.com/lleontor705/cortex-ia/internal/components/permissions"
	"github.com/lleontor705/cortex-ia/internal/components/persona"
	"github.com/lleontor705/cortex-ia/internal/model"
)

// TestMarkerAudit_PersonaMarkerIsCleaned verifies the uninstall cleaner
// removes the exact marker the persona injector writes. If persona's marker
// ever changes, this test fails immediately rather than leaving a dangling
// marker after `cortex-ia uninstall`.
func TestMarkerAudit_PersonaMarkerIsCleaned(t *testing.T) {
	home := t.TempDir()
	adapter := claude.NewAdapter()
	if _, err := persona.Inject(home, adapter, model.PersonaProfessional); err != nil {
		t.Fatalf("persona.Inject: %v", err)
	}
	prompt := adapter.SystemPromptFile(home)
	if data, _ := os.ReadFile(prompt); !strings.Contains(string(data), "cortex-ia:cortex-persona") {
		t.Fatalf("persona did not inject expected marker; injector may have changed")
	}
	for _, id := range markersByComponent[model.ComponentPersona] {
		if changed, err := rewriteMarkdownSection(prompt, id); err != nil || !changed {
			t.Fatalf("uninstall cleaner failed for persona marker %q: changed=%v err=%v", id, changed, err)
		}
	}
	data, _ := os.ReadFile(prompt)
	if strings.Contains(string(data), "cortex-persona") {
		t.Errorf("persona marker survived uninstall:\n%s", string(data))
	}
}

func TestMarkerAudit_ConventionsMarkerIsCleaned(t *testing.T) {
	home := t.TempDir()
	adapter := claude.NewAdapter()
	if _, err := conventions.Inject(home, adapter); err != nil {
		t.Fatalf("conventions.Inject: %v", err)
	}
	prompt := adapter.SystemPromptFile(home)
	for _, id := range markersByComponent[model.ComponentConventions] {
		if _, err := rewriteMarkdownSection(prompt, id); err != nil {
			t.Fatalf("uninstall cleaner err for conventions marker %q: %v", id, err)
		}
	}
	data, _ := os.ReadFile(prompt)
	if strings.Contains(string(data), "cortex-protocol") {
		t.Errorf("conventions marker survived:\n%s", string(data))
	}
}

func TestMarkerAudit_PermissionsMarkerIsCleaned(t *testing.T) {
	home := t.TempDir()
	adapter := claude.NewAdapter()
	if _, err := permissions.Inject(home, adapter); err != nil {
		t.Fatalf("permissions.Inject: %v", err)
	}
	prompt := adapter.SystemPromptFile(home)
	if data, _ := os.ReadFile(prompt); !strings.Contains(string(data), "cortex-ia:cortex-permissions") {
		t.Skipf("permissions skipped injection on this adapter; nothing to audit")
	}
	for _, id := range markersByComponent[model.ComponentPermissions] {
		if _, err := rewriteMarkdownSection(prompt, id); err != nil {
			t.Fatalf("uninstall cleaner err for permissions marker %q: %v", id, err)
		}
	}
	data, _ := os.ReadFile(prompt)
	if strings.Contains(string(data), "cortex-permissions") {
		t.Errorf("permissions marker survived:\n%s", string(data))
	}
	_ = filepath.Base(prompt) // silence unused import warnings if filepath isn't needed elsewhere
}
