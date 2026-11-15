package targets

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testBundle(home, agentBody string) Bundle {
	return Bundle{
		Target:    TargetClaude,
		Home:      home,
		OwnedDirs: []string{filepath.Join(home, ".cortex-ia", "claude")},
		Files: []File{
			{Path: filepath.Join(home, ".claude", "agents", "a.md"), Content: []byte(agentBody)},
			{Path: filepath.Join(home, ".cortex-ia", "claude", "contracts", "c.md"), Content: []byte("contract")},
		},
		JSONSets: []JSONSet{{
			Path:  filepath.Join(home, ".claude.json"),
			Key:   []string{"mcpServers", "cortex_ia"},
			Value: map[string]any{"command": "cortex-ia"},
		}},
	}
}

func hasChangePrefix(changes []string, prefix string) bool {
	for _, c := range changes {
		if strings.HasPrefix(c, prefix) {
			return true
		}
	}
	return false
}

func readJSONMap(t *testing.T, path string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return m
}

func TestApplyCreatesThenIsIdempotent(t *testing.T) {
	home := t.TempDir()
	b := testBundle(home, "v1")

	res, err := Apply(b, ApplyOptions{})
	if err != nil {
		t.Fatalf("first apply: %v", err)
	}
	if !hasChangePrefix(res.Changes, "create .claude/agents/a.md") || !hasChangePrefix(res.Changes, "set .claude.json:mcpServers.cortex_ia") {
		t.Fatalf("first apply changes = %v", res.Changes)
	}
	if !Installed(home, TargetClaude) {
		t.Fatal("manifest missing after apply")
	}

	res, err = Apply(b, ApplyOptions{})
	if err != nil {
		t.Fatalf("second apply: %v", err)
	}
	for _, prefix := range []string{"create ", "update ", "set ", "remove ", "backup "} {
		if hasChangePrefix(res.Changes, prefix) {
			t.Errorf("idempotent re-apply reported %q in %v", prefix, res.Changes)
		}
	}
}

func TestApplyConflictBlocksAndWritesNothing(t *testing.T) {
	home := t.TempDir()
	userFile := filepath.Join(home, ".claude", "agents", "a.md")
	if err := os.MkdirAll(filepath.Dir(userFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(userFile, []byte("user written"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Apply(testBundle(home, "v1"), ApplyOptions{})
	if err == nil {
		t.Fatal("expected conflict error")
	}
	if !strings.Contains(err.Error(), ".claude/agents/a.md") {
		t.Errorf("error does not name the conflicting path: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(home, ".cortex-ia", "claude", "contracts", "c.md")); statErr == nil {
		t.Error("conflict must not write any other file")
	}
	if _, statErr := os.Stat(filepath.Join(home, ".claude.json")); statErr == nil {
		t.Error("conflict must not mutate the JSON file")
	}
	if Installed(home, TargetClaude) {
		t.Error("conflict must not write a manifest")
	}
	if raw, _ := os.ReadFile(userFile); string(raw) != "user written" {
		t.Errorf("user file changed to %q", raw)
	}
}

func TestOverwriteReplacesAndBacksUp(t *testing.T) {
	home := t.TempDir()
	userFile := filepath.Join(home, ".claude", "agents", "a.md")
	if err := os.MkdirAll(filepath.Dir(userFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(userFile, []byte("user written"), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := Apply(testBundle(home, "v1"), ApplyOptions{Overwrite: true})
	if err != nil {
		t.Fatalf("overwrite apply: %v", err)
	}
	if raw, _ := os.ReadFile(userFile); string(raw) != "v1" {
		t.Fatalf("file = %q, want v1", raw)
	}
	if !hasChangePrefix(res.Changes, "backup → .cortex-ia/targets/claude/backups/") {
		t.Errorf("no backup reported: %v", res.Changes)
	}
	backups, _ := filepath.Glob(filepath.Join(home, ".cortex-ia", "targets", "claude", "backups", "*", ".claude", "agents", "a.md"))
	if len(backups) != 1 {
		t.Fatalf("backup copies = %v", backups)
	}
	if raw, _ := os.ReadFile(backups[0]); string(raw) != "user written" {
		t.Errorf("backup = %q, want original user content", raw)
	}
}

func TestJSONSetPreservesUnrelatedKeys(t *testing.T) {
	home := t.TempDir()
	claudeJSON := filepath.Join(home, ".claude.json")
	original := `{
  "numStartups": 3,
  "mcpServers": {"other": {"command": "other"}}
}
`
	if err := os.WriteFile(claudeJSON, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Apply(testBundle(home, "v1"), ApplyOptions{}); err != nil {
		t.Fatalf("apply: %v", err)
	}
	m := readJSONMap(t, claudeJSON)
	if m["numStartups"] != float64(3) {
		t.Errorf("numStartups lost: %v", m)
	}
	servers := m["mcpServers"].(map[string]any)
	if _, ok := servers["other"]; !ok {
		t.Error("unrelated mcpServers.other lost")
	}
	if _, ok := servers["cortex_ia"]; !ok {
		t.Error("mcpServers.cortex_ia not set")
	}
}

func TestJSONSetDifferentUserValueConflicts(t *testing.T) {
	home := t.TempDir()
	claudeJSON := filepath.Join(home, ".claude.json")
	original := `{"mcpServers": {"cortex_ia": {"command": "custom"}}}`
	if err := os.WriteFile(claudeJSON, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Apply(testBundle(home, "v1"), ApplyOptions{})
	if err == nil || !strings.Contains(err.Error(), "mcpServers.cortex_ia") {
		t.Fatalf("expected JSON conflict naming the key, got %v", err)
	}
	if raw, _ := os.ReadFile(claudeJSON); string(raw) != original {
		t.Errorf("JSON changed on conflict: %s", raw)
	}
}

func TestDryRunWritesNothing(t *testing.T) {
	home := t.TempDir()
	res, err := Apply(testBundle(home, "v1"), ApplyOptions{DryRun: true})
	if err != nil {
		t.Fatalf("dry run: %v", err)
	}
	if len(res.Changes) == 0 {
		t.Error("dry run reported no changes")
	}
	entries, err := os.ReadDir(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("dry run wrote into home: %v", entries)
	}
}

func TestRejectsPathOutsideHome(t *testing.T) {
	home := t.TempDir()
	b := testBundle(home, "v1")
	b.Files = append(b.Files, File{Path: filepath.Join(home, "..", "escape.md"), Content: []byte("x")})
	if _, err := Apply(b, ApplyOptions{DryRun: true}); err == nil {
		t.Fatal("expected rejection of a path outside home")
	}
}
