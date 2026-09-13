package updater

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
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

	backupPath := targetPath + ".old"
	if _, err := os.Stat(backupPath); err == nil {
		return fmt.Errorf("%w: %s", ErrUnrelatedOldFile, backupPath)
	}

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

	stagedDigest, err := FileDigest(stagePath)
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
		}

		if err := os.Rename(stagePath, targetPath); err != nil {
			if targetExists {
				if rErr := os.Rename(backupPath, targetPath); rErr != nil {
					return fmt.Errorf("%w: apply error %v; rollback error %v", ErrRollbackFailed, err, rErr)
				}
			}
			return fmt.Errorf("failed to move staged binary to target: %w", err)
		}

		finalDigest, err := FileDigest(targetPath)
		if err != nil || !strings.EqualFold(finalDigest, expectedDigest) {
			if targetExists {
				_ = os.Remove(targetPath)
				if rErr := os.Rename(backupPath, targetPath); rErr != nil {
					return fmt.Errorf("%w: digest error; rollback error %v", ErrRollbackFailed, rErr)
				}
			}
			return fmt.Errorf("%w: expected %s, got %s", ErrFinalDigestMismatch, expectedDigest, finalDigest)
		}

		if targetExists {
			_ = os.Remove(backupPath)
		}
		return nil
	}

	targetExists := false
	if _, err := os.Stat(targetPath); err == nil {
		targetExists = true
		if err := os.Rename(targetPath, backupPath); err != nil {
			return fmt.Errorf("failed to backup original binary: %w", err)
		}
	}

	if err := os.Rename(stagePath, targetPath); err != nil {
		if targetExists {
			_ = os.Rename(backupPath, targetPath)
		}
		return fmt.Errorf("failed to replace binary: %w", err)
	}

	finalDigest, err := FileDigest(targetPath)
	if err != nil || !strings.EqualFold(finalDigest, expectedDigest) {
		if targetExists {
			_ = os.Remove(targetPath)
			if rErr := os.Rename(backupPath, targetPath); rErr != nil {
				return fmt.Errorf("%w: digest error; rollback error %v", ErrRollbackFailed, rErr)
			}
		}
		return fmt.Errorf("%w: expected %s, got %s", ErrFinalDigestMismatch, expectedDigest, finalDigest)
	}

	if targetExists {
		_ = os.Remove(backupPath)
	}
	return nil
}
