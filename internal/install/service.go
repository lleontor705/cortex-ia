// Package install composes the OpenCode transactional copy engine
// (internal/pipeline), the OpenCode MCP manager (internal/mcpmanager), the
// v2 state documents (internal/state), and the retained backup primitives
// (internal/backup) into the single installation service consumed by the CLI
// and the TUI.
//
// The service is OpenCode-only by construction: it never dispatches on agent
// identities and exposes exactly the operations a front end needs — plan,
// install, sync, doctor, rollback, uninstall, and managed MCP
// add/list/remove. It owns no copy, digest, or merge logic of its own: asset
// planning and transactional apply live in the pipeline, MCP ownership and
// JSONC merging live in the MCP manager, and digests are the recorded v2
// metadata digests or the manager's semantic digests.
package install

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/lleontor705/cortex-ia/internal/backup"
	"github.com/lleontor705/cortex-ia/internal/delegation"
	"github.com/lleontor705/cortex-ia/internal/homelock"
	"github.com/lleontor705/cortex-ia/internal/mcpmanager"
	"github.com/lleontor705/cortex-ia/internal/pipeline"
	"github.com/lleontor705/cortex-ia/internal/state"
)

// ErrHomeBusy reports that another process holds this home's mutation lock.
// It is the homelock sentinel re-exported so front ends can classify
// contention with errors.Is against the install package alone. A busy
// result never implies mutation.
var ErrHomeBusy = homelock.ErrHomeBusy

// DefaultHomeLockTimeout bounds canonical-home lock acquisition for every
// mutating service operation when the caller does not override it. The wait
// is bounded so contention surfaces as a typed ErrHomeBusy instead of an
// unbounded stall.
const DefaultHomeLockTimeout = 5 * time.Second

// Service is the OpenCode-only installation service bound to exactly one
// target home directory. It never reads process-global home state; every
// operation resolves paths beneath the supplied home.
type Service struct {
	homeDir string
}

// New returns a Service for the given home directory. An empty home is
// rejected: the service never falls back to the process home.
func New(homeDir string) (*Service, error) {
	if strings.TrimSpace(homeDir) == "" {
		return nil, errors.New("install service: home directory is required; the service never falls back to the process home")
	}
	absolute, err := filepath.Abs(homeDir)
	if err != nil {
		return nil, fmt.Errorf("install service: resolve home directory: %w", err)
	}
	return &Service{homeDir: filepath.Clean(absolute)}, nil
}

// HomeDir returns the absolute target home directory.
func (s *Service) HomeDir() string {
	return s.homeDir
}

// Options is the explicit, per-call intent for install and sync. The zero
// value is the safest request: no overwrite authorization, no dry-run flag,
// and no managed MCP selected. Nothing in the service upgrades these values
// implicitly; Overwrite and DryRun travel to the engine exactly as supplied.
type Options struct {
	// Cortex and Context7 select the active managed MCP presets.
	Cortex   bool
	Context7 bool
	// Overwrite explicitly authorizes replacing unmanaged conflicting
	// files. The engine still captures and verifies a restorable backup
	// of every overwritten target before mutating it.
	Overwrite bool
	// DryRun returns the plan and a receipt without any filesystem effect.
	DryRun bool
	// LockTimeout bounds canonical-home lock acquisition for the mutating
	// path. Zero or negative uses DefaultHomeLockTimeout. It is ignored by
	// read-only and zero-write paths, which never acquire the lock.
	LockTimeout time.Duration
	// Version labels the install in receipts.
	Version string
	// Now overrides the clock for deterministic runs.
	Now func() time.Time
	// Probes optionally supply MCP qualification evidence per server name.
	Probes map[string][]mcpmanager.ProbeFunc
	// ExpectedPlanDigest optionally binds a mutating call to one previously
	// displayed plan. Empty keeps the legacy unbound behavior; a non-empty
	// value travels to the engine exactly as supplied, which re-plans with
	// identical options and rejects any digest mismatch as a typed
	// stale-plan error before backup or mutation.
	ExpectedPlanDigest string
	// DelegationConfig optionally carries user choices for Herdr and external CLI delegation.
	DelegationConfig *delegation.DelegationConfig
}

