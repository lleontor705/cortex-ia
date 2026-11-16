package filemerge

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func makeSymlinkOrSkip(t *testing.T, target, link string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink creation not permitted: %v", err)
	}
}

func TestWriteFileAtomic_RefusesSymlinkedManagedSubdirectory(t *testing.T) {
	root := t.TempDir()
	elsewhere := filepath.Join(root, "elsewhere")
	if err := os.MkdirAll(elsewhere, 0o755); err != nil {
		t.Fatal(err)
	}
	managed := filepath.Join(root, "managed")
	if err := os.MkdirAll(managed, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(managed, "agents")
	makeSymlinkOrSkip(t, elsewhere, link)

	target := filepath.Join(link, "planted.md")
	if _, err := WriteFileAtomic(target, []byte("x"), 0o644); err == nil {
		t.Fatalf("write through symlinked managed subdirectory %q succeeded", link)
	}
	if _, err := os.Stat(filepath.Join(elsewhere, "planted.md")); err == nil {
		t.Fatal("write landed in the symlink target")
	}
}

func TestWriteFileAtomic_NestedPathUnderTempDirWorks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a", "b", "c", "file.json")

	if _, err := WriteFileAtomic(path, []byte(`{}`), 0o644); err != nil {
		t.Fatalf("nested write under TempDir failed: %v", err)
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != `{}` {
		t.Fatalf("read back = %q, %v", got, err)
	}
}

func TestWriteFileAtomic_RelativeCwdCollisionDoesNotFalseReject(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("cwd symlink named after a drive-letter component is not representable")
	}
	target := filepath.Join(t.TempDir(), "nested", "file.json")
	firstComponent := strings.SplitN(strings.TrimPrefix(filepath.ToSlash(target), "/"), "/", 2)[0]

	cwd := t.TempDir()
	makeSymlinkOrSkip(t, t.TempDir(), filepath.Join(cwd, firstComponent))
	t.Chdir(cwd)

	if _, err := WriteFileAtomic(target, []byte("ok"), 0o644); err != nil {
		t.Fatalf("absolute write rejected due to cwd symlink %q: %v", firstComponent, err)
	}
}

func TestWriteFileAtomic_HomeDotEntrySymlinkAccepted(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dotfiles := filepath.Join(t.TempDir(), "dotfiles", "config")
	if err := os.MkdirAll(dotfiles, 0o755); err != nil {
		t.Fatal(err)
	}
	makeSymlinkOrSkip(t, dotfiles, filepath.Join(home, ".config"))

	if _, err := WriteFileAtomic(filepath.Join(home, ".config", "app", "x.json"), []byte("{}"), 0o644); err != nil {
		t.Fatalf("dotfile-manager symlink under home rejected: %v", err)
	}
}
