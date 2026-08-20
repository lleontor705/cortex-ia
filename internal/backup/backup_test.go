package backup

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSnapshotAndRestore(t *testing.T) {
	// Setup: create source files
	srcDir := t.TempDir()
	file1 := filepath.Join(srcDir, "config.json")
	file2 := filepath.Join(srcDir, "settings.md")
	nonExistent := filepath.Join(srcDir, "missing.txt")

	if err := os.WriteFile(file1, []byte(`{"key": "value"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file2, []byte("# Settings\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create snapshot
	backupDir := filepath.Join(t.TempDir(), "backup-001")
	snap := NewSnapshotter()
	manifest, err := snap.Create(backupDir, []string{file1, file2, nonExistent})
	if err != nil {
		t.Fatal(err)
	}

	if manifest.FileCount != 2 {
		t.Errorf("FileCount = %d, want 2", manifest.FileCount)
	}
	if len(manifest.Entries) != 3 {
		t.Errorf("Entries = %d, want 3", len(manifest.Entries))
	}

	// Verify non-existent entry
	for _, e := range manifest.Entries {
		if e.OriginalPath == filepath.Clean(nonExistent) && e.Existed {
			t.Error("expected Existed=false for missing file")
		}
	}

	// Modify source files
	if err := os.WriteFile(file1, []byte("modified"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Restore
	svc := RestoreService{}
	if err := svc.Restore(manifest); err != nil {
		t.Fatal(err)
	}

	// Verify restoration
	content, err := os.ReadFile(file1)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != `{"key": "value"}` {
		t.Errorf("restored content = %s, want original", content)
	}
}

func TestManifestReadWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")

	manifest := Manifest{
		ID:               "test-001",
		CreatedAt:        time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		RootDir:          dir,
		Source:           BackupSourceInstall,
		Description:      "test backup",
		FileCount:        3,
		CreatedByVersion: "dev",
		Entries: []ManifestEntry{
			{OriginalPath: "/home/user/.claude/CLAUDE.md", SnapshotPath: "/backup/file1", Existed: true, Mode: 0o644},
		},
	}

	if err := WriteManifest(path, manifest); err != nil {
		t.Fatal(err)
	}

	read, err := ReadManifest(path)
	if err != nil {
		t.Fatal(err)
	}

	if read.ID != "test-001" {
		t.Errorf("ID = %s", read.ID)
	}
	if read.Source != BackupSourceInstall {
		t.Errorf("Source = %s", read.Source)
	}
	if read.FileCount != 3 {
		t.Errorf("FileCount = %d", read.FileCount)
	}
	if len(read.Entries) != 1 {
		t.Errorf("Entries = %d", len(read.Entries))
	}
}

func TestManifestDisplayLabel(t *testing.T) {
	m := Manifest{
		Source:    BackupSourceInstall,
		CreatedAt: time.Date(2026, 3, 15, 14, 30, 0, 0, time.UTC),
		FileCount: 5,
	}

	label := m.DisplayLabel()
	if label == "" {
		t.Error("expected non-empty label")
	}
}

func TestDeleteBackup(t *testing.T) {
	dir := t.TempDir()
	backupDir := filepath.Join(dir, "backup-to-delete")
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backupDir, "file.txt"), []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := Manifest{RootDir: backupDir}
	if err := DeleteBackup(m); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(backupDir); !os.IsNotExist(err) {
		t.Error("backup directory should be deleted")
	}
}