// DefaultOptions returns the recommended OpenCode selection as data: Cortex
// on and Context7 optional. Work control is built into cortex-ia. Returning this
// value is the only place the default exists — no service method injects or
// extends a selection implicitly.
func DefaultOptions() Options {
	return Options{Cortex: true, Context7: false}
}

// request projects the service options onto the engine request type.
func (s *Service) request(opts Options) pipeline.Request {
	return pipeline.Request{
		HomeDir:            s.homeDir,
		Version:            opts.Version,
		Cortex:             opts.Cortex,
		Context7:           opts.Context7,
		Overwrite:          opts.Overwrite,
		DryRun:             opts.DryRun,
		Now:                opts.Now,
		Probes:             opts.Probes,
		ExpectedPlanDigest: opts.ExpectedPlanDigest,
	}
}

// lockForMutation acquires the canonical cross-process home lock for a
// mutating operation. The home directory is materialized first (the
// operation is already authorized to create everything beneath it); a
// symlinked or irregular home still fails closed inside homelock. It
// returns the release closure the caller must defer — the lock is always
// released, including on every error path after acquisition. A release
// failure cannot strand the home: lock authority is the process-local OS
// handle, which the OS frees at process exit, so the operation's own
// outcome stands and the error is explicitly discarded. Contention past
// the bounded timeout returns an error wrapping ErrHomeBusy without any
// install mutation: no backup, journal, state, or config write runs.
func (s *Service) lockForMutation(timeout time.Duration) (func(), error) {
	if timeout <= 0 {
		timeout = DefaultHomeLockTimeout
	}
	if err := os.MkdirAll(s.homeDir, 0o755); err != nil {
		return nil, fmt.Errorf("acquire home lock: create home directory: %w", err)
	}
	lock, err := homelock.Acquire(s.homeDir, timeout)
	if err != nil {
		if errors.Is(err, homelock.ErrHomeBusy) {
			return nil, fmt.Errorf("%w: %s", ErrHomeBusy, s.homeDir)
		}
		return nil, fmt.Errorf("acquire home lock: %w", err)
	}
	release := func() { _ = lock.Release() }
	return release, nil
}

// Plan derives the complete install or sync operation set for the options. It is
// read-only: no directories, journals, backups, state, or lock files are
// created, and no MCP entry is touched.
func (s *Service) Plan(opts Options) (*pipeline.Plan, error) {
	req := s.request(opts)
	meta := state.LoadMetadataV2(s.homeDir)
	if meta.Presence == state.PresenceV2 {
		return pipeline.PlanSync(req)
	}
	return pipeline.PlanInstall(req)
}

