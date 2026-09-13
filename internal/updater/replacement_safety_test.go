package updater

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestUpdateReplacement(t *testing.T) {
	origBytes := []byte("ORIGINAL_BINARY_BYTES_OK")
	newBytes := []byte("NEW_AUTHENTICATED_BINARY_BYTES_OK")
	newHash := sha256.Sum256(newBytes)
	newSHA := hex.EncodeToString(newHash[:])

	t.Run("SuccessfulReplacement", func(t *testing.T) {
		tmpDir := t.TempDir()
		target := filepath.Join(tmpDir, "cortex-ia.exe")
		if err := os.WriteFile(target, origBytes, 0o755); err != nil {
			t.Fatal(err)
		}

		if err := ReplaceExecutable(target, newBytes, newSHA); err != nil {
			t.Fatalf("expected successful replacement, got: %v", err)
		}

		got, err := os.ReadFile(target)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != string(newBytes) {
			t.Fatalf("target content mismatch: got %q, want %q", string(got), string(newBytes))
		}

		// Ensure no backup remains
		if _, err := os.Stat(target + ".old"); err == nil {
			t.Error("unexpected surviving .old backup on success")
		}
	})

	t.Run("PreserveUnrelatedOldFile", func(t *testing.T) {
		tmpDir := t.TempDir()
		target := filepath.Join(tmpDir, "cortex-ia.exe")
		unrelatedOld := target + ".old"
		unrelatedContent := []byte("DO_NOT_OVERWRITE_PREVIOUS_BACKUP")

		if err := os.WriteFile(target, origBytes, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(unrelatedOld, unrelatedContent, 0o644); err != nil {
			t.Fatal(err)
		}

		err := ReplaceExecutable(target, newBytes, newSHA)
		if !errors.Is(err, ErrUnrelatedOldFile) {
			t.Fatalf("expected ErrUnrelatedOldFile, got: %v", err)
		}

		// Unrelated .old must be preserved
		oldGot, _ := os.ReadFile(unrelatedOld)
		if string(oldGot) != string(unrelatedContent) {
			t.Fatalf("unrelated .old was overwritten: %q", string(oldGot))
		}

		// Target remains unchanged
		targetGot, _ := os.ReadFile(target)
		if string(targetGot) != string(origBytes) {
			t.Fatalf("target was mutated despite refusal: %q", string(targetGot))
		}
	})

	t.Run("StagedDigestMismatch", func(t *testing.T) {
		tmpDir := t.TempDir()
		target := filepath.Join(tmpDir, "cortex-ia.exe")
		if err := os.WriteFile(target, origBytes, 0o755); err != nil {
			t.Fatal(err)
		}

		wrongSHA := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
		err := ReplaceExecutable(target, newBytes, wrongSHA)
		if !errors.Is(err, ErrStagedDigestMismatch) {
			t.Fatalf("expected ErrStagedDigestMismatch, got: %v", err)
		}

		// Target remains unchanged
		targetGot, _ := os.ReadFile(target)
		if string(targetGot) != string(origBytes) {
			t.Fatalf("target was mutated on staged digest mismatch")
		}
	})

	t.Run("EmptyTargetPathRejection", func(t *testing.T) {
		if err := ReplaceExecutable("", newBytes, newSHA); !errors.Is(err, ErrEmptyTargetPath) {
			t.Fatalf("expected ErrEmptyTargetPath, got: %v", err)
		}
		if err := ReplaceExecutable("   ", newBytes, newSHA); !errors.Is(err, ErrEmptyTargetPath) {
			t.Fatalf("expected ErrEmptyTargetPath for spaces, got: %v", err)
		}
	})
}
