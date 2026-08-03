package backup

import (
	"fmt"
	"os"
)

// Verify proves that every expected snapshot entry is present and still
// matches the digest recorded when the backup was created.
func Verify(manifest Manifest) error {
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
		info, err := os.Stat(entry.SnapshotPath)
		if err != nil {
			return fmt.Errorf("verify snapshot %q: %w", entry.SnapshotPath, err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("verify snapshot %q: not a regular file", entry.SnapshotPath)
		}
		digest, err := fileSHA256(entry.SnapshotPath)
		if err != nil {
			return err
		}
		if digest != entry.SHA256 {
			return fmt.Errorf("verify snapshot %q: SHA-256 mismatch", entry.SnapshotPath)
		}
	}
	return nil
}
