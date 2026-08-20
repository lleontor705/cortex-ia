package filemerge

import (
	"os"
	"path/filepath"
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
