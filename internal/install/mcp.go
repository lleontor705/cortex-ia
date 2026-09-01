package install

import (
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/lleontor705/cortex-ia/internal/installmeta"
	"github.com/lleontor705/cortex-ia/internal/mcpmanager"
	"github.com/lleontor705/cortex-ia/internal/state"
)

// ErrNotInstalled is returned by mutating MCP operations when no agreed v2
// installation metadata exists. Without recorded ownership evidence the
// manager cannot accredit anything, and a record written outside a v2 state
// document would be unverifiable, so the operation fails closed.
var ErrNotInstalled = errors.New("install service: no agreed v2 installation metadata; run install first")

// fingerprintContext resolves this home's local MCP postimage fingerprint
// sidecar and HMAC salt. A corrupt sidecar is a typed error so every
// destructive MCP accreditation fails closed. An absent sidecar yields a
// fresh in-memory salt — persisted only by a successful mutating add — so
// listings and dry-runs classify legacy homes honestly (legacy-ownership
// conflicts) instead of silently downgrading to identity-only accreditation.
// The salt never leaves the process except through the state sidecar.
func (s *Service) fingerprintContext() (state.FingerprintDocument, bool, []byte, error) {
	doc, present, err := state.LoadFingerprintDocument(s.homeDir)
	if err != nil {
		return state.FingerprintDocument{}, false, nil, fmt.Errorf("MCP fingerprint store is unreadable: %w", err)
	}
	if !present {
		saltHex, genErr := state.RandomFingerprintSalt()
		if genErr != nil {
			return state.FingerprintDocument{}, false, nil, genErr
		}
		doc = state.FingerprintDocument{SchemaVersion: state.FingerprintSchemaV1, Salt: saltHex}
	}
	salt, err := doc.SaltBytes()
	if err != nil {
		return state.FingerprintDocument{}, false, nil, fmt.Errorf("MCP fingerprint salt is unusable: %w", err)
	}
	return doc, present, salt, nil
}

// withPostImageEvidence attaches recorded mcpv2 full-postimage fingerprints
// to the semantic ownership evidence, matched by server name and config
// path. Evidence without a matching fingerprint stays legacy mcpv1 and fails
// closed at destructive accreditation.
func withPostImageEvidence(evidence []mcpmanager.OwnershipRecord, doc state.FingerprintDocument, meta state.MetadataV2) []mcpmanager.OwnershipRecord {
	if len(doc.Records) == 0 {
		return evidence
	}
	byName := make(map[string]state.FingerprintRecord, len(doc.Records))
	for _, record := range doc.Records {
		byName[record.Name] = record
	}
	for i := range evidence {
		fingerprint, ok := byName[evidence[i].Name]
		if !ok {
			continue
		}
		rel, relErr := filepath.Rel(meta.OpencodeRoot, evidence[i].ConfigPath)
		if relErr != nil || filepath.ToSlash(rel) != fingerprint.ConfigPath {
			continue
		}
		evidence[i].PostImageDigest = fingerprint.PostImageDigest
	}
	return evidence
}

// persistFingerprintRecordTxn records one server's mcpv2 postimage
// fingerprint inside the running transaction. The sidecar write is journaled
// like every other declared target, so a later failure restores the exact
// pre-transaction sidecar bytes. Legacy records (empty digest) are never
// persisted: the sidecar only ever carries mcpv2 evidence.
func (s *Service) persistFingerprintRecordTxn(txn *serviceTxn, store *state.FingerprintDocument, name, configAbs, digest, opencodeRoot string) error {
	if digest == "" {
		return nil
	}
	rel, relErr := filepath.Rel(opencodeRoot, configAbs)
	if relErr != nil {
		return fmt.Errorf("resolve fingerprint config path: %w", relErr)
	}
	*store = state.UpsertFingerprintRecord(*store, state.FingerprintRecord{
		Name:            name,
		ConfigPath:      filepath.ToSlash(rel),
		PostImageDigest: digest,
	})
	return txn.run(state.FingerprintPath(s.homeDir), func() error {
		return state.SaveFingerprintDocument(s.homeDir, *store)
	})
}

