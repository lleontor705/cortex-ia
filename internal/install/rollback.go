package install

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lleontor705/cortex-ia/internal/assets"
	"github.com/lleontor705/cortex-ia/internal/backup"
	"github.com/lleontor705/cortex-ia/internal/components/filemerge"
	"github.com/lleontor705/cortex-ia/internal/mcpmanager"
	"github.com/lleontor705/cortex-ia/internal/pipeline"
	"github.com/lleontor705/cortex-ia/internal/state"
)

// ErrRollbackDrift reports that the current installation no longer matches
// the recorded ownership, so rollback refuses every inverse mutation and
// leaves the home byte-identical to how it was found.
var ErrRollbackDrift = errors.New("rollback: current installation no longer matches the recorded ownership (drift); refusing to restore")

// configMCPKey mirrors the manager's MCP object key; it is repeated here
// only to strip managed entries during the rollback preflight's semantic
// comparison, which never mutates the document.
const configMCPKey = "mcp"

// RollbackReceipt is the typed outcome of a rollback.
type RollbackReceipt struct {
	// BackupID identifies the restored backup.
	BackupID string `json:"backup_id"`
	// Verified reports the manifest was proven restorable before the
	// restore began and the restored bytes were proven afterwards.
	Verified bool `json:"verified"`
	// Restored lists the files rewritten to their backed-up bytes.
	Restored []string `json:"restored,omitempty"`
	// Removed lists the files deleted because they did not exist when the
	// backup was captured.
	Removed []string `json:"removed,omitempty"`
}

// Rollback restores the pre-run state captured by a backup manifest. An
// empty backupID resolves to the backup recorded in v2 metadata; homes
// without v2 metadata fall back to the engine's legacy resolution. Both
// paths converge on one unified restore: the manifest must prove
// restorable, every entry must pass home-contained manifest validation, and
// the restore itself is journaled so a partial restoration is reverted to
// the exact pre-rollback bytes (the rollback of the rollback). An agreed
// v2 installation must additionally pass the ownership preflight: every
// managed artifact must still own its recorded digest, every recorded MCP
// entry must still be accredited, and both config candidates the restore
// will rewrite must differ from their preimages only by managed entries.
// Any drift aborts with zero mutations.
//
// The canonical home lock is acquired before any resolution or validation
// and held through the verified restore, so the ownership evidence the
// preflight checks and the preimages the journal captures are the same
// serialized state the restore acts on. Only a fully verified and durably
// committed restoration reports success.
func (s *Service) Rollback(backupID string) (*RollbackReceipt, error) {
	release, err := s.lockForMutation(0)
	if err != nil {
		return nil, fmt.Errorf("rollback: %w", err)
	}
	defer release()

	metaLoad := state.LoadMetadataV2(s.homeDir)
	lockLoad := state.LoadLockV2(s.homeDir)
	id := backupID
	legacy := false
	if id == "" {
		switch metaLoad.Presence {
		case state.PresenceV2:
			id = metaLoad.Metadata.BackupID
			if id == "" {
				return nil, fmt.Errorf("rollback: v2 metadata records no backup")
			}
		case state.PresenceMalformed:
			return nil, fmt.Errorf("rollback: state metadata is malformed: %s", metaLoad.Detail)
		default:
			// Absent or legacy: delegate to the engine's legacy backup
			// resolution, which reads the v1 state and lock.
			legacy = true
		}
	}
	if legacy {
		// Legacy homes carry no digest evidence, so no ownership preflight
		// exists for them; the restore itself still passes the same
		// manifest validation and journaled rollback-of-rollback every
		// rollback uses.
		resolvedID, manifest, resolveErr := pipeline.ResolveRollback(s.homeDir, "")
		if resolveErr != nil {
			return nil, fmt.Errorf("rollback: %w", resolveErr)
		}
		return s.restoredRollback(resolvedID, manifest, metaLoad, lockLoad)
	}
	if !validBackupID.MatchString(id) {
		return nil, fmt.Errorf("rollback: invalid backup ID format: %q", id)
	}
	resolvedID, manifest, err := pipeline.ResolveRollback(s.homeDir, id)
	if err != nil {
		return nil, fmt.Errorf("rollback: %w", err)
	}
	return s.restoredRollback(resolvedID, manifest, metaLoad, lockLoad)
}

