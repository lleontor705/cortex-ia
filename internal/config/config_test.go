package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/model"
)

func TestWriteDefault(t *testing.T) {
	dir := t.TempDir()
	path, err := WriteDefault(dir)
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty config")
	}
}

func TestLoadFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	os.WriteFile(path, []byte(`
preset: minimal
persona: mentor
model-preset: economy
agents:
  - claude-code
  - opencode
disabled-components:
  - mailbox
custom-skills:
  - path: ./skills/reviewer
`), 0o644)

	cfg, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Preset != "minimal" {
		t.Errorf("preset = %q, want minimal", cfg.Preset)
	}
	if cfg.Persona != "mentor" {
		t.Errorf("persona = %q, want mentor", cfg.Persona)
	}
	if len(cfg.Agents) != 2 {
		t.Errorf("agents = %d, want 2", len(cfg.Agents))
	}
	if len(cfg.DisabledComponents) != 1 {
		t.Errorf("disabled = %d, want 1", len(cfg.DisabledComponents))
	}
	if len(cfg.CustomSkills) != 1 {
		t.Errorf("custom-skills = %d, want 1", len(cfg.CustomSkills))
	}
}

func TestFindProjectConfig_Found(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, FileName), []byte("preset: full\n"), 0o644)

	subDir := filepath.Join(dir, "src", "pkg")
	os.MkdirAll(subDir, 0o755)

	cfg, foundDir, err := FindProjectConfig(subDir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg == nil {
		t.Fatal("expected config to be found")
	}
	if cfg.Preset != "full" {
		t.Errorf("preset = %q, want full", cfg.Preset)
	}
	if foundDir != dir {
		t.Errorf("found dir = %q, want %q", foundDir, dir)
	}
}

func TestFindProjectConfig_NotFound(t *testing.T) {
	cfg, _, err := FindProjectConfig(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if cfg != nil {
		t.Error("expected nil config when not found")
	}
}

func TestApplyToSelection(t *testing.T) {
	cfg := &ProjectConfig{
		Preset:      "minimal",
		Persona:     "mentor",
		ModelPreset: "economy",
		Agents:      []string{"claude-code"},
	}

	sel := model.Selection{}
	ApplyToSelection(cfg, &sel)

	if sel.Preset != "minimal" {
		t.Errorf("preset = %q, want minimal", sel.Preset)
	}
	if sel.Persona != "mentor" {
		t.Errorf("persona = %q, want mentor", sel.Persona)
	}
	if sel.ModelAssignments == nil {
		t.Error("expected model assignments")
	}
	if len(sel.Agents) != 1 {
		t.Errorf("agents = %d, want 1", len(sel.Agents))
	}
}

func TestApplyToSelection_NilConfig(t *testing.T) {
	sel := model.Selection{Preset: "full"}
	ApplyToSelection(nil, &sel)
	if sel.Preset != "full" {
		t.Error("nil config should not modify selection")
	}
}

func TestApplyToSelection_NoOverride(t *testing.T) {
	cfg := &ProjectConfig{Preset: "minimal"}
	sel := model.Selection{Preset: "full"} // already set

	ApplyToSelection(cfg, &sel)
	if sel.Preset != "full" {
		t.Error("should not override existing selection values")
	}
}
