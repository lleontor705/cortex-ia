package uninstall

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

func TestRewriteMarkdownSection_Removes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "CLAUDE.md")
	original := "# Header\n\n<!-- cortex-ia:cortex-persona -->\nManaged tone\n<!-- /cortex-ia:cortex-persona -->\n\nUser content.\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	changed, err := rewriteMarkdownSection(path, "cortex-persona")
	if err != nil {
		t.Fatalf("rewriteMarkdownSection: %v", err)
	}
	if !changed {
		t.Fatal("expected changed=true")
	}

	got, _ := os.ReadFile(path)
	if strings.Contains(string(got), "cortex-persona") {
		t.Errorf("marker still present after removal:\n%s", string(got))
	}
	if !strings.Contains(string(got), "User content.") {
		t.Errorf("user content was removed:\n%s", string(got))
	}
}

func TestRewriteMarkdownSection_NoOp_MissingFile(t *testing.T) {
	changed, err := rewriteMarkdownSection(filepath.Join(t.TempDir(), "missing.md"), "x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if changed {
		t.Errorf("expected changed=false for missing file")
	}
}

func TestRewriteMarkdownSection_NoOp_MarkerAbsent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "CLAUDE.md")
	if err := os.WriteFile(path, []byte("nothing managed here\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	changed, err := rewriteMarkdownSection(path, "cortex-persona")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if changed {
		t.Errorf("expected changed=false when marker absent")
	}
}

func TestRemoveFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cortex.json")
	if err := os.WriteFile(path, []byte("{}"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	changed, err := removeFile(path)
	if err != nil {
		t.Fatalf("removeFile: %v", err)
	}
	if !changed {
		t.Error("expected changed=true")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("file still present after remove: %v", err)
	}

	// Second call: no-op.
	changed, err = removeFile(path)
	if err != nil || changed {
		t.Errorf("expected no-op on missing file, got changed=%v err=%v", changed, err)
	}
}

func TestRemoveTree(t *testing.T) {
	dir := t.TempDir()
	tree := filepath.Join(dir, "skills")
	_ = os.MkdirAll(filepath.Join(tree, "bootstrap"), 0o755)
	_ = os.WriteFile(filepath.Join(tree, "bootstrap", "SKILL.md"), []byte("x"), 0o644)

	changed, err := removeTree(tree)
	if err != nil || !changed {
		t.Fatalf("removeTree: changed=%v err=%v", changed, err)
	}
	if _, err := os.Stat(tree); !os.IsNotExist(err) {
		t.Errorf("tree still present: %v", err)
	}
}

func TestRemoveIfEmpty_TrueWhenEmpty(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "empty")
	_ = os.MkdirAll(target, 0o755)

	changed, err := removeIfEmpty(target)
	if err != nil || !changed {
		t.Fatalf("removeIfEmpty: changed=%v err=%v", changed, err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Errorf("empty dir not removed")
	}
}

func TestRemoveIfEmpty_FalseWhenContainsFile(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "with-file")
	_ = os.MkdirAll(target, 0o755)
	_ = os.WriteFile(filepath.Join(target, "user.md"), []byte("user content"), 0o644)

	changed, err := removeIfEmpty(target)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if changed {
		t.Error("expected changed=false when dir has files")
	}
	if _, err := os.Stat(target); err != nil {
		t.Errorf("dir was unexpectedly removed: %v", err)
	}
}

func TestRemoveIfEmpty_RecursivelyEmpty(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "outer")
	_ = os.MkdirAll(filepath.Join(target, "inner", "deeper"), 0o755)

	changed, err := removeIfEmpty(target)
	if err != nil || !changed {
		t.Fatalf("removeIfEmpty: changed=%v err=%v", changed, err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Errorf("nested empty tree not removed")
	}
}

