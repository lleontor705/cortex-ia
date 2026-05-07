package components_test

import (
	"path/filepath"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/agents"
	"github.com/lleontor705/cortex-ia/internal/components/context7"
	"github.com/lleontor705/cortex-ia/internal/components/conventions"
	"github.com/lleontor705/cortex-ia/internal/components/cortex"
	"github.com/lleontor705/cortex-ia/internal/components/forgespec"
	"github.com/lleontor705/cortex-ia/internal/components/mailbox"
	"github.com/lleontor705/cortex-ia/internal/components/persona"
	"github.com/lleontor705/cortex-ia/internal/model"
)

// ---------------------------------------------------------------------------
// MCP-component golden tests (cortex / forgespec / mailbox / context7)
//
// Each test runs the real injector against a temp $HOME and snapshots the
// resulting agent config file. Adapters use distinct paths/strategies:
//   - claude          → ~/.claude/mcp/<name>.json (separate file)
//   - opencode        → ~/.config/opencode/opencode.json (merged "mcp" key)
//   - windsurf        → ~/.codeium/windsurf/mcp_config.json (overlay)
//   - antigravity     → ~/.antigravity/mcp_config.json (overlay)
//   - vscode          → ~/.vscode/settings.json (servers key)
//   - cursor          → ~/.cursor/mcp.json (overlay)
//   - codex           → ~/.codex/config.toml (TOML)
//
// We cover claude + opencode + windsurf + antigravity for each MCP component
// to lock the four most-used per-strategy outputs.
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
func mailboxInject(home string, adapter agents.Adapter) error {
	_, err := mailbox.Inject(home, adapter)
	return err
}
func context7Inject(home string, adapter agents.Adapter) error {
	_, err := context7.Inject(home, adapter)
	return err
}

// ---------------------------------------------------------------------------
// cortex (Cortex memory MCP)
// ---------------------------------------------------------------------------

func TestGoldenCortex_Claude(t *testing.T) {
	home := runMCP(t, cortexInject, claudeAdapter())
	got := readTestFile(t, filepath.Join(home, ".claude", "mcp", "cortex.json"))
	assertGolden(t, "cortex-claude-mcp.golden", got)
}

func TestGoldenCortex_OpenCode(t *testing.T) {
	home := runMCP(t, cortexInject, opencodeAdapter())
	got := readTestFile(t, filepath.Join(home, ".config", "opencode", "opencode.json"))
	assertGolden(t, "cortex-opencode-settings.golden", got)
}

func TestGoldenCortex_Windsurf(t *testing.T) {
	adapter := windsurfAdapter()
	home := runMCP(t, cortexInject, adapter)
	cfg := adapter.MCPConfigPath(home, "cortex")
	got := readTestFile(t, cfg)
	assertGolden(t, "cortex-windsurf-mcp.golden", got)
}

func TestGoldenCortex_Antigravity(t *testing.T) {
	adapter := antigravityAdapter()
	home := runMCP(t, cortexInject, adapter)
	cfg := adapter.MCPConfigPath(home, "cortex")
	got := readTestFile(t, cfg)
	assertGolden(t, "cortex-antigravity-mcp.golden", got)
}

// ---------------------------------------------------------------------------
// forgespec
// ---------------------------------------------------------------------------

func TestGoldenForgespec_Claude(t *testing.T) {
	home := runMCP(t, forgespecInject, claudeAdapter())
	got := readTestFile(t, filepath.Join(home, ".claude", "mcp", "forgespec.json"))
	assertGolden(t, "forgespec-claude-mcp.golden", got)
}

func TestGoldenForgespec_OpenCode(t *testing.T) {
	home := runMCP(t, forgespecInject, opencodeAdapter())
	got := readTestFile(t, filepath.Join(home, ".config", "opencode", "opencode.json"))
	assertGolden(t, "forgespec-opencode-settings.golden", got)
}

// ---------------------------------------------------------------------------
// mailbox
// ---------------------------------------------------------------------------