// restoredRollback verifies one resolved manifest restorable, runs the
// ownership preflight an agreed v2 installation demands, and executes the
// journaled restore. It runs entirely under the caller-held home lock and
// fails closed before the first write on every refusal.
func (s *Service) restoredRollback(backupID string, manifest backup.Manifest, metaLoad state.MetadataLoad, lockLoad state.LockLoad) (*RollbackReceipt, error) {
	if err := backup.Verify(manifest); err != nil {
		return nil, fmt.Errorf("rollback: backup is not restorable: %w", err)
	}

	// Ownership preflight: only an agreed, undrifted v2 installation may be
	// inverse-mutated. This runs for explicitly supplied backup IDs too,
	// because the guard protects the current installation, not the backup.
	switch metaLoad.Presence {
	case state.PresenceV2:
		if lockLoad.Presence != state.PresenceV2 {
			return nil, fmt.Errorf("rollback: v2 state without an agreeing v2 lock (lock=%s); ownership cannot be proven, failing closed", lockLoad.Presence)
		}
		if err := state.CheckAgreementV2(metaLoad.Metadata, lockLoad.Lock); err != nil {
			return nil, fmt.Errorf("rollback: state/lock disagree: %w", err)
		}
		if err := s.rollbackPreflight(manifest, metaLoad.Metadata); err != nil {
			return nil, err
		}
	case state.PresenceMalformed:
		return nil, fmt.Errorf("rollback: state metadata is malformed: %s", metaLoad.Detail)
	default:
		// Legacy or no metadata with an explicit backup ID: the v1 surface
		// records no digests, so the restore stays snapshot-verified only,
		// exactly like the engine's legacy rollback.
	}

	if err := pipeline.JournaledRestore(s.homeDir, backupID, manifest); err != nil {
		return nil, err
	}
	receipt := rollbackReceiptFromManifest(manifest)
	receipt.Verified = true
	return receipt, nil
}

