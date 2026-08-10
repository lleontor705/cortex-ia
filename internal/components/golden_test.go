package components_test

import (
	"path/filepath"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/agents"
	"github.com/lleontor705/cortex-ia/internal/components/context7"
	"github.com/lleontor705/cortex-ia/internal/components/conventions"
	"github.com/lleontor705/cortex-ia/internal/components/cortex"
	"github.com/lleontor705/cortex-ia/internal/components/forgespec"
	"github.com/lleontor705/cortex-ia/internal/components/persona"
	"github.com/lleontor705/cortex-ia/internal/model"
)

// ---------------------------------------------------------------------------
// MCP-component golden tests (cortex / forgespec / context7)
//
// Each test runs the real injector against a temp $HOME and snapshots the
// resulting agent config file. Adapters use distinct paths/strategies:
//   - claude          → ~/.claude/mcp/<name>.json (separate file)
//   - opencode        → ~/.config/opencode/opencode.json (merged "mcp" key)
//   - vscode          → ~/.vscode/settings.json (servers key)
//   - codex           → ~/.codex/config.toml (TOML)
//
// We cover claude and opencode for each MCP component to lock the supported
// per-strategy outputs.
// ---------------------------------------------------------------------------

type mcpInjector func(home string, adapter agents.Adapter) error

func runMCP(t *testing.T, inj mcpInjector, adapter agents.Adapter) string {
	t.Helper()
	home := t.TempDir()
	if err := inj(home, adapter); err != nil {
		t.Fatalf("inject error: %v", err)
	}
	return home
}

func cortexInject(home string, adapter agents.Adapter) error {
	_, err := cortex.Inject(home, adapter)
	return err
}
func forgespecInject(home string, adapter agents.Adapter) error {
	_, err := forgespec.Inject(home, adapter)
	return err
}
func context7Inject(home string, adapter agents.Adapter) error {
	_, err := context7.Inject(home, adapter)
	return err
}

func TestGoldenCortex_OpenCode(t *testing.T) {
	home := runMCP(t, cortexInject, opencodeAdapter())
	got := readTestFile(t, filepath.Join(home, ".config", "opencode", "opencode.json"))
	assertGolden(t, "cortex-opencode-settings.golden", got)
}

func TestGoldenForgespec_OpenCode(t *testing.T) {
	home := runMCP(t, forgespecInject, opencodeAdapter())
	got := readTestFile(t, filepath.Join(home, ".config", "opencode", "opencode.json"))
	assertGolden(t, "forgespec-opencode-settings.golden", got)
}

func TestGoldenContext7_OpenCode(t *testing.T) {
	home := runMCP(t, context7Inject, opencodeAdapter())
	got := readTestFile(t, filepath.Join(home, ".config", "opencode", "opencode.json"))
	assertGolden(t, "context7-opencode-settings.golden", got)
}

func TestGoldenPersona_OpenCode_Professional(t *testing.T) {
	home := t.TempDir()
	adapter := opencodeAdapter()
	if _, err := persona.Inject(home, adapter, model.PersonaProfessional); err != nil {
		t.Fatalf("persona.Inject error: %v", err)
	}
	prompt := adapter.SystemPromptFile(home)
	got := readTestFile(t, prompt)
	assertGolden(t, "persona-opencode-professional.golden", got)
}

func TestGoldenConventions_OpenCode(t *testing.T) {
	home := t.TempDir()
	adapter := opencodeAdapter()
	if _, err := conventions.Inject(home, adapter); err != nil {
		t.Fatalf("conventions.Inject error: %v", err)
	}
	prompt := adapter.SystemPromptFile(home)
	got := readTestFile(t, prompt)
	assertGolden(t, "conventions-opencode-agentsmd.golden", got)
}

func TestGoldenCortexTemplates(t *testing.T) {
	tmpl := cortex.Templates()
	assertGolden(t, "cortex-template-separate.json", tmpl.SeparateFileJSON)
	assertGolden(t, "cortex-template-default-overlay.json", tmpl.DefaultOverlayJSON)
	assertGolden(t, "cortex-template-opencode-overlay.json", tmpl.OpenCodeOverlayJSON)
}
