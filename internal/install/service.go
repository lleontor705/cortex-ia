// Package install composes the OpenCode transactional copy engine, MCP manager,
// v2 state documents, and backup primitives into the single installation service.
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
	// SkipEnvironment disables environment configuration.
	SkipEnvironment bool
	// SkipTUIPlugin disables OpenCode TUI plugin registration.
	SkipTUIPlugin bool
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

// lockForMutation acquires the canonical cross-process home lock for mutating operations.
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

// Plan derives the complete install or sync operation set for the options.
func (s *Service) Plan(opts Options) (*pipeline.Plan, error) {
	req := s.request(opts)
	meta := state.LoadMetadataV2(s.homeDir)
	if meta.Presence == state.PresenceV2 {
		return pipeline.PlanSync(req)
	}
	return pipeline.PlanInstall(req)
}

// Install plans and applies the embedded OpenCode asset set and managed MCP selection.
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
	return s.applyServicePlan(opts, req, plan, pipeline.PlanInstall, "install")
}

// Sync reconciles an installed home with the current embedded asset set:
// every asset and MCP effect is re-planned, stale owned artifacts are added
// as deletions, and the result is transactionally applied. Stale deletion is
// ownership- and digest-accredited by the engine; anything else fails
// closed.
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
	return s.applyServicePlan(opts, req, plan, pipeline.PlanSync, "sync")
}

func (s *Service) applyServicePlan(opts Options, req pipeline.Request, plan *pipeline.Plan, planner func(pipeline.Request) (*pipeline.Plan, error), op string) (*InstallReceipt, error) {
	if opts.ExpectedPlanDigest != "" && (plan.Converged || len(plan.Conflicts) > 0) {
		return s.applyPlanWithConfirmation(opts, req, plan, planner, op)
	}
	if plan.Converged {
		receipt := newInstallReceipt(plan, &pipeline.Receipt{PlanDigest: plan.Digest, Converged: true})
		release, err := s.lockForMutation(opts.LockTimeout)
		if err != nil {
			return receipt, fmt.Errorf("%s: %w", op, err)
		}
		defer release()
		err = applyPostPipelineEffects(s.homeDir, opts, receipt)
		return receipt, err
	}
	if len(plan.Conflicts) > 0 {
		return newInstallReceipt(plan, &pipeline.Receipt{PlanDigest: plan.Digest, Conflicts: plan.Conflicts}), &pipeline.ConflictError{Conflicts: plan.Conflicts}
	}
	return s.applyPlanWithConfirmation(opts, req, plan, planner, op)
}

