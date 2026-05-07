package agents

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/model"
	"github.com/lleontor705/cortex-ia/internal/system"
)

// stubAdapter is a minimal Adapter implementation for discovery tests.
type stubAdapter struct {
	agent     model.AgentID
	configDir string // value returned by GlobalConfigDir (may be empty)
}

func (s stubAdapter) Agent() model.AgentID      { return s.agent }
func (s stubAdapter) Tier() model.SupportTier   { return model.TierFull }
func (s stubAdapter) SupportsAutoInstall() bool { return false }
func (s stubAdapter) Detect(_ string) (bool, string, string, bool, error) {
	return false, "", "", false, nil
}
func (s stubAdapter) InstallCommands(system.PlatformProfile) [][]string { return nil }

// GlobalConfigDir returns the pre-configured dir for the stub.
func (s stubAdapter) GlobalConfigDir(_ string) string  { return s.configDir }
func (s stubAdapter) SystemPromptDir(_ string) string  { return "" }
func (s stubAdapter) SystemPromptFile(_ string) string { return "" }
func (s stubAdapter) SkillsDir(_ string) string        { return "" }
func (s stubAdapter) SettingsPath(_ string) string     { return "" }
func (s stubAdapter) SystemPromptStrategy() model.SystemPromptStrategy {
	return model.StrategyMarkdownSections
}
func (s stubAdapter) MCPStrategy() model.MCPStrategy          { return model.StrategySeparateMCPFiles }
func (s stubAdapter) MCPConfigPath(_ string, _ string) string { return "" }
func (s stubAdapter) SupportsSlashCommands() bool             { return false }
func (s stubAdapter) CommandsDir(_ string) string             { return "" }
func (s stubAdapter) SupportsTaskDelegation() bool            { return false }
func (s stubAdapter) SupportsSubAgents() bool                 { return false }
func (s stubAdapter) SubAgentsDir(_ string) string            { return "" }
func (s stubAdapter) SupportsSkills() bool                    { return true }
func (s stubAdapter) SupportsSystemPrompt() bool              { return true }
func (s stubAdapter) SupportsMCP() bool                       { return true }

// newStubRegistry creates a Registry from stub adapters.
func newStubRegistry(adapters ...stubAdapter) *Registry {
	r := NewRegistry()
	for _, a := range adapters {
		r.Register(a)
	}
	return r
}

// ─── DiscoverInstalled ───────────────────────────────────────────────────

func TestDiscoverInstalled_ReturnsOnlyInstalledAgents(t *testing.T) {
	home := t.TempDir()

	claudeDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	reg := newStubRegistry(
		stubAdapter{agent: model.AgentClaudeCode, configDir: claudeDir},
		stubAdapter{agent: model.AgentOpenCode, configDir: filepath.Join(home, ".config", "opencode")},
	)

	got := DiscoverInstalled(reg, home)

	if len(got) != 1 {
		t.Fatalf("DiscoverInstalled() returned %d agents, want 1; got %v", len(got), got)
	}
	if got[0].ID != model.AgentClaudeCode {
		t.Errorf("DiscoverInstalled() agent = %q, want %q", got[0].ID, model.AgentClaudeCode)
	}
	if got[0].ConfigDir != claudeDir {
		t.Errorf("DiscoverInstalled() ConfigDir = %q, want %q", got[0].ConfigDir, claudeDir)
	}
}

func TestDiscoverInstalled_EmptyGlobalConfigDirIsSkipped(t *testing.T) {
	home := t.TempDir()

	reg := newStubRegistry(
		stubAdapter{agent: model.AgentClaudeCode, configDir: ""},
	)

	got := DiscoverInstalled(reg, home)

	if len(got) != 0 {
		t.Errorf("DiscoverInstalled() expected empty result for empty GlobalConfigDir, got %v", got)
	}
}

func TestDiscoverInstalled_MissingDirIsSkipped(t *testing.T) {
	home := t.TempDir()

	reg := newStubRegistry(
		stubAdapter{agent: model.AgentOpenCode, configDir: filepath.Join(home, ".config", "opencode")},
	)

	got := DiscoverInstalled(reg, home)

	if len(got) != 0 {
		t.Errorf("DiscoverInstalled() expected empty result for missing dir, got %v", got)
	}
}

