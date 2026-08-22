package install

import (
	"fmt"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/lleontor705/cortex-ia/internal/mcpmanager"
	"github.com/lleontor705/cortex-ia/internal/pipeline"
	"github.com/lleontor705/cortex-ia/internal/state"
)

// DoctorVerdict summarizes a doctor report.
type DoctorVerdict string

const (
	// DoctorHealthy: the recorded installation matches the disk exactly.
	DoctorHealthy DoctorVerdict = "healthy"
	// DoctorDegraded: the installation is present but some recorded
	// artifact is missing or drifted, or a selected MCP entry is no longer
	// accredited.
	DoctorDegraded DoctorVerdict = "degraded"
	// DoctorBlocked: state or lock is malformed or the two disagree, so no
	// safe verdict about ownership exists.
	DoctorBlocked DoctorVerdict = "blocked"
	// DoctorNotInstalled: no v2 installation metadata exists (absent or
	// legacy; legacy carries a read-only migration assessment).
	DoctorNotInstalled DoctorVerdict = "not-installed"
)

// ArtifactStatus classifies one recorded artifact against the disk.
type ArtifactStatus string

const (
	ArtifactOK        ArtifactStatus = "ok"
	ArtifactMissing   ArtifactStatus = "missing"
	ArtifactDrifted   ArtifactStatus = "drifted"
	ArtifactIrregular ArtifactStatus = "irregular"
)

// ArtifactCheck is the doctor verdict for one recorded artifact.
type ArtifactCheck struct {
	// Path is the artifact path relative to the OpenCode root.
	Path string `json:"path"`
	// Dest is the artifact destination relative to the home.
	Dest string `json:"dest"`
	// Status compares the recorded digest with the observed bytes.
	Status ArtifactStatus `json:"status"`
	// ExpectedDigest is the digest recorded in v2 metadata.
	ExpectedDigest string `json:"expected_digest,omitempty"`
	// ObservedDigest is the current on-disk digest, when readable.
	ObservedDigest string `json:"observed_digest,omitempty"`
	// Problem explains a non-converged status, or carries an informational
	// note for statuses that are fine by design.
	Problem string `json:"problem,omitempty"`
}

// MCPCheck is the doctor verdict for one managed MCP preset.
type MCPCheck struct {
	Name   string                 `json:"name"`
	Status mcpmanager.EntryStatus `json:"status"`
	// Digest is the observed semantic digest, when an entry exists.
	Digest string `json:"digest,omitempty"`
	// Expected reports whether v2 metadata records the preset as an
	// installed, ownership-accredited selection.
	Expected bool `json:"expected"`
}

// JournalCandidate is one retained journal checkpoint classified for
// recovery: its identity, transaction state, declared target summary, and
// whether recovering it is validated and safe. It carries no secrets —
// journal checkpoints record digests and paths only.
type JournalCandidate struct {
	// ID is the journal checkpoint directory name (journal-<timestamp>).
	ID string `json:"id"`
	// BackupID is the backup transaction that left the journal.
	BackupID string `json:"backup_id,omitempty"`
	// State is the journal's recorded transaction state; "unreadable" marks
	// a checkpoint that failed validated loading.
	State pipeline.JournalState `json:"state"`
	// Recoverable reports that recover may restore this journal after
	// explicit confirmation.
	Recoverable bool `json:"recoverable"`
	// Reason explains the classification, including why a candidate is not
	// recoverable.
	Reason string `json:"reason,omitempty"`
	// CheckpointPath is the journal.json path beneath the home's backups.
	CheckpointPath string `json:"checkpoint_path"`
	// Targets lists the declared home-relative targets of the journal.
	Targets []string `json:"targets,omitempty"`
}

// journalStateUnreadable marks a checkpoint whose validated load failed; it
// can never accredit recovery.
const journalStateUnreadable pipeline.JournalState = "unreadable"

// backupsRoot returns the retained backup tree root beneath the service
// home, where every engine, service, and rollback journal lives.
func (s *Service) backupsRoot() string {
	return filepath.Join(s.homeDir, ".cortex-ia", "backups")
}

