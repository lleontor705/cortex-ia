package filemerge

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDecodeJSONObject(t *testing.T) {
	// Valid JSONC with comments and trailing comma
	jsonc := []byte(`{
		// Comment here
		"name": "test",
		"items": [1, 2, 3],
	}`)

	obj, err := DecodeJSONObject(jsonc)
	if err != nil {
		t.Fatalf("DecodeJSONObject failed: %v", err)
	}
	if obj["name"] != "test" {
		t.Errorf("expected name 'test', got %v", obj["name"])
	}

	// Empty input
	emptyObj, err := DecodeJSONObject([]byte(""))
	if err != nil {
		t.Fatalf("DecodeJSONObject on empty failed: %v", err)
	}
	if len(emptyObj) != 0 {
		t.Errorf("expected empty map, got: %+v", emptyObj)
	}

	// Invalid input (non-object)
	_, err = DecodeJSONObject([]byte(`[1, 2, 3]`))
	if err == nil {
		t.Error("expected error for JSON array root")
	}
}

func TestMutateJSONFile(t *testing.T) {
	tempDir := t.TempDir()
	jsonPath := filepath.Join(tempDir, "config.jsonc")

	// 1. Create file via MutateJSONFile
	mutation := JSONMutation{
		Overlay: []byte(`{"version": "1.0", "mcp": {"cortex": {"enabled": true}}}`),
	}
	res, err := MutateJSONFile(jsonPath, mutation)
	if err != nil {
		t.Fatalf("MutateJSONFile create failed: %v", err)
	}
	if !res.Created {
		t.Error("expected Created true on fresh file")
	}

	// 2. Mutate existing file (remove and overlay)
	mutation2 := JSONMutation{
		Overlay:     []byte(`{"mcp": {"cortex": {"enabled": false}, "new_tool": {"enabled": true}}}`),
		RemovePaths: [][]string{{"version"}},
	}
	res2, err := MutateJSONFile(jsonPath, mutation2)
	if err != nil {
		t.Fatalf("MutateJSONFile update failed: %v", err)
	}
	if !res2.Changed {
		t.Error("expected Changed true on mutation")
	}

	// Verify file content
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	parsed, err := DecodeJSONObject(data)
	if err != nil {
		t.Fatalf("DecodeJSONObject after mutation failed: %v", err)
	}
	if _, exists := parsed["version"]; exists {
		t.Error("expected 'version' to be removed")
	}
}
