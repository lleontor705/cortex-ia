package updater

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func hashHex(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func TestReplaceExecutableRecoversFromOwnedStaleBackup(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "cortex-ia")
	original := []byte("previously-installed-version")
	replacement := []byte("freshly-downloaded-version")

	if err := os.WriteFile(target, original, 0o755); err != nil {
		t.Fatal(err)
	}
	// A locked process image left behind by an earlier update carries an
	// executable signature, which is what identifies it as updater-owned.
	stale := append([]byte{0x7f, 'E', 'L', 'F', 0x02, 0x01, 0x01, 0x00}, []byte("old-image")...)
	if err := os.WriteFile(target+backupSuffix, stale, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := ReplaceExecutable(target, replacement, hashHex(replacement)); err != nil {
		t.Fatalf("expected apply to recover from an updater-owned stale backup, got: %v", err)
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(replacement) {
		t.Fatalf("target mismatch: got %q, want %q", got, replacement)
	}
	if _, err := os.Stat(target + backupSuffix); err == nil {
		t.Fatal("stale updater backup survived a successful replacement")
	}
}

func TestReplaceExecutableRejectsForeignOldArtifact(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "cortex-ia")
	original := []byte("previously-installed-version")
	replacement := []byte("freshly-downloaded-version")
	foreign := target + backupSuffix + ".bak"
	foreignContent := []byte("user-managed backup that the updater must not touch")

	if err := os.WriteFile(target, original, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(foreign, foreignContent, 0o644); err != nil {
		t.Fatal(err)
	}

	err := ReplaceExecutable(target, replacement, hashHex(replacement))
	if !errors.Is(err, ErrUnrelatedOldFile) {
		t.Fatalf("expected ErrUnrelatedOldFile for a foreign artifact, got: %v", err)
	}

	gotForeign, _ := os.ReadFile(foreign)
	if string(gotForeign) != string(foreignContent) {
		t.Fatalf("foreign artifact was mutated: %q", gotForeign)
	}
	gotTarget, _ := os.ReadFile(target)
	if string(gotTarget) != string(original) {
		t.Fatalf("target was mutated despite refusal: %q", gotTarget)
	}
}

func TestPrepareReplacementDefersLockedOwnedBackup(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "cortex-ia")
	if err := os.WriteFile(target, []byte("current"), 0o755); err != nil {
		t.Fatal(err)
	}

	backupPath := target + backupSuffix
	if err := os.Mkdir(backupPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backupPath, "holder"), []byte("locked"), 0o644); err != nil {
		t.Fatal(err)
	}
	markerPath := backupPath + ownerMarkerSuffix
	markUpdaterOwned(markerPath)

	if err := prepareReplacementPath(target); err != nil {
		t.Fatalf("expected deferred deletion instead of failure, got: %v", err)
	}
	if _, err := os.Stat(markerPath); err != nil {
		t.Fatal("delete-next-run marker missing after a deferred removal")
	}
	if _, err := os.Stat(backupPath); err != nil {
		t.Fatal("undeletable backup should remain for the next run")
	}
}

func TestReplaceExecutableRollbackOnFinalDigestMismatch(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "cortex-ia")
	original := []byte("rollback-original-bytes")
	replacement := []byte("replacement-bytes")
	if err := os.WriteFile(target, original, 0o755); err != nil {
		t.Fatal(err)
	}

	restore := digestFunc
	digestFunc = func(p string) (string, error) {
		if p == target {
			return strings.Repeat("0", 64), nil
		}
		return restore(p)
	}
	defer func() { digestFunc = restore }()

	err := ReplaceExecutable(target, replacement, hashHex(replacement))
	if !errors.Is(err, ErrFinalDigestMismatch) {
		t.Fatalf("expected ErrFinalDigestMismatch, got: %v", err)
	}

	got, readErr := os.ReadFile(target)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != string(original) {
		t.Fatalf("rollback did not restore the original binary: %q", got)
	}
	if _, err := os.Stat(target + backupSuffix); err == nil {
		t.Fatal("backup survived a rollback restore")
	}
}