func TestDiscoverInstalled_FileInsteadOfDirIsSkipped(t *testing.T) {
	home := t.TempDir()

	// Create a file at the expected config-dir location.
	notDir := filepath.Join(home, "config-as-file")
	if err := os.WriteFile(notDir, []byte("not a dir"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	reg := newStubRegistry(
		stubAdapter{agent: model.AgentClaudeCode, configDir: notDir},
	)

	got := DiscoverInstalled(reg, home)

	if len(got) != 0 {
		t.Errorf("DiscoverInstalled() expected empty for file path, got %v", got)
	}
}

func TestDiscoverInstalled_MultipleInstalled(t *testing.T) {
	home := t.TempDir()

	claudeDir := filepath.Join(home, ".claude")
	opencodeDir := filepath.Join(home, ".config", "opencode")
	for _, dir := range []string{claudeDir, opencodeDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("MkdirAll(%q): %v", dir, err)
		}
	}

	reg := newStubRegistry(
		stubAdapter{agent: model.AgentClaudeCode, configDir: claudeDir},
		stubAdapter{agent: model.AgentOpenCode, configDir: opencodeDir},
		stubAdapter{agent: model.AgentGeminiCLI, configDir: filepath.Join(home, ".gemini")}, // not created
	)

	got := DiscoverInstalled(reg, home)

	if len(got) != 2 {
		t.Fatalf("DiscoverInstalled() returned %d agents, want 2; got %v", len(got), got)
	}
}

func TestDiscoverInstalled_EmptyRegistryReturnsEmpty(t *testing.T) {
	home := t.TempDir()

	reg := newStubRegistry()

	got := DiscoverInstalled(reg, home)

	if len(got) != 0 {
		t.Errorf("DiscoverInstalled() expected empty slice, got %v", got)
	}
}

func TestDiscoverInstalled_NilRegistryReturnsEmpty(t *testing.T) {
	got := DiscoverInstalled(nil, t.TempDir())
	if len(got) != 0 {
		t.Errorf("DiscoverInstalled(nil) expected empty slice, got %v", got)
	}
}

// ─── ConfigRootsForBackup ────────────────────────────────────────────────

func TestConfigRootsForBackup_ReturnsInstalledDirs(t *testing.T) {
	home := t.TempDir()

	claudeDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	reg := newStubRegistry(
		stubAdapter{agent: model.AgentClaudeCode, configDir: claudeDir},
		stubAdapter{agent: model.AgentOpenCode, configDir: filepath.Join(home, ".config", "opencode")},
	)

	roots := ConfigRootsForBackup(reg, home)

	if len(roots) != 1 {
		t.Fatalf("ConfigRootsForBackup() returned %d roots, want 1; got %v", len(roots), roots)
	}
	if roots[0] != claudeDir {
		t.Errorf("ConfigRootsForBackup() root = %q, want %q", roots[0], claudeDir)
	}
}

func TestConfigRootsForBackup_DeduplicatesSharedDirs(t *testing.T) {
	home := t.TempDir()

	sharedDir := filepath.Join(home, ".shared-config")
	if err := os.MkdirAll(sharedDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	reg := newStubRegistry(
		stubAdapter{agent: model.AgentClaudeCode, configDir: sharedDir},
		stubAdapter{agent: model.AgentOpenCode, configDir: sharedDir},
	)

	roots := ConfigRootsForBackup(reg, home)

	if len(roots) != 1 {
		t.Fatalf("ConfigRootsForBackup() returned %d roots with duplicate dir, want 1; got %v", len(roots), roots)
	}
}

func TestConfigRootsForBackup_NilSafeOnEmptyRegistry(t *testing.T) {
	home := t.TempDir()
	reg := newStubRegistry()

	roots := ConfigRootsForBackup(reg, home)
	if roots == nil {
		t.Errorf("ConfigRootsForBackup() returned nil, want non-nil slice")
	}
	if len(roots) != 0 {
		t.Errorf("ConfigRootsForBackup() expected empty for empty registry, got %v", roots)
	}
}

// ─── Integration: DefaultRegistry ────────────────────────────────────────

func TestDiscoverInstalled_WithDefaultRegistryAndRealFS(t *testing.T) {
	home := t.TempDir()

	claudeDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	reg := NewDefaultRegistry()

	got := DiscoverInstalled(reg, home)

	if len(got) != 1 {
		t.Fatalf("DiscoverInstalled() with real registry returned %d agents, want 1; got %v", len(got), got)
	}
	if got[0].ID != model.AgentClaudeCode {
		t.Errorf("DiscoverInstalled() agent = %q, want %q", got[0].ID, model.AgentClaudeCode)
	}
}
