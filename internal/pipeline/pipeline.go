package pipeline

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/backup"
	"github.com/lleontor705/cortex-ia/internal/state"
)

// validBackupID matches safe backup IDs (alphanumeric, hyphens, underscores).
var validBackupID = regexp.MustCompile(`^[a-zA-Z0-9\-_]+$`)

// ResolveRollback resolves a rollback request to its manifest without
// restoring anything. An empty backupID resolves to the backup recorded in
// the legacy v1 state and lock; v2 homes resolve their backup through the
// install service, which reads the v2 metadata first and only falls back to
// this legacy resolution when no v2 metadata exists. The returned ID is the
// validated backup identifier the manifest was read from; callers pass it
// back into JournaledRestore so the journal checkpoint root can never be
// steered by manifest-supplied data.
func ResolveRollback(homeDir, backupID string) (string, backup.Manifest, error) {
	if backupID == "" {
		installed, err := state.Load(homeDir)
		if err != nil {
			return "", backup.Manifest{}, err
		}
		lock, err := state.LoadLock(homeDir)
		if err != nil {
			return "", backup.Manifest{}, err
		}
		backupID = firstNonEmptyString(lock.LastBackupID, installed.LastBackupID)
	}

	if backupID == "" {
		return "", backup.Manifest{}, fmt.Errorf("no backup available for rollback")
	}
	if !validBackupID.MatchString(backupID) {
		return "", backup.Manifest{}, fmt.Errorf("invalid backup ID format: %q", backupID)
	}

	checkpointRoot := filepath.Join(homeDir, ".cortex-ia", "backups", backupID)
	manifestPath := filepath.Join(checkpointRoot, backup.ManifestFilename)
	manifest, err := backup.ReadManifest(manifestPath)
	if err != nil {
		return "", backup.Manifest{}, err
	}
	if err := manifest.ValidateForRestore(checkpointRoot, backupID); err != nil {
		return "", backup.Manifest{}, fmt.Errorf("rollback manifest does not match expected checkpoint: %w", err)
	}
	if err := backup.Verify(manifest); err != nil {
		return "", backup.Manifest{}, fmt.Errorf("backup is not restorable: %w", err)
	}
	return backupID, manifest, nil
}

// Rollback restores managed files from a previous backup manifest through
// the same containment-validated, journaled restore every service rollback
// uses: the manifest must prove restorable, every entry must be a
// home-contained file target, the pre-rollback bytes of every target are
// journaled before the first write, and any later restore, verification, or
// commit failure reverts the partial restoration to the exact pre-rollback
// bytes (the rollback of the rollback). An empty backupID resolves to the
// backup recorded in the legacy v1 state and lock; v2 homes resolve their
// backup through the install service, which additionally runs the ownership
// preflight and holds the canonical home lock around this restore. Callers
// of this engine-level entry must already serialize the home.
func Rollback(homeDir, backupID string) (backup.Manifest, error) {
	id, manifest, err := ResolveRollback(homeDir, backupID)
	if err != nil {
		return backup.Manifest{}, err
	}
	if err := JournaledRestore(homeDir, id, manifest); err != nil {
		return backup.Manifest{}, err
	}
	return manifest, nil
}

// RestoreManifestFn is the manifest restoration seam. Production code
// always routes through backup.RestoreService; the exported indirection
// exists so adversarial verification in other packages can prove the
// rollback-of-rollback contract under injected partial restoration failures
// without ever touching a real home, mirroring backup.BackupRootFn.
var RestoreManifestFn = func(manifest backup.Manifest) error {
	return (backup.RestoreService{}).Restore(manifest)
}

