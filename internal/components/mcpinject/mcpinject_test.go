package mcpinject

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/model"
	"github.com/lleontor705/cortex-ia/internal/system"
)

// testAdapter is a minimal adapter for testing MCP injection strategies.
type testAdapter struct {
	agent           model.AgentID
	mcpStrategy     model.MCPStrategy
	supportsMCP     bool
	globalConfigDir string
	settingsPath    string
}

func (a *testAdapter) Agent() model.AgentID    { return a.agent }
func (a *testAdapter) Tier() model.SupportTier { return model.TierFull }
func (a *testAdapter) Detect(_ string) (bool, string, string, bool, error) {
	return false, "", "", false, nil
}
func (a *testAdapter) GlobalConfigDir(_ string) string                  { return a.globalConfigDir }
func (a *testAdapter) SystemPromptDir(_ string) string                  { return "" }
func (a *testAdapter) SystemPromptFile(_ string) string                 { return "" }
func (a *testAdapter) SkillsDir(_ string) string                        { return "" }
func (a *testAdapter) SettingsPath(_ string) string                     { return a.settingsPath }
func (a *testAdapter) SystemPromptStrategy() model.SystemPromptStrategy { return 0 }
func (a *testAdapter) MCPStrategy() model.MCPStrategy                   { return a.mcpStrategy }
func (a *testAdapter) MCPConfigPath(homeDir string, serverName string) string {
	return filepath.Join(a.globalConfigDir, "mcp", serverName+".json")
}
func (a *testAdapter) SupportsSkills() bool                                { return false }
func (a *testAdapter) SupportsSystemPrompt() bool                          { return false }
func (a *testAdapter) SupportsMCP() bool                                   { return a.supportsMCP }
func (a *testAdapter) SupportsSlashCommands() bool                         { return false }
func (a *testAdapter) CommandsDir(_ string) string                         { return "" }
func (a *testAdapter) SupportsTaskDelegation() bool                        { return false }
func (a *testAdapter) SupportsSubAgents() bool                             { return false }
func (a *testAdapter) SubAgentsDir(_ string) string                        { return "" }
func (a *testAdapter) SupportsAutoInstall() bool                           { return false }
func (a *testAdapter) InstallCommands(_ system.PlatformProfile) [][]string { return nil }

func testTemplates() ServerTemplates {
	return ServerTemplates{
		Name:                "test-server",
		SeparateFileJSON:    []byte(`{"command": "test-cmd", "args": ["serve"]}` + "\n"),
		DefaultOverlayJSON:  []byte(`{"mcpServers": {"test-server": {"command": "test-cmd", "args": ["serve"]}}}` + "\n"),
		OpenCodeOverlayJSON: []byte(`{"mcp": {"test-server": {"type": "local", "command": ["test-cmd", "serve"], "enabled": true}}}` + "\n"),
		VSCodeOverlayJSON:   []byte(`{"servers": {"test-server": {"type": "stdio", "command": "test-cmd", "args": ["serve"]}}}` + "\n"),
		TOMLCommand:         "test-cmd",
		TOMLArgs:            []string{"serve"},
	}
}

func TestInject_NoMCPSupport(t *testing.T) {
	adapter := &testAdapter{supportsMCP: false}
	result, err := Inject("", adapter, testTemplates())
	if err != nil {
		t.Fatal(err)
	}
	if result.Changed {
		t.Error("expected no changes for unsupported MCP")
	}
}

func TestInject_SeparateFile(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".claude")

	adapter := &testAdapter{
		agent:           model.AgentClaudeCode,
		mcpStrategy:     model.StrategySeparateMCPFiles,
		supportsMCP:     true,
		globalConfigDir: configDir,
	}

	result, err := Inject(tmpDir, adapter, testTemplates())
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed {
		t.Error("expected Changed=true")
	}
	if len(result.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(result.Files))
	}

	content, err := os.ReadFile(result.Files[0])
	if err != nil {
		t.Fatal(err)
	}

	var m map[string]any
	if err := json.Unmarshal(content, &m); err != nil {
		t.Fatal(err)
	}
	if m["command"] != "test-cmd" {
		t.Errorf("command = %v", m["command"])
	}
}