// Install plans and (unless the options carry DryRun) transactionally applies
// the embedded OpenCode asset set and the managed MCP selection. Preview
// planning is lock-free and read-only. When no ExpectedPlanDigest is supplied,
// every zero-write outcome (dry-run, converged, or conflicts) returns without
// touching the lock. When ExpectedPlanDigest is supplied, converged and
// conflict outcomes are re-planned under lock via pipeline.ApplyConfirmed so
// stale confirmations fail as typed drift before mutation.
// The mutating path acquires the canonical cross-process home lock and then re-plans
// under that lock through pipeline.ApplyConfirmed: the freshly derived plan
// reflects any concurrent mutation that landed before the lock was
// acquired, and the caller-confirmed ExpectedPlanDigest (when supplied) is
// compared against the fresh plan digest before the backup or journal may
// begin — a stale confirmation is a typed drift error with zero mutation.
// The lock is released on every exit; contention is a typed ErrHomeBusy
// with zero mutation. The receipt is the durable evidence of what was
// configured, qualified, changed, and backed up; a converged request
// performs zero writes.
func (s *Service) Install(opts Options) (*InstallReceipt, error) {
	req := s.request(opts)
	plan, err := pipeline.PlanInstall(req)
	if err != nil {
		return newInstallReceipt(plan, &pipeline.Receipt{DryRun: opts.DryRun}), err
	}
	if opts.DryRun {
		plan, receipt, err := pipeline.InstallV2(req)
		return newInstallReceipt(plan, receipt), err
	}
	if opts.ExpectedPlanDigest != "" && (plan.Converged || len(plan.Conflicts) > 0) {
		return s.applyPlanWithConfirmation(opts, req, plan, pipeline.PlanInstall, "install")
	}
	if plan.Converged {
		receipt := newInstallReceipt(plan, &pipeline.Receipt{PlanDigest: plan.Digest, Converged: true})
		changed, err := s.saveDelegationWithLock(opts)
		if changed {
			receipt.Converged = false
			receipt.Changed = append(receipt.Changed, "managed-update .config/opencode/cortex-delegation.json")
		}
		return receipt, err
	}
	if len(plan.Conflicts) > 0 {
		return newInstallReceipt(plan, &pipeline.Receipt{PlanDigest: plan.Digest, Conflicts: plan.Conflicts}), &pipeline.ConflictError{Conflicts: plan.Conflicts}
	}
	release, err := s.lockForMutation(opts.LockTimeout)
	if err != nil {
		return newInstallReceipt(plan, &pipeline.Receipt{PlanDigest: plan.Digest}), fmt.Errorf("install: %w", err)
	}
	defer release()
	plan, receipt, err := pipeline.ApplyConfirmed(req, pipeline.PlanInstall)
	if err == nil {
		var changed bool
		changed, err = s.saveDelegation(opts)
		if changed && receipt != nil {
			receipt.Changes = append(receipt.Changes, "managed-update .config/opencode/cortex-delegation.json")
		}
	}
	return newInstallReceipt(plan, receipt), err
}

func (s *Service) applyPlanWithConfirmation(opts Options, req pipeline.Request, plan *pipeline.Plan, planner func(pipeline.Request) (*pipeline.Plan, error), op string) (*InstallReceipt, error) {
	release, err := s.lockForMutation(opts.LockTimeout)
	if err != nil {
		return newInstallReceipt(plan, &pipeline.Receipt{PlanDigest: planDigestForReceipt(plan)}), fmt.Errorf("%s: %w", op, err)
	}
	defer release()
	plan, receipt, err := pipeline.ApplyConfirmed(req, planner)
	if err == nil {
		var changed bool
		changed, err = s.saveDelegation(opts)
		if changed && receipt != nil {
			receipt.Changes = append(receipt.Changes, "managed-update .config/opencode/cortex-delegation.json")
		}
	}
	return newInstallReceipt(plan, receipt), err
}