func TestGoldenMailbox_Claude(t *testing.T) {
	home := runMCP(t, mailboxInject, claudeAdapter())
	got := readTestFile(t, filepath.Join(home, ".claude", "mcp", "agent-mailbox.json"))
	assertGolden(t, "mailbox-claude-mcp.golden", got)
}

func TestGoldenMailbox_OpenCode(t *testing.T) {
	home := runMCP(t, mailboxInject, opencodeAdapter())
	got := readTestFile(t, filepath.Join(home, ".config", "opencode", "opencode.json"))
	assertGolden(t, "mailbox-opencode-settings.golden", got)
}

// ---------------------------------------------------------------------------
// context7
// ---------------------------------------------------------------------------

func TestGoldenContext7_Claude(t *testing.T) {
	home := runMCP(t, context7Inject, claudeAdapter())
	got := readTestFile(t, filepath.Join(home, ".claude", "mcp", "context7.json"))
	assertGolden(t, "context7-claude-mcp.golden", got)
}

func TestGoldenContext7_OpenCode(t *testing.T) {
	home := runMCP(t, context7Inject, opencodeAdapter())
	got := readTestFile(t, filepath.Join(home, ".config", "opencode", "opencode.json"))
	assertGolden(t, "context7-opencode-settings.golden", got)
}

// ---------------------------------------------------------------------------
// persona (system-prompt injection)
// ---------------------------------------------------------------------------

func TestGoldenPersona_Claude_Professional(t *testing.T) {
	home := t.TempDir()
	adapter := claudeAdapter()
	if _, err := persona.Inject(home, adapter, model.PersonaProfessional); err != nil {
		t.Fatalf("persona.Inject error: %v", err)
	}
	prompt := adapter.SystemPromptFile(home)
	got := readTestFile(t, prompt)
	assertGolden(t, "persona-claude-professional.golden", got)
}

func TestGoldenPersona_Claude_Mentor(t *testing.T) {
	home := t.TempDir()
	adapter := claudeAdapter()
	if _, err := persona.Inject(home, adapter, model.PersonaMentor); err != nil {
		t.Fatalf("persona.Inject error: %v", err)
	}
	prompt := adapter.SystemPromptFile(home)
	got := readTestFile(t, prompt)
	assertGolden(t, "persona-claude-mentor.golden", got)
}

func TestGoldenPersona_Claude_Minimal(t *testing.T) {
	home := t.TempDir()
	adapter := claudeAdapter()
	if _, err := persona.Inject(home, adapter, model.PersonaMinimal); err != nil {
		t.Fatalf("persona.Inject error: %v", err)
	}
	prompt := adapter.SystemPromptFile(home)
	got := readTestFile(t, prompt)
	assertGolden(t, "persona-claude-minimal.golden", got)
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

// ---------------------------------------------------------------------------
// conventions (shared memory protocol injection)
// ---------------------------------------------------------------------------

func TestGoldenConventions_Claude(t *testing.T) {
	home := t.TempDir()
	adapter := claudeAdapter()
	if _, err := conventions.Inject(home, adapter); err != nil {
		t.Fatalf("conventions.Inject error: %v", err)
	}
	prompt := adapter.SystemPromptFile(home)
	got := readTestFile(t, prompt)
	assertGolden(t, "conventions-claude-claudemd.golden", got)
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

// ---------------------------------------------------------------------------
// MCP server templates (raw byte snapshots — guard against accidental edits)
// ---------------------------------------------------------------------------

func TestGoldenCortexTemplates(t *testing.T) {
	tmpl := cortex.Templates()
	assertGolden(t, "cortex-template-separate.json", tmpl.SeparateFileJSON)
	assertGolden(t, "cortex-template-default-overlay.json", tmpl.DefaultOverlayJSON)
	assertGolden(t, "cortex-template-opencode-overlay.json", tmpl.OpenCodeOverlayJSON)
	assertGolden(t, "cortex-template-vscode-overlay.json", tmpl.VSCodeOverlayJSON)
}
