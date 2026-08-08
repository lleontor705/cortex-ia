package pipeline

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestJournalCapturesAbsentEmptyAndDirectoryPreimages(t *testing.T) {
	root := t.TempDir()
	empty := filepath.Join(root, "empty")
	if err := os.WriteFile(empty, nil, 0o640); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "existing-dir")
	if err := os.Mkdir(dir, 0o750); err != nil {
		t.Fatal(err)
	}

	journal, err := BeginInstallJournal(root, filepath.Join(root, ".journal"), []ManagedTarget{
		{Path: "absent", Kind: TargetFile},
		{Path: "empty", Kind: TargetFile},
		{Path: "existing-dir", Kind: TargetDirectory},
	})
	if err != nil {
		t.Fatalf("BeginInstallJournal() error = %v", err)
	}
	if got := journal.Entries[0].Presence; got != PresenceAbsent {
		t.Fatalf("absent presence = %q, want %q", got, PresenceAbsent)
	}
	if got := journal.Entries[1]; got.Presence != PresenceRegularFile || got.Size != 0 || got.SHA256 != journalSHA256(nil) || got.Mode.Perm() != mustMode(t, empty) || got.SnapshotPath == "" {
		t.Fatalf("empty preimage = %+v, want regular zero-byte snapshot", got)
	}
	if got := journal.Entries[2]; got.Presence != PresenceDirectory || got.Mode.Perm() != mustMode(t, dir) {
		t.Fatalf("directory preimage = %+v", got)
	}
	if _, err := os.Stat(journal.CheckpointPath); err != nil {
		t.Fatalf("durable checkpoint missing: %v", err)
	}
}

func TestJournalRestoreRemovesAbsentTargetsAndOnlyEmptyCreatedDirectories(t *testing.T) {
	root := t.TempDir()
	journal, err := BeginInstallJournal(root, filepath.Join(root, ".journal"), []ManagedTarget{{Path: "generated/config", Kind: TargetFile}})
	if err != nil {
		t.Fatal(err)
	}
	created := filepath.Join(root, "generated")
	if err := os.MkdirAll(created, 0o700); err != nil {
		t.Fatal(err)
	}
	generated := filepath.Join(created, "config")
	if err := os.WriteFile(generated, []byte("after"), 0o600); err != nil {
		t.Fatal(err)
	}
	after, err := inspectPath(generated, "generated/config")
	if err != nil {
		t.Fatal(err)
	}
	after.CreatedDirs = []string{"generated"}
	if err := journal.Record(after); err != nil {
		t.Fatal(err)
	}
	if err := journal.RestoreAndVerify(); err != nil {
		t.Fatalf("RestoreAndVerify() error = %v", err)
	}
	if _, err := os.Stat(generated); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("generated file exists after restore: %v", err)
	}
	if _, err := os.Stat(created); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("empty created directory exists after restore: %v", err)
	}
	if err := journal.RestoreAndVerify(); err != nil {
		t.Fatalf("repeat RestoreAndVerify() error = %v", err)
	}
}

