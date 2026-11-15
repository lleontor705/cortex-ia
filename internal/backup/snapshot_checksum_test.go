package backup

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSnapshotter_ChecksumMatchesComputeChecksum(t *testing.T) {
	tempDir := t.TempDir()
	sourceDir := filepath.Join(tempDir, "sources")
	snapshotDir := filepath.Join(tempDir, "snapshot-1")

	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatalf("create source dir: %v", err)
	}

	f1 := filepath.Join(sourceDir, "alpha.txt")
	f2 := filepath.Join(sourceDir, "beta.txt")
	if err := os.WriteFile(f1, []byte("alpha content\n"), 0o644); err != nil {
		t.Fatalf("write f1: %v", err)
	}
	if err := os.WriteFile(f2, []byte("beta content\n"), 0o644); err != nil {
		t.Fatalf("write f2: %v", err)
	}

	paths := []string{f1, f2}
	expectedChecksum, err := ComputeChecksum(paths)
	if err != nil {
		t.Fatalf("ComputeChecksum failed: %v", err)
	}

	snapshotter := NewSnapshotter()
	manifest, err := snapshotter.Create(snapshotDir, paths)
	if err != nil {
		t.Fatalf("Snapshotter.Create failed: %v", err)
	}

	if manifest.Checksum == "" {
		t.Fatal("expected manifest.Checksum to be populated, got empty")
	}
	if manifest.Checksum != expectedChecksum {
		t.Fatalf("manifest.Checksum = %q, want %q (from ComputeChecksum)", manifest.Checksum, expectedChecksum)
	}
}