// rollbackPreflight proves the current home still owns the recorded
// installation before any restore may run. Every check is read-only; any
// drift, unknown backup target, or escaped path aborts with zero mutations.
func (s *Service) rollbackPreflight(manifest backup.Manifest, meta state.MetadataV2) error {
	// Every managed artifact except the merged settings file must still
	// own its recorded bytes; the settings file is judged semantically
	// below because managed MCP churn legitimately extends it.
	for _, artifact := range meta.Artifacts {
		if artifact.Ownership != state.OwnershipManaged || artifact.Kind == state.KindMCPConfig {
			continue
		}
		abs := filepath.Join(meta.OpencodeRoot, filepath.FromSlash(artifact.Path))
		rel := homeRelative(s.homeDir, abs)
		exists, digest, err := fileDigest(abs)
		switch {
		case err != nil:
			return fmt.Errorf("%w: inspect %q: %v", ErrRollbackDrift, rel, err)
		case !exists:
			return fmt.Errorf("%w: managed artifact %q is missing", ErrRollbackDrift, rel)
		case digest != artifact.Digest:
			return fmt.Errorf("%w: managed artifact %q changed after install", ErrRollbackDrift, rel)
		}
	}

	// The restore only touches manifest entries, so every entry must be a
	// known, home-contained target: a managed artifact, an MCP config
	// candidate, or the v2 state and lock files. Anything else fails closed.
	known := make(map[string]bool, len(meta.Artifacts)+4)
	known[filepath.Clean(state.StatePath(s.homeDir))] = true
	known[filepath.Clean(state.LockPath(s.homeDir))] = true
	for _, candidate := range s.mcpConfigCandidatesAbs() {
		known[filepath.Clean(candidate)] = true
	}
	for _, artifact := range meta.Artifacts {
		if artifact.Ownership == state.OwnershipManaged {
			known[filepath.Clean(filepath.Join(meta.OpencodeRoot, filepath.FromSlash(artifact.Path)))] = true
		}
	}
	candidates := make(map[string]bool, 2)
	for _, candidate := range s.mcpConfigCandidatesAbs() {
		candidates[filepath.Clean(candidate)] = true
	}
	for _, entry := range manifest.Entries {
		clean := filepath.Clean(entry.OriginalPath)
		if !pathUnderHome(s.homeDir, clean) {
			return fmt.Errorf("rollback: backup entry %q escapes the service home; failing closed", entry.OriginalPath)
		}
		if !known[clean] {
			return fmt.Errorf("rollback: backup entry %q is not an ownership-accredited target; failing closed", homeRelative(s.homeDir, clean))
		}
	}

	// Every MCP entry recorded as managed must still be accredited in the
	// effective config: user edits, deletions, or hand-crafted equivalents
	// are drift.
	manager := mcpmanager.New(s.homeDir)
	listing, err := manager.List(ownershipEvidence(meta))
	if err != nil {
		return fmt.Errorf("rollback: assess MCP ownership: %w", err)
	}
	statuses := make(map[string]mcpmanager.EntryStatus, len(listing.Entries))
	for _, report := range listing.Entries {
		statuses[report.Name] = report.Status
	}
	for _, mcp := range meta.MCPs {
		if mcp.Ownership != state.OwnershipManaged {
			continue
		}
		if statuses[mcp.Name] != mcpmanager.StatusManaged {
			return fmt.Errorf("%w: managed MCP %q is not accredited anymore (status %q)", ErrRollbackDrift, mcp.Name, statuses[mcp.Name])
		}
	}

	// Both config candidates the restore will rewrite must differ from
	// their captured preimage only by managed entries and the settings
	// template the installer legitimately merges. Managed names and
	// template values are stripped from both sides, so legitimate MCP churn
	// and template merges cancel out while any unrelated user change (or a
	// foreign entry added after the backup) aborts the restore.
	managedNames := make([]string, 0, len(meta.MCPs))
	for _, mcp := range meta.MCPs {
		if mcp.Ownership == state.OwnershipManaged {
			managedNames = append(managedNames, mcp.Name)
		}
	}
	template, err := settingsTemplate()
	if err != nil {
		return fmt.Errorf("rollback: read settings template: %w", err)
	}
	for _, entry := range manifest.Entries {
		clean := filepath.Clean(entry.OriginalPath)
		if !candidates[clean] {
			continue
		}
		rel := homeRelative(s.homeDir, clean)
		exists, _, err := fileDigest(clean)
		if err != nil {
			return fmt.Errorf("%w: inspect %q: %v", ErrRollbackDrift, rel, err)
		}
		if !exists {
			if entry.Existed {
				return fmt.Errorf("%w: config %q was deleted after the backup", ErrRollbackDrift, rel)
			}
			continue
		}
		currentRaw, err := os.ReadFile(clean)
		if err != nil {
			return fmt.Errorf("%w: read %q: %v", ErrRollbackDrift, rel, err)
		}
		current, err := filemerge.DecodeJSONObject(currentRaw)
		if err != nil {
			return fmt.Errorf("%w: decode %q: %v", ErrRollbackDrift, rel, err)
		}
		stripManagedEntries(current, managedNames)
		stripTemplateValues(current, template)
		want := map[string]any{}
		if entry.Existed {
			preimageRaw, err := os.ReadFile(entry.SnapshotPath)
			if err != nil {
				return fmt.Errorf("rollback: read config preimage for %q: %w", rel, err)
			}
			want, err = filemerge.DecodeJSONObject(preimageRaw)
			if err != nil {
				return fmt.Errorf("%w: decode preimage of %q: %v", ErrRollbackDrift, rel, err)
			}
			stripManagedEntries(want, managedNames)
			stripTemplateValues(want, template)
		}
		if !semanticDocumentsEqual(current, want) {
			return fmt.Errorf("%w: unrelated configuration in %q changed after the backup", ErrRollbackDrift, rel)
		}
	}
	return nil
}

// rollbackReceiptFromManifest splits the manifest entries into restored
// (existed pre-backup) and removed (created after the backup) paths.
func rollbackReceiptFromManifest(manifest backup.Manifest) *RollbackReceipt {
	receipt := &RollbackReceipt{BackupID: manifest.ID}
	for _, entry := range manifest.Entries {
		if entry.Existed {
			receipt.Restored = append(receipt.Restored, entry.OriginalPath)
		} else {
			receipt.Removed = append(receipt.Removed, entry.OriginalPath)
		}
	}
	return receipt
}

