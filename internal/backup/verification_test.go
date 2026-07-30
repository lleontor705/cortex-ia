package backup

import (
	"os"
	"path/filepath"
	"testing"
)

func TestVerifySnapshotRejectsTamperedBackup(t *testing.T) {
	source := filepath.Join(t.TempDir(), "ownership.json")
	if err := os.WriteFile(source, []byte("trusted metadata\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	manifest, err := NewSnapshotter().Create(filepath.Join(t.TempDir(), "backup"), []string{source})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := Verify(manifest); err != nil {
		t.Fatalf("Verify() fresh snapshot error = %v", err)
	}
	if manifest.Entries[0].SHA256 == "" {
		t.Fatal("snapshot entry must record a content digest")
	}

	if err := os.WriteFile(manifest.Entries[0].SnapshotPath, []byte("tampered\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Verify(manifest); err == nil {
		t.Fatal("Verify() must reject a tampered snapshot")
	}
}
