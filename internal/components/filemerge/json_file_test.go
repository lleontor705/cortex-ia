package filemerge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMutateJSONFilePreservesJSONCTriviaAndIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "opencode.jsonc")
	before := []byte("{\n  // Keep this explanation.\n  \"theme\": \"dark\",\n  \"agent\": {\n    \"implement\": { \"mode\": \"subagent\" },\n  },\n}\n")
	if err := os.WriteFile(path, before, 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := MutateJSONFile(path, JSONMutation{Overlay: []byte(`{"agent":{"implement":{"model":"provider/model"}}}`)})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed || result.Created || string(result.Before) != string(before) {
		t.Fatalf("mutation result = %+v", result)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range []string{"// Keep this explanation.", `"theme": "dark",`, `"mode": "subagent"`, `"model": "provider/model"`, "},\n}"} {
		if !strings.Contains(string(after), marker) {
			t.Errorf("patched JSONC missing %q:\n%s", marker, after)
		}
	}

	second, err := MutateJSONFile(path, JSONMutation{Overlay: []byte(`{"agent":{"implement":{"model":"provider/model"}}}`)})
	if err != nil {
		t.Fatal(err)
	}
	if second.Changed {
		t.Fatal("identical JSONC mutation rewrote the file")
	}
}

func TestMutateJSONFilePreservesExistingMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "opencode.jsonc")
	if err := os.WriteFile(path, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := MutateJSONFile(path, JSONMutation{Overlay: []byte(`{"share":"disabled"}`)}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := info.Mode().Perm(), before.Mode().Perm(); got != want {
		t.Fatalf("mode = %04o, want existing %04o", got, want)
	}
}

func TestMutateJSONFileRemovesPathsAndPreservesSiblings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "opencode.jsonc")
	before := []byte("{\n  \"mcp\": {\n    // Managed entry.\n    \"cortex\": { \"enabled\": true },\n    \"user\": { \"enabled\": true },\n  },\n}\n")
	if err := os.WriteFile(path, before, 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := MutateJSONFile(path, JSONMutation{RemovePaths: [][]string{{"mcp", "cortex"}}})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed {
		t.Fatal("JSONC removal was a no-op")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(after), `"cortex"`) || !strings.Contains(string(after), `"user"`) {
		t.Fatalf("JSONC removal changed the wrong members:\n%s", after)
	}
}

func TestMutateJSONFileRejectsInvalidBaseWithoutMutation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "opencode.jsonc")
	before := []byte(`{"agent":`)
	if err := os.WriteFile(path, before, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := MutateJSONFile(path, JSONMutation{Overlay: []byte(`{"agent":{}}`)}); err == nil {
		t.Fatal("invalid JSONC base unexpectedly accepted")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatalf("invalid base was mutated: %q", after)
	}
}

func TestMutateJSONFileRejectsInvalidOverlayWithoutCreation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "opencode.jsonc")
	for _, overlay := range [][]byte{
		[]byte(`{"agent":`),
		[]byte(`[{"agent":{}}]`),
		[]byte("{\n  // overlays are generated strict JSON\n  \"agent\": {},\n}"),
		[]byte(`{"agent":{},"agent":{"implement":{}}}`),
	} {
		if _, err := MutateJSONFile(path, JSONMutation{Overlay: overlay}); err == nil {
			t.Errorf("invalid overlay unexpectedly accepted: %s", overlay)
		}
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("invalid overlay created %q", path)
		}
	}
}

func TestMutateJSONFileSupportsReplaceSentinel(t *testing.T) {
	path := filepath.Join(t.TempDir(), "opencode.jsonc")
	before := []byte("{\n  \"permission\": {\n    \"read\": { \"*\": \"allow\", \"secret\": \"deny\" },\n  },\n}\n")
	if err := os.WriteFile(path, before, 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := MutateJSONFile(path, JSONMutation{Overlay: []byte(`{"permission":{"read":{"__replace__":{"*":"ask"}}}}`)})
	if err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(after), `"secret"`) || !strings.Contains(string(after), `"*":"ask"`) {
		t.Fatalf("replace sentinel was not atomic:\n%s", after)
	}
}
