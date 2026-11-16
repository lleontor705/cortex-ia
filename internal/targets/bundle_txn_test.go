package targets

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lleontor705/cortex-ia/internal/homelock"
)

func txnBundle(home string, files map[string]string) Bundle {
	b := Bundle{Target: TargetClaude, Home: home}
	for rel, body := range files {
		b.Files = append(b.Files, File{Path: filepath.Join(home, filepath.FromSlash(rel)), Content: []byte(body)})
	}
	return b
}

func txnRead(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

// A failure after the writes restores every earlier file, removes what the run
// created, and leaves the previous manifest byte-for-byte in place.
func TestApplyFailureRestoresEarlierFilesAndManifest(t *testing.T) {
	home := t.TempDir()
	v1 := txnBundle(home, map[string]string{".claude/a.md": "a1", ".claude/b.md": "b1"})
	if _, err := Apply(v1, ApplyOptions{}); err != nil {
		t.Fatalf("first apply: %v", err)
	}
	manifestPath := filepath.Join(home, ".cortex-ia", "targets", string(TargetClaude), manifestFileName)
	before := txnRead(t, manifestPath)

	v2 := txnBundle(home, map[string]string{
		".claude/a.md": "a2",
		".claude/b.md": "b2",
		".claude/c.md": "c2",
	})
	v2.Validate = func() (string, error) { return "", errors.New("validator rejected the install") }
	_, err := Apply(v2, ApplyOptions{})
	if err == nil || !strings.Contains(err.Error(), "previous state restored") {
		t.Fatalf("expected a failed apply with restoration, got %v", err)
	}

	if got := txnRead(t, filepath.Join(home, ".claude", "a.md")); got != "a1" {
		t.Errorf("a.md = %q after failed apply, want restored a1", got)
	}
	if got := txnRead(t, filepath.Join(home, ".claude", "b.md")); got != "b1" {
		t.Errorf("b.md = %q after failed apply, want restored b1", got)
	}
	if _, statErr := os.Stat(filepath.Join(home, ".claude", "c.md")); !errors.Is(statErr, fs.ErrNotExist) {
		t.Errorf("c.md created by the failed run was not removed (err=%v)", statErr)
	}
	if got := txnRead(t, manifestPath); got != before {
		t.Error("previous manifest was replaced by a failed apply")
	}
}

// A rewrite keeps the mode of the file it replaces, and the backup copy is
// written private as well, inside a private backup tree.
func TestApplyRewritePreservesModeAndPrivateBackups(t *testing.T) {
	home := t.TempDir()
	secret := filepath.Join(home, ".claude", "secret.md")
	if err := os.MkdirAll(filepath.Dir(secret), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(secret, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}

	b := txnBundle(home, map[string]string{".claude/secret.md": "new"})
	if _, err := Apply(b, ApplyOptions{Overwrite: true}); err != nil {
		t.Fatalf("overwrite apply: %v", err)
	}
	info, err := os.Stat(secret)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("rewritten file mode = %v, want 0600", info.Mode().Perm())
	}

	backups := filepath.Join(home, ".cortex-ia", "targets", string(TargetClaude), backupDirName)
	var sawCopy bool
	if err := filepath.WalkDir(backups, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if d.IsDir() && info.Mode().Perm() != 0o700 {
			t.Errorf("backup directory %s mode = %v, want 0700", path, info.Mode().Perm())
		}
		if !d.IsDir() && filepath.Base(path) == "secret.md" {
			sawCopy = true
			if info.Mode().Perm() != 0o600 {
				t.Errorf("backup copy mode = %v, want 0600", info.Mode().Perm())
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if !sawCopy {
		t.Fatal("no backup copy of the overwritten file was written")
	}
}

// Backup roots from the same instant differ, so two runs never share one.
func TestBackupRootsAreUniqueWithinOneSecond(t *testing.T) {
	stateDir := t.TempDir()
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	first, err := newBackupRoot(stateDir, now)
	if err != nil {
		t.Fatal(err)
	}
	second, err := newBackupRoot(stateDir, now)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatalf("two runs in the same instant share backup root %s", first)
	}
	if !strings.HasPrefix(filepath.Base(first), "20260102T030405") {
		t.Fatalf("backup root %q does not carry the run timestamp", first)
	}
}

// A symlinked target is never replaced silently: without Overwrite the apply is
// a conflict that writes nothing, and the link and its target stay intact.
func TestApplyRefusesSymlinkedTargetWithoutOverwrite(t *testing.T) {
	home := t.TempDir()
	realFile := filepath.Join(home, "dotfiles", "CLAUDE.md")
	if err := os.MkdirAll(filepath.Dir(realFile), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(realFile, []byte("user"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(home, ".claude", "CLAUDE.md")
	if err := os.MkdirAll(filepath.Dir(link), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(realFile, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	b := txnBundle(home, map[string]string{".claude/CLAUDE.md": "managed"})
	_, err := Apply(b, ApplyOptions{})
	if err == nil || !strings.Contains(err.Error(), "is a symlink") {
		t.Fatalf("expected a symlink conflict, got %v", err)
	}
	if info, statErr := os.Lstat(link); statErr != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("symlink was replaced by a conflicting apply (err=%v)", statErr)
	}
	if got := txnRead(t, realFile); got != "user" {
		t.Fatalf("symlink target changed to %q", got)
	}

	if _, err := Apply(b, ApplyOptions{Overwrite: true}); err != nil {
		t.Fatalf("explicit overwrite: %v", err)
	}
	if info, statErr := os.Lstat(link); statErr != nil || info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("explicit overwrite should replace the link (err=%v)", statErr)
	}
	if got := txnRead(t, realFile); got != "user" {
		t.Fatalf("explicit overwrite changed the link target to %q", got)
	}
}

// Apply and Remove take the canonical home lock, so they cannot interleave
// with another process working on the same home.
func TestApplyFailsWhileHomeIsLocked(t *testing.T) {
	home := t.TempDir()
	held, err := homelock.Acquire(home, time.Second)
	if err != nil {
		t.Fatalf("hold lock: %v", err)
	}
	defer func() { _ = held.Release() }()

	_, err = Apply(txnBundle(home, map[string]string{".claude/a.md": "a"}), ApplyOptions{})
	if !errors.Is(err, homelock.ErrHomeBusy) {
		t.Fatalf("apply under a held lock = %v, want ErrHomeBusy", err)
	}
	if _, statErr := os.Stat(filepath.Join(home, ".claude", "a.md")); !errors.Is(statErr, fs.ErrNotExist) {
		t.Fatal("a locked-out apply still wrote its file")
	}
}
