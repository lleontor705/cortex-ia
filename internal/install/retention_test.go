package install

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lleontor705/cortex-ia/v2/internal/backup"
	"github.com/lleontor705/cortex-ia/v2/internal/pipeline"
)

// seedBackups creates count backup directories oldest first, each a real
// verified snapshot manifest, and returns their IDs in creation order.
func seedBackups(t *testing.T, home string, count int) []string {
	t.Helper()
	root := filepath.Join(home, ".cortex-ia", "backups")
	ids := make([]string, 0, count)
	for i := 0; i < count; i++ {
		dir := filepath.Join(root, "seed-"+string(rune('a'+i)))
		if _, err := backup.NewSnapshotter().Create(dir, nil); err != nil {
			t.Fatalf("seed backup %d: %v", i, err)
		}
		ids = append(ids, filepath.Base(dir))
		time.Sleep(2 * time.Millisecond)
	}
	return ids
}

func backupExists(home, id string) bool {
	_, err := os.Stat(filepath.Join(home, ".cortex-ia", "backups", id))
	return err == nil
}

// A backup whose journal is still incomplete is never pruned, even when it is
// the oldest, and it does not consume a retention slot.
func TestPruneBackupsProtectsPendingJournal(t *testing.T) {
	home := t.TempDir()
	ids := seedBackups(t, home, backup.DefaultRetentionCount+3)
	oldest := ids[0]
	journalRoot := filepath.Join(home, ".cortex-ia", "backups", oldest, "journal")
	target := pipeline.ManagedTarget{Path: ".cortex-ia/pending.json", Kind: pipeline.TargetFile, Owner: "test"}
	if _, err := pipeline.BeginInstallJournal(home, journalRoot, []pipeline.ManagedTarget{target}); err != nil {
		t.Fatalf("begin pending journal: %v", err)
	}

	pruneBackups(home)

	if !backupExists(home, oldest) {
		t.Fatal("backup referenced by an incomplete journal was pruned")
	}
	if got, want := countBackups(t, home), 1+backup.DefaultRetentionCount; got != want {
		t.Fatalf("retained %d backups, want %d (protected + newest %d)", got, want, backup.DefaultRetentionCount)
	}
}

// A committed install prunes older backups while holding the home lock, so
// the directory stops growing without bound.
func TestInstallCommitRunsRetention(t *testing.T) {
	cleanup := SetWindowsEnvRunnerForTesting(func(cmd string, args ...string) ([]byte, error) {
		return []byte("ok"), nil
	})
	defer cleanup()

	home := t.TempDir()
	seedBackups(t, home, backup.DefaultRetentionCount+2)
	svc, err := New(home)
	if err != nil {
		t.Fatal(err)
	}
	opts := DefaultOptions()
	opts.Cortex = false
	receipt, err := svc.Install(opts)
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if !backupExists(home, receipt.BackupID) {
		t.Fatal("the committed install's own backup must survive retention")
	}
	if got, want := countBackups(t, home), 1+backup.DefaultRetentionCount; got != want {
		t.Fatalf("retained %d backups after commit, want %d (live backup + newest %d)", got, want, backup.DefaultRetentionCount)
	}
}

func countBackups(t *testing.T, home string) int {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(home, ".cortex-ia", "backups"))
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, e := range entries {
		if e.IsDir() {
			n++
		}
	}
	return n
}
