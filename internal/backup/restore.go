package backup

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"github.com/lleontor705/cortex-ia/internal/components/filemerge"
)

// RestoreService restores files from a validated backup manifest.
type RestoreService struct{}

// Restore re-applies a manifest to its recorded originals. It fails closed
// before the first write unless the manifest passes Validate, every original
// accredits as a link-free absolute path, and every restored byte matches
// the recorded SHA-256.
func (s RestoreService) Restore(manifest Manifest) error {
	if err := manifest.Validate(); err != nil {
		return err
	}
	// Accreditation is all-or-nothing: one rejected entry blocks every
	// write in the set. Originals and recorded snapshot locations alike
	// are accredited no-follow before the first byte moves.
	for _, entry := range manifest.Entries {
		if err := rejectLinkChain(entry.OriginalPath, entry.OriginalPath); err != nil {
			return err
		}
		if entry.Existed {
			if err := rejectLinkChain(entry.SnapshotPath, fmt.Sprintf("snapshot for %q", entry.OriginalPath)); err != nil {
				return err
			}
		}
	}
	for _, entry := range manifest.Entries {
		if entry.Existed {
			if err := restoreEntry(entry); err != nil {
				return err
			}
			continue
		}

		info, err := os.Lstat(entry.OriginalPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("inspect path %q: %w", entry.OriginalPath, err)
		}
		if info.Mode()&os.ModeSymlink != 0 || info.Mode()&os.ModeIrregular != 0 {
			return fmt.Errorf("%w: %q", ErrUnsupportedLink, entry.OriginalPath)
		}
		if err := os.Remove(entry.OriginalPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove path %q: %w", entry.OriginalPath, err)
		}
	}
	return nil
}

func restoreEntry(entry ManifestEntry) error {
	if err := rejectLinkChain(entry.OriginalPath, entry.OriginalPath); err != nil {
		return err
	}
	content, err := readSnapshotNoFollow(entry)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(entry.OriginalPath), 0o755); err != nil {
		return fmt.Errorf("create restore directory for %q: %w", entry.OriginalPath, err)
	}

	if _, err := filemerge.WriteFileAtomic(entry.OriginalPath, content, os.FileMode(entry.Mode).Perm()); err != nil {
		return fmt.Errorf("restore path %q: %w", entry.OriginalPath, err)
	}
	return nil
}

// readSnapshotNoFollow revalidates the snapshot location immediately before
// the read: the path must Lstat as a regular, non-link file, the bytes read
// must match the recorded SHA-256, and the file observed after the read
// must be the same one accredited before it. Any swap across the read —
// including a replacement that carries identical bytes — fails closed.
func readSnapshotNoFollow(entry ManifestEntry) ([]byte, error) {
	before, err := lstatSnapshotNoFollow(entry.SnapshotPath)
	if err != nil {
		return nil, err
	}
	content, err := os.ReadFile(entry.SnapshotPath)
	if err != nil {
		return nil, fmt.Errorf("read snapshot file %q: %w", entry.SnapshotPath, err)
	}
	after, err := lstatSnapshotNoFollow(entry.SnapshotPath)
	if err != nil {
		return nil, err
	}
	if snapshotIdentity(before) != snapshotIdentity(after) {
		return nil, fmt.Errorf("%w: snapshot %q was replaced during restore", ErrManifestInvalid, entry.SnapshotPath)
	}
	if digest := sha256.Sum256(content); hex.EncodeToString(digest[:]) != entry.SHA256 {
		return nil, fmt.Errorf("%w: snapshot %q does not match the recorded SHA-256", ErrManifestInvalid, entry.SnapshotPath)
	}
	return content, nil
}