// dropFingerprintRecordTxn removes one server's postimage fingerprint inside
// the running transaction, journaling the sidecar write. Absent sidecars and
// unrecorded names are a no-op.
func (s *Service) dropFingerprintRecordTxn(txn *serviceTxn, store *state.FingerprintDocument, present bool, name string) error {
	if !present || !store.HasFingerprintRecord(name) {
		return nil
	}
	*store = state.DropFingerprintRecord(*store, name)
	return txn.run(state.FingerprintPath(s.homeDir), func() error {
		return state.SaveFingerprintDocument(s.homeDir, *store)
	})
}

// MCPOptions is the explicit per-call intent for MCP operations. The zero
// value is a real run with no probes.
type MCPOptions struct {
	DryRun bool
	// LockTimeout bounds canonical-home lock acquisition for the mutating
	// path. Zero or negative uses install.DefaultHomeLockTimeout. Dry-runs
	// never acquire the lock.
	LockTimeout time.Duration
	// Probes supply explicit qualification evidence. The manager fails a
	// configuration qualified only when every probe returns valid evidence;
	// no probe means unqualified, never silently qualified.
	Probes []mcpmanager.ProbeFunc
	// Now overrides the clock for deterministic runs.
	Now func() time.Time
}

func (o MCPOptions) now() time.Time {
	if o.Now != nil {
		return o.Now().UTC()
	}
	return time.Now().UTC()
}

// MCPReceipt is the typed outcome of one managed MCP add or remove. It keeps
// configuration and qualification separated: Configured reports the managed
// entry is present with accredited ownership, Qualified reports explicit
// probe evidence validated it during this call, and Installed is only true
// when both hold.
type MCPReceipt struct {
	Name       string `json:"name"`
	Action     string `json:"action"`
	ConfigPath string `json:"config_path,omitempty"`
	DryRun     bool   `json:"dry_run"`
	Configured bool   `json:"configured"`
	Qualified  bool   `json:"qualified"`
	Installed  bool   `json:"installed"`
	// Changed reports that config file bytes changed.
	Changed bool `json:"changed"`
	// BackupID locates the verified pre-operation backup (real runs that
	// intend to mutate only).
	BackupID string `json:"backup_id,omitempty"`
	// Restored reports a failed run completed a verified inverse
	// restoration of every mutated destination.
	Restored bool `json:"restored,omitempty"`
	// RestoreError describes a restoration that could not be verified; the
	// journal checkpoint is retained for a safe retry.
	RestoreError string `json:"restore_error,omitempty"`
	// Qualification carries the evaluated probe outcome, when a probe ran.
	Qualification *mcpmanager.ProbeEvidence `json:"qualification,omitempty"`
	Warnings      []string                  `json:"warnings,omitempty"`
}

// MCPListReport is the read-only listing of managed and unknown MCP entries.
type MCPListReport struct {
	ConfigPath string                   `json:"config_path"`
	Entries    []mcpmanager.EntryReport `json:"entries"`
	Unknown    []string                 `json:"unknown,omitempty"`
	// Installed reports whether the listing was accredited by an agreed v2
	// installation; without it every equal entry reports as user-owned.
	Installed bool `json:"installed"`
}