func (s *Service) applyPlanWithConfirmation(opts Options, req pipeline.Request, plan *pipeline.Plan, planner func(pipeline.Request) (*pipeline.Plan, error), op string) (*InstallReceipt, error) {
	release, err := s.lockForMutation(opts.LockTimeout)
	if err != nil {
		return newInstallReceipt(plan, &pipeline.Receipt{PlanDigest: planDigestForReceipt(plan)}), fmt.Errorf("%s: %w", op, err)
	}
	defer release()
	plan, receipt, err := pipeline.ApplyConfirmed(req, planner)
	outReceipt := newInstallReceipt(plan, receipt)
	if err == nil {
		postErr := applyPostPipelineEffects(s.homeDir, opts, outReceipt)
		return outReceipt, postErr
	}
	return outReceipt, err
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

// EffectRecoveryOptions defines explicit parameters for retrying separate post-pipeline effects.
type EffectRecoveryOptions struct {
	LockTimeout      time.Duration
	DelegationConfig *delegation.DelegationConfig
	ExpectedPreimage []byte
	RequirePreimage  bool
	RetryEnvironment bool
	RetryTUIPlugin   bool
}

// RecoverEffect re-inspects and retries separate post-pipeline effects under the canonical home lock
// with explicit authorization bound to an expected preimage rather than pipeline PlanDigest.
// Pipeline journal, backup, and state remain untouched.
func (s *Service) RecoverEffect(opts EffectRecoveryOptions) (*InstallReceipt, error) {
	release, err := s.lockForMutation(opts.LockTimeout)
	if err != nil {
		return nil, fmt.Errorf("recover effect: %w", err)
	}
	defer release()

	meta := state.LoadMetadataV2(s.homeDir)
	receipt := &InstallReceipt{
		TransactionID: meta.Metadata.TransactionID,
		BackupID:      meta.Metadata.BackupID,
		Converged:     true,
	}

	var effects []PostPipelineEffect
	var firstErr error
	var survivingChanges bool

	if opts.RetryEnvironment {
		changed, err := ConfigureEnvironmentWithResult(s.homeDir)
		if err != nil {
			firstErr = fmt.Errorf("recover environment: %w", err)
			effects = append(effects, PostPipelineEffect{
				Kind: "environment", Status: EffectStatusFailed, Error: err.Error(),
			})
		} else if changed {
			survivingChanges = true
			effects = append(effects, PostPipelineEffect{
				Kind: "environment", Status: EffectStatusChanged,
			})
		} else {
			effects = append(effects, PostPipelineEffect{
				Kind: "environment", Status: EffectStatusUnchanged,
			})
		}
	}

	if opts.RetryTUIPlugin && firstErr == nil {
		tuiPath, changed, err := ConfigureTUIPluginWithResult(s.homeDir)
		if err != nil {
			firstErr = fmt.Errorf("recover tui plugin: %w", err)
			effects = append(effects, PostPipelineEffect{
				Kind: "tui_plugin", Status: EffectStatusFailed, Error: err.Error(),
			})
		} else if changed {
			survivingChanges = true
			receipt.Changed = append(receipt.Changed, "managed-update .config/opencode/tui.jsonc")
			effects = append(effects, PostPipelineEffect{
				Kind: "tui_plugin", Status: EffectStatusChanged, Destination: tuiPath,
			})
		} else {
			effects = append(effects, PostPipelineEffect{
				Kind: "tui_plugin", Status: EffectStatusUnchanged, Destination: tuiPath,
			})
		}
	}

	if opts.DelegationConfig != nil && firstErr == nil {
		configDir := filepath.Join(s.homeDir, ".config", "opencode")
		saveOpts := delegation.SaveOptions{
			ExpectedPreimage: opts.ExpectedPreimage,
			RequirePreimage:  opts.RequirePreimage,
			AllowOverwrite:   true,
		}
		res, err := delegation.SaveWithResult(configDir, *opts.DelegationConfig, saveOpts)
		if err != nil {
			firstErr = fmt.Errorf("recover delegation config: %w", err)
			effects = append(effects, PostPipelineEffect{
				Kind: "delegation_config", Status: EffectStatusFailed, Error: err.Error(),
			})
		} else if res.Outcome == delegation.ConfigOutcomeChanged || res.Outcome == delegation.ConfigOutcomeCreated {
			survivingChanges = true
			receipt.Changed = append(receipt.Changed, "managed-update .config/opencode/cortex-delegation.json")
			effects = append(effects, PostPipelineEffect{
				Kind: "delegation_config", Status: EffectStatusChanged, Destination: filepath.Join(configDir, "cortex-delegation.json"),
			})
		} else {
			effects = append(effects, PostPipelineEffect{
				Kind: "delegation_config", Status: EffectStatusUnchanged, Destination: filepath.Join(configDir, "cortex-delegation.json"),
			})
		}
	}

	receipt.PostPipelineEffects = effects
	if survivingChanges {
		receipt.Converged = false
	}
	if firstErr != nil && survivingChanges {
		receipt.PartialSuccess = true
	}
	return receipt, firstErr
}

// RecoverPostPipelineEffects is an alias for RecoverEffect.
func (s *Service) RecoverPostPipelineEffects(opts EffectRecoveryOptions) (*InstallReceipt, error) {
	return s.RecoverEffect(opts)
}
