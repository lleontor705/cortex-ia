package install

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/lleontor705/cortex-ia/internal/mcpmanager"
	"github.com/lleontor705/cortex-ia/internal/state"
)

// UninstallOptions is the explicit per-call intent for uninstall. The zero
// value is a real run.
type UninstallOptions struct {
	DryRun bool
	// LockTimeout bounds canonical-home lock acquisition for the mutating
	// path. Zero or negative uses install.DefaultHomeLockTimeout. Dry-runs
	// and not-installed homes never acquire the lock.
	LockTimeout time.Duration
	// Now overrides the clock for deterministic runs.
	Now func() time.Time
}

func (o UninstallOptions) now() time.Time {
	if o.Now != nil {
		return o.Now().UTC()
	}
	return time.Now().UTC()
}

// RetainedItem records one file or MCP entry uninstall refused to touch
// because ownership or digest evidence could not prove it safe.
type RetainedItem struct {
	// Target is the home-relative path or MCP server name.
	Target string `json:"target"`
	Reason string `json:"reason"`
}

// UninstallReceipt is the typed outcome of an uninstall run. Every removal
// is ownership- and digest-accredited: only artifacts recorded as managed in
// agreed v2 metadata whose current bytes still match the recorded digest are
// deleted, and only MCP entries accredited by matching ownership records are
// deregistered. Everything else — pre-existing equal files, user-modified
// artifacts, legacy-unverified content — is retained and reported.
type UninstallReceipt struct {
	DryRun bool `json:"dry_run"`
	// NotInstalled reports that no v2 installation metadata exists, so
	// nothing was touched.
	NotInstalled bool `json:"not_installed"`
	// Removed lists home-relative artifacts deleted this run.
	Removed []string `json:"removed,omitempty"`
	// Preserved lists managed documents intentionally kept because they are
	// co-owned with the user: the merged settings document is extended by
	// design (user keys, managed MCP entries), so it is deleted only when
	// prior-presence evidence proves cortex-ia created it and its bytes
	// are unchanged. Its managed MCP entries are always removed
	// individually through the manager.
	Preserved []string `json:"preserved,omitempty"`
	// AlreadyAbsent lists recorded artifacts already missing on disk.
	AlreadyAbsent []string `json:"already_absent,omitempty"`
	// RemovedDirs lists directories pruned after becoming empty; pruning
	// uses os.Remove only, so a non-empty directory is never removed.
	RemovedDirs []string `json:"removed_dirs,omitempty"`
	// MCPRemoved lists managed MCP entries deregistered this run.
	MCPRemoved []string `json:"mcp_removed,omitempty"`
	// Retained lists every target uninstall refused to delete.
	Retained []RetainedItem `json:"retained,omitempty"`
	// StateRemoved reports the v2 state and lock files were deleted after
	// a fully verified uninstall.
	StateRemoved bool `json:"state_removed"`
	// Complete is true only when every deletable artifact and MCP entry was
	// removed (or already absent), nothing was retained, and the v2 state
	// documents were removed. Preserved co-owned documents do not block
	// completion.
	Complete bool `json:"complete"`
	// BackupID locates the verified pre-uninstall backup.
	BackupID string `json:"backup_id,omitempty"`
	// Restored reports a failed run completed a verified inverse
	// restoration of every mutated destination.
	Restored bool `json:"restored,omitempty"`
	// RestoreError describes a restoration that could not be verified; the
	// journal checkpoint is retained for a safe retry.
	RestoreError string `json:"restore_error,omitempty"`
}

// removeArtifact is the file-removal seam. Production code always routes
// through os.Remove; the indirection exists so transactional recovery can
// be proven under injected mid-uninstall failures without touching a real
// home.
var removeArtifact = os.Remove

