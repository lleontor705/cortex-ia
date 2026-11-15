package pipeline

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/lleontor705/cortex-ia/internal/agents/opencode"
	"github.com/lleontor705/cortex-ia/internal/mcpmanager"
	"github.com/lleontor705/cortex-ia/internal/state"
)

// EffectKind enumerates the operations the copy engine is allowed to plan.
type EffectKind string

const (
	// EffectCreate writes an embedded asset whose destination is absent.
	EffectCreate EffectKind = "create"
	// EffectNoop records an already-converged destination.
	EffectNoop EffectKind = "noop"
	// EffectManagedUpdate replaces a file cortex-ia still owns (the recorded
	// digest matches the bytes on disk) with a newer embedded source.
	EffectManagedUpdate EffectKind = "managed-update"
	// EffectOverwrite replaces an unmanaged or user-drifted file. It is only
	// planned when the request carries an explicit overwrite authorization,
	// and apply captures and verifies a restorable backup first.
	EffectOverwrite EffectKind = "overwrite"
	// EffectSafeMerge merges the embedded settings template into an existing
	// well-formed opencode.jsonc, preserving unrelated keys and comments.
	EffectSafeMerge EffectKind = "safe-merge"
	// EffectDelete removes a stale owned artifact during sync.
	EffectDelete EffectKind = "delete"
	// EffectMCPAdd registers a selected managed MCP preset.
	EffectMCPAdd EffectKind = "mcp-add"
	// EffectMCPNoop records an already-converged managed MCP entry.
	EffectMCPNoop EffectKind = "mcp-noop"
	// EffectMCPRemove deregisters a managed MCP preset that is no longer
	// selected, accredited by transactional ownership evidence.
	EffectMCPRemove EffectKind = "mcp-remove"
)

// ConflictKind classifies fail-closed planning blockers.
type ConflictKind string

const (
	// ConflictUnmanagedExisting: a desired destination holds a file that no
	// recorded ownership evidence covers.
	ConflictUnmanagedExisting ConflictKind = "unmanaged-existing"
	// ConflictUnmanagedDrift: a file recorded as managed no longer matches
	// its recorded digest, so cortex-ia no longer owns it.
	ConflictUnmanagedDrift ConflictKind = "unmanaged-drift"
	// ConflictMalformedConfig: the existing settings file cannot be parsed
	// as a JSON(C) object, so no safe merge exists. Overwrite never clears
	// this conflict.
	ConflictMalformedConfig ConflictKind = "malformed-config"
	// ConflictMCP: the MCP manager reported a fail-closed ownership or
	// content conflict. Overwrite never clears this conflict.
	ConflictMCP ConflictKind = "mcp-conflict"
	// ConflictStaleDrift: sync found a stale owned artifact whose bytes were
	// modified after installation; it is never deleted.
	ConflictStaleDrift ConflictKind = "stale-drift"
)

// Conflict describes one fail-closed blocker with its remediation hint.
type Conflict struct {
	// Target is the home-relative slash path, or the MCP server name for
	// MCP conflicts.
	Target string       `json:"target"`
	Kind   ConflictKind `json:"kind"`
	Reason string       `json:"reason"`
	// OverwriteAuthorized reports whether an explicit overwrite request
	// would convert this conflict into an EffectOverwrite.
	OverwriteAuthorized bool `json:"overwrite_authorized"`
}

// ConflictError is returned by apply when the plan carries unresolved
// conflicts. Nothing is mutated.
type ConflictError struct {
	Conflicts []Conflict
}

func (e *ConflictError) Error() string {
	parts := make([]string, 0, len(e.Conflicts))
	for _, conflict := range e.Conflicts {
		parts = append(parts, fmt.Sprintf("%s: %s (%s)", conflict.Target, conflict.Kind, conflict.Reason))
	}
	return "planning conflicts prevent any mutation: " + strings.Join(parts, "; ")
}

// ErrPlanDrift is the sentinel every stale-plan rejection satisfies. It is
// returned before any backup, write, or metadata mutation.
var ErrPlanDrift = errors.New("plan drift: the confirmed plan no longer matches the current home")

// PlanDriftError reports that a caller-confirmed plan digest does not match
// the freshly derived plan, so the confirmation was issued for a different
// operation set. Nothing was mutated.
type PlanDriftError struct {
	// Expected is the digest the caller confirmed.
	Expected string
	// Observed is the digest of the fresh replan.
	Observed string
}

