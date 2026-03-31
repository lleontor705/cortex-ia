package persona

import (
	"os"
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/agents/claude"
	"github.com/lleontor705/cortex-ia/internal/model"
)

func TestInject_Professional(t *testing.T) {
	tmpDir := t.TempDir()
	adapter := claude.NewAdapter()

	result, err := Inject(tmpDir, adapter, model.PersonaProfessional)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed {
		t.Error("expected Changed=true")
	}

	data, _ := os.ReadFile(adapter.SystemPromptFile(tmpDir))
	if !strings.Contains(string(data), "cortex-ia:cortex-persona") {
		t.Error("expected persona marker")
	}
	if !strings.Contains(string(data), "senior software engineer") {
		t.Error("expected professional persona content")
	}
}

func TestInject_Mentor(t *testing.T) {
	tmpDir := t.TempDir()
	adapter := claude.NewAdapter()

	result, err := Inject(tmpDir, adapter, model.PersonaMentor)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed {
		t.Error("expected Changed=true")
	}

	data, _ := os.ReadFile(adapter.SystemPromptFile(tmpDir))
	if !strings.Contains(string(data), "mentor") {
		t.Error("expected mentor persona content")
	}
}

func TestInject_Minimal(t *testing.T) {
	tmpDir := t.TempDir()
	adapter := claude.NewAdapter()

	result, err := Inject(tmpDir, adapter, model.PersonaMinimal)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed {
		t.Error("expected Changed=true")
	}

	data, _ := os.ReadFile(adapter.SystemPromptFile(tmpDir))
	if !strings.Contains(string(data), "Minimal output") {
		t.Error("expected minimal persona content")
	}
}

func TestInject_Default(t *testing.T) {
	tmpDir := t.TempDir()
	adapter := claude.NewAdapter()

	// Empty persona should default to professional.
	result, err := Inject(tmpDir, adapter, "")
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed {
		t.Error("expected Changed=true")
	}
}

func TestInject_Idempotent(t *testing.T) {
	tmpDir := t.TempDir()
	adapter := claude.NewAdapter()

	if _, err := Inject(tmpDir, adapter, model.PersonaProfessional); err != nil {
		t.Fatal(err)
	}
	second, err := Inject(tmpDir, adapter, model.PersonaProfessional)
	if err != nil {
		t.Fatal(err)
	}
	if second.Changed {
		t.Error("expected idempotent")
	}
}

func TestInject_InvalidPersona(t *testing.T) {
	tmpDir := t.TempDir()
	adapter := claude.NewAdapter()

	_, err := Inject(tmpDir, adapter, "nonexistent")
	if err == nil {
		t.Fatal("expected error for invalid persona")
	}
}
