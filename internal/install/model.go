package install

import (
	"fmt"
	"path/filepath"
	"sort"
	"time"

	"github.com/lleontor705/cortex-ia/internal/modelmgr"
	"github.com/lleontor705/cortex-ia/internal/state"
)

// agentModelSidecarNamespace prefixes the sidecar record names this service
// persists for managed agent-model entries. Sidecar lookups match names
// exactly, so the prefix keeps agent-model records independent from the MCP
// records sharing the same document.
const agentModelSidecarNamespace = "agent-model/"

// ModelOptions is the explicit per-call intent for agent-model operations.
// The zero value is a real run.
type ModelOptions struct {
	DryRun bool
	// LockTimeout bounds canonical-home lock acquisition for the mutating
	// path. Zero or negative uses install.DefaultHomeLockTimeout. Dry-runs
	// never acquire the lock.
	LockTimeout time.Duration
	// Now overrides the clock for deterministic runs.
	Now func() time.Time
}

func (o ModelOptions) now() time.Time {
	if o.Now != nil {
		return o.Now().UTC()
	}
	return time.Now().UTC()
}

// ModelReceipt is the typed outcome of one managed agent-model set or unset.
// Previous always carries the effective compact reference observed before the
// call, so a set that overwrites a user value never does so silently.
type ModelReceipt struct {
	Agent      string `json:"agent"`
	Action     string `json:"action"`
	ConfigPath string `json:"config_path,omitempty"`
	DryRun     bool   `json:"dry_run"`
	// Previous is the effective compact reference before the call.
	Previous string `json:"previous,omitempty"`
	// Value is the resulting compact reference; empty for an unset.
	Value string `json:"value,omitempty"`
	// Changed reports that config file bytes changed.
	Changed bool `json:"changed"`
	// Managed reports the call ended with an accredited ownership record.
	Managed bool `json:"managed"`
	// BackupID locates the verified pre-operation backup (real runs that
	// intend to mutate only).
	BackupID string `json:"backup_id,omitempty"`
	// Restored reports a failed run completed a verified inverse
	// restoration of every mutated destination.
	Restored bool `json:"restored,omitempty"`
	// RestoreError describes a restoration that could not be verified; the
	// journal checkpoint is retained for a safe retry.
	RestoreError string `json:"restore_error,omitempty"`
	// Warnings carries the markdown frontmatter note when one exists.
	Warnings []string `json:"warnings,omitempty"`
}

// ModelListReport is the read-only listing of every registry agent's
// effective model. It works honestly on homes without a v2 installation:
// without recorded ownership evidence, entries report as plain config or
// markdown instead of claiming accreditation.
type ModelListReport struct {
	ConfigPath string                `json:"config_path"`
	Installed  bool                  `json:"installed"`
	Agents     []modelmgr.AgentEntry `json:"agents"`
}

// ModelGetReport is the read-only projection of one registry agent.
type ModelGetReport struct {
	ConfigPath string              `json:"config_path"`
	Installed  bool                `json:"installed"`
	Agent      modelmgr.AgentEntry `json:"agent"`
}