// MCPAdd installs the managed preset entry for name. Ownership conflicts,
// unmanaged names, and malformed config fail closed through the manager with
// a *mcpmanager.ConflictError and mutate nothing. Real runs are fully
// transactional: config candidates and the v2 state and lock files are
// backed up, verified restorable, and journaled before the first write; the
// config mutation and the metadata commit each record a verified postimage;
// and any later failure restores the exact preimages in reverse order. The
// canonical home lock is acquired first, and the entire mutating context —
// the v2 metadata, the fingerprint sidecar, and the config the manager then
// classifies — is loaded and reclassified under that lock, so a concurrent
// process's committed mutation is never overwritten from stale pre-lock
// evidence. The lock is released on every exit; contention is a typed
// ErrHomeBusy with zero mutation. Dry-runs stay read-only and never lock.
func (s *Service) MCPAdd(name string, opts MCPOptions) (*MCPReceipt, error) {
	preset, ok := mcpmanager.Lookup(name)
	if !ok {
		return nil, &mcpmanager.ConflictError{Name: name, Kind: mcpmanager.ConflictUnmanaged}
	}

	if opts.DryRun {
		_, evidence, err := s.v2Context()
		if err != nil {
			return nil, err
		}
		_, _, salt, err := s.fingerprintContext()
		if err != nil {
			return nil, err
		}
		manager := mcpmanager.NewFingerprinting(s.homeDir, salt)
		return s.mcpDryRun(manager, name, evidence, "add")
	}

	release, err := s.lockForMutation(opts.LockTimeout)
	if err != nil {
		return nil, fmt.Errorf("MCP add %q: %w", name, err)
	}
	defer release()

	// Reload the mutating context under the lock: another process may have
	// committed a different state, fingerprint sidecar, or config between
	// this call and lock acquisition.
	meta, evidence, err := s.v2Context()
	if err != nil {
		return nil, err
	}
	store, _, salt, err := s.fingerprintContext()
	if err != nil {
		return nil, err
	}
	manager := mcpmanager.NewFingerprinting(s.homeDir, salt)

	receipt := &MCPReceipt{Name: name, ConfigPath: manager.ConfigPath()}
	txn, err := s.beginServiceTxn("mcp", opts.now(), state.FingerprintPath(s.homeDir))
	if err != nil {
		return receipt, fmt.Errorf("MCP add %q: %w", name, err)
	}
	receipt.BackupID = txn.backupID
	var status txnStatus
	stage := fmt.Sprintf("MCP add %q", name)
	// fail aborts the transaction and immediately mirrors the restore
	// verdict onto the receipt, so a failed run never reports success
	// fields without its restoration evidence.
	fail := func(err error) error {
		aborted := txn.abort(&status, stage, err)
		receipt.Restored, receipt.RestoreError = status.Restored, status.RestoreError
		return aborted
	}

	var result mcpmanager.Result
	if err := txn.run(manager.ConfigPath(), func() error {
		var inner error
		result, inner = manager.Add(name, evidence, opts.Probes...)
		return inner
	}); err != nil {
		return receipt, fail(err)
	}
	receipt.Action = result.Action
	receipt.ConfigPath = result.ConfigPath
	receipt.Configured = result.Configured
	receipt.Qualified = result.Qualified
	receipt.Installed = result.Installed
	receipt.Changed = result.Changed
	receipt.Qualification = result.Qualification

	record, err := mcpRecordFromPreset(preset, result.ConfigPath, meta.OpencodeRoot)
	if err != nil {
		return receipt, fail(fmt.Errorf("record ownership: %w", err))
	}
	if err := txn.commitState(meta, upsertMCPRecord(meta, record), opts.now()); err != nil {
		return receipt, fail(fmt.Errorf("commit state v2: %w", err))
	}
	fingerprintDigest := ""
	if result.Ownership != nil {
		fingerprintDigest = result.Ownership.PostImageDigest
	}
	if err := s.persistFingerprintRecordTxn(txn, &store, name, result.ConfigPath, fingerprintDigest, meta.OpencodeRoot); err != nil {
		return receipt, fail(fmt.Errorf("record MCP postimage fingerprint: %w", err))
	}
	if err := txn.commit(); err != nil {
		return receipt, fail(err)
	}
	return receipt, nil
}