func TestRemoveJSONKey_TopLevel(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	original := map[string]any{
		"mcpServers": map[string]any{
			"cortex":  map[string]any{"command": "cortex"},
			"context": map[string]any{"command": "ctx"},
		},
		"otherSetting": true,
	}
	data, _ := json.MarshalIndent(original, "", "  ")
	_ = os.WriteFile(path, data, 0o644)

	changed, err := removeJSONKey(path, []string{"mcpServers", "cortex"})
	if err != nil || !changed {
		t.Fatalf("removeJSONKey: changed=%v err=%v", changed, err)
	}

	got, _ := os.ReadFile(path)
	if strings.Contains(string(got), `"cortex"`) {
		t.Errorf("cortex key still present:\n%s", string(got))
	}
	if !strings.Contains(string(got), `"context"`) {
		t.Errorf("sibling key was removed:\n%s", string(got))
	}
	if !strings.Contains(string(got), `"otherSetting"`) {
		t.Errorf("user setting was removed:\n%s", string(got))
	}
}

func TestRemoveJSONKey_PrunesEmptyParent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	original := map[string]any{
		"mcpServers": map[string]any{
			"cortex": map[string]any{"command": "cortex"},
		},
		"keep": "value",
	}
	data, _ := json.MarshalIndent(original, "", "  ")
	_ = os.WriteFile(path, data, 0o644)

	if _, err := removeJSONKey(path, []string{"mcpServers", "cortex"}); err != nil {
		t.Fatalf("removeJSONKey: %v", err)
	}

	got, _ := os.ReadFile(path)
	if strings.Contains(string(got), `"mcpServers"`) {
		t.Errorf("empty mcpServers parent should have been pruned:\n%s", string(got))
	}
	if !strings.Contains(string(got), `"keep"`) {
		t.Errorf("sibling top-level key was removed:\n%s", string(got))
	}
}

func TestRemoveJSONKey_NoOp_MissingKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	_ = os.WriteFile(path, []byte(`{"mcpServers":{"other":{}}}`), 0o644)
	changed, err := removeJSONKey(path, []string{"mcpServers", "cortex"})
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if changed {
		t.Errorf("expected no-op when key absent")
	}
}

func TestRemoveJSONKey_NoOp_MissingFile(t *testing.T) {
	changed, err := removeJSONKey(filepath.Join(t.TempDir(), "missing.json"), []string{"any"})
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if changed {
		t.Errorf("expected no-op for missing file")
	}
}

func TestRemoveJSONKey_JSONCPreservesComments(t *testing.T) {
	path := filepath.Join(t.TempDir(), "opencode.jsonc")
	before := []byte("{\n  \"mcp\": {\n    \"cortex\": { \"enabled\": true },\n    \"user\": { \"enabled\": true },\n  },\n  // Keep user configuration.\n  \"share\": \"disabled\",\n}\n")
	if err := os.WriteFile(path, before, 0o644); err != nil {
		t.Fatal(err)
	}
	changed, err := removeJSONKey(path, []string{"mcp", "cortex"})
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("expected JSONC uninstall mutation")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(after), `"cortex"`) || !strings.Contains(string(after), `"user"`) || !strings.Contains(string(after), "// Keep user configuration.") {
		t.Fatalf("JSONC uninstall changed user content:\n%s", after)
	}
}

func TestRemoveJSONKey_InvalidJSONReturnsErrorWithoutMutation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "opencode.jsonc")
	before := []byte(`{"mcp":`)
	if err := os.WriteFile(path, before, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := removeJSONKey(path, []string{"mcp", "cortex"}); err == nil {
		t.Fatal("invalid JSONC uninstall unexpectedly succeeded")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatalf("invalid JSONC changed: %s", after)
	}
}

func TestRemoveTOMLRegion_OwnedAndLegacyCortexOnly(t *testing.T) {
	owned := ownedCortexTOML(t, "\r\n")
	legacy := "[mcp_servers.cortex]\ncommand = \"cortex\"\nargs = [\"mcp\", \"--tools=agent\"]\n"
	tests := []struct {
		name   string
		before string
		want   string
	}{
		{
			name:   "current ownership preserves CRLF siblings comments order and whitespace",
			before: "# user header\r\n# user keeps this separator\r\n\r\n" + owned + "\r\n[mcp_servers.user]\r\n# user table comment\r\ncommand = \"user\"\r\nargs = [\"serve\"]\r\n",
			want:   "# user header\r\n# user keeps this separator\r\n\r\n[mcp_servers.user]\r\n# user table comment\r\ncommand = \"user\"\r\nargs = [\"serve\"]\r\n",
		},
		{
			name:   "finite exact legacy cortex region",
			before: legacy,
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.toml")
			if err := os.WriteFile(path, []byte(tt.before), 0o644); err != nil {
				t.Fatal(err)
			}

			changed, err := removeTOMLRegion(path, "cortex")
			if err != nil || !changed {
				t.Fatalf("removeTOMLRegion() = changed %v, err %v", changed, err)
			}
			after, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if got := string(after); got != tt.want {
				t.Fatalf("after = %q, want %q", got, tt.want)
			}
			changed, err = removeTOMLRegion(path, "cortex")
			if err != nil || changed {
				t.Fatalf("repeat removal = changed %v, err %v, want no-op", changed, err)
			}
		})
	}
}

