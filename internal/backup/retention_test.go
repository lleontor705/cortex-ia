package backup

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Prune tests
// ---------------------------------------------------------------------------

// makeBackupAt creates a backup directory with the given ID and timestamp.
// Returns the Manifest that was written.
func makeBackupAt(t *testing.T, backupDir, id string, createdAt time.Time, pinned bool) Manifest {
	t.Helper()
	dir := filepath.Join(backupDir, id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll %s: %v", dir, err)
	}

	m := Manifest{
		ID:        id,
		CreatedAt: createdAt,
		RootDir:   dir,
		Pinned:    pinned,
		Entries:   []ManifestEntry{},
	}

	// Write manifest manually using JSON to handle the Pinned field
	// regardless of whether the struct field exists yet in manifest.go.
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent: %v", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(filepath.Join(dir, ManifestFilename), data, 0o644); err != nil {
		t.Fatalf("WriteFile manifest: %v", err)
	}
	return m
}

// TestPrune_DeletesOldestUnpinned verifies that when there are more unpinned
// backups than the retention count, the oldest ones are deleted.
func TestPrune_DeletesOldestUnpinned(t *testing.T) {
	backupDir := t.TempDir()
	origBackupRootFn := BackupRootFn
	t.Cleanup(func() { BackupRootFn = origBackupRootFn })
	BackupRootFn = func() (string, error) { return backupDir, nil }

	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 1; i <= 6; i++ {
		makeBackupAt(t, backupDir, fmt.Sprintf("backup-%02d", i), base.Add(time.Duration(i)*time.Hour), false)
	}

	deleted, err := Prune(backupDir, 5)
	if err != nil {
		t.Fatalf("Prune() error = %v", err)
	}

	// Should have deleted exactly 1 (oldest: backup-01).
	if len(deleted) != 1 {
		t.Errorf("Prune() deleted %d backups, want 1", len(deleted))
	}
	if len(deleted) == 1 && deleted[0] != "backup-01" {
		t.Errorf("Prune() deleted %q, want %q (oldest backup)", deleted[0], "backup-01")
	}

	// backup-01 directory must be gone.
	if _, err := os.Stat(filepath.Join(backupDir, "backup-01")); !os.IsNotExist(err) {
		t.Error("backup-01 directory still exists after Prune")
	}
	// backup-02 must still be there (inside the retention window).
	if _, err := os.Stat(filepath.Join(backupDir, "backup-02")); os.IsNotExist(err) {
		t.Error("backup-02 was unexpectedly deleted by Prune")
	}
}

// TestPrune_SkipsPinnedBackups verifies that pinned backups are never deleted
// even when total unpinned count exceeds the limit.
func TestPrune_SkipsPinnedBackups(t *testing.T) {
	backupDir := t.TempDir()
	origBackupRootFn := BackupRootFn
	t.Cleanup(func() { BackupRootFn = origBackupRootFn })
	BackupRootFn = func() (string, error) { return backupDir, nil }

	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	// 4 unpinned + 2 pinned = 6 backups, retentionCount = 3.
	makeBackupAt(t, backupDir, "unpinned-01", base.Add(1*time.Hour), false)
	makeBackupAt(t, backupDir, "unpinned-02", base.Add(2*time.Hour), false)
	makeBackupAt(t, backupDir, "unpinned-03", base.Add(3*time.Hour), false)
	makeBackupAt(t, backupDir, "unpinned-04", base.Add(4*time.Hour), false)
	makeBackupAt(t, backupDir, "pinned-01", base.Add(5*time.Hour), true)
	makeBackupAt(t, backupDir, "pinned-02", base.Add(6*time.Hour), true)

	deleted, err := Prune(backupDir, 3)
	if err != nil {
		t.Fatalf("Prune() error = %v", err)
	}

	// 4 unpinned - keep 3 = delete 1 (the oldest unpinned: unpinned-01).
	if len(deleted) != 1 {
		t.Errorf("Prune() deleted %d backups, want 1", len(deleted))
	}

	// Pinned backups must still exist.
	if _, err := os.Stat(filepath.Join(backupDir, "pinned-01")); os.IsNotExist(err) {
		t.Error("pinned-01 was deleted by Prune — pinned backups must be preserved")
	}
	if _, err := os.Stat(filepath.Join(backupDir, "pinned-02")); os.IsNotExist(err) {
		t.Error("pinned-02 was deleted by Prune — pinned backups must be preserved")
	}
}