func TestJournalRestoreConflictPreflightDoesNotPartiallyRestore(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "first")
	second := filepath.Join(root, "second")
	if err := os.WriteFile(first, []byte("before-first"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(second, []byte("before-second"), 0o600); err != nil {
		t.Fatal(err)
	}
	journal, err := BeginInstallJournal(root, filepath.Join(root, ".journal"), []ManagedTarget{{Path: "first", Kind: TargetFile}, {Path: "second", Kind: TargetFile}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(first, []byte("after-first"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(second, []byte("after-second"), 0o600); err != nil {
		t.Fatal(err)
	}
	firstAfter, err := inspectPath(first, "first")
	if err != nil {
		t.Fatal(err)
	}
	if err := journal.Record(firstAfter); err != nil {
		t.Fatal(err)
	}
	secondAfter, err := inspectPath(second, "second")
	if err != nil {
		t.Fatal(err)
	}
	if err := journal.Record(secondAfter); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(second, []byte("user change"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := journal.RestoreAndVerify(); !errors.Is(err, ErrJournalConflict) {
		t.Fatalf("RestoreAndVerify() error = %v, want ErrJournalConflict", err)
	}
	got, err := os.ReadFile(first)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "after-first" {
		t.Fatalf("first changed despite conflict = %q", got)
	}
}

func TestJournalReloadsDurableCheckpointForRetry(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "config")
	if err := os.WriteFile(path, []byte("before"), 0o600); err != nil {
		t.Fatal(err)
	}
	journal, err := BeginInstallJournal(root, filepath.Join(root, ".journal"), []ManagedTarget{{Path: "config", Kind: TargetFile}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("after"), 0o600); err != nil {
		t.Fatal(err)
	}
	after, err := inspectPath(path, "config")
	if err != nil {
		t.Fatal(err)
	}
	if err := journal.Record(after); err != nil {
		t.Fatal(err)
	}
	reloaded, err := LoadInstallJournal(journal.CheckpointPath)
	if err != nil {
		t.Fatalf("LoadInstallJournal() error = %v", err)
	}
	if err := reloaded.RestoreAndVerify(); err != nil {
		t.Fatalf("reloaded RestoreAndVerify() error = %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "before" {
		t.Fatalf("restored bytes = %q, want before", got)
	}
}

func mustMode(t *testing.T, path string) fs.FileMode {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info.Mode().Perm()
}

func TestJournalCheckpointPreservesFirstDurabilityErrorAndCleansTemporary(t *testing.T) {
	writeFailure := errors.New("write failed")
	syncFailure := errors.New("sync failed")
	closeFailure := errors.New("close failed")
	renameFailure := errors.New("rename failed")
	tests := []struct {
		name      string
		writeErr  error
		syncErr   error
		closeErr  error
		renameErr error
		wantErr   error
		wantOps   []string
	}{
		{
			name:     "write failure skips sync but closes",
			writeErr: writeFailure,
			closeErr: closeFailure,
			wantErr:  writeFailure,
			wantOps:  []string{"write", "close", "remove"},
		},
		{
			name:     "sync failure wins over close",
			syncErr:  syncFailure,
			closeErr: closeFailure,
			wantErr:  syncFailure,
			wantOps:  []string{"write", "sync", "close", "remove"},
		},
		{
			name:     "close failure follows successful write and sync",
			closeErr: closeFailure,
			wantErr:  closeFailure,
			wantOps:  []string{"write", "sync", "close", "remove"},
		},
		{
			name:    "rename follows successful write sync and close",
			wantOps: []string{"write", "sync", "close", "rename"},
		},
		{
			name:      "rename failure cleans temporary after close",
			renameErr: renameFailure,
			wantErr:   renameFailure,
			wantOps:   []string{"write", "sync", "close", "rename", "remove"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var operations []string
			file := &checkpointTestFile{operations: &operations, writeErr: tt.writeErr, syncErr: tt.syncErr, closeErr: tt.closeErr}
			withJournalCheckpointFileOps(t, file, tt.renameErr, &operations)

			journal := &InstallJournal{CheckpointPath: filepath.Join(t.TempDir(), "journal.json")}
			err := journal.checkpoint()
			if tt.wantErr == nil && err != nil {
				t.Fatalf("checkpoint() error = %v, want nil", err)
			}
			if tt.wantErr != nil && (err == nil || !errors.Is(err, tt.wantErr)) {
				t.Fatalf("checkpoint() error = %v, want %v", err, tt.wantErr)
			}
			if got := operations; !equalStrings(got, tt.wantOps) {
				t.Fatalf("operations = %v, want %v", got, tt.wantOps)
			}
		})
	}
}

type checkpointTestFile struct {
	operations *[]string
	writeErr   error
	syncErr    error
	closeErr   error
}

func (f *checkpointTestFile) Write(p []byte) (int, error) {
	*f.operations = append(*f.operations, "write")
	if f.writeErr != nil {
		return 0, f.writeErr
	}
	return len(p), nil
}

func (f *checkpointTestFile) Sync() error {
	*f.operations = append(*f.operations, "sync")
	return f.syncErr
}

func (f *checkpointTestFile) Close() error {
	*f.operations = append(*f.operations, "close")
	return f.closeErr
}

func withJournalCheckpointFileOps(t *testing.T, file journalCheckpointFile, renameErr error, operations *[]string) {
	t.Helper()
	oldOpen, oldRemove, oldRename := openJournalCheckpoint, removeJournalCheckpoint, renameJournalCheckpoint
	openJournalCheckpoint = func(string, int, fs.FileMode) (journalCheckpointFile, error) {
		return file, nil
	}
	removeJournalCheckpoint = func(string) error {
		*operations = append(*operations, "remove")
		return nil
	}
	renameJournalCheckpoint = func(string, string) error {
		*operations = append(*operations, "rename")
		return renameErr
	}
	t.Cleanup(func() {
		openJournalCheckpoint, removeJournalCheckpoint, renameJournalCheckpoint = oldOpen, oldRemove, oldRename
	})
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