// ModelSet assigns desired to the agent the manager resolves. It mirrors the
// MCPAddDesired recipe: the desired reference is validated before any state
// access, real runs gate on an agreed v2 installation, acquire the canonical
// home lock, and reload the v2 metadata and fingerprint sidecar under it, and
// the manager mutation runs inside a service transaction with a verified
// backup and a recorded postimage. The ownership record and the namespaced
// sidecar record are committed in the same transaction as the config
// mutation, and any later failure restores the exact preimages in reverse
// order. Dry-runs report the planned action without locking or writing.
func (s *Service) ModelSet(desired modelmgr.Desired, opts ModelOptions) (*ModelReceipt, error) {
	if err := desired.Validate(); err != nil {
		return nil, err
	}

	if opts.DryRun {
		_, evidence, err := s.v2ModelContext()
		if err != nil {
			return nil, err
		}
		result, err := modelmgr.New(s.homeDir).InspectSet(desired, evidence)
		if err != nil {
			return nil, err
		}
		receipt := &ModelReceipt{DryRun: true}
		applyModelResult(receipt, result)
		return receipt, nil
	}

	release, err := s.lockForMutation(opts.LockTimeout)
	if err != nil {
		return nil, fmt.Errorf("model set %q: %w", desired.Agent, err)
	}
	defer release()

	// Reload the mutating context under the lock: another process may have
	// committed a different state, fingerprint sidecar, or config between
	// this call and lock acquisition.
	meta, evidence, err := s.v2ModelContext()
	if err != nil {
		return nil, err
	}
	store, _, _, err := s.fingerprintContext()
	if err != nil {
		return nil, err
	}
	manager := modelmgr.New(s.homeDir)

	receipt := &ModelReceipt{Agent: desired.Agent, ConfigPath: manager.ConfigPath()}
	txn, err := s.beginServiceTxn("model", opts.now(), state.FingerprintPath(s.homeDir))
	if err != nil {
		return receipt, fmt.Errorf("model set %q: %w", desired.Agent, err)
	}
	receipt.BackupID = txn.backupID
	var status txnStatus
	stage := fmt.Sprintf("model set %q", desired.Agent)
	// fail aborts the transaction and immediately mirrors the restore
	// verdict onto the receipt, so a failed run never reports success fields
	// without its restoration evidence.
	fail := func(err error) error {
		aborted := txn.abort(&status, stage, err)
		receipt.Restored, receipt.RestoreError = status.Restored, status.RestoreError
		return aborted
	}

	var result modelmgr.Result
	if err := txn.run(manager.ConfigPath(), func() error {
		var inner error
		result, inner = manager.Set(desired, evidence)
		return inner
	}); err != nil {
		return receipt, fail(err)
	}
	applyModelResult(receipt, result)

	records := meta.AgentModels
	if result.Ownership != nil {
		record, err := agentModelRecord(result, meta.OpencodeRoot)
		if err != nil {
			return receipt, fail(fmt.Errorf("record ownership: %w", err))
		}
		records = upsertAgentModelRecord(meta, record)
		if err := txn.commitStateModels(meta, records, opts.now()); err != nil {
			return receipt, fail(fmt.Errorf("commit state v2: %w", err))
		}
		if err := s.persistFingerprintRecordTxn(txn, &store, agentModelSidecarName(record.Agent), result.ConfigPath, record.SemanticDigest, meta.OpencodeRoot); err != nil {
			return receipt, fail(fmt.Errorf("record agent-model fingerprint: %w", err))
		}
	} else if err := txn.commitStateModels(meta, records, opts.now()); err != nil {
		return receipt, fail(fmt.Errorf("commit state v2: %w", err))
	}
	if err := txn.commit(); err != nil {
		return receipt, fail(err)
	}
	return receipt, nil
}

// ModelUnset removes agents.<agent>.model exactly like ModelSet removes a
// managed entry: the same ErrNotInstalled gate, home lock, verified backup,
// journaled metadata commit, and namespaced sidecar record drop. The
// ownership record and sidecar record are removed with the config member, so
// no stale accreditation outlives the entry. Dry-runs report the planned
// action without locking or writing.
func (s *Service) ModelUnset(agent string, opts ModelOptions) (*ModelReceipt, error) {
	if opts.DryRun {
		_, evidence, err := s.v2ModelContext()
		if err != nil {
			return nil, err
		}
		manager := modelmgr.New(s.homeDir)
		entry, err := manager.Get(agent, evidence)
		if err != nil {
			return nil, err
		}
		receipt := &ModelReceipt{
			Agent:      entry.Agent,
			ConfigPath: manager.ConfigPath(),
			DryRun:     true,
			Previous:   compactAgentEntry(entry),
			Action:     "already-unset",
		}
		if entry.Source == modelmgr.SourceConfig || entry.Source == modelmgr.SourceManaged {
			receipt.Action = "unset"
		}
		return receipt, nil
	}

	release, err := s.lockForMutation(opts.LockTimeout)
	if err != nil {
		return nil, fmt.Errorf("model unset %q: %w", agent, err)
	}
	defer release()

	// Reload the mutating context under the lock, exactly like ModelSet.
	meta, evidence, err := s.v2ModelContext()
	if err != nil {
		return nil, err
	}
	store, storePresent, _, err := s.fingerprintContext()
	if err != nil {
		return nil, err
	}
	manager := modelmgr.New(s.homeDir)

	receipt := &ModelReceipt{Agent: agent, ConfigPath: manager.ConfigPath()}
	txn, err := s.beginServiceTxn("model", opts.now(), state.FingerprintPath(s.homeDir))
	if err != nil {
		return receipt, fmt.Errorf("model unset %q: %w", agent, err)
	}
	receipt.BackupID = txn.backupID
	var status txnStatus
	stage := fmt.Sprintf("model unset %q", agent)
	fail := func(err error) error {
		aborted := txn.abort(&status, stage, err)
		receipt.Restored, receipt.RestoreError = status.Restored, status.RestoreError
		return aborted
	}

	var result modelmgr.Result
	if err := txn.run(manager.ConfigPath(), func() error {
		var inner error
		result, inner = manager.Unset(agent, evidence)
		return inner
	}); err != nil {
		return receipt, fail(err)
	}
	applyModelResult(receipt, result)

	if err := txn.commitStateModels(meta, dropAgentModelRecord(meta, result.Agent), opts.now()); err != nil {
		return receipt, fail(fmt.Errorf("commit state v2: %w", err))
	}
	if err := s.dropFingerprintRecordTxn(txn, &store, storePresent, agentModelSidecarName(result.Agent)); err != nil {
		return receipt, fail(fmt.Errorf("drop agent-model fingerprint: %w", err))
	}
	if err := txn.commit(); err != nil {
		return receipt, fail(err)
	}
	return receipt, nil
}

