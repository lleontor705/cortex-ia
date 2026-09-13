package delegation

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestWindowsBoundarySmoke(t *testing.T) {
	t.Logf("OS: %s, Toolchain: %s, Architecture: %s", runtime.GOOS, runtime.Version(), runtime.GOARCH)

	tempDir := t.TempDir()
	targetFile := filepath.Join(tempDir, "boundary_target.txt")
	targetPayload := []byte("boundary-smoke-payload-delegation")

	// 1. Initial write to target
	if err := os.WriteFile(targetFile, targetPayload, 0600); err != nil {
		t.Fatalf("failed to write initial target: %v", err)
	}

	// 2. Controlled canonicalization check
	t.Log("Operation: CanonicalWorkspace on tempDir")
	canonical, err := CanonicalWorkspace(tempDir)
	if err != nil {
		t.Fatalf("CanonicalWorkspace failed on tempDir: %v", err)
	}
	if canonical == "" {
		t.Fatal("CanonicalWorkspace returned empty string")
	}

	// 3. Controlled open handle and canonicalization while open
	t.Log("Operation: CanonicalWorkspace with active file handle open")
	handle, err := os.OpenFile(targetFile, os.O_RDWR, 0600)
	if err != nil {
		t.Fatalf("failed to open handle to target: %v", err)
	}

	// Test canonicalization of directory while handle is open
	dirCanonical, err := CanonicalWorkspace(tempDir)
	if err != nil {
		_ = handle.Close()
		t.Fatalf("CanonicalWorkspace failed while handle open: %v", err)
	}
	if dirCanonical != canonical {
		_ = handle.Close()
		t.Fatalf("canonical mismatch while handle open: got %s, want %s", dirCanonical, canonical)
	}

	// Close handle
	if err := handle.Close(); err != nil {
		t.Fatalf("failed to close handle: %v", err)
	}

	// 4. Controlled rename boundary in temporary state
	t.Log("Operation: Controlled rename in temporary directory")
	renamedFile := filepath.Join(tempDir, "boundary_target_renamed.txt")
	if err := os.Rename(targetFile, renamedFile); err != nil {
		t.Fatalf("failed to rename target file: %v", err)
	}

	// Verify content after rename
	content, err := os.ReadFile(renamedFile)
	if err != nil {
		t.Fatalf("failed to read renamed target: %v", err)
	}
	if string(content) != string(targetPayload) {
		t.Fatalf("target content mismatch after rename: got %s, want %s", content, targetPayload)
	}

	// Canonicalize subdirectory
	subDir := filepath.Join(tempDir, "SubFolder")
	if err := os.Mkdir(subDir, 0700); err != nil {
		t.Fatalf("failed to create subfolder: %v", err)
	}
	subCanonical, err := CanonicalWorkspace(subDir)
	if err != nil {
		t.Fatalf("CanonicalWorkspace on subDir failed: %v", err)
	}
	if !WorkspacesCompatible(tempDir, subDir) {
		t.Fatalf("WorkspacesCompatible failed for parent %s and child %s", canonical, subCanonical)
	}

	t.Log("Assessment: delegation workspace canonicalization boundaries verified safely without target corruption")
}