// MCPAddDesired installs the typed desired MCP server: a catalog preset, a
// custom local server with an exact argv vector (never a shell string), or
// a custom remote http(s) endpoint. The desired description is validated
// before any state access: a malformed request fails closed as
// *mcpmanager.InvalidDesiredError without touching the home.
//
// Real runs are fully transactional exactly like MCPAdd: config candidates
// and the v2 state and lock files are backed up, verified restorable, and
// journaled before the first write; the config mutation and the metadata
// commit each record a verified postimage; and any later failure restores
// the exact preimages in reverse order. The canonical home lock is acquired
// first, and the entire mutating context — the v2 metadata, the fingerprint
// sidecar, and the config the manager then classifies — is loaded and
// reclassified under that lock, exactly like MCPAdd. Env and header values
// reach the configuration file only: they are not representable in the
// ownership record, the state, the lock, or this receipt. When no probe is
// supplied the manager applies the kind-compatible offline default (command
// resolution for local and preset servers, URL validation for remote
// ones); Installed still requires valid probe evidence, so configured-only
// is never a success.
func (s *Service) MCPAddDesired(desired mcpmanager.Desired, opts MCPOptions) (*MCPReceipt, error) {
	if err := desired.Validate(); err != nil {
		return nil, err
	}

	if opts.DryRun {
		_, evidence, err := s.v2Context()
		if err != nil {
			return nil, err
		}
		_, _, salt, err := s.fingerprintContext()
		if err != nil {
			return nil, err
		}
		manager := mcpmanager.NewFingerprinting(s.homeDir, salt)
		receipt := &MCPReceipt{Name: desired.Name, ConfigPath: manager.ConfigPath(), DryRun: true}
		action, configured, err := manager.InspectDesired(desired, evidence)
		if err != nil {
			return receipt, err
		}
		receipt.Action = action
		receipt.Configured = configured
		if action == "add" {
			receipt.Warnings = append(receipt.Warnings, "dry-run: no probes run; qualification is not evaluated")
		}
		return receipt, nil
	}

	release, err := s.lockForMutation(opts.LockTimeout)
	if err != nil {
		return nil, fmt.Errorf("MCP add desired %q: %w", desired.Name, err)
	}
	defer release()

	// Reload the mutating context under the lock, exactly like MCPAdd:
	// another process may have committed a different state, fingerprint
	// sidecar, or config between this call and lock acquisition.
	meta, evidence, err := s.v2Context()
	if err != nil {
		return nil, err
	}
	store, _, salt, err := s.fingerprintContext()
	if err != nil {
		return nil, err
	}
	manager := mcpmanager.NewFingerprinting(s.homeDir, salt)

	receipt := &MCPReceipt{Name: desired.Name, ConfigPath: manager.ConfigPath()}
	txn, err := s.beginServiceTxn("mcp", opts.now(), state.FingerprintPath(s.homeDir))
	if err != nil {
		return receipt, fmt.Errorf("MCP add desired %q: %w", desired.Name, err)
	}
	receipt.BackupID = txn.backupID
	var status txnStatus
	stage := fmt.Sprintf("MCP add desired %q", desired.Name)
	// fail aborts the transaction and immediately mirrors the restore
	// verdict onto the receipt, exactly like MCPAdd.
	fail := func(err error) error {
		aborted := txn.abort(&status, stage, err)
		receipt.Restored, receipt.RestoreError = status.Restored, status.RestoreError
		return aborted
	}

	var result mcpmanager.Result
	if err := txn.run(manager.ConfigPath(), func() error {
		var inner error
		result, inner = manager.AddDesired(desired, evidence, opts.Probes...)
		return inner
	}); err != nil {
		return receipt, fail(err)
	}
	receipt.Action = result.Action
	receipt.ConfigPath = result.ConfigPath
	receipt.Configured = result.Configured
	receipt.Qualified = result.Qualified
	receipt.Installed = result.Installed
	receipt.Changed = result.Changed
	receipt.Qualification = result.Qualification

	entry, err := desired.Entry()
	if err != nil {
		return receipt, fail(err)
	}
	record, err := mcpRecordFromPreset(mcpmanager.Preset{Name: desired.Name, Entry: entry}, result.ConfigPath, meta.OpencodeRoot)
	if err != nil {
		return receipt, fail(fmt.Errorf("record ownership: %w", err))
	}
	if err := txn.commitState(meta, upsertMCPRecord(meta, record), opts.now()); err != nil {
		return receipt, fail(fmt.Errorf("commit state v2: %w", err))
	}
	fingerprintDigest := ""
	if result.Ownership != nil {
		fingerprintDigest = result.Ownership.PostImageDigest
	}
	if err := s.persistFingerprintRecordTxn(txn, &store, desired.Name, result.ConfigPath, fingerprintDigest, meta.OpencodeRoot); err != nil {
		return receipt, fail(fmt.Errorf("record MCP postimage fingerprint: %w", err))
	}
	if err := txn.commit(); err != nil {
		return receipt, fail(err)
	}
	return receipt, nil
}