// ModelList reports every registry agent's effective model reference, source,
// and accredited ownership state. Listing is read-only and works on homes
// without a v2 installation: without recorded ownership evidence, equal
// entries honestly report as plain config or markdown.
func (s *Service) ModelList() (*ModelListReport, error) {
	evidence, installed := s.modelEvidence()
	manager := modelmgr.New(s.homeDir)
	report := &ModelListReport{ConfigPath: manager.ConfigPath(), Installed: installed}
	listing, err := manager.List(evidence)
	if err != nil {
		return nil, err
	}
	report.ConfigPath = listing.ConfigPath
	report.Agents = listing.Agents
	return report, nil
}

// ModelGet reports one registry agent's effective model, source, and
// accredited ownership state under the same read-only, honest rules as
// ModelList.
func (s *Service) ModelGet(agent string) (*ModelGetReport, error) {
	evidence, installed := s.modelEvidence()
	manager := modelmgr.New(s.homeDir)
	report := &ModelGetReport{ConfigPath: manager.ConfigPath(), Installed: installed}
	entry, err := manager.Get(agent, evidence)
	if err != nil {
		return nil, err
	}
	report.Agent = entry
	return report, nil
}

// modelEvidence loads the recorded agent-model ownership evidence when the
// home has an agreed v2 installation. Absent, legacy, malformed, or
// disagreeing documents yield no evidence instead of failing: list and get
// are read-only and must stay honest on any home.
func (s *Service) modelEvidence() ([]modelmgr.OwnershipRecord, bool) {
	metaLoad := state.LoadMetadataV2(s.homeDir)
	if metaLoad.Presence != state.PresenceV2 {
		return nil, false
	}
	lockLoad := state.LoadLockV2(s.homeDir)
	if lockLoad.Presence != state.PresenceV2 || state.CheckAgreementV2(metaLoad.Metadata, lockLoad.Lock) != nil {
		return nil, false
	}
	return modelOwnershipEvidence(metaLoad.Metadata), true
}

// v2ModelContext loads the agreed v2 metadata and projects its recorded
// agent-model ownership onto manager records, failing closed exactly like
// v2Context on absent, legacy, malformed, or disagreeing documents.
func (s *Service) v2ModelContext() (state.MetadataV2, []modelmgr.OwnershipRecord, error) {
	meta, _, err := s.v2Context()
	if err != nil {
		return state.MetadataV2{}, nil, err
	}
	return meta, modelOwnershipEvidence(meta), nil
}

