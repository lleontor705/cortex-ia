package targets

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRemoveDeletesOwnedOnly(t *testing.T) {
	home := t.TempDir()
	b := testBundle(home, "v1")
	if _, err := Apply(b, ApplyOptions{}); err != nil {
		t.Fatalf("apply: %v", err)
	}
	userExtra := filepath.Join(home, ".cortex-ia", "claude", "notes.txt")
	if err := os.WriteFile(userExtra, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	modified := filepath.Join(home, ".claude", "agents", "a.md")
	if err := os.WriteFile(modified, []byte("edited by user"), 0o644); err != nil {
		t.Fatal(err)
	}
	claudeJSON := filepath.Join(home, ".claude.json")
	raw, _ := os.ReadFile(claudeJSON)
	edited := strings.Replace(string(raw), "{", "{\n  \"keep\": true,", 1)
	if err := os.WriteFile(claudeJSON, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := Remove(b, false)
	if err != nil {
		t.Fatalf("remove: %v", err)
	}
	if !hasChangePrefix(res.Changes, "kept modified file .claude/agents/a.md") {
		t.Errorf("modified file not kept: %v", res.Changes)
	}
	if _, err := os.Stat(filepath.Join(home, ".cortex-ia", "claude", "contracts", "c.md")); err == nil {
		t.Error("unmodified owned contract not removed")
	}
	if _, err := os.Stat(modified); err != nil {
		t.Error("modified file was deleted")
	}
	if _, err := os.Stat(userExtra); err != nil {
		t.Error("user file in owned dir was deleted")
	}
	if Installed(home, TargetClaude) {
		t.Error("manifest still present after remove")
	}
	m := readJSONMap(t, claudeJSON)
	if m["keep"] != true {
		t.Errorf("unrelated JSON key lost: %v", m)
	}
	if _, ok := m["mcpServers"]; ok {
		t.Errorf("empty mcpServers should be gone: %v", m)
	}
}

func TestRemoveNotInstalled(t *testing.T) {
	res, err := Remove(testBundle(t.TempDir(), "v1"), false)
	if err != nil {
		t.Fatalf("remove on fresh home returned error: %v", err)
	}
	if !strings.Contains(res.Message, "not installed") {
		t.Errorf("message = %q", res.Message)
	}
}