// MCPRemove deregisters the managed MCP entry for name. Catalog preset
// names follow the preset removal path (full-value semantic equality);
// custom names follow the accredited custom path, which additionally demands
// a recorded mcpv2 full-postimage fingerprint that matches the live entry:
// legacy mcpv1-only records fail closed with the manual re-add remedy, and
// any URL/env/header-value/enabled/type/argv/config-path drift fails closed
// as typed drift. Nothing is mutated on any refusal. Real runs are fully
// transactional exactly like MCPAdd: the config mutation and the metadata
// commit are journaled with verified postimages, and any later failure
// restores the exact preimages. The canonical home lock is acquired first,
// and the entire accreditation context — the v2 metadata, the fingerprint
// sidecar, and the config the manager then classifies — is loaded and
// reclassified under that lock, so ownership is never accredited from
// stale pre-lock evidence. Dry-runs stay read-only and never lock.
func (s *Service) MCPRemove(name string, opts MCPOptions) (*MCPReceipt, error) {
	_, isCatalog := mcpmanager.Lookup(name)
	isCatalog = isCatalog || mcpmanager.IsRetired(name)

	if opts.DryRun {
		meta, evidence, err := s.v2Context()
		if err != nil {
			return nil, err
		}
		store, _, salt, err := s.fingerprintContext()
		if err != nil {
			return nil, err
		}
		evidence = withPostImageEvidence(evidence, store, meta)
		manager := mcpmanager.NewFingerprinting(s.homeDir, salt)
		return s.mcpDryRun(manager, name, evidence, "remove")
	}

	release, err := s.lockForMutation(opts.LockTimeout)
	if err != nil {
		return nil, fmt.Errorf("MCP remove %q: %w", name, err)
	}
	defer release()

	// Reload the accreditation context under the lock: another process may
	// have committed a different state, fingerprint sidecar, or config
	// between this call and lock acquisition.
	meta, evidence, err := s.v2Context()
	if err != nil {
		return nil, err
	}
	store, storePresent, salt, err := s.fingerprintContext()
	if err != nil {
		return nil, err
	}
	evidence = withPostImageEvidence(evidence, store, meta)
	manager := mcpmanager.NewFingerprinting(s.homeDir, salt)

	receipt := &MCPReceipt{Name: name, ConfigPath: manager.ConfigPath()}
	txn, err := s.beginServiceTxn("mcp", opts.now(), state.FingerprintPath(s.homeDir))
	if err != nil {
		return receipt, fmt.Errorf("MCP remove %q: %w", name, err)
	}
	receipt.BackupID = txn.backupID
	var status txnStatus
	stage := fmt.Sprintf("MCP remove %q", name)
	// fail aborts the transaction and immediately mirrors the restore
	// verdict onto the receipt, exactly like MCPAdd.
	fail := func(err error) error {
		aborted := txn.abort(&status, stage, err)
		receipt.Restored, receipt.RestoreError = status.Restored, status.RestoreError
		return aborted
	}

	var result mcpmanager.Result
	if err := txn.run(manager.ConfigPath(), func() error {
		var inner error
		if isCatalog {
			result, inner = manager.Remove(name, evidence)
		} else {
			result, inner = manager.RemoveCustom(name, evidence)
		}
		return inner
	}); err != nil {
		return receipt, fail(err)
	}
	receipt.Action = result.Action
	receipt.ConfigPath = result.ConfigPath
	receipt.Changed = result.Changed
	if result.Action == "already-absent" {
		if err := s.dropFingerprintRecordTxn(txn, &store, storePresent, name); err != nil {
			return receipt, fail(fmt.Errorf("drop MCP postimage fingerprint: %w", err))
		}
		if err := txn.commit(); err != nil {
			return receipt, fail(err)
		}
		return receipt, nil
	}
	if err := txn.commitState(meta, dropMCPRecord(meta, name), opts.now()); err != nil {
		return receipt, fail(fmt.Errorf("commit state v2: %w", err))
	}
	if err := s.dropFingerprintRecordTxn(txn, &store, storePresent, name); err != nil {
		return receipt, fail(fmt.Errorf("drop MCP postimage fingerprint: %w", err))
	}
	if err := txn.commit(); err != nil {
		return receipt, fail(err)
	}
	return receipt, nil
}