// PendingJournals enumerates every retained journal beneath the service
// home's backups and classifies it read-only. Only incomplete transactions
// are returned: committed and restored journals are terminal and excluded.
// A candidate is Recoverable only when its checkpoint decodes, obeys the
// journal contract (schema, containment, alias rejection), and targets
// exactly this home; corrupt or foreign journals are reported as
// non-recoverable with their typed reason instead of failing the listing.
// The call never mutates anything and never acquires the home lock.
func (s *Service) PendingJournals() ([]JournalCandidate, error) {
	checkpoints, err := pipeline.DiscoverJournals(s.backupsRoot())
	if err != nil {
		return nil, err
	}
	candidates := make([]JournalCandidate, 0, len(checkpoints))
	for _, checkpoint := range checkpoints {
		candidate := classifyJournalCheckpoint(s.homeDir, checkpoint)
		if candidate != nil {
			candidates = append(candidates, *candidate)
		}
	}
	return candidates, nil
}

// classifyJournalCheckpoint loads and classifies one checkpoint, returning
// nil for terminal (committed or restored) journals that are not recovery
// candidates. Loading and validation errors are findings, never panics.
func classifyJournalCheckpoint(home, checkpoint string) *JournalCandidate {
	journalID, backupID := journalIDFromCheckpoint(checkpoint)
	journal, err := pipeline.LoadInstallJournal(checkpoint)
	if err != nil {
		return &JournalCandidate{
			ID: journalID, BackupID: backupID, State: journalStateUnreadable,
			Recoverable: false, CheckpointPath: checkpoint,
			Reason: fmt.Sprintf("journal checkpoint is corrupt or invalid: %v", err),
		}
	}
	if !pipeline.JournalPending(journal.State) {
		return nil
	}
	candidate := &JournalCandidate{
		ID: journalID, BackupID: backupID, State: journal.State,
		Recoverable: true, CheckpointPath: checkpoint,
		Reason: "incomplete transaction; recover restores the journaled preimages after validation",
	}
	for _, target := range journal.Targets {
		candidate.Targets = append(candidate.Targets, target.Path)
	}
	if !sameHomeRoot(home, journal.TargetRoot) {
		candidate.Recoverable = false
		candidate.Reason = fmt.Sprintf("journal targets foreign home root %q", journal.TargetRoot)
	}
	return candidate
}

// journalIDFromCheckpoint derives the journal and backup identifiers from
// the canonical checkpoint layout
// <backups>/<backupID>/journal/<journalID>/journal.json.
func journalIDFromCheckpoint(checkpoint string) (journalID, backupID string) {
	journalDir := filepath.Dir(checkpoint)
	journalID = filepath.Base(journalDir)
	journalRoot := filepath.Dir(journalDir)
	backupDir := filepath.Dir(journalRoot)
	backupID = filepath.Base(backupDir)
	return journalID, backupID
}

// sameHomeRoot compares journal and service home roots under the platform's
// path-identity policy: case-folding on Windows and macOS, exact on Unix.
func sameHomeRoot(home, target string) bool {
	home = filepath.Clean(home)
	target = filepath.Clean(target)
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		return strings.EqualFold(home, target)
	}
	return home == target
}

// DoctorReport is the read-only health assessment of one home. Doctor never
// mutates anything: it loads state tolerantly, compares recorded digests
// with observed bytes, and lists MCP ownership through the manager.
type DoctorReport struct {
	HomeDir string `json:"home_dir"`
	// OpencodeRoot is the recorded OpenCode root (v2 metadata only).
	OpencodeRoot string `json:"opencode_root,omitempty"`
	// MetadataPresence and LockPresence classify the state documents.
	MetadataPresence state.MetadataPresence `json:"metadata_presence"`
	LockPresence     state.MetadataPresence `json:"lock_presence"`
	// AgreementError describes a state/lock disagreement.
	AgreementError string `json:"agreement_error,omitempty"`
	// Migration carries the read-only legacy assessment when no v2
	// metadata exists.
	Migration *state.MigrationDecision `json:"migration,omitempty"`
	// Selection mirrors the recorded install intent.
	Selection state.SelectionV2 `json:"selection"`
	// Artifacts are the per-artifact checks in recorded order.
	Artifacts []ArtifactCheck `json:"artifacts,omitempty"`
	// MCPConfigPath is the config file the manager would mutate.
	MCPConfigPath string `json:"mcp_config_path,omitempty"`
	// MCPs are the per-preset ownership checks.
	MCPs []MCPCheck `json:"mcps,omitempty"`
	// UnknownMCPs names non-managed MCP entries found in the config.
	UnknownMCPs []string `json:"unknown_mcps,omitempty"`
	// Journals lists incomplete install journals found beneath the home's
	// backups, with their recoverability classification.
	Journals []JournalCandidate `json:"journals,omitempty"`
	// Verdict is the overall assessment.
	Verdict DoctorVerdict `json:"verdict"`
	// Findings are human-readable observations, including informational
	// notes that do not affect the verdict.
	Findings []string `json:"findings,omitempty"`
}