// Uninstall removes the cortex-ia installation from the service home. It
// fails closed per target: an artifact is deleted only when v2 metadata
// records it as managed and its current digest still matches the record; an
// MCP entry is deregistered only through the manager's ownership-accredited
// removal. The .cortex-ia backup directory is never removed, and no
// directory is ever removed recursively. The mutating path acquires the
// canonical home lock first and then reloads and reclassifies the entire
// removal set under that lock, so a concurrent process's committed
// mutation is never overwritten from stale pre-lock evidence. Dry-runs stay
// read-only and never lock.
func (s *Service) Uninstall(opts UninstallOptions) (*UninstallReceipt, error) {
	metaLoad := state.LoadMetadataV2(s.homeDir)
	lockLoad := state.LoadLockV2(s.homeDir)
	switch {
	case metaLoad.Presence == state.PresenceAbsent && lockLoad.Presence == state.PresenceAbsent:
		return &UninstallReceipt{DryRun: opts.DryRun, NotInstalled: true, Complete: true}, nil
	case metaLoad.Presence == state.PresenceMalformed:
		return nil, fmt.Errorf("uninstall: state metadata is malformed: %s", metaLoad.Detail)
	case lockLoad.Presence == state.PresenceMalformed:
		return nil, fmt.Errorf("uninstall: lock metadata is malformed: %s", lockLoad.Detail)
	case metaLoad.Presence != state.PresenceV2 || lockLoad.Presence != state.PresenceV2:
		// Legacy or partial v2 documents can never accredit ownership.
		return nil, fmt.Errorf("uninstall: no agreed v2 installation metadata (metadata=%s lock=%s); legacy-unverified content is never deleted",
			metaLoad.Presence, lockLoad.Presence)
	}
	if err := state.CheckAgreementV2(metaLoad.Metadata, lockLoad.Lock); err != nil {
		return nil, fmt.Errorf("uninstall: state/lock disagree: %w", err)
	}

	receipt := &UninstallReceipt{DryRun: opts.DryRun}

	if opts.DryRun {
		// Read-only preview: classify every recorded artifact by the
		// ownership and digest evidence on disk right now.
		removable := s.classifyRemovals(metaLoad.Metadata, receipt)
		for _, item := range removable {
			receipt.Removed = append(receipt.Removed, item.rel)
		}
		for _, mcp := range metaLoad.Metadata.MCPs {
			if mcp.Ownership == state.OwnershipManaged {
				receipt.MCPRemoved = append(receipt.MCPRemoved, mcp.Name)
			}
		}
		return receipt, nil
	}

	release, err := s.lockForMutation(opts.LockTimeout)
	if err != nil {
		return receipt, fmt.Errorf("uninstall: %w", err)
	}
	defer release()

	// Reload and reclassify under the lock: another process may have
	// mutated the home before this lock was acquired, so the removal set
	// and the metadata the transaction commits must be derived from the
	// post-lock state, never from the pre-lock read.
	meta, removable, err := s.lockedUninstallContext(receipt)
	if err != nil {
		return receipt, err
	}

	// The whole uninstall is one journal-backed transaction: every declared
	// target (removable artifacts, both config candidates, the v2 state and
	// lock files, and the MCP fingerprint sidecar) is backed up, verified
	// restorable, and preimaged before the first mutation; each completed
	// mutation records a verified postimage; any later failure restores the
	// exact preimages in reverse order. Everything below runs under the
	// canonical home lock held above.
	txnTargets := make([]string, 0, len(removable)+5)
	for _, item := range removable {
		txnTargets = append(txnTargets, item.abs)
	}
	txnTargets = append(txnTargets, state.FingerprintPath(s.homeDir))
	txn, err := s.beginServiceTxn("uninstall", opts.now(), txnTargets...)
	if err != nil {
		return receipt, fmt.Errorf("uninstall: %w", err)
	}
	receipt.BackupID = txn.backupID
	var status txnStatus
	// abort aborts the transaction and immediately mirrors the restore
	// verdict onto the receipt, so a failed run never reports success
	// fields without its restoration evidence.
	abort := func(err error) error {
		aborted := txn.abort(&status, "uninstall", err)
		receipt.Restored, receipt.RestoreError = status.Restored, status.RestoreError
		return aborted
	}

	for _, item := range removable {
		item := item
		if err := txn.run(item.abs, func() error {
			if err := removeArtifact(item.abs); err != nil {
				return fmt.Errorf("remove %q: %w", item.rel, err)
			}
			return nil
		}); err != nil {
			return receipt, abort(err)
		}
		receipt.Removed = append(receipt.Removed, item.rel)
		s.pruneEmptyDirs(item.abs, receipt)
	}

	// MCP deregistration goes through the manager, which enforces the same
	// ownership accreditation as every other mutation. Catalog preset
	// names follow the preset removal path; custom local/remote names
	// follow the accredited custom path exactly like Service.MCPRemove,
	// because preset-only Remove would misclassify every custom name as
	// unmanaged and retain it. Custom removal additionally demands a
	// recorded mcpv2 full-postimage fingerprint: a corrupt or missing
	// fingerprint sidecar leaves the salt empty and the evidence legacy,
	// so drifted or unverifiable customs fail closed and are retained.
	store, storePresent, salt, storeErr := s.fingerprintContext()
	if storeErr != nil {
		// Corrupt local fingerprint evidence can never accredit a
		// destructive removal; keep the typed fail-closed reason in the
		// retained list instead of aborting artifact removal.
		store, storePresent, salt = state.FingerprintDocument{}, false, nil
	}
	manager := mcpmanager.NewFingerprinting(s.homeDir, salt)
	evidence := ownershipEvidence(meta)
	if storePresent {
		evidence = withPostImageEvidence(evidence, store, meta)
	}
	remainingMCPs := make([]state.MCPV2, 0, len(meta.MCPs))
	for _, mcp := range meta.MCPs {
		if mcp.Ownership != state.OwnershipManaged {
			remainingMCPs = append(remainingMCPs, mcp)
			continue
		}
		var result mcpmanager.Result
		var err error
		if _, isCatalog := mcpmanager.Lookup(mcp.Name); isCatalog {
			result, err = manager.Remove(mcp.Name, evidence)
		} else {
			result, err = manager.RemoveCustom(mcp.Name, evidence)
		}
		if err != nil {
			receipt.Retained = append(receipt.Retained, RetainedItem{Target: mcp.Name, Reason: fmt.Sprintf("MCP removal failed closed: %v", err)})
			remainingMCPs = append(remainingMCPs, mcp)
			continue
		}
		if result.Action == "removed" {
			// The manager mutated the declared config target; record its
			// verified postimage so a later failure restores the exact
			// pre-transaction config bytes.
			if err := txn.record(manager.ConfigPath()); err != nil {
				return receipt, abort(err)
			}
		}
		if err := s.dropFingerprintRecordTxn(txn, &store, storePresent, mcp.Name); err != nil {
			return receipt, abort(fmt.Errorf("drop MCP postimage fingerprint: %w", err))
		}
		if result.Action == "removed" || result.Action == "already-absent" {
			receipt.MCPRemoved = append(receipt.MCPRemoved, mcp.Name)
			continue
		}
		remainingMCPs = append(remainingMCPs, mcp)
	}

	if len(receipt.Retained) == 0 {
		// Fully verified uninstall: the state documents themselves are the
		// last managed files to go. Each is removed individually and
		// journaled; the .cortex-ia directory and every backup inside it
		// are kept.
		if err := txn.run(state.StatePath(s.homeDir), func() error {
			if err := removeArtifact(state.StatePath(s.homeDir)); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("remove v2 state: %w", err)
			}
			return nil
		}); err != nil {
			return receipt, abort(err)
		}
		if err := txn.run(state.LockPath(s.homeDir), func() error {
			if err := removeArtifact(state.LockPath(s.homeDir)); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("remove v2 lock: %w", err)
			}
			return nil
		}); err != nil {
			return receipt, abort(err)
		}
		// The local MCP fingerprint sidecar (salt plus mcpv2 records) is
		// state-local ownership evidence with no value outside this
		// installation; a fully verified uninstall removes it with the
		// state documents.
		if err := txn.run(state.FingerprintPath(s.homeDir), func() error {
			if err := removeArtifact(state.FingerprintPath(s.homeDir)); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("remove MCP fingerprint sidecar: %w", err)
			}
			return nil
		}); err != nil {
			return receipt, abort(err)
		}
		if err := txn.commit(); err != nil {
			return receipt, abort(err)
		}
		receipt.StateRemoved = true
		receipt.Complete = true
		receipt.Restored, receipt.RestoreError = status.Restored, status.RestoreError
		return receipt, nil
	}

	// Partial uninstall: keep truthful metadata for everything retained so
	// future doctor, sync, and uninstall runs still have ownership proof.
	meta.Artifacts = s.retainedArtifacts(meta, receipt)
	if err := txn.commitState(meta, remainingMCPs, opts.now()); err != nil {
		return receipt, abort(fmt.Errorf("commit partial state v2: %w", err))
	}
	if err := txn.commit(); err != nil {
		return receipt, abort(err)
	}
	receipt.Restored, receipt.RestoreError = status.Restored, status.RestoreError
	return receipt, nil
}

