package filemerge

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestWriteFileAtomic_CreatesNew(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "test.json")

	result, err := WriteFileAtomic(path, []byte(`{"test": true}`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	if !result.Created {
		t.Error("expected Created=true")
	}
	if !result.Changed {
		t.Error("expected Changed=true")
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != `{"test": true}` {
		t.Errorf("content = %s", content)
	}
}

func TestWriteFileAtomic_NoChangeIfIdentical(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")

	content := []byte(`{"test": true}`)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := WriteFileAtomic(path, content, 0o644)
	if err != nil {
		t.Fatal(err)
	}

	if result.Changed {
		t.Error("expected Changed=false for identical content")
	}
	if result.Created {
		t.Error("expected Created=false")
	}
}

func TestWriteFileAtomic_OverwritesDifferent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")

	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := WriteFileAtomic(path, []byte("new"), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	if !result.Changed {
		t.Error("expected Changed=true")
	}
	if result.Created {
		t.Error("expected Created=false")
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "new" {
		t.Errorf("content = %s, want new", content)
	}
}

func TestWriteFileAtomic_DoesNotChmodExistingParentDir(t *testing.T) {
	if os.Getenv("GOOS") != "" && os.Getenv("GOOS") == "windows" || runtime.GOOS == "windows" {
		t.Skip("directory permission bits are not enforced on Windows")
	}
	dir := t.TempDir()
	parent := filepath.Join(dir, "secure")
	if err := os.MkdirAll(parent, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(parent, "test.json")
	if _, err := WriteFileAtomic(path, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(parent)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o700 {
		t.Fatalf("parent dir perm = %04o, want 0700 (must not be chmod'd to 0755)", got)
	}
}

func TestWriteFileAtomic_RejectsSymlinkParent(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "real")
	link := filepath.Join(dir, "link")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink creation requires elevated privileges: %v", err)
	}
	path := filepath.Join(link, "test.json")
	if _, err := WriteFileAtomic(path, []byte("{}"), 0o644); err == nil {
		t.Fatal("WriteFileAtomic unexpectedly wrote through a symlink parent")
	}
}