// MCPList reports the ownership status of every managed preset plus the
// unknown MCP entries found in the configuration. Listing is read-only and
// works on homes without a v2 installation: without recorded ownership
// evidence, equal entries honestly report as user-owned. On agreed v2 homes
// the listing runs with the home's postimage fingerprint salt: accredited
// customs report managed only when their recorded mcpv2 fingerprint matches
// the live entry, and drifted or legacy-owned customs report the typed
// conflict (drift detected / re-add remedy) without exposing any value.
func (s *Service) MCPList() (*MCPListReport, error) {
	evidence := []mcpmanager.OwnershipRecord(nil)
	var salt []byte
	report := &MCPListReport{}
	metaLoad := state.LoadMetadataV2(s.homeDir)
	if metaLoad.Presence == state.PresenceV2 {
		lockLoad := state.LoadLockV2(s.homeDir)
		if lockLoad.Presence == state.PresenceV2 && state.CheckAgreementV2(metaLoad.Metadata, lockLoad.Lock) == nil {
			store, _, storeSalt, err := s.fingerprintContext()
			if err != nil {
				return nil, err
			}
			report.Installed = true
			evidence = withPostImageEvidence(ownershipEvidence(metaLoad.Metadata), store, metaLoad.Metadata)
			salt = storeSalt
		}
	}
	manager := mcpmanager.NewFingerprinting(s.homeDir, salt)
	report.ConfigPath = manager.ConfigPath()
	listing, err := manager.List(evidence)
	if err != nil {
		return nil, err
	}
	report.Entries = listing.Entries
	report.Unknown = listing.Unknown
	return report, nil
}

// mcpDryRun predicts the outcome of an add ("add") or remove ("remove")
// operation for name from the read-only listing. Absent entries predict the
// honest per-operation outcome ("add" versus "already-absent"), accredited
// entries predict the operation's effect ("already-present" versus
// "remove"), and user-owned entries predict the fail-closed conflict the
// real operation would raise. No probe runs, so qualification is never
// reported.
func (s *Service) mcpDryRun(manager *mcpmanager.Manager, name string, evidence []mcpmanager.OwnershipRecord, op string) (*MCPReceipt, error) {
	listing, err := manager.List(evidence)
	if err != nil {
		return nil, err
	}
	receipt := &MCPReceipt{Name: name, ConfigPath: listing.ConfigPath, DryRun: true}
	absent := func() (*MCPReceipt, error) {
		if op == "add" {
			receipt.Action = "add"
			receipt.Warnings = append(receipt.Warnings, "dry-run: no probes run; qualification is not evaluated")
		} else {
			receipt.Action = "already-absent"
		}
		return receipt, nil
	}
	for _, entry := range listing.Entries {
		if entry.Name != name {
			continue
		}
		switch entry.Status {
		case mcpmanager.StatusAbsent:
			return absent()
		case mcpmanager.StatusManaged:
			if op == "add" {
				receipt.Action = "already-present"
				receipt.Configured = true
			} else {
				receipt.Action = "remove"
			}
			return receipt, nil
		default:
			return receipt, &mcpmanager.ConflictError{
				Name:           name,
				Kind:           mcpmanager.ConflictModified,
				ExpectedDigest: "",
				ObservedDigest: entry.Digest,
				Detail:         fmt.Sprintf("entry is user-owned (%s) and would fail closed", entry.Status),
			}
		}
	}
	for _, unknown := range listing.Unknown {
		if unknown == name {
			return receipt, &mcpmanager.ConflictError{
				Name:   name,
				Kind:   mcpmanager.ConflictUnaccredited,
				Detail: "entry is user-owned (no ownership record accredits it) and would fail closed",
			}
		}
	}
	return absent()
}

