package conformance

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAuthorityInventoryRejectsDuplicatePolicyOwners(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "canonical.go"), "package canonical\nvar RetryCeiling = 3\n")
	writeTestFile(t, filepath.Join(root, "duplicate.md"), "retry ceiling: 3\n")

	report, err := ScanAuthorityInventory(root)
	if err != nil {
		t.Fatalf("ScanAuthorityInventory() error = %v", err)
	}
	if report.Complete || !report.HasDuplicateOwner {
		t.Fatalf("duplicate authority was not rejected: %+v", report)
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write test file %q: %v", path, err)
	}
}

func TestAuthorityInventoryAcceptsCanonicalGeneratedReference(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "canonical.go"), "package canonical\nvar RetryCeiling = 3\n")
	writeTestFile(t, filepath.Join(root, "generated.md"), "See policy key retry.profile.default.\n")

	report, err := ScanAuthorityInventory(root)
	if err != nil {
		t.Fatalf("ScanAuthorityInventory() error = %v", err)
	}
	if !report.Complete || report.HasDuplicateOwner {
		t.Fatalf("canonical authority inventory failed: %+v", report)
	}
}
