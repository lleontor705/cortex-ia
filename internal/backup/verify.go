package backup

import (
	"fmt"
	"os"
)

// Verify proves that every expected snapshot entry is present and still
// matches the digest recorded when the backup was created. Snapshot
// locations are accredited with Lstat (no-follow): a symlink or reparse
// point at the snapshot location is rejected before any byte is read, and
// a swap of the snapshot file across the read is detected and fails closed
// instead of trusting bytes read through a replacement.
func Verify(manifest Manifest) error {
	if err := manifest.Validate(); err != nil {
		return err
	}
	for _, entry := range manifest.Entries {
		if !entry.Existed {
			if entry.SnapshotPath != "" || entry.SHA256 != "" {
				return fmt.Errorf("backup entry %q records snapshot data for a missing source", entry.OriginalPath)
			}
			continue
		}
		if entry.SnapshotPath == "" || entry.SHA256 == "" {
			return fmt.Errorf("backup entry %q is missing verification metadata", entry.OriginalPath)
		}
		if err := verifySnapshotNoFollow(entry); err != nil {
			return err
		}
	}
	return nil
}

// lstatSnapshotNoFollow accredits a snapshot location with Lstat and fails
// closed when it is a symlink/reparse point or anything other than a
// regular file. It never follows the final component, so an external
// target cannot stand in for the recorded snapshot.
func lstatSnapshotNoFollow(snapshotPath string) (os.FileInfo, error) {
	info, err := os.Lstat(snapshotPath)
	if err != nil {
		return nil, fmt.Errorf("accredit snapshot %q: %w", snapshotPath, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || info.Mode()&os.ModeIrregular != 0 {
		return nil, fmt.Errorf("%w: snapshot %q is a symlink/reparse point", ErrUnsupportedLink, snapshotPath)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("snapshot %q: not a regular file", snapshotPath)
	}
	return info, nil
}

// snapshotIdentity captures the observable identity of an accredited
// snapshot so a replacement between accreditation and read is detectable.
func snapshotIdentity(info os.FileInfo) string {
	return fmt.Sprintf("mode=%s size=%d modtime=%d", info.Mode(), info.Size(), info.ModTime().UnixNano())
}

func verifySnapshotNoFollow(entry ManifestEntry) error {
	before, err := lstatSnapshotNoFollow(entry.SnapshotPath)
	if err != nil {
		return err
	}
	digest, err := fileSHA256(entry.SnapshotPath)
	if err != nil {
		return err
	}
	after, err := lstatSnapshotNoFollow(entry.SnapshotPath)
	if err != nil {
		return err
	}
	if snapshotIdentity(before) != snapshotIdentity(after) {
		return fmt.Errorf("%w: snapshot %q was replaced during verification", ErrManifestInvalid, entry.SnapshotPath)
	}
	if digest != entry.SHA256 {
		return fmt.Errorf("verify snapshot %q: SHA-256 mismatch", entry.SnapshotPath)
	}
	return nil
}