func (e *PlanDriftError) Error() string {
	return fmt.Sprintf("stale plan: confirmed digest %s but replanned digest %s; re-review and confirm the new plan",
		planDigestPrefix(e.Expected), planDigestPrefix(e.Observed))
}

// Unwrap binds every plan-drift rejection to the ErrPlanDrift sentinel.
func (e *PlanDriftError) Unwrap() error { return ErrPlanDrift }

// Effect is one planned operation with the evidence it was derived from.
type Effect struct {
	Kind EffectKind `json:"kind"`
	// Dest is the home-relative slash destination (or MCP server name for
	// MCP effects).
	Dest string `json:"dest"`
	// Source is the embedded asset path for copy effects.
	Source string `json:"source,omitempty"`
	// SourceSHA is the embedded source digest.
	SourceSHA string `json:"source_sha,omitempty"`
	// CurrentSHA is the observed preimage digest; empty when absent.
	CurrentSHA string `json:"current_sha,omitempty"`
	// PriorExists records whether the destination existed at planning time.
	PriorExists bool   `json:"prior_exists"`
	Reason      string `json:"reason,omitempty"`
}

// Plan is the pure, read-only result of planning. Deriving it never touches
// the filesystem: no directories, journals, backups, state, or temporaries
// are created.
type Plan struct {
	// HomeDir is the absolute target home.
	HomeDir string `json:"home_dir"`
	// OpencodeRoot is the absolute OpenCode configuration root.
	OpencodeRoot string `json:"opencode_root"`
	// Effects is the sorted operation set (including noops).
	Effects []Effect `json:"effects"`
	// Conflicts are the fail-closed blockers; apply refuses while any exist.
	Conflicts []Conflict `json:"conflicts"`
	// Converged reports that a second identical execution needs zero writes,
	// backups, journals, and metadata churn.
	Converged bool `json:"converged"`
	// Digest binds confirmations to exactly this operation set.
	Digest string `json:"digest"`
	// Metadata is the agreed prior v2 metadata; zero when absent or legacy.
	Metadata state.MetadataV2 `json:"metadata"`
	// MetadataPresence and LockPresence classify what was found on disk.
	MetadataPresence state.MetadataPresence `json:"metadata_presence"`
	LockPresence     state.MetadataPresence `json:"lock_presence"`
	// Migration is the read-only legacy assessment when no v2 metadata was
	// present; nil otherwise.
	Migration *state.MigrationDecision `json:"migration,omitempty"`
	// MCPConfigRel is the home-relative slash config file the MCP manager
	// targets in this home, or empty when no MCP effect was planned.
	MCPConfigRel string `json:"mcp_config_rel,omitempty"`

	// mappings and evidence are the unexported planning context: the derived
	// asset mappings and the accredited MCP ownership evidence. They never
	// serialize and never escape the package.
	mappings []opencode.Mapping
	evidence []mcpmanager.OwnershipRecord
}

// mutating reports whether the effect writes, merges, or deletes anything.
func (e Effect) mutating() bool {
	switch e.Kind {
	case EffectNoop, EffectMCPNoop:
		return false
	default:
		return true
	}
}

// Receipt is the typed outcome of one engine execution.
type Receipt struct {
	PlanDigest     string     `json:"plan_digest"`
	TransactionID  string     `json:"transaction_id,omitempty"`
	BackupID       string     `json:"backup_id,omitempty"`
	BackupVerified bool       `json:"backup_verified"`
	DryRun         bool       `json:"dry_run"`
	Converged      bool       `json:"converged"`
	Changes        []string   `json:"changes,omitempty"`
	Conflicts      []Conflict `json:"conflicts,omitempty"`
	// Restored reports that a failed apply completed a verified reverse
	// restoration of every preimage.
	Restored bool `json:"restored,omitempty"`
	// RestoreError describes a failed restoration, which always leaves the
	// journal on disk for a safe retry.
	RestoreError string   `json:"restore_error,omitempty"`
	Warnings     []string `json:"warnings,omitempty"`
}

