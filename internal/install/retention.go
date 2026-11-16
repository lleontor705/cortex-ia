package install

import (
	"os"
	"path/filepath"

	"github.com/lleontor705/cortex-ia/v2/internal/backup"
	"github.com/lleontor705/cortex-ia/v2/internal/logging"
	"github.com/lleontor705/cortex-ia/v2/internal/pipeline"
	"github.com/lleontor705/cortex-ia/v2/internal/state"
)

// pruneBackups applies the default backup retention under the caller's home
// lock, after a journal commit. Protected backups are never evicted: the one
// the current installation records for rollback, and every backup whose
// journal is still incomplete or cannot be decoded (fail closed). Failure is
// logged, never returned, because the mutation it follows already committed.
func pruneBackups(home string) {
	root := filepath.Join(home, ".cortex-ia", "backups")
	keep := map[string]bool{}
	if meta := state.LoadMetadataV2(home); meta.Presence == state.PresenceV2 && meta.Metadata.BackupID != "" {
		keep[meta.Metadata.BackupID] = true
	}
	checkpoints, err := pipeline.DiscoverJournals(root)
	if err != nil {
		logging.Debugf("install.retention.discover home=%s error=%v", home, err)
		return
	}
	for _, checkpoint := range checkpoints {
		_, backupID := journalIDFromCheckpoint(checkpoint)
		journal, loadErr := pipeline.LoadInstallJournal(checkpoint)
		if loadErr != nil || pipeline.JournalPending(journal.State) {
			keep[backupID] = true
		}
	}
	if _, err := os.Stat(root); err != nil {
		return
	}
	deleted, err := backup.PruneKeeping(root, backup.DefaultRetentionCount, keep)
	if err != nil {
		logging.Debugf("install.retention.prune home=%s error=%v", home, err)
		return
	}
	if len(deleted) > 0 {
		logging.Debugf("install.retention.pruned home=%s count=%d", home, len(deleted))
	}
}
