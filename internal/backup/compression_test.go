package backup

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// TestCreateArchive_CreatesValidTarGz verifies that CreateArchive produces a
// valid tar.gz file at the given path that contains the expected entries.
func TestCreateArchive_CreatesValidTarGz(t *testing.T) {
	dir := t.TempDir()

	// Create two source files.
	srcA := filepath.Join(dir, "a.txt")
	srcB := filepath.Join(dir, "b.txt")
	if err := os.WriteFile(srcA, []byte("hello"), 0o644); err != nil {
		t.Fatalf("WriteFile a.txt: %v", err)
	}
	if err := os.WriteFile(srcB, []byte("world"), 0o644); err != nil {
		t.Fatalf("WriteFile b.txt: %v", err)
	}

	archivePath := filepath.Join(dir, "snapshot.tar.gz")
	entries := []ArchiveEntry{
		{RelPath: "files/a.txt", SourcePath: srcA, Mode: 0o644},
		{RelPath: "files/b.txt", SourcePath: srcB, Mode: 0o644},
	}

	if err := CreateArchive(archivePath, entries); err != nil {
		t.Fatalf("CreateArchive() error = %v", err)
	}

	// Archive must exist.
	if _, err := os.Stat(archivePath); err != nil {
		t.Fatalf("snapshot.tar.gz not created: %v", err)
	}

	// Open and verify the archive contains both entries.
	found := openAndListTar(t, archivePath)
	if _, ok := found["files/a.txt"]; !ok {
		t.Errorf("archive does not contain files/a.txt; got %v", found)
	}
	if _, ok := found["files/b.txt"]; !ok {
		t.Errorf("archive does not contain files/b.txt; got %v", found)
	}
}

// TestCreateArchive_PreservesPermissions verifies that file modes are preserved
// inside the archive header.
func TestCreateArchive_PreservesPermissions(t *testing.T) {
	dir := t.TempDir()

	src := filepath.Join(dir, "exec.sh")
	if err := os.WriteFile(src, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("WriteFile exec.sh: %v", err)
	}

	archivePath := filepath.Join(dir, "snapshot.tar.gz")
	entries := []ArchiveEntry{
		{RelPath: "files/exec.sh", SourcePath: src, Mode: 0o755},
	}

	if err := CreateArchive(archivePath, entries); err != nil {
		t.Fatalf("CreateArchive() error = %v", err)
	}

	found := openAndListTar(t, archivePath)
	hdr, ok := found["files/exec.sh"]
	if !ok {
		t.Fatalf("archive does not contain files/exec.sh")
	}
	// Tar header mode should include execute bits.
	if hdr.Mode&0o111 == 0 {
		t.Errorf("exec.sh mode = %04o, want execute bits set (e.g. 0755)", hdr.Mode)
	}
}

// --- helpers ---

// openAndListTar opens a tar.gz file and returns a map of entry names → headers.
func openAndListTar(t *testing.T, archivePath string) map[string]*tar.Header {
	t.Helper()
	f, err := os.Open(archivePath)
	if err != nil {
		t.Fatalf("Open archive: %v", err)
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatalf("gzip.NewReader: %v", err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	found := make(map[string]*tar.Header)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("tar.Next: %v", err)
		}
		// Copy header so we can store it safely.
		h := *hdr
		found[hdr.Name] = &h
	}
	return found
}