// Sync reconciles an installed home with the current embedded asset set:
// every asset and MCP effect is re-planned, stale owned artifacts are added
// as deletions, and the result is transactionally applied. Stale deletion is
// ownership- and digest-accredited by the engine; anything else fails
// closed. Locking follows Install exactly: preview planning is lock-free, and
// when no ExpectedPlanDigest is supplied, zero-write outcomes never lock. When
// ExpectedPlanDigest is supplied, converged/conflict returns also lock and
// re-validate the confirmed digest before returning. The mutating path then holds
// the canonical home lock from before the backup until the apply finishes,
// releasing it
// on every exit. The mutating path re-plans under the lock through
// pipeline.ApplyConfirmed, so the applied plan reflects concurrent
// mutations and the caller-confirmed ExpectedPlanDigest is compared against
// the fresh plan digest before the backup or journal may begin.
func (s *Service) Sync(opts Options) (*InstallReceipt, error) {
	req := s.request(opts)
	plan, err := pipeline.PlanSync(req)
	if err != nil {
		return newInstallReceipt(plan, &pipeline.Receipt{DryRun: opts.DryRun}), err
	}
	if opts.DryRun {
		plan, receipt, err := pipeline.SyncV2(req)
		return newInstallReceipt(plan, receipt), err
	}
	if opts.ExpectedPlanDigest != "" && (plan.Converged || len(plan.Conflicts) > 0) {
		return s.applyPlanWithConfirmation(opts, req, plan, pipeline.PlanSync, "sync")
	}
	if plan.Converged {
		receipt := newInstallReceipt(plan, &pipeline.Receipt{PlanDigest: plan.Digest, Converged: true})
		changed, err := s.saveDelegationWithLock(opts)
		if changed {
			receipt.Converged = false
			receipt.Changed = append(receipt.Changed, "managed-update .config/opencode/cortex-delegation.json")
		}
		return receipt, err
	}
	if len(plan.Conflicts) > 0 {
		return newInstallReceipt(plan, &pipeline.Receipt{PlanDigest: plan.Digest, Conflicts: plan.Conflicts}), &pipeline.ConflictError{Conflicts: plan.Conflicts}
	}
	release, err := s.lockForMutation(opts.LockTimeout)
	if err != nil {
		return newInstallReceipt(plan, &pipeline.Receipt{PlanDigest: plan.Digest}), fmt.Errorf("sync: %w", err)
	}
	defer release()
	plan, receipt, err := pipeline.ApplyConfirmed(req, pipeline.PlanSync)
	if err == nil {
		var changed bool
		changed, err = s.saveDelegation(opts)
		if changed && receipt != nil {
			receipt.Changes = append(receipt.Changes, "managed-update .config/opencode/cortex-delegation.json")
		}
	}
	return newInstallReceipt(plan, receipt), err
}

func (s *Service) saveDelegation(opts Options) (bool, error) {
	if opts.DryRun {
		return false, nil
	}
	// Configure OPENCODE_EXPERIMENTAL_BACKGROUND_SUBAGENTS="true" across OS
	_ = ConfigureEnvironment(s.homeDir)

	if opts.DelegationConfig == nil {
		return false, nil
	}
	configDir := filepath.Join(s.homeDir, ".config", "opencode")
	needed, err := delegation.NeedsSave(configDir, *opts.DelegationConfig)
	if err != nil {
		return false, fmt.Errorf("inspect delegation bridge configuration: %w", err)
	}
	if !needed {
		return false, nil
	}
	if err := delegation.Save(configDir, *opts.DelegationConfig); err != nil {
		return false, fmt.Errorf("save delegation bridge configuration: %w", err)
	}
	return true, nil
}

func (s *Service) saveDelegationWithLock(opts Options) (bool, error) {
	if opts.DryRun || opts.DelegationConfig == nil {
		return false, nil
	}
	configDir := filepath.Join(s.homeDir, ".config", "opencode")
	needed, err := delegation.NeedsSave(configDir, *opts.DelegationConfig)
	if err != nil {
		return false, fmt.Errorf("inspect delegation bridge configuration: %w", err)
	}
	if !needed {
		return false, nil
	}
	release, err := s.lockForMutation(opts.LockTimeout)
	if err != nil {
		return false, fmt.Errorf("save delegation bridge configuration: %w", err)
	}
	defer release()
	return s.saveDelegation(opts)
}

func planDigestForReceipt(plan *pipeline.Plan) string {
	if plan == nil {
		return ""
	}
	return plan.Digest
}

// ListBackups returns all available backup manifests sorted from newest to oldest.
func (s *Service) ListBackups() ([]backup.Manifest, error) {
	backupsDir := filepath.Join(s.homeDir, ".cortex-ia", "backups")
	result := backup.ListManifests(backupsDir)
	manifests := result.Manifests
	sort.Slice(manifests, func(i, j int) bool {
		return manifests[i].CreatedAt.After(manifests[j].CreatedAt)
	})
	return manifests, nil
}