// Request is the typed input of the OpenCode transactional copy engine.
type Request struct {
	// HomeDir is the target home directory. Tests always supply a temporary
	// directory; the engine never falls back to the process home.
	HomeDir string
	// Version labels the install in receipts.
	Version string
	// Cortex and Context7 select managed MCP presets. Names are
	// never hardcoded by the engine: it resolves presets from the manager
	// catalog and fails closed on unknown selections.
	Cortex   bool
	Context7 bool
	// Overwrite explicitly authorizes replacing unmanaged conflicting
	// files. Apply still captures and verifies a restorable backup of every
	// overwritten target before mutating it.
	Overwrite bool
	// DryRun returns the plan and a receipt without any filesystem effect.
	DryRun bool
	// ExpectedPlanDigest optionally binds this call to one previously
	// confirmed plan digest. When set, the freshly derived plan must carry
	// exactly this digest or the call aborts with ErrPlanDrift before any
	// backup or write. Empty preserves the historical unbound behavior.
	ExpectedPlanDigest string
	// Now overrides the clock for deterministic tests.
	Now func() time.Time
	// Probes optionally supply MCP qualification evidence per server name.
	Probes map[string][]mcpmanager.ProbeFunc
}

func (r Request) now() time.Time {
	if r.Now != nil {
		return r.Now().UTC()
	}
	return time.Now().UTC()
}

// mcpSelection resolves the requested preset selection against the manager
// catalog. Unknown requested names cannot exist in this typed request, but
// the selection is still derived from Presets(), never from a local catalog.
func (r Request) mcpSelection() map[string]bool {
	requested := map[string]bool{
		"cortex":   r.Cortex,
		"context7": r.Context7,
	}
	selection := make(map[string]bool, len(requested))
	for _, preset := range mcpmanager.Presets() {
		selection[preset.Name] = requested[preset.Name]
	}
	return selection
}

// PlanInstall derives the complete install operation set for the request.
// It is read-only: deriving a plan never creates directories, journals,
// backups, state, lock, or destination temporaries.
func PlanInstall(req Request) (*Plan, error) {
	plan, err := planCommon(req)
	if err != nil {
		return nil, err
	}
	if err := planAssetEffects(plan, req.Overwrite); err != nil {
		return nil, err
	}
	if err := planMCPEffects(plan, req); err != nil {
		return nil, err
	}
	finalizePlan(plan, req)
	return plan, nil
}

// InstallV2 plans and (unless dry-run) transactionally applies the embedded
// OpenCode asset set and managed MCP selection. The receipt is the durable
// evidence of what changed; a converged request performs zero writes.
func InstallV2(req Request) (*Plan, *Receipt, error) {
	plan, err := PlanInstall(req)
	if err != nil {
		return plan, &Receipt{DryRun: req.DryRun}, err
	}
	if err := checkPlanDrift(req, plan); err != nil {
		return plan, &Receipt{DryRun: req.DryRun}, err
	}
	if req.DryRun {
		return plan, &Receipt{
			PlanDigest: plan.Digest, DryRun: true, Converged: plan.Converged,
			Conflicts: plan.Conflicts,
		}, nil
	}
	receipt, err := Apply(req, plan)
	return plan, receipt, err
}

// PlanSync derives the complete sync operation set for the request, including
// stale artifact deletion effects.
func PlanSync(req Request) (*Plan, error) {
	plan, err := planCommon(req)
	if err != nil {
		return nil, err
	}
	if err := planAssetEffects(plan, req.Overwrite); err != nil {
		return nil, err
	}
	if err := planMCPEffects(plan, req); err != nil {
		return nil, err
	}
	if err := planStaleEffects(plan); err != nil {
		return nil, err
	}
	finalizePlan(plan, req)
	return plan, nil
}

// SyncV2 plans and (unless dry-run) transactionally applies reconciliation.
func SyncV2(req Request) (*Plan, *Receipt, error) {
	plan, err := PlanSync(req)
	if err != nil {
		return plan, &Receipt{DryRun: req.DryRun}, err
	}
	if err := checkPlanDrift(req, plan); err != nil {
		return plan, &Receipt{DryRun: req.DryRun}, err
	}
	if req.DryRun {
		return plan, &Receipt{
			PlanDigest: plan.Digest, DryRun: true, Converged: plan.Converged,
			Conflicts: plan.Conflicts,
		}, nil
	}
	receipt, err := Apply(req, plan)
	return plan, receipt, err
}

// checkPlanDrift compares a caller-confirmed digest against the freshly
// derived plan before any side effect can begin. An empty expectation keeps
// the current unbound compatibility.
func checkPlanDrift(req Request, plan *Plan) error {
	if req.ExpectedPlanDigest == "" {
		return nil
	}
	if plan.Digest != req.ExpectedPlanDigest {
		return &PlanDriftError{Expected: req.ExpectedPlanDigest, Observed: plan.Digest}
	}
	return nil
}

