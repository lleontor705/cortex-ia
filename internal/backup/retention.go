package backup

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
)

// DefaultRetentionCount is the default number of unpinned backups to keep.
// Pinned backups do not count toward this limit and are never pruned.
const DefaultRetentionCount = 5

// Prune deletes the oldest unpinned backups in backupDir, keeping at most
// retentionCount unpinned backups. Pinned backups are never deleted.
//
// If retentionCount <= 0 the function is a no-op (unlimited retention).
//
// Returns the list of deleted backup IDs.
func Prune(backupDir string, retentionCount int) ([]string, error) {
	return PruneKeeping(backupDir, retentionCount, nil)
}

// PruneKeeping is Prune with a protected set of backup IDs. A protected
// backup is never deleted and does not count toward retentionCount, so a
// backup an incomplete journal still needs can never be the one evicted.
func PruneKeeping(backupDir string, retentionCount int, keep map[string]bool) ([]string, error) {
	if retentionCount <= 0 {
		return nil, nil
	}
	base, err := filepath.Abs(backupDir)
	if err != nil {
		return nil, fmt.Errorf("resolve backup dir %q: %w", backupDir, err)
	}

	manifests, err := listManifestsForRetention(backupDir)
	if err != nil {
		return nil, err
	}

	sort.Slice(manifests, func(i, j int) bool {
		return manifests[i].CreatedAt.After(manifests[j].CreatedAt)
	})

	var unpinned []Manifest
	for _, m := range manifests {
		if !m.Pinned && !keep[m.ID] {
			unpinned = append(unpinned, m)
		}
	}

	if len(unpinned) <= retentionCount {
		return nil, nil
	}

	toDelete := unpinned[retentionCount:]
	var deleted []string
	for _, m := range toDelete {
		// A manifest is only prunable when its root is contained in the
		// backups directory being pruned; anything else fails closed.
		if err := containedPath(base, filepath.Clean(m.RootDir)); err != nil {
			log.Printf("backup: prune: refusing uncontained root %q: %v", m.RootDir, err)
			continue
		}
		if err := DeleteBackup(m); err != nil {
			log.Printf("backup: prune: failed to delete %q: %v", m.RootDir, err)
			continue
		}
		deleted = append(deleted, m.ID)
	}

	return deleted, nil
}

// listManifestsForRetention reads all backup directories inside backupDir and
// returns the parsed manifests. Subdirectories without a readable manifest.json
// are skipped silently (they don't count toward retention).
func listManifestsForRetention(backupDir string) ([]Manifest, error) {
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("list backup dir %q: %w", backupDir, err)
	}

	var manifests []Manifest
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		manifestPath := filepath.Join(backupDir, e.Name(), ManifestFilename)
		m, err := ReadManifest(manifestPath)
		if err != nil {
			continue
		}
		manifests = append(manifests, m)
	}

	return manifests, nil
}
