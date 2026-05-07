package uninstall

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