// planDigest binds confirmations to exactly this operation set: the target
// home, the resolved MCP config destination, every effect with its
// destination and preimage preconditions, and every conflict. Writes
// to a hash never fail; the discarded errors are explicit.
func planDigest(plan *Plan) string {
	hash := sha256.New()
	_, _ = fmt.Fprintf(hash, "home\x00%s\x00mcp-config\x00%s\x00", plan.HomeDir, plan.MCPConfigRel)
	for _, effect := range plan.Effects {
		_, _ = fmt.Fprintf(hash, "%s\x00%s\x00%s\x00%s\x00%t\x00", effect.Kind, effect.Dest, effect.SourceSHA, effect.CurrentSHA, effect.PriorExists)
	}
	for _, conflict := range plan.Conflicts {
		_, _ = fmt.Fprintf(hash, "conflict\x00%s\x00%s\x00%t\x00", conflict.Target, conflict.Kind, conflict.OverwriteAuthorized)
	}
	return hex.EncodeToString(hash.Sum(nil))
}

// finalizePlan sorts effects deterministically, computes convergence and the
// plan digest. Mutating file effects sort by destination; MCP effects follow.
func finalizePlan(plan *Plan, req Request) {
	sort.SliceStable(plan.Effects, func(i, j int) bool {
		oi, oj := effectOrder(plan.Effects[i].Kind), effectOrder(plan.Effects[j].Kind)
		if oi != oj {
			return oi < oj
		}
		if plan.Effects[i].Kind != plan.Effects[j].Kind {
			return plan.Effects[i].Kind < plan.Effects[j].Kind
		}
		return plan.Effects[i].Dest < plan.Effects[j].Dest
	})
	plan.Digest = planDigest(plan)
	plan.Converged = planConverged(plan, req)
}

func effectOrder(kind EffectKind) int {
	switch kind {
	case EffectDelete:
		return 0
	case EffectCreate, EffectManagedUpdate, EffectOverwrite, EffectSafeMerge:
		return 1
	case EffectMCPAdd, EffectMCPRemove, EffectMCPNoop:
		return 2
	default: // EffectNoop
		return 3
	}
}

// planConverged reports whether applying this plan would be a pure no-op:
// every effect converged and the recorded metadata already equals the
// desired artifact, MCP, and selection sets.
func planConverged(plan *Plan, req Request) bool {
	if len(plan.Conflicts) > 0 {
		return false
	}
	if plan.MetadataPresence != state.PresenceV2 || plan.LockPresence != state.PresenceV2 {
		return false
	}
	desired := desiredArtifacts(plan)
	if len(plan.Metadata.Artifacts) != len(desired) {
		return false
	}
	for path, artifact := range desired {
		recorded, ok := artifactIndex(plan.Metadata.Artifacts)[path]
		if !ok || recorded.Digest != artifact.Digest || recorded.Ownership != state.OwnershipManaged {
			return false
		}
	}
	selection := req.mcpSelection()
	recordedMCPs := mcpIndex(plan.Metadata.MCPs)
	for _, preset := range mcpmanager.Presets() {
		_, managed := recordedMCPs[preset.Name]
		if selection[preset.Name] != managed {
			return false
		}
	}
	if len(recordedMCPs) != countSelected(selection) {
		return false
	}
	if plan.Metadata.Selection.Cortex != req.Cortex ||
		plan.Metadata.Selection.Context7 != req.Context7 {
		return false
	}
	for _, effect := range plan.Effects {
		if effect.mutating() {
			return false
		}
	}
	return true
}

func countSelected(selection map[string]bool) int {
	count := 0
	for _, on := range selection {
		if on {
			count++
		}
	}
	return count
}

// artifactIndex indexes artifacts by path.
func artifactIndex(artifacts []state.ArtifactV2) map[string]state.ArtifactV2 {
	index := make(map[string]state.ArtifactV2, len(artifacts))
	for _, artifact := range artifacts {
		index[artifact.Path] = artifact
	}
	return index
}

// mcpIndex indexes MCP records by server name.
func mcpIndex(mcps []state.MCPV2) map[string]state.MCPV2 {
	index := make(map[string]state.MCPV2, len(mcps))
	for _, mcp := range mcps {
		index[mcp.Name] = mcp
	}
	return index
}

// opencodeRootAbs resolves the absolute OpenCode configuration root for a
// home directory.
func opencodeRootAbs(homeDir string) (string, error) {
	root := filepath.Join(homeDir, filepath.FromSlash(".config/opencode"))
	absolute, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve OpenCode root: %w", err)
	}
	return filepath.Clean(absolute), nil
}
