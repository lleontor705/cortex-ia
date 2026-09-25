package updater

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

var (
	ErrUnrelatedOldFile     = errors.New("unrelated .old backup file exists; refusing to overwrite")
	ErrFinalDigestMismatch  = errors.New("final executable digest mismatch after replacement")
	ErrStagedDigestMismatch = errors.New("staged binary digest mismatch")
	ErrRollbackFailed       = errors.New("rollback to original executable failed; backup preserved")
	ErrEmptyTargetPath      = errors.New("target executable path cannot be empty")
)

const (
	backupSuffix      = ".old"
	ownerMarkerSuffix = ".cortex-ia-owned"
)

// digestFunc is a seam so tests can force the post-replacement rollback path.
var digestFunc = FileDigest

func FileDigest(p string) (string, error) {
	data, err := os.ReadFile(p)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:]), nil
}

func ReplaceExecutable(targetPath string, newBytes []byte, expectedSHA256 string) error {
	if strings.TrimSpace(targetPath) == "" {
		return ErrEmptyTargetPath
	}

	targetPath = filepath.Clean(targetPath)

	h := sha256.Sum256(newBytes)
	actualSHA := hex.EncodeToString(h[:])
	if expectedSHA256 != "" && !strings.EqualFold(actualSHA, expectedSHA256) {
		return fmt.Errorf("%w: expected %s, got %s", ErrStagedDigestMismatch, expectedSHA256, actualSHA)
	}
	expectedDigest := actualSHA

	if err := prepareReplacementPath(targetPath); err != nil {
		return err
	}

	backupPath, markerPath := chooseBackupPath(targetPath)

	var randBytes [8]byte
	_, _ = rand.Read(randBytes[:])
	stagePath := fmt.Sprintf("%s.tmp.%s", targetPath, hex.EncodeToString(randBytes[:]))
	defer func() {
		_ = os.Remove(stagePath)
	}()

	f, err := os.OpenFile(stagePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return fmt.Errorf("failed to create staged file: %w", err)
	}
	if _, err := f.Write(newBytes); err != nil {
		_ = f.Close()
		return fmt.Errorf("failed to write staged file: %w", err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return fmt.Errorf("failed to sync staged file: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("failed to close staged file: %w", err)
	}

	stagedDigest, err := digestFunc(stagePath)
	if err != nil {
		return fmt.Errorf("failed to verify staged file: %w", err)
	}
	if !strings.EqualFold(stagedDigest, expectedDigest) {
		return fmt.Errorf("%w: staged digest %s != %s", ErrStagedDigestMismatch, stagedDigest, expectedDigest)
	}

	if runtime.GOOS == "windows" {
		targetExists := false
		if _, err := os.Stat(targetPath); err == nil {
			targetExists = true
			if err := os.Rename(targetPath, backupPath); err != nil {
				return fmt.Errorf("failed to backup running binary: %w", err)
			}
			markUpdaterOwned(markerPath)
		}

		if err := os.Rename(stagePath, targetPath); err != nil {
			if targetExists {
				if rErr := os.Rename(backupPath, targetPath); rErr != nil {
					return fmt.Errorf("%w: apply error %v; rollback error %v", ErrRollbackFailed, err, rErr)
				}
				_ = os.Remove(markerPath)
			}
			return fmt.Errorf("failed to move staged binary to target: %w", err)
		}

		finalDigest, err := digestFunc(targetPath)
		if err != nil || !strings.EqualFold(finalDigest, expectedDigest) {
			if targetExists {
				_ = os.Remove(targetPath)
				if rErr := os.Rename(backupPath, targetPath); rErr != nil {
					return fmt.Errorf("%w: digest error; rollback error %v", ErrRollbackFailed, rErr)
				}
				_ = os.Remove(markerPath)
			}
			return fmt.Errorf("%w: expected %s, got %s", ErrFinalDigestMismatch, expectedDigest, finalDigest)
		}

		if targetExists {
			finalizeBackupRemoval(backupPath, markerPath)
		}
		return nil
	}

	targetExists := false
	if _, err := os.Stat(targetPath); err == nil {
		targetExists = true
		if err := os.Rename(targetPath, backupPath); err != nil {
			return fmt.Errorf("failed to backup original binary: %w", err)
		}
		markUpdaterOwned(markerPath)
	}

	if err := os.Rename(stagePath, targetPath); err != nil {
		if targetExists {
			_ = os.Rename(backupPath, targetPath)
			_ = os.Remove(markerPath)
		}
		return fmt.Errorf("failed to replace binary: %w", err)
	}

	finalDigest, err := digestFunc(targetPath)
	if err != nil || !strings.EqualFold(finalDigest, expectedDigest) {
		if targetExists {
			_ = os.Remove(targetPath)
			if rErr := os.Rename(backupPath, targetPath); rErr != nil {
				return fmt.Errorf("%w: digest error; rollback error %v", ErrRollbackFailed, rErr)
			}
			_ = os.Remove(markerPath)
		}
		return fmt.Errorf("%w: expected %s, got %s", ErrFinalDigestMismatch, expectedDigest, finalDigest)
	}

	if targetExists {
		finalizeBackupRemoval(backupPath, markerPath)
	}
	return nil
}

// prepareReplacementPath clears stale backups the updater itself created in a
// previous run and refuses to proceed while a foreign .old artifact sits next
// to the managed executable. A leftover updater backup is recognized either by
// its ownership marker or by carrying an executable image signature, which is
// what a copy of a previously-installed cortex-ia binary always has.
func prepareReplacementPath(targetPath string) error {
	dir := filepath.Dir(targetPath)
	targetBase := filepath.Base(targetPath)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to inspect replacement directory %s: %w", dir, err)
	}

	for _, entry := range entries {
		name := entry.Name()
		if strings.HasSuffix(name, ownerMarkerSuffix) {
			continue
		}
		if !isBackupArtifactName(targetBase, name) {
			continue
		}
		artifactPath := filepath.Join(dir, name)
		markerPath := artifactPath + ownerMarkerSuffix
		if !backupIsUpdaterOwned(artifactPath, markerPath) {
			return fmt.Errorf("%w: %s", ErrUnrelatedOldFile, artifactPath)
		}
		if err := removeOwnedBackup(artifactPath, markerPath); err != nil {
			return err
		}
	}
	return nil
}

func isBackupArtifactName(targetBase, name string) bool {
	if name == targetBase || !strings.HasPrefix(name, targetBase+".") {
		return false
	}
	rest := strings.TrimPrefix(name, targetBase+".")
	return rest == "old" || strings.HasPrefix(rest, "old.") || strings.HasSuffix(rest, ".old")
}

func backupIsUpdaterOwned(artifactPath, markerPath string) bool {
	if _, err := os.Stat(markerPath); err == nil {
		return true
	}
	return payloadLooksExecutable(artifactPath)
}

// removeOwnedBackup deletes a stale updater backup. On Windows the just-exited
// process image stays locked under the backup name, so a failed deletion is
// recorded for the next run instead of blocking the current update.
func removeOwnedBackup(artifactPath, markerPath string) error {
	if err := os.Remove(artifactPath); err != nil {
		if werr := markUpdaterOwnedErr(markerPath); werr != nil {
			return fmt.Errorf("failed to remove stale backup %s: %w", artifactPath, err)
		}
		return nil
	}
	_ = os.Remove(markerPath)
	return nil
}

// chooseBackupPath prefers the canonical <target>.old name and falls back to a
// uniquely suffixed artifact when a locked remnant still occupies it.
func chooseBackupPath(targetPath string) (backupPath, markerPath string) {
	canonical := targetPath + backupSuffix
	if _, err := os.Lstat(canonical); os.IsNotExist(err) {
		return canonical, canonical + ownerMarkerSuffix
	}
	var randBytes [8]byte
	_, _ = rand.Read(randBytes[:])
	backupPath = fmt.Sprintf("%s%s.%s", targetPath, backupSuffix, hex.EncodeToString(randBytes[:]))
	return backupPath, backupPath + ownerMarkerSuffix
}

// finalizeBackupRemoval is always best-effort; the update has already succeeded.
func finalizeBackupRemoval(backupPath, markerPath string) {
	if err := os.Remove(backupPath); err != nil {
		_ = markUpdaterOwnedErr(markerPath)
		return
	}
	_ = os.Remove(markerPath)
}

func markUpdaterOwned(markerPath string) {
	_ = markUpdaterOwnedErr(markerPath)
}

func markUpdaterOwnedErr(markerPath string) error {
	return os.WriteFile(markerPath, []byte("pending-delete\n"), 0o644)
}

func payloadLooksExecutable(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	var magic [4]byte
	if _, err := io.ReadFull(f, magic[:]); err != nil {
		return false
	}
	if magic == [4]byte{0x7f, 'E', 'L', 'F'} {
		return true
	}
	if magic[0] == 'M' && magic[1] == 'Z' {
		return true
	}
	switch magic {
	case [4]byte{0xfe, 0xed, 0xfa, 0xce}, [4]byte{0xce, 0xfa, 0xed, 0xfe},
		[4]byte{0xfe, 0xed, 0xfa, 0xcf}, [4]byte{0xcf, 0xfa, 0xed, 0xfe},
		[4]byte{0xca, 0xfe, 0xba, 0xbe}, [4]byte{0xbe, 0xba, 0xfe, 0xca},
		[4]byte{0xca, 0xfe, 0xba, 0xbf}, [4]byte{0xbf, 0xba, 0xfe, 0xca}:
		return true
	}
	return false
}