// stripManagedEntries removes the named managed MCP entries from a decoded
// config document, normalizing an emptied MCP object away so documents that
// differ only by managed churn compare equal.
func stripManagedEntries(document map[string]any, managedNames []string) {
	entries, isMap := document[configMCPKey].(map[string]any)
	if !isMap {
		return
	}
	for _, name := range managedNames {
		delete(entries, name)
	}
	if len(entries) == 0 {
		delete(document, configMCPKey)
	}
}

// semanticDocumentsEqual compares two decoded JSONC documents by canonical
// value equality; formatting and comment differences are intentionally
// ignored.
func semanticDocumentsEqual(a, b map[string]any) bool {
	left, err := json.Marshal(a)
	if err != nil {
		return false
	}
	right, err := json.Marshal(b)
	if err != nil {
		return false
	}
	return string(left) == string(right)
}

// stripTemplateValues removes the embedded settings template's values from
// a decoded config document, recursing into objects and deleting only
// exactly equal values, so a key the user legitimately overrode survives.
// An object emptied by stripping is removed to keep both compared sides
// normalized.
func stripTemplateValues(document, template map[string]any) {
	for key, overlay := range template {
		base, ok := document[key]
		if !ok {
			continue
		}
		overlayMap, overlayIsMap := overlay.(map[string]any)
		baseMap, baseIsMap := base.(map[string]any)
		switch {
		case overlayIsMap && baseIsMap:
			stripTemplateValues(baseMap, overlayMap)
			if len(baseMap) == 0 {
				delete(document, key)
			}
		case !overlayIsMap && !baseIsMap && jsonValuesEqual(base, overlay):
			delete(document, key)
		}
	}
}

// jsonValuesEqual compares two decoded JSON values by their canonical
// encodings, which are key-sorted and float-normalized by construction.
func jsonValuesEqual(a, b any) bool {
	left, leftErr := json.Marshal(a)
	right, rightErr := json.Marshal(b)
	return leftErr == nil && rightErr == nil && string(left) == string(right)
}

// settingsTemplate decodes the embedded settings overlay (the single
// KindConfig asset) that the installer merges into the settings file.
func settingsTemplate() (map[string]any, error) {
	inventory, err := assets.Inventory()
	if err != nil {
		return nil, err
	}
	for _, file := range inventory {
		if file.Kind != assets.KindConfig {
			continue
		}
		raw, err := assets.ReadBytes(file.Path)
		if err != nil {
			return nil, err
		}
		return filemerge.DecodeJSONObject(raw)
	}
	return nil, errors.New("embedded settings template not found")
}