// v2Context loads the agreed v2 metadata and its ownership evidence, failing
// closed on absent, legacy, malformed, or disagreeing documents exactly like
// engine planning.
func (s *Service) v2Context() (state.MetadataV2, []mcpmanager.OwnershipRecord, error) {
	metaLoad := state.LoadMetadataV2(s.homeDir)
	lockLoad := state.LoadLockV2(s.homeDir)
	switch {
	case metaLoad.Presence == state.PresenceMalformed:
		return state.MetadataV2{}, nil, fmt.Errorf("%w: state metadata is malformed: %s", ErrNotInstalled, metaLoad.Detail)
	case lockLoad.Presence == state.PresenceMalformed:
		return state.MetadataV2{}, nil, fmt.Errorf("%w: lock metadata is malformed: %s", ErrNotInstalled, lockLoad.Detail)
	case metaLoad.Presence != state.PresenceV2 || lockLoad.Presence != state.PresenceV2:
		return state.MetadataV2{}, nil, fmt.Errorf("%w: metadata=%s lock=%s", ErrNotInstalled, metaLoad.Presence, lockLoad.Presence)
	}
	if err := state.CheckAgreementV2(metaLoad.Metadata, lockLoad.Lock); err != nil {
		return state.MetadataV2{}, nil, fmt.Errorf("%w: state/lock disagree: %w", ErrNotInstalled, err)
	}
	return metaLoad.Metadata, ownershipEvidence(metaLoad.Metadata), nil
}

// mcpRecordFromPreset derives the sanitized v2 ownership record for one
// configured preset, using the shared semantic identity digest.
func mcpRecordFromPreset(preset mcpmanager.Preset, configAbs, opencodeRoot string) (state.MCPV2, error) {
	identity, err := installmeta.MCPServerIdentityFromEntry(preset.Name, preset.Entry)
	if err != nil {
		return state.MCPV2{}, err
	}
	rel, err := filepath.Rel(opencodeRoot, configAbs)
	if err != nil {
		return state.MCPV2{}, fmt.Errorf("resolve config path relative to the OpenCode root: %w", err)
	}
	return state.NewMCPV2(identity, filepath.ToSlash(rel), state.OwnershipManaged)
}

// upsertMCPRecord replaces the record for the preset name, or appends it,
// returning a name-sorted set.
func upsertMCPRecord(meta state.MetadataV2, record state.MCPV2) []state.MCPV2 {
	records := make([]state.MCPV2, 0, len(meta.MCPs)+1)
	replaced := false
	for _, existing := range meta.MCPs {
		if existing.Name == record.Name {
			records = append(records, record)
			replaced = true
			continue
		}
		records = append(records, existing)
	}
	if !replaced {
		records = append(records, record)
	}
	sortMCPRecords(records)
	return records
}

// dropMCPRecord removes the record for the preset name.
func dropMCPRecord(meta state.MetadataV2, name string) []state.MCPV2 {
	records := make([]state.MCPV2, 0, len(meta.MCPs))
	for _, existing := range meta.MCPs {
		if existing.Name != name {
			records = append(records, existing)
		}
	}
	sortMCPRecords(records)
	return records
}

func sortMCPRecords(records []state.MCPV2) {
	for i := 1; i < len(records); i++ {
		for j := i; j > 0 && records[j].Name < records[j-1].Name; j-- {
			records[j], records[j-1] = records[j-1], records[j]
		}
	}
}
