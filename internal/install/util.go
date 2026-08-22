package install

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/lleontor705/cortex-ia/internal/agents/opencode"
	"github.com/lleontor705/cortex-ia/internal/state"
)

// artifactAbs returns the absolute destination path for a recorded artifact.
func (s *Service) artifactAbs(artifact state.ArtifactV2) string {
	rel := opencode.DestinationForArtifact(artifact.Path, string(artifact.Kind))
	return filepath.Join(s.homeDir, filepath.FromSlash(rel))
}

// validBackupID matches the safe backup ID grammar shared with the engine
// (alphanumeric, hyphens, underscores).
var validBackupID = regexp.MustCompile(`^[a-zA-Z0-9\-_]+$`)

// fileDigest reports whether the path holds a regular file and the lowercase
// hex sha256 of its bytes. Directories, symlinks, and other non-regular
// shapes fail closed. This is a generic byte-level inspection primitive: the
// values it compares against are always digests recorded by the pipeline or
// the MCP manager, never recomputed asset or semantic digests.
func fileDigest(abs string) (exists bool, digest string, err error) {
	info, err := os.Lstat(abs)
	if errors.Is(err, os.ErrNotExist) {
		return false, "", nil
	}
	if err != nil {
		return true, "", err
	}
	if info.IsDir() {
		return true, "", fmt.Errorf("%q is a directory, not a regular file", abs)
	}
	if !info.Mode().IsRegular() {
		return true, "", fmt.Errorf("%q is not a regular file", abs)
	}
	content, err := os.ReadFile(abs)
	if err != nil {
		return true, "", err
	}
	sum := sha256.Sum256(content)
	return true, hex.EncodeToString(sum[:]), nil
}

// timestampedID formats a backup ID in the engine's grammar.
func timestampedID(prefix string, now time.Time) string {
	return fmt.Sprintf("%s-%s-%09d", prefix, now.UTC().Format("20060102T150405"), now.Nanosecond())
}

// opencodeRoot resolves the absolute OpenCode configuration root for a home
// directory, mirroring the engine's layout rule.
func opencodeRoot(homeDir string) (string, error) {
	absolute, err := filepath.Abs(filepath.Join(homeDir, filepath.FromSlash(".config/opencode")))
	if err != nil {
		return "", fmt.Errorf("resolve OpenCode root: %w", err)
	}
	return filepath.Clean(absolute), nil
}

// homeRelative converts an absolute path under the home into its
// slash-relative form for receipts and reports.
func homeRelative(home, abs string) string {
	relative, err := filepath.Rel(home, abs)
	if err != nil {
		return filepath.ToSlash(abs)
	}
	return filepath.ToSlash(relative)
}
