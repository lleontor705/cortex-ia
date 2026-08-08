package cortex

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExportAndImportProjectMemory(t *testing.T) {
	tmpDir := t.TempDir()

	err := ExportProjectMemory(tmpDir, "test-project")
	if err != nil {
		t.Fatalf("ExportProjectMemory failed: %v", err)
	}

	manifestPath := filepath.Join(tmpDir, ".cortex", "manifest.json")
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		t.Fatalf("expected .cortex/manifest.json to exist")
	}

	manifest, err := ImportProjectMemory(tmpDir)
	if err != nil {
		t.Fatalf("ImportProjectMemory failed: %v", err)
	}

	if manifest.Project != "test-project" {
		t.Errorf("expected project test-project, got %s", manifest.Project)
	}
}
