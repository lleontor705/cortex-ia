package install

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/lleontor705/cortex-ia/internal/backup"
	"github.com/lleontor705/cortex-ia/internal/pipeline"
	"github.com/lleontor705/cortex-ia/internal/state"
)

// commitStateV2 and commitLockV2 are the metadata commit seams. Production
// code always routes through the state package; the indirection exists so
// transactional recovery can be proven under injected commit failures
// without ever touching a real home, mirroring the state package's own
// writeFileAtomic seam.
var (
	commitStateV2 = state.SaveMetadataV2
	commitLockV2  = state.SaveLockV2
)

// serviceTxn groups one service-level mutation set (managed MCP add/remove
// or uninstall) into a journal-backed transaction with the same contract as
// the engine's apply phase: every declared target is backed up, proven
// restorable, and preimaged before the first write; each completed mutation
// records its verified postimage; and any later failure restores the exact
// preimages in reverse order and verifies the restoration.
type serviceTxn struct {
	journal  *pipeline.InstallJournal
	homeDir  string
	backupID string
}

// txnStatus is the honest restore accounting surfaced on receipts whenever
// a transaction fails after its first potential mutation.
type txnStatus struct {
	// Restored reports that a failure path completed a verified inverse
	// restoration of every mutated target.
	Restored bool
	// RestoreError describes a restoration that could not be verified; the
	// journal checkpoint is retained for a safe retry.
	RestoreError string
}

// beginServiceTxn captures and verifies the retained backup, then begins
// the install journal over every declared target: the caller's extra
// targets plus both MCP config candidates and the v2 state and lock files.
// The journal checkpoint lives inside the backup directory so transaction
// evidence can never become an untracked managed write.
func (s *Service) beginServiceTxn(prefix string, now time.Time, extraTargets ...string) (*serviceTxn, error) {
	backupID := timestampedID(prefix, now)
	if !validBackupID.MatchString(backupID) {
		return nil, fmt.Errorf("internal backup ID %q is invalid", backupID)
	}
	paths := make([]string, 0, len(extraTargets)+4)
	seen := make(map[string]bool, len(extraTargets)+4)
	add := func(abs string) {
		clean := filepath.Clean(abs)
		if clean != "" && !seen[clean] {
			seen[clean] = true
			paths = append(paths, clean)
		}
	}
	for _, abs := range extraTargets {
		add(abs)
	}
	for _, abs := range s.mcpConfigCandidatesAbs() {
		add(abs)
	}
	add(state.StatePath(s.homeDir))
	add(state.LockPath(s.homeDir))

	snapshotDir := filepath.Join(s.homeDir, ".cortex-ia", "backups", backupID)
	manifest, err := backup.NewSnapshotter().Create(snapshotDir, paths)
	if err != nil {
		return nil, err
	}
	if err := backup.Verify(manifest); err != nil {
		return nil, fmt.Errorf("backup is not restorable: %w", err)
	}

	targets := make([]pipeline.ManagedTarget, 0, len(paths))
	for _, abs := range paths {
		rel, err := filepath.Rel(s.homeDir, abs)
		if err != nil {
			return nil, fmt.Errorf("resolve transaction target %q: %w", abs, err)
		}
		targets = append(targets, pipeline.ManagedTarget{
			Path:  filepath.ToSlash(rel),
			Kind:  pipeline.TargetFile,
			Owner: prefix,
		})
	}
	journal, err := pipeline.BeginInstallJournal(s.homeDir, filepath.Join(snapshotDir, "journal"), targets)
	if err != nil {
		return nil, err
	}
	return &serviceTxn{journal: journal, homeDir: s.homeDir, backupID: backupID}, nil
}

// run executes exactly one declared mutation and records its verified
// postimage in the journal.
func (t *serviceTxn) run(abs string, mutate func() error) error {
	if err := mutate(); err != nil {
		return err
	}
	return t.record(abs)
}

// record captures the current image of one declared target as a verified
// journal outcome. It is used both after run mutations and after mutations
// performed by collaborators (the MCP manager) on declared targets.
func (t *serviceTxn) record(abs string) error {
	image, err := pipeline.InspectOutcome(t.homeDir, t.rel(abs))
	if err != nil {
		return err
	}
	return t.journal.Record(image)
}

// commitState persists the updated MCP record set as agreeing v2 state and
// lock documents, journaling both writes so a failure restores the exact
// pre-transaction metadata bytes.
func (t *serviceTxn) commitState(meta state.MetadataV2, records []state.MCPV2, now time.Time) error {
	meta.MCPs = records
	meta.UpdatedAt = now
	if err := t.run(state.StatePath(t.homeDir), func() error {
		return commitStateV2(t.homeDir, meta)
	}); err != nil {
		return err
	}
	lock := state.NewLockFromMetadataV2(meta)
	return t.run(state.LockPath(t.homeDir), func() error {
		return commitLockV2(t.homeDir, lock)
	})
}

// abort restores every preimage in reverse order and verifies the
// restoration. The returned error joins the stage error with the restore
// verdict so neither can mask the other; an unverifiable restoration keeps
// the journal checkpoint for a safe retry.
func (t *serviceTxn) abort(status *txnStatus, stage string, err error) error {
	wrapped := fmt.Errorf("%s: %w", stage, err)
	mutated := len(t.journal.Outcomes) > 0
	if restoreErr := t.journal.RestoreAndVerify(); restoreErr != nil {
		status.RestoreError = restoreErr.Error()
		return fmt.Errorf("%w; reverse restore failed, journal retained for safe retry: %v", wrapped, restoreErr)
	}
	status.Restored = mutated
	return wrapped
}

// commit marks the transaction committed after every acceptance check
// passed. A commit failure itself triggers a verified restoration.
func (t *serviceTxn) commit() error {
	return t.journal.Commit()
}

// rel converts an absolute target under the transaction home into its
// slash-relative journal declaration.
func (t *serviceTxn) rel(abs string) string {
	rel, err := filepath.Rel(t.homeDir, abs)
	if err != nil {
		return filepath.ToSlash(abs)
	}
	return filepath.ToSlash(rel)
}

// mcpConfigCandidatesAbs lists both OpenCode global config files the manager
// may resolve, as absolute paths under the service home, mirroring the
// engine's candidate declaration.
func (s *Service) mcpConfigCandidatesAbs() []string {
	root, err := opencodeRoot(s.homeDir)
	if err != nil {
		return nil
	}
	return []string{
		filepath.Join(root, "opencode.json"),
		filepath.Join(root, "opencode.jsonc"),
	}
}