// pathUnderHome reports whether abs is strictly inside home.
func pathUnderHome(home, abs string) bool {
	rel, err := filepath.Rel(home, abs)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// ErrConfirmationRequired reports a recovery attempt without the explicit
// user confirmation the operation demands. Nothing is written.
var ErrConfirmationRequired = errors.New("install service: recovery requires explicit confirmation")

// ErrJournalNotRecoverable reports a recovery candidate that is not a
// validated pending journal of this home: unknown, corrupt, foreign,
// escaped, drifted, or already terminal. Nothing is written.
var ErrJournalNotRecoverable = errors.New("install service: journal is not a recoverable transaction of this home")

// RecoveryDisposition records the terminal outcome of a recovery attempt.
type RecoveryDisposition string

const (
	// RecoveryRestored: the journaled preimages were restored and the
	// complete restoration was verified; the journal checkpoint now records
	// the restored state, so it is no longer a pending candidate.
	RecoveryRestored RecoveryDisposition = "restored"
	// RecoveryRetained: the restoration could not be verified; the journal
	// checkpoint is retained for a safe retry or manual remediation.
	RecoveryRetained RecoveryDisposition = "retained-for-retry"
)

// RecoveryReceipt is the typed outcome of one journal recovery run.
type RecoveryReceipt struct {
	// JournalID identifies the restored journal checkpoint.
	JournalID string `json:"journal_id"`
	// BackupID locates the backup transaction that left the journal.
	BackupID string `json:"backup_id,omitempty"`
	// Confirmed mirrors the explicit confirmation that authorized the run.
	Confirmed bool `json:"confirmed"`
	// Restored lists the home-relative targets restored to their journaled
	// preimages.
	Restored []string `json:"restored,omitempty"`
	// Verified reports the complete restoration was proven byte-for-byte.
	Verified bool `json:"verified"`
	// Disposition records whether the journal is restored or retained.
	Disposition RecoveryDisposition `json:"disposition"`
	// RestoreError describes an unverified restoration; the journal is
	// retained for a safe retry.
	RestoreError string `json:"restore_error,omitempty"`
}

// RecoverOptions is the explicit per-call intent for journal recovery.
type RecoverOptions struct {
	// Confirmed carries the user's explicit confirmation for exactly this
	// journal ID; recovery without it fails closed with
	// ErrConfirmationRequired and writes nothing.
	Confirmed bool
	// LockTimeout bounds canonical-home lock acquisition. Zero or negative
	// uses DefaultHomeLockTimeout.
	LockTimeout time.Duration
}

// Recover restores one pending journal after explicit confirmation. The
// candidate set is resolved read-only first, then the canonical home lock
// is acquired and the candidate is reloaded and revalidated under it: the
// checkpoint must decode, obey the journal contract (schema, containment,
// no aliases), and target exactly this home. RestoreAndVerify then proves
// every recorded postimage still holds before any inverse mutation,
// restores the preimages in reverse order, and verifies the complete
// result. Success durably records the restored disposition on the journal
// checkpoint; a failed verification retains the journal for a safe retry.
// Nothing is written before confirmation or lock acquisition.
func (s *Service) Recover(journalID string, opts RecoverOptions) (*RecoveryReceipt, error) {
	receipt := &RecoveryReceipt{JournalID: journalID, Confirmed: opts.Confirmed}
	if strings.TrimSpace(journalID) == "" {
		return receipt, fmt.Errorf("%w: empty journal ID", ErrJournalNotRecoverable)
	}
	candidate, err := s.findJournalCandidate(journalID)
	if err != nil {
		return receipt, err
	}
	if !opts.Confirmed {
		return receipt, fmt.Errorf("%w: review the candidate and re-run with confirmation", ErrConfirmationRequired)
	}
	release, err := s.lockForMutation(opts.LockTimeout)
	if err != nil {
		return receipt, fmt.Errorf("recover: %w", err)
	}
	defer release()

	// Reload under the lock: the candidate set can change while waiting.
	candidate, err = s.findJournalCandidate(journalID)
	if err != nil {
		return receipt, err
	}
	journal, loadErr := pipeline.LoadInstallJournal(candidate.CheckpointPath)
	if loadErr != nil {
		return receipt, fmt.Errorf("%w: %v", ErrJournalNotRecoverable, loadErr)
	}
	if err := journal.RestoreAndVerify(); err != nil {
		receipt.BackupID = candidate.BackupID
		receipt.Disposition = RecoveryRetained
		receipt.RestoreError = err.Error()
		return receipt, fmt.Errorf("recover %q: restoration could not be verified; journal retained for safe retry: %w", journalID, err)
	}
	receipt.BackupID = candidate.BackupID
	receipt.Disposition = RecoveryRestored
	receipt.Verified = true
	for _, entry := range journal.Entries {
		receipt.Restored = append(receipt.Restored, entry.Path)
	}
	return receipt, nil
}

// findJournalCandidate locates one pending journal by ID through the same
// read-only enumeration doctor uses, failing closed on unknown, corrupt, or
// foreign candidates.
func (s *Service) findJournalCandidate(journalID string) (JournalCandidate, error) {
	candidates, err := s.PendingJournals()
	if err != nil {
		return JournalCandidate{}, fmt.Errorf("recover: %w", err)
	}
	for _, candidate := range candidates {
		if candidate.ID != journalID {
			continue
		}
		if !candidate.Recoverable {
			return candidate, fmt.Errorf("%w: %s", ErrJournalNotRecoverable, candidate.Reason)
		}
		return candidate, nil
	}
	return JournalCandidate{}, fmt.Errorf("%w: no pending journal %q in this home", ErrJournalNotRecoverable, journalID)
}