// Doctor assesses the health of the installation in the service home. It is
// strictly read-only: state is loaded tolerantly, artifacts are compared by
// digest, MCP ownership is listed through the manager (which never writes),
// and retained journals are enumerated and validated without ever acquiring
// the home lock — doctor can run during another process's mutation and
// never steals the writer lock. Shape problems are reported as a blocked
// verdict, never as an error, so front ends can always render a report.
// Incomplete journals degrade the verdict and name the recover remedy.
func (s *Service) Doctor() (*DoctorReport, error) {
	report := &DoctorReport{HomeDir: s.homeDir, Verdict: DoctorHealthy}
	s.assessState(report)
	s.assessJournals(report)
	return report, nil
}

// assessState fills the state, artifact, and MCP checks and derives their
// verdict contribution.
func (s *Service) assessState(report *DoctorReport) {
	metaLoad := state.LoadMetadataV2(s.homeDir)
	lockLoad := state.LoadLockV2(s.homeDir)
	report.MetadataPresence = metaLoad.Presence
	report.LockPresence = lockLoad.Presence

	switch {
	case metaLoad.Presence == state.PresenceMalformed:
		report.Verdict = DoctorBlocked
		report.Findings = append(report.Findings, fmt.Sprintf("state metadata is malformed: %s", metaLoad.Detail))
		return
	case lockLoad.Presence == state.PresenceMalformed:
		report.Verdict = DoctorBlocked
		report.Findings = append(report.Findings, fmt.Sprintf("lock metadata is malformed: %s", lockLoad.Detail))
		return
	case metaLoad.Presence == state.PresenceV2 && lockLoad.Presence != state.PresenceV2:
		report.Verdict = DoctorBlocked
		report.Findings = append(report.Findings, "v2 state without an agreeing v2 lock: fail closed")
		return
	case metaLoad.Presence != state.PresenceV2 && lockLoad.Presence == state.PresenceV2:
		report.Verdict = DoctorBlocked
		report.Findings = append(report.Findings, "v2 lock without an agreeing v2 state: fail closed")
		return
	case metaLoad.Presence == state.PresenceV2:
		if err := state.CheckAgreementV2(metaLoad.Metadata, lockLoad.Lock); err != nil {
			report.Verdict = DoctorBlocked
			report.AgreementError = err.Error()
			report.Findings = append(report.Findings, fmt.Sprintf("state/lock disagree: %s", err))
			return
		}
		s.doctorInstalled(report, metaLoad.Metadata)
		return
	default:
		// No v2 metadata: absent or legacy. Report the read-only legacy
		// assessment so operators can see why migration would block.
		report.Verdict = DoctorNotInstalled
		root, err := opencodeRoot(s.homeDir)
		if err != nil {
			report.Findings = append(report.Findings, fmt.Sprintf("resolve OpenCode root: %v", err))
			return
		}
		decision := state.AssessMigration(s.homeDir, root, time.Now().UTC())
		report.Migration = &decision
		if metaLoad.Presence == state.PresenceLegacy || lockLoad.Presence == state.PresenceLegacy {
			report.Findings = append(report.Findings, "legacy (v1) metadata found; it is never rewritten or removed")
		}
		if decision.Blocker != nil {
			report.Findings = append(report.Findings, fmt.Sprintf("legacy migration blocked: %s; %s", decision.Blocker.Reason, decision.Blocker.Remediation))
		}
		return
	}
}

// assessJournals classifies every retained journal beneath the home's
// backups, read-only. A recoverable incomplete journal degrades the verdict
// and names the recover remedy; a corrupt or foreign journal degrades it
// with a manual-remediation finding. An unreadable backups tree degrades
// instead of failing the whole report.
func (s *Service) assessJournals(report *DoctorReport) {
	candidates, err := s.PendingJournals()
	if err != nil {
		report.Findings = append(report.Findings, fmt.Sprintf("install journals could not be assessed: %v (manual inspection required)", err))
		if report.Verdict == DoctorHealthy {
			report.Verdict = DoctorDegraded
		}
		return
	}
	for _, candidate := range candidates {
		report.Journals = append(report.Journals, candidate)
		if candidate.Recoverable {
			report.Findings = append(report.Findings, fmt.Sprintf(
				"incomplete install journal %q (state %s, backup %q): run recover with explicit confirmation to restore the journaled preimages",
				candidate.ID, candidate.State, candidate.BackupID))
		} else {
			report.Findings = append(report.Findings, fmt.Sprintf(
				"retained journal %q is not recoverable: %s; manual remediation required",
				candidate.ID, candidate.Reason))
		}
		if report.Verdict == DoctorHealthy {
			report.Verdict = DoctorDegraded
		}
	}
}