// removal records one artifact the classification proved deletable this
// run: managed ownership plus an intact on-disk digest.
type removal struct {
	rel string
	abs string
}

// lockedUninstallContext reloads the v2 metadata and lock documents under
// the already-held home lock, re-proves their agreement, and reclassifies
// every recorded artifact from the post-lock disk state. It exists so the
// mutating uninstall never commits metadata or removes artifacts from
// evidence captured before lock acquisition.
func (s *Service) lockedUninstallContext(receipt *UninstallReceipt) (state.MetadataV2, []removal, error) {
	metaLoad := state.LoadMetadataV2(s.homeDir)
	lockLoad := state.LoadLockV2(s.homeDir)
	switch {
	case metaLoad.Presence == state.PresenceMalformed:
		return state.MetadataV2{}, nil, fmt.Errorf("uninstall: state metadata is malformed: %s", metaLoad.Detail)
	case lockLoad.Presence == state.PresenceMalformed:
		return state.MetadataV2{}, nil, fmt.Errorf("uninstall: lock metadata is malformed: %s", lockLoad.Detail)
	case metaLoad.Presence != state.PresenceV2 || lockLoad.Presence != state.PresenceV2:
		// The installation vanished or degraded while waiting for the
		// lock: fail closed rather than delete from stale accreditation.
		return state.MetadataV2{}, nil, fmt.Errorf("uninstall: no agreed v2 installation metadata after lock acquisition (metadata=%s lock=%s); legacy-unverified content is never deleted",
			metaLoad.Presence, lockLoad.Presence)
	}
	meta := metaLoad.Metadata
	if err := state.CheckAgreementV2(meta, lockLoad.Lock); err != nil {
		return state.MetadataV2{}, nil, fmt.Errorf("uninstall: state/lock disagree: %w", err)
	}
	return meta, s.classifyRemovals(meta, receipt), nil
}