func TestInject_MergeIntoSettings_NewFile(t *testing.T) {
	tmpDir := t.TempDir()
	settingsPath := filepath.Join(tmpDir, "settings.json")

	adapter := &testAdapter{
		agent:        model.AgentClaudeCode,
		mcpStrategy:  model.StrategyMergeIntoSettings,
		supportsMCP:  true,
		settingsPath: settingsPath,
	}

	result, err := Inject(tmpDir, adapter, testTemplates())
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed {
		t.Error("expected Changed=true")
	}

	content, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}

	var m map[string]any
	if err := json.Unmarshal(content, &m); err != nil {
		t.Fatal(err)
	}
	servers := m["mcpServers"].(map[string]any)
	if servers["test-server"] == nil {
		t.Error("expected test-server in mcpServers")
	}
}

func TestInject_MergeIntoSettings_OpenCode(t *testing.T) {
	tmpDir := t.TempDir()
	settingsPath := filepath.Join(tmpDir, "opencode.json")

	// Pre-existing settings
	if err := os.WriteFile(settingsPath, []byte(`{"theme": "dark"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	adapter := &testAdapter{
		agent:        model.AgentOpenCode,
		mcpStrategy:  model.StrategyMergeIntoSettings,
		supportsMCP:  true,
		settingsPath: settingsPath,
	}

	result, err := Inject(tmpDir, adapter, testTemplates())
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed {
		t.Error("expected Changed=true")
	}

	content, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}

	var m map[string]any
	if err := json.Unmarshal(content, &m); err != nil {
		t.Fatal(err)
	}

	// Verify existing content preserved
	if m["theme"] != "dark" {
		t.Error("expected existing theme to be preserved")
	}

	// Verify OpenCode overlay uses "mcp" key
	mcp := m["mcp"].(map[string]any)
	server := mcp["test-server"].(map[string]any)
	if server["type"] != "local" {
		t.Errorf("type = %v, want local", server["type"])
	}
}

func TestInject_MergeIntoSettings_OpenCodeJSONCPreservesComments(t *testing.T) {
	tmpDir := t.TempDir()
	settingsPath := filepath.Join(tmpDir, "opencode.jsonc")
	before := []byte("{\n  // User setting.\n  \"share\": \"disabled\",\n}\n")
	if err := os.WriteFile(settingsPath, before, 0o644); err != nil {
		t.Fatal(err)
	}
	adapter := &testAdapter{
		agent: model.AgentOpenCode, mcpStrategy: model.StrategyMergeIntoSettings,
		supportsMCP: true, settingsPath: settingsPath,
	}
	result, err := Inject(tmpDir, adapter, testTemplates())
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed {
		t.Fatal("expected JSONC settings to change")
	}
	content, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range []string{"// User setting.", `"share": "disabled",`, `"test-server"`} {
		if !strings.Contains(string(content), marker) {
			t.Errorf("patched OpenCode JSONC missing %q:\n%s", marker, content)
		}
	}
}

func TestInject_MCPConfigFile_VSCode(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".copilot")

	adapter := &testAdapter{
		agent:           model.AgentVSCodeCopilot,
		mcpStrategy:     model.StrategyMCPConfigFile,
		supportsMCP:     true,
		globalConfigDir: configDir,
	}

	result, err := Inject(tmpDir, adapter, testTemplates())
	if err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(result.Files[0])
	if err != nil {
		t.Fatal(err)
	}

	var m map[string]any
	if err := json.Unmarshal(content, &m); err != nil {
		t.Fatal(err)
	}

	// VS Code uses "servers" key
	servers := m["servers"].(map[string]any)
	server := servers["test-server"].(map[string]any)
	if server["type"] != "stdio" {
		t.Errorf("type = %v, want stdio", server["type"])
	}
}

func TestInject_TOML(t *testing.T) {
	tmpDir := t.TempDir()
	settingsPath := filepath.Join(tmpDir, "config.toml")

	// Pre-existing TOML
	if err := os.WriteFile(settingsPath, []byte("model = \"gpt-4\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	adapter := &testAdapter{
		agent:        model.AgentCodex,
		mcpStrategy:  model.StrategyTOMLFile,
		supportsMCP:  true,
		settingsPath: settingsPath,
	}

	result, err := Inject(tmpDir, adapter, testTemplates())
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed {
		t.Error("expected Changed=true")
	}

	content, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}

	s := string(content)
	if !strings.Contains(s, "[mcp_servers.test-server]") {
		t.Error("expected TOML section header")
	}
	if !strings.Contains(s, `command = "test-cmd"`) {
		t.Error("expected TOML command")
	}
	if !strings.Contains(s, `"gpt-4"`) {
		t.Error("expected existing content preserved")
	}
}

func TestInject_TOMLPersistsRegionOwnershipEvidence(t *testing.T) {
	tmpDir := t.TempDir()
	settingsPath := filepath.Join(tmpDir, "config.toml")
	templates := testTemplates()
	templates.Name = "cortex"
	templates.TOMLCommand = filepath.Join("Program Files", "Cortex", "cortex")
	templates.TOMLArgs = []string{"mcp", "--tools=agent"}
	adapter := &testAdapter{
		agent:        model.AgentCodex,
		mcpStrategy:  model.StrategyTOMLFile,
		supportsMCP:  true,
		settingsPath: settingsPath,
	}

	if _, err := Inject(tmpDir, adapter, templates); err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	var marker string
	for _, line := range strings.Split(string(content), "\n") {
		if strings.Contains(line, "cortex-ia:toml-ownership ") {
			marker = line
			break
		}
	}
	if marker == "" {
		t.Fatal("expected TOML ownership marker")
	}
	var ownership tomlRegionOwnership
	if err := json.Unmarshal([]byte(strings.TrimPrefix(strings.TrimSpace(marker), "# cortex-ia:toml-ownership ")), &ownership); err != nil {
		t.Fatalf("parse ownership marker: %v", err)
	}
	if ownership.Owner != "cortex-ia" {
		t.Errorf("owner = %q, want cortex-ia", ownership.Owner)
	}
	if ownership.SemanticID != "mcp/codex/cortex" {
		t.Errorf("semantic ID = %q, want mcp/codex/cortex", ownership.SemanticID)
	}
	if got, want := ownership.TablePath, []string{"mcp_servers", "cortex"}; strings.Join(got, "/") != strings.Join(want, "/") {
		t.Errorf("table path = %q, want %q", got, want)
	}
	if got, want := ownership.Command, append([]string{templates.TOMLCommand}, templates.TOMLArgs...); strings.Join(got, "\x00") != strings.Join(want, "\x00") {
		t.Errorf("command = %q, want %q", got, want)
	}
	base := []byte(fmt.Sprintf("[mcp_servers.cortex]\ncommand = %q\nargs = [\"mcp\", \"--tools=agent\"]\n", templates.TOMLCommand))
	baseHash := fmt.Sprintf("%x", sha256.Sum256(base))
	if ownership.BaseSHA256 != baseHash || ownership.OwnershipSHA != tomlOwnershipDigest(ownership) {
		t.Errorf("ownership hashes = (%q, %q), want region base hash %q and matching ownership proof", ownership.BaseSHA256, ownership.OwnershipSHA, baseHash)
	}
	if !strings.Contains(string(content), fmt.Sprintf("command = %q", templates.TOMLCommand)) || !strings.Contains(string(content), `args = ["mcp", "--tools=agent"]`) {
		t.Errorf("TOML did not preserve exact command vector:\n%s", content)
	}
}

func TestInject_TOMLFailedWriteLeavesNoOwnershipEvidence(t *testing.T) {
	tmpDir := t.TempDir()
	blockedParent := filepath.Join(tmpDir, "not-a-directory")
	if err := os.WriteFile(blockedParent, []byte("block"), 0o644); err != nil {
		t.Fatal(err)
	}
	settingsPath := filepath.Join(blockedParent, "config.toml")
	adapter := &testAdapter{
		agent:        model.AgentCodex,
		mcpStrategy:  model.StrategyTOMLFile,
		supportsMCP:  true,
		settingsPath: settingsPath,
	}

	if _, err := Inject(tmpDir, adapter, testTemplates()); err == nil {
		t.Fatal("expected atomic TOML write to fail")
	}
	content, err := os.ReadFile(settingsPath)
	if !os.IsNotExist(err) {
		t.Fatalf("failed write left config or unreadable evidence: content=%q err=%v", content, err)
	}
}

func TestInject_Idempotent(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".claude")

	adapter := &testAdapter{
		agent:           model.AgentClaudeCode,
		mcpStrategy:     model.StrategySeparateMCPFiles,
		supportsMCP:     true,
		globalConfigDir: configDir,
	}

	// First injection
	_, err := Inject(tmpDir, adapter, testTemplates())
	if err != nil {
		t.Fatal(err)
	}

	// Second injection — should not change
	result, err := Inject(tmpDir, adapter, testTemplates())
	if err != nil {
		t.Fatal(err)
	}
	if result.Changed {
		t.Error("expected Changed=false on second injection (idempotent)")
	}
}