// modelOwnershipEvidence projects recorded managed agent-model entries onto
// manager records. The absolute config path is reconstructed from the
// recorded OpenCode root so accreditation stays bound to one file, exactly
// like ownershipEvidence does for MCPs.
func modelOwnershipEvidence(meta state.MetadataV2) []modelmgr.OwnershipRecord {
	records := make([]modelmgr.OwnershipRecord, 0, len(meta.AgentModels))
	for _, model := range meta.AgentModels {
		if model.Ownership != state.OwnershipManaged {
			continue
		}
		records = append(records, modelmgr.OwnershipRecord{
			Agent:      model.Agent,
			Digest:     model.SemanticDigest,
			ConfigPath: filepath.Join(meta.OpencodeRoot, filepath.FromSlash(model.ConfigPath)),
		})
	}
	return records
}

// agentModelRecord derives the sanitized v2 ownership record from the
// manager's own ownership projection, so the persisted digest is exactly the
// evidence that accredited the entry.
func agentModelRecord(result modelmgr.Result, opencodeRoot string) (state.AgentModelV2, error) {
	rel, err := filepath.Rel(opencodeRoot, result.ConfigPath)
	if err != nil {
		return state.AgentModelV2{}, fmt.Errorf("resolve config path relative to the OpenCode root: %w", err)
	}
	return state.AgentModelV2{
		Agent:          result.Ownership.Agent,
		ConfigPath:     filepath.ToSlash(rel),
		SemanticDigest: result.Ownership.Digest,
		Ownership:      state.OwnershipManaged,
	}, nil
}

// upsertAgentModelRecord replaces the record for the agent, or appends it,
// returning an agent-sorted set.
func upsertAgentModelRecord(meta state.MetadataV2, record state.AgentModelV2) []state.AgentModelV2 {
	records := make([]state.AgentModelV2, 0, len(meta.AgentModels)+1)
	replaced := false
	for _, existing := range meta.AgentModels {
		if existing.Agent == record.Agent {
			records = append(records, record)
			replaced = true
			continue
		}
		records = append(records, existing)
	}
	if !replaced {
		records = append(records, record)
	}
	sortAgentModelRecords(records)
	return records
}

// dropAgentModelRecord removes the record for the agent.
func dropAgentModelRecord(meta state.MetadataV2, agent string) []state.AgentModelV2 {
	records := make([]state.AgentModelV2, 0, len(meta.AgentModels))
	for _, existing := range meta.AgentModels {
		if existing.Agent != agent {
			records = append(records, existing)
		}
	}
	sortAgentModelRecords(records)
	return records
}

func sortAgentModelRecords(records []state.AgentModelV2) {
	sort.Slice(records, func(i, j int) bool { return records[i].Agent < records[j].Agent })
}

// commitStateModels persists the updated agent-model record set as agreeing
// v2 state and lock documents, journaling both writes so a failure restores
// the exact pre-transaction metadata bytes. MCP records are preserved
// untouched.
func (t *serviceTxn) commitStateModels(meta state.MetadataV2, records []state.AgentModelV2, now time.Time) error {
	return t.commitStateRecords(meta, meta.MCPs, records, now)
}

// commitStateRecords persists both managed record sets together so a partial
// uninstall can commit truthful MCP and agent-model metadata in one
// journaled transaction.
func (t *serviceTxn) commitStateRecords(meta state.MetadataV2, mcps []state.MCPV2, models []state.AgentModelV2, now time.Time) error {
	meta.MCPs = mcps
	meta.AgentModels = models
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

// agentModelSidecarName namespaces one agent's sidecar record.
func agentModelSidecarName(agent string) string {
	return agentModelSidecarNamespace + agent
}

// applyModelResult projects one manager result onto the receipt.
func applyModelResult(receipt *ModelReceipt, result modelmgr.Result) {
	receipt.Agent = result.Agent
	receipt.Action = result.Action
	receipt.ConfigPath = result.ConfigPath
	receipt.Previous = result.Previous
	receipt.Value = result.Value
	receipt.Changed = result.Changed
	receipt.Managed = result.Ownership != nil
	if result.Warning != "" {
		receipt.Warnings = append(receipt.Warnings, result.Warning)
	}
}

// compactAgentEntry renders an entry in the canonical compact reference form,
// disclosing the markdown pin when no config entry provides a value.
func compactAgentEntry(entry modelmgr.AgentEntry) string {
	switch {
	case entry.Model == "":
		return entry.MarkdownPin
	case entry.Variant == "":
		return entry.Model
	default:
		return entry.Model + "#" + entry.Variant
	}
}
