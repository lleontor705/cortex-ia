package backup

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestListManifests_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	result := ListManifests(dir)
	if len(result.Manifests) != 0 {
		t.Errorf("expected 0 manifests for empty dir, got %d", len(result.Manifests))
	}
	if len(result.Warnings) != 0 {
		t.Errorf("expected 0 warnings for empty dir, got %d", len(result.Warnings))
	}
}

func TestListManifests_NonexistentDir(t *testing.T) {
	result := ListManifests(filepath.Join(t.TempDir(), "does-not-exist"))
	if result.Manifests != nil {
		t.Errorf("expected nil manifests for non-existent dir, got %v", result.Manifests)
	}
}

func TestListManifests_WithManifests(t *testing.T) {
	dir := t.TempDir()

	// Create two valid backup subdirs with manifest.json
	for _, id := range []string{"backup-001", "backup-002"} {
		backupDir := filepath.Join(dir, id)
		if err := os.MkdirAll(backupDir, 0o755); err != nil {
			t.Fatal(err)
		}
		m := Manifest{
			ID:        id,
			CreatedAt: time.Now(),
			RootDir:   backupDir,
			Source:    BackupSourceInstall,
			FileCount: 1,
		}
		if err := WriteManifest(filepath.Join(backupDir, ManifestFilename), m); err != nil {
			t.Fatal(err)
		}
	}

	result := ListManifests(dir)
	if len(result.Manifests) != 2 {
		t.Errorf("expected 2 manifests, got %d", len(result.Manifests))
	}
	if len(result.Warnings) != 0 {
		t.Errorf("expected 0 warnings, got %d: %v", len(result.Warnings), result.Warnings)
	}
}

func TestListManifests_SkipsInvalidWithWarnings(t *testing.T) {
	dir := t.TempDir()

	// Valid backup
	validDir := filepath.Join(dir, "valid-backup")
	if err := os.MkdirAll(validDir, 0o755); err != nil {
		t.Fatal(err)
	}
	m := Manifest{
		ID:        "valid",
		CreatedAt: time.Now(),
		RootDir:   validDir,
		Source:    BackupSourceSync,
		FileCount: 2,
	}
	if err := WriteManifest(filepath.Join(validDir, ManifestFilename), m); err != nil {
		t.Fatal(err)
	}

	// Invalid backup (bad JSON)
	invalidDir := filepath.Join(dir, "invalid-backup")
	if err := os.MkdirAll(invalidDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(invalidDir, ManifestFilename), []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Dir without manifest
	emptyDir := filepath.Join(dir, "no-manifest")
	if err := os.MkdirAll(emptyDir, 0o755); err != nil {
		t.Fatal(err)
	}

	result := ListManifests(dir)
	if len(result.Manifests) != 1 {
		t.Errorf("expected 1 valid manifest, got %d", len(result.Manifests))
	}
	if len(result.Manifests) > 0 && result.Manifests[0].ID != "valid" {
		t.Errorf("expected manifest ID 'valid', got %q", result.Manifests[0].ID)
	}
	if len(result.Warnings) != 2 {
		t.Errorf("expected 2 warnings (missing + unreadable), got %d: %v", len(result.Warnings), result.Warnings)
	}
}