func TestRemoveTOMLRegion_RefusesUnsafeCortexRegionsWithoutMutation(t *testing.T) {
	owned := ownedCortexTOML(t, "\n")
	tests := []struct {
		name   string
		before string
	}{
		{name: "customized command", before: strings.Replace(owned, `command = "cortex"`, `command = "custom"`, 1)},
		{name: "missing ownership is not legacy", before: "[mcp_servers.cortex]\ncommand = \"cortex\"\nargs = [\"mcp\", \"--tools=agent\"]\n# user note\n"},
		{name: "stale ownership", before: strings.Replace(owned, `"base_sha256":"`, `"base_sha256":"stale`, 1)},
		{name: "contradictory ownership", before: strings.Replace(owned, `"semantic_id":"mcp/codex/cortex"`, `"semantic_id":"mcp/codex/other"`, 1)},
		{name: "malformed TOML", before: "[mcp_servers.cortex\ncommand = \"cortex\"\nargs = [\"mcp\", \"--tools=agent\"]\n"},
		{name: "ambiguous descendant", before: owned + "[mcp_servers.cortex.extra]\nenabled = true\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.toml")
			before := []byte(tt.before)
			if err := os.WriteFile(path, before, 0o644); err != nil {
				t.Fatal(err)
			}
			if changed, err := removeTOMLRegion(path, "cortex"); err == nil || changed {
				t.Fatalf("removeTOMLRegion() = changed %v, err %v, want refusal", changed, err)
			}
			after, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(after) != string(before) {
				t.Fatalf("refusal mutated config:\n got %q\nwant %q", after, before)
			}
		})
	}
}