// TestPrune_ZeroRetention verifies that Prune does nothing when retentionCount
// is 0 (unlimited mode).
func TestPrune_ZeroRetention(t *testing.T) {
	backupDir := t.TempDir()

	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 1; i <= 10; i++ {
		makeBackupAt(t, backupDir, fmt.Sprintf("backup-%02d", i), base.Add(time.Duration(i)*time.Hour), false)
	}

	deleted, err := Prune(backupDir, 0)
	if err != nil {
		t.Fatalf("Prune() error = %v", err)
	}

	if len(deleted) != 0 {
		t.Errorf("Prune() deleted %d backups, want 0 for retentionCount=0 (unlimited)", len(deleted))
	}
}

// TestPrune_FewerThanRetention verifies that nothing is deleted when the number
// of unpinned backups is already at or below the retention limit.
func TestPrune_FewerThanRetention(t *testing.T) {
	backupDir := t.TempDir()

	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	makeBackupAt(t, backupDir, "backup-01", base.Add(1*time.Hour), false)
	makeBackupAt(t, backupDir, "backup-02", base.Add(2*time.Hour), false)
	makeBackupAt(t, backupDir, "backup-03", base.Add(3*time.Hour), false)

	deleted, err := Prune(backupDir, 5)
	if err != nil {
		t.Fatalf("Prune() error = %v", err)
	}

	if len(deleted) != 0 {
		t.Errorf("Prune() deleted %d backups, want 0 when count (%d) < retention (%d)", len(deleted), 3, 5)
	}
}

// TestPrune_AllPinned verifies that when all backups are pinned, nothing is
// deleted even when the total exceeds retentionCount.
func TestPrune_AllPinned(t *testing.T) {
	backupDir := t.TempDir()

	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 1; i <= 7; i++ {
		makeBackupAt(t, backupDir, fmt.Sprintf("pinned-%02d", i), base.Add(time.Duration(i)*time.Hour), true)
	}

	deleted, err := Prune(backupDir, 5)
	if err != nil {
		t.Fatalf("Prune() error = %v", err)
	}

	if len(deleted) != 0 {
		t.Errorf("Prune() deleted %d backups, want 0 when all backups are pinned", len(deleted))
	}

	// Spot-check that a pinned backup still exists.
	if _, err := os.Stat(filepath.Join(backupDir, "pinned-01")); os.IsNotExist(err) {
		t.Error("pinned-01 was deleted by Prune — must not delete pinned backups")
	}
}

// TestPrune_ReturnsDeletedIDs verifies that the returned slice contains the
// IDs (backup directory names) of all deleted backups in order.
func TestPrune_ReturnsDeletedIDs(t *testing.T) {
	backupDir := t.TempDir()
	origBackupRootFn := BackupRootFn
	t.Cleanup(func() { BackupRootFn = origBackupRootFn })
	BackupRootFn = func() (string, error) { return backupDir, nil }

	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	// Create 5 unpinned, keep 3 → expect 2 deleted (oldest: backup-01, backup-02).
	makeBackupAt(t, backupDir, "backup-01", base.Add(1*time.Hour), false)
	makeBackupAt(t, backupDir, "backup-02", base.Add(2*time.Hour), false)
	makeBackupAt(t, backupDir, "backup-03", base.Add(3*time.Hour), false)
	makeBackupAt(t, backupDir, "backup-04", base.Add(4*time.Hour), false)
	makeBackupAt(t, backupDir, "backup-05", base.Add(5*time.Hour), false)

	deleted, err := Prune(backupDir, 3)
	if err != nil {
		t.Fatalf("Prune() error = %v", err)
	}

	if len(deleted) != 2 {
		t.Fatalf("Prune() returned %d IDs, want 2: %v", len(deleted), deleted)
	}

	wantDeleted := map[string]bool{"backup-01": true, "backup-02": true}
	for _, id := range deleted {
		if !wantDeleted[id] {
			t.Errorf("Prune() deleted unexpected backup %q", id)
		}
	}

	// Verify both are gone from disk.
	for id := range wantDeleted {
		if _, err := os.Stat(filepath.Join(backupDir, id)); !os.IsNotExist(err) {
			t.Errorf("backup %q still exists on disk after Prune", id)
		}
	}
}