// JournaledRestore executes the manifest restoration as a journaled
// transaction. Every manifest entry must be a home-contained file target
// (absolute, traversal, alias, duplicate, or escaping manifests fail closed
// before any write); the journal then captures the current bytes of every
// target as preimages before the first restore write, and any later
// restore, verification, or commit failure reverts the partial restoration
// to the exact pre-rollback bytes. The journal checkpoint lives inside the
// backup's own journal root, so a crash mid-rollback leaves a recoverable
// candidate exactly like every other transaction. Callers must already hold
// the canonical home lock.
func JournaledRestore(homeDir, backupID string, manifest backup.Manifest) error {
	checkpointRoot := filepath.Join(homeDir, ".cortex-ia", "backups", backupID)
	if err := manifest.ValidateForRestore(checkpointRoot, backupID); err != nil {
		return fmt.Errorf("rollback: %w", err)
	}
	if err := backup.Verify(manifest); err != nil {
		return fmt.Errorf("rollback: backup is not restorable: %w", err)
	}

	targets, err := rollbackJournalTargets(homeDir, manifest)
	if err != nil {
		return fmt.Errorf("rollback: %w", err)
	}
	var journal *InstallJournal
	if len(targets) > 0 {
		journalRoot := filepath.Join(checkpointRoot, "journal")
		journal, err = BeginInstallJournal(homeDir, journalRoot, targets)
		if err != nil {
			return fmt.Errorf("rollback: begin journaled restore: %w", err)
		}
	}
	restoreErr := RestoreManifestFn(manifest)
	if restoreErr == nil {
		restoreErr = verifyRestoredManifest(manifest)
	}
	if restoreErr == nil && journal != nil {
		if err := journal.Commit(); err != nil {
			// The restoration cannot be durably recorded as complete, so
			// the home must not keep an unprovable postimage: revert to
			// the verified pre-rollback bytes and report failure.
			restoreErr = fmt.Errorf("commit rollback journal: %w", err)
		}
	}
	if restoreErr == nil {
		return nil
	}
	if journal == nil {
		return fmt.Errorf("rollback: %w", restoreErr)
	}
	if revertErr := journal.RestoreAndVerify(); revertErr != nil {
		return fmt.Errorf("rollback: %w; reverting the partial restoration also failed, journal retained for safe retry: %v", restoreErr, revertErr)
	}
	return fmt.Errorf("rollback: %w; the partial restoration was reverted to the pre-rollback bytes", restoreErr)
}

// rollbackJournalTargets converts manifest entries into declared journal
// targets relative to the home. Every entry must be strictly
// home-contained: absolute, traversal, alias, duplicate, or
// checkpoint-escaping manifests fail closed here, before any write.
func rollbackJournalTargets(homeDir string, manifest backup.Manifest) ([]ManagedTarget, error) {
	targets := make([]ManagedTarget, 0, len(manifest.Entries))
	seen := make(map[string]bool, len(manifest.Entries))
	for _, entry := range manifest.Entries {
		clean := filepath.Clean(entry.OriginalPath)
		if !pathUnderHome(homeDir, clean) {
			return nil, fmt.Errorf("backup entry %q escapes the service home; failing closed", entry.OriginalPath)
		}
		rel, err := filepath.Rel(homeDir, clean)
		if err != nil {
			return nil, fmt.Errorf("resolve backup entry %q: %w", entry.OriginalPath, err)
		}
		slash := filepath.ToSlash(rel)
		if seen[slash] {
			return nil, fmt.Errorf("backup entry %q is declared more than once; failing closed", slash)
		}
		seen[slash] = true
		targets = append(targets, ManagedTarget{Path: slash, Kind: TargetFile, Owner: "rollback"})
	}
	return targets, nil
}

// verifyRestoredManifest proves the restore actually converged: every entry
// that existed pre-backup owns its snapshot bytes again and every created
// target is gone.
func verifyRestoredManifest(manifest backup.Manifest) error {
	for _, entry := range manifest.Entries {
		if !entry.Existed {
			_, statErr := os.Lstat(entry.OriginalPath)
			if statErr == nil {
				return fmt.Errorf("restored target %q must be absent", entry.OriginalPath)
			}
			if !os.IsNotExist(statErr) {
				return fmt.Errorf("verify absence of %q: %w", entry.OriginalPath, statErr)
			}
			continue
		}
		exists, digest, err := inspectFileTarget(entry.OriginalPath)
		if err != nil {
			return fmt.Errorf("verify restored %q: %w", entry.OriginalPath, err)
		}
		if !exists || digest != entry.SHA256 {
			return fmt.Errorf("restored target %q does not match the backup bytes", entry.OriginalPath)
		}
	}
	return nil
}

// pathUnderHome reports whether abs is strictly inside home.
func pathUnderHome(home, abs string) bool {
	rel, err := filepath.Rel(home, abs)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