func TestRemoveTOMLRegion_AcceptsCurrentOwnershipWithResolvedCommand(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	before := ownedTOML(t, "cortex", `C:\Program Files\Cortex\cortex.exe`, []string{"mcp", "--tools=agent"}, "\n")
	if err := os.WriteFile(path, []byte(before), 0o644); err != nil {
		t.Fatal(err)
	}
	changed, err := removeTOMLRegion(path, "cortex")
	if err != nil || !changed {
		t.Fatalf("removeTOMLRegion() = changed %v, err %v", changed, err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != 0 {
		t.Fatalf("after = %q, want empty", after)
	}
}

func TestRemoveTOMLRegion_AcceptsPinnedForgeSpecCommand(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	before := ownedTOML(t, "forgespec", "npx", []string{"-y", "forgespec-mcp@1.4.0"}, "\n")
	if err := os.WriteFile(path, []byte(before), 0o644); err != nil {
		t.Fatal(err)
	}

	changed, err := removeTOMLRegion(path, "forgespec")
	if err != nil || !changed {
		t.Fatalf("removeTOMLRegion() = changed %v, err %v", changed, err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(content), "forgespec") {
		t.Fatalf("pinned ForgeSpec region remains after removal:\n%s", content)
	}
}

func TestComponentOperations_TOMLUsesTOMLRegionOperation(t *testing.T) {
	adapter := &cleanerTestAdapter{agent: model.AgentOpenCode, strategy: model.StrategyTOMLFile, settings: filepath.Join(t.TempDir(), "config.toml")}
	ops := componentOperations("", adapter, model.ComponentCortex)
	if len(ops) != 1 || ops[0].typeID != opRemoveTOMLRegion || ops[0].path != adapter.settings || ops[0].tomlServer != "cortex" {
		t.Fatalf("TOML operations = %#v, want one TOML region operation", ops)
	}
}

type cleanerTestAdapter struct {
	agent    model.AgentID
	strategy model.MCPStrategy
	settings string
}

func (a *cleanerTestAdapter) Agent() model.AgentID    { return a.agent }
func (a *cleanerTestAdapter) Tier() model.SupportTier { return model.TierFull }
func (a *cleanerTestAdapter) Detect(string) (bool, string, string, bool, error) {
	return false, "", "", false, nil
}
func (a *cleanerTestAdapter) GlobalConfigDir(string) string                     { return "" }
func (a *cleanerTestAdapter) SystemPromptDir(string) string                     { return "" }
func (a *cleanerTestAdapter) SystemPromptFile(string) string                    { return "" }
func (a *cleanerTestAdapter) SkillsDir(string) string                           { return "" }
func (a *cleanerTestAdapter) SettingsPath(string) string                        { return a.settings }
func (a *cleanerTestAdapter) SystemPromptStrategy() model.SystemPromptStrategy  { return 0 }
func (a *cleanerTestAdapter) MCPStrategy() model.MCPStrategy                    { return a.strategy }
func (a *cleanerTestAdapter) MCPConfigPath(string, string) string               { return a.settings }
func (a *cleanerTestAdapter) SupportsSkills() bool                              { return false }
func (a *cleanerTestAdapter) SupportsSystemPrompt() bool                        { return false }
func (a *cleanerTestAdapter) SupportsMCP() bool                                 { return true }
func (a *cleanerTestAdapter) SupportsSlashCommands() bool                       { return false }
func (a *cleanerTestAdapter) CommandsDir(string) string                         { return "" }
func (a *cleanerTestAdapter) SupportsTaskDelegation() bool                      { return false }
func (a *cleanerTestAdapter) SupportsSubAgents() bool                           { return false }
func (a *cleanerTestAdapter) SubAgentsDir(string) string                        { return "" }
func (a *cleanerTestAdapter) SupportsAutoInstall() bool                         { return false }
func (a *cleanerTestAdapter) InstallCommands(system.PlatformProfile) [][]string { return nil }

func ownedCortexTOML(t *testing.T, newline string) string {
	return ownedTOML(t, "cortex", "cortex", []string{"mcp", "--tools=agent"}, newline)
}

func ownedTOML(t *testing.T, server, command string, args []string, newline string) string {
	t.Helper()
	var argsTOML []string
	for _, arg := range args {
		argsTOML = append(argsTOML, fmt.Sprintf("%q", arg))
	}
	base := []byte(fmt.Sprintf("[mcp_servers.%s]\ncommand = %q\nargs = [%s]\n", server, command, strings.Join(argsTOML, ", ")))
	baseSHA := fmt.Sprintf("%x", sha256.Sum256(base))
	commandVector := append([]string{command}, args...)
	values := append([]string{"cortex-ia", "mcp/codex/" + server, "mcp_servers", server}, commandVector...)
	values = append(values, baseSHA)
	ownership := map[string]any{
		"owner":            "cortex-ia",
		"semantic_id":      "mcp/codex/" + server,
		"table_path":       []string{"mcp_servers", server},
		"command":          commandVector,
		"base_sha256":      baseSHA,
		"ownership_sha256": "",
	}
	ownership["ownership_sha256"] = fmt.Sprintf("%x", sha256.Sum256([]byte(strings.Join(values, "\x00"))))
	encoded, err := json.Marshal(ownership)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Join([]string{
		"[mcp_servers." + server + "]",
		"# cortex-ia:toml-ownership " + string(encoded),
		fmt.Sprintf("command = %q", command),
		"args = [" + strings.Join(argsTOML, ", ") + "]",
		"",
	}, newline)
}

func TestDedupeOperations(t *testing.T) {
	ops := []operation{
		{typeID: opRemoveFile, path: "/x"},
		{typeID: opRemoveFile, path: "/x"},
		{typeID: opRemoveFile, path: "/y"},
		{typeID: opRewriteFile, path: "/x", sectionID: "cortex-persona"},
		{typeID: opRewriteFile, path: "/x", sectionID: "cortex-persona"},
	}
	got := dedupeOperations(ops)
	if len(got) != 3 {
		t.Errorf("dedupe length = %d, want 3 (got %v)", len(got), got)
	}
}