// classifyRemovals classifies every recorded artifact by the ownership and
// digest evidence on disk right now. Removable artifacts (managed and
// digest-intact) are returned; already-absent, preserved, and retained
// classifications are recorded on the receipt.
func (s *Service) classifyRemovals(meta state.MetadataV2, receipt *UninstallReceipt) []removal {
	var removable []removal
	for _, artifact := range meta.Artifacts {
		if artifact.Ownership != state.OwnershipManaged {
			continue
		}
		abs := s.artifactAbs(artifact)
		rel := homeRelative(s.homeDir, abs)
		exists, digest, err := fileDigest(abs)
		switch {
		case err != nil:
			receipt.Retained = append(receipt.Retained, RetainedItem{Target: rel, Reason: fmt.Sprintf("cannot inspect artifact: %v", err)})
		case !exists:
			receipt.AlreadyAbsent = append(receipt.AlreadyAbsent, rel)
		case artifact.Kind == state.KindMCPConfig && !cortexCreatedWhole(artifact):
			receipt.Preserved = append(receipt.Preserved, rel)
		case digest == artifact.Digest:
			removable = append(removable, removal{rel: rel, abs: abs})
		case artifact.Kind == state.KindMCPConfig:
			// Cortex-ia created the file and no MCP churn extended it, but
			// the bytes changed anyway: the user edited it, so it is no
			// longer provably ours.
			receipt.Preserved = append(receipt.Preserved, rel)
		default:
			receipt.Retained = append(receipt.Retained, RetainedItem{
				Target: rel,
				Reason: "artifact was modified after installation; the recorded digest no longer matches, so cortex-ia no longer owns it",
			})
		}
	}
	return removable
}

// cortexCreatedWhole reports whether prior-presence evidence proves the
// recorded artifact did not exist before cortex-ia installed it. Only then
// may a merged settings document be file-deleted by uninstall.
func cortexCreatedWhole(artifact state.ArtifactV2) bool {
	return artifact.Prior != nil && !artifact.Prior.Existed
}

// retainedArtifacts filters the recorded artifacts down to those the receipt
// shows as retained or user-owned: removed and already-absent entries no
// longer describe the disk.
func (s *Service) retainedArtifacts(meta state.MetadataV2, receipt *UninstallReceipt) []state.ArtifactV2 {
	dropped := make(map[string]bool, len(receipt.Removed)+len(receipt.AlreadyAbsent))
	for _, rel := range receipt.Removed {
		dropped[rel] = true
	}
	for _, rel := range receipt.AlreadyAbsent {
		dropped[rel] = true
	}
	kept := make([]state.ArtifactV2, 0, len(meta.Artifacts))
	for _, artifact := range meta.Artifacts {
		abs := s.artifactAbs(artifact)
		if !dropped[homeRelative(s.homeDir, abs)] {
			kept = append(kept, artifact)
		}
	}
	return kept
}

// pruneEmptyDirs removes now-empty parent directories of a deleted artifact,
// strictly inside the OpenCode root or skills root. It uses os.Remove only, so a directory
// holding anything else is never removed, and the roots themselves are kept.
func (s *Service) pruneEmptyDirs(removedAbs string, receipt *UninstallReceipt) {
	dir := filepath.Dir(removedAbs)
	for {
		if dir == s.homeDir || len(dir) <= len(s.homeDir) {
			return
		}
		if dir == filepath.Join(s.homeDir, ".config", "opencode") ||
			dir == filepath.Join(s.homeDir, ".config") ||
			dir == filepath.Join(s.homeDir, ".agents", "skills") ||
			dir == filepath.Join(s.homeDir, ".agents") {
			return
		}
		if err := os.Remove(dir); err != nil {
			return // non-empty, missing, or not ours to take: keep it
		}
		receipt.RemovedDirs = append(receipt.RemovedDirs, homeRelative(s.homeDir, dir))
		dir = filepath.Dir(dir)
	}
}