// doctorInstalled fills the per-artifact and per-MCP checks for an agreed v2
// installation and derives the final verdict from the engine's own read-only
// reconciliation plan, so doctor semantics are exactly the engine's
// convergence semantics — never a re-implemented digest or merge policy.
func (s *Service) doctorInstalled(report *DoctorReport, meta state.MetadataV2) {
	report.OpencodeRoot = meta.OpencodeRoot
	report.Selection = meta.Selection

	userOwned := 0
	for _, artifact := range meta.Artifacts {
		if artifact.Ownership != state.OwnershipManaged {
			userOwned++
			continue
		}
		abs := s.artifactAbs(artifact)
		check := ArtifactCheck{
			Path:           artifact.Path,
			Dest:           homeRelative(s.homeDir, abs),
			ExpectedDigest: artifact.Digest,
		}
		exists, digest, err := fileDigest(abs)
		switch {
		case err != nil:
			check.Status = ArtifactIrregular
			check.ObservedDigest = digest
			check.Problem = err.Error()
		case !exists:
			check.Status = ArtifactMissing
		case digest == artifact.Digest:
			check.Status = ArtifactOK
			check.ObservedDigest = digest
		case artifact.Kind == state.KindMCPConfig:
			// The settings template is installed by value-level merge and
			// is legitimately extended afterwards (user keys, managed MCP
			// entries), so byte equality is not the ownership contract.
			// Value-level convergence is judged by the engine plan below;
			// here presence and readability are the local facts.
			check.Status = ArtifactOK
			check.ObservedDigest = digest
			check.Problem = "settings file extends the template (value-level convergence is verified by the reconciliation plan)"
		default:
			check.Status = ArtifactDrifted
			check.ObservedDigest = digest
			check.Problem = "on-disk bytes no longer match the recorded digest; cortex-ia no longer owns this artifact"
		}
		if check.Status != ArtifactOK {
			report.Findings = append(report.Findings, fmt.Sprintf("artifact %q: %s %s", check.Dest, check.Status, check.Problem))
		}
		report.Artifacts = append(report.Artifacts, check)
	}
	if userOwned > 0 {
		report.Findings = append(report.Findings, fmt.Sprintf("%d user-owned artifact(s) recorded; doctor never inspects or touches them", userOwned))
	}

	manager := mcpmanager.New(s.homeDir)
	fingerprintMissing := false
	fingerprintPresent := false
	customManaged := make(map[string]struct{})
	report.MCPConfigPath = manager.ConfigPath()
	expected := make(map[string]bool, len(meta.MCPs))
	for _, mcp := range meta.MCPs {
		if mcp.Ownership != state.OwnershipManaged {
			continue
		}
		expected[mcp.Name] = true
		if _, isCatalog := mcpmanager.Lookup(mcp.Name); !isCatalog {
			customManaged[mcp.Name] = struct{}{}
		}
	}

	fingerprintDoc, fpPresent, fpErr := state.LoadFingerprintDocument(s.homeDir)
	if fpErr != nil {
		report.Verdict = DoctorBlocked
		report.Findings = append(report.Findings, fmt.Sprintf("MCP postimage fingerprint sidecar is unreadable: %v", fpErr))
		return
	}
	if fpPresent {
		fingerprintPresent = true
		salt, saltErr := fingerprintDoc.SaltBytes()
		if saltErr != nil {
			report.Verdict = DoctorBlocked
			report.Findings = append(report.Findings, fmt.Sprintf("MCP postimage fingerprint sidecar is invalid: %v", saltErr))
			return
		}
		manager = mcpmanager.NewFingerprinting(s.homeDir, salt)
	}
	if !fpPresent {
		fingerprintMissing = true
		report.Findings = append(report.Findings, "MCP postimage fingerprint sidecar is missing; drift of custom MCPs cannot be verified with postimage certainty (re-run custom MCP add commands to regenerate fingerprints)")
	}
	if fingerprintPresent {
		recordsEvidence := withPostImageEvidence(ownershipEvidence(meta), fingerprintDoc, meta)
		listing, err := manager.List(recordsEvidence)
		if err != nil {
			report.Verdict = DoctorBlocked
			report.Findings = append(report.Findings, fmt.Sprintf("MCP config cannot be assessed: %v", err))
			return
		}
		report.UnknownMCPs = listing.Unknown
		for _, entry := range listing.Entries {
			check := MCPCheck{Name: entry.Name, Status: entry.Status, Digest: entry.Digest, Expected: expected[entry.Name]}
			switch {
			case check.Expected && entry.Status != mcpmanager.StatusManaged:
				report.Findings = append(report.Findings, fmt.Sprintf("MCP %q is recorded as installed but is not ownership-accredited (status %q)", entry.Name, entry.Status))
			case !check.Expected && entry.Status == mcpmanager.StatusManaged:
				report.Findings = append(report.Findings, fmt.Sprintf("MCP %q is accredited but not recorded as installed", entry.Name))
			}
			report.MCPs = append(report.MCPs, check)
		}
	} else {
		listing, err := manager.List(ownershipEvidence(meta))
		if err != nil {
			report.Verdict = DoctorBlocked
			report.Findings = append(report.Findings, fmt.Sprintf("MCP config cannot be assessed: %v", err))
			return
		}
		report.UnknownMCPs = listing.Unknown
		for _, entry := range listing.Entries {
			check := MCPCheck{Name: entry.Name, Status: entry.Status, Digest: entry.Digest, Expected: expected[entry.Name]}
			switch {
			case check.Expected && entry.Status != mcpmanager.StatusManaged:
				report.Findings = append(report.Findings, fmt.Sprintf("MCP %q is recorded as installed but is not ownership-accredited (status %q)", entry.Name, entry.Status))
			case !check.Expected && entry.Status == mcpmanager.StatusManaged:
				report.Findings = append(report.Findings, fmt.Sprintf("MCP %q is accredited but not recorded as installed", entry.Name))
			}
			report.MCPs = append(report.MCPs, check)
		}
	}

	if fingerprintMissing && len(customManaged) > 0 {
		report.Verdict = DoctorDegraded
		missingCustom := make([]string, 0, len(customManaged))
		for name := range customManaged {
			missingCustom = append(missingCustom, name)
		}
		sort.Strings(missingCustom)
		report.Findings = append(report.Findings, fmt.Sprintf("MCP postimage fingerprints are missing for managed custom MCPs %v; these entries cannot be drift-checked with postimage certainty", missingCustom))
	}
	if len(report.UnknownMCPs) > 0 {
		report.Findings = append(report.Findings, fmt.Sprintf("unknown MCP entries present (informational, never touched): %v", report.UnknownMCPs))
	}

	// The verdict is the engine's own read-only reconciliation against the
	// recorded selection: converged means healthy, conflicts and pending
	// mutations mean degraded, and a planning error means the ownership
	// picture cannot be trusted.
	plan, err := pipeline.PlanSync(s.request(Options{
		Cortex:    meta.Selection.Cortex,
		ForgeSpec: meta.Selection.ForgeSpec,
		Context7:  meta.Selection.Context7,
	}))
	if err != nil {
		report.Verdict = DoctorBlocked
		report.Findings = append(report.Findings, fmt.Sprintf("reconciliation planning failed: %v", err))
		return
	}
	if plan.Converged {
		return
	}
	report.Verdict = DoctorDegraded
	for _, conflict := range plan.Conflicts {
		report.Findings = append(report.Findings, fmt.Sprintf("conflict %s: %s (%s)", conflict.Target, conflict.Kind, conflict.Reason))
	}
	for _, effect := range plan.Effects {
		if effect.Kind == pipeline.EffectNoop || effect.Kind == pipeline.EffectMCPNoop {
			continue
		}
		report.Findings = append(report.Findings, fmt.Sprintf("pending reconciliation: %s %s", effect.Kind, effect.Dest))
	}
}

// ownershipEvidence projects recorded MCP ownership onto manager records.
// The absolute config path is reconstructed from the recorded OpenCode root
// so accreditation stays bound to one file, exactly like the engine's
// planning evidence.
func ownershipEvidence(meta state.MetadataV2) []mcpmanager.OwnershipRecord {
	records := make([]mcpmanager.OwnershipRecord, 0, len(meta.MCPs))
	for _, mcp := range meta.MCPs {
		if mcp.Ownership != state.OwnershipManaged {
			continue
		}
		records = append(records, mcpmanager.OwnershipRecord{
			Name:       mcp.Name,
			Digest:     mcp.SemanticDigest,
			ConfigPath: filepath.Join(meta.OpencodeRoot, filepath.FromSlash(mcp.ConfigPath)),
		})
	}
	return records
}
