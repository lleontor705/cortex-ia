package pipeline

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/backup"
	"github.com/lleontor705/cortex-ia/internal/state"
)

func TestRollbackUsesLastBackupIDFromLockfile(t *testing.T) {
	homeDir := t.TempDir()
	targetFile := filepath.Join(homeDir, ".codex", "AGENTS.md")
	if err := os.MkdirAll(filepath.Dir(targetFile), 0o755); err != nil {
		t.Fatalf("mkdir target dir: %v", err)
	}
	if err := os.WriteFile(targetFile, []byte("modified"), 0o644); err != nil {
		t.Fatalf("write target file: %v", err)
	}

	backupID := "backup-001"
	backupDir := filepath.Join(homeDir, ".cortex-ia", "backups", backupID)
	snapshotPath := filepath.Join(backupDir, "files", ".codex", "AGENTS.md")
	if err := os.MkdirAll(filepath.Dir(snapshotPath), 0o755); err != nil {
		t.Fatalf("mkdir snapshot dir: %v", err)
	}
	if err := os.WriteFile(snapshotPath, []byte("original"), 0o644); err != nil {
		t.Fatalf("write snapshot file: %v", err)
	}

	manifest := backup.Manifest{
		ID:       backupID,
		RootDir:  backupDir,
		FileCount: 1,
		Entries: []backup.ManifestEntry{
			{
				OriginalPath: targetFile,
				SnapshotPath: snapshotPath,
				Existed:      true,
				Mode:         0o644,
			},
		},
	}
	if err := backup.WriteManifest(filepath.Join(backupDir, backup.ManifestFilename), manifest); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	if err := state.SaveLock(homeDir, state.Lockfile{LastBackupID: backupID}); err != nil {
		t.Fatalf("save lock: %v", err)
	}

	got, err := Rollback(homeDir, "")
	if err != nil {
		t.Fatalf("Rollback() error = %v", err)
	}
	if got.ID != backupID {
		t.Fatalf("Rollback() backup = %q, want %q", got.ID, backupID)
	}

	content, err := os.ReadFile(targetFile)
	if err != nil {
		t.Fatalf("read restored file: %v", err)
	}
	if string(content) != "original" {
		t.Fatalf("restored content = %q, want %q", string(content), "original")
	}
}

func TestRollbackRequiresBackupMetadata(t *testing.T) {
	_, err := Rollback(t.TempDir(), "")
	if err == nil {
		t.Fatal("Rollback() expected error, got nil")
	}
}
