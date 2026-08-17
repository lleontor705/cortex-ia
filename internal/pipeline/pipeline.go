package pipeline

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/lleontor705/cortex-ia/internal/agents"
	"github.com/lleontor705/cortex-ia/internal/backup"
	"github.com/lleontor705/cortex-ia/internal/catalog"
	"github.com/lleontor705/cortex-ia/internal/components/context7"
	"github.com/lleontor705/cortex-ia/internal/components/conventions"
	cortexcomp "github.com/lleontor705/cortex-ia/internal/components/cortex"
	forgespeccomp "github.com/lleontor705/cortex-ia/internal/components/forgespec"
	"github.com/lleontor705/cortex-ia/internal/components/mailbox"
	"github.com/lleontor705/cortex-ia/internal/components/sdd"
	operationalassets "github.com/lleontor705/cortex-ia/internal/components/sdd/assets"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/capability"
	sddinstall "github.com/lleontor705/cortex-ia/internal/components/sdd/install"
	sddregistry "github.com/lleontor705/cortex-ia/internal/components/sdd/registry"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/renderers"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/skillcore"
	skillscomp "github.com/lleontor705/cortex-ia/internal/components/skills"
	"github.com/lleontor705/cortex-ia/internal/model"
	"github.com/lleontor705/cortex-ia/internal/state"
	"github.com/lleontor705/cortex-ia/internal/verify"
)

// validBackupID matches safe backup IDs (alphanumeric, hyphens, underscores).
var validBackupID = regexp.MustCompile(`^[a-zA-Z0-9\-_]+$`)

// ProgressFunc is called by the pipeline to report step-level progress.
// Implementations must be safe for concurrent use.
type ProgressFunc func(stepID string, status string, err error)

// InstallResult describes the outcome of a full installation.
type InstallResult struct {
	BackupID            string
	FilesChanged        []string
	ComponentsDone      []model.ComponentID
	Errors              []string
	WorkflowFingerprint string
	WorkflowPlan        sddinstall.Plan
	WorkflowDoctor      verify.DoctorReport
	WorkflowReceipt     sddinstall.Receipt
	WorkflowCutover     forgespeccomp.ForgeSpecResolution
	WorkflowRollback    bool
}

// installDependencies is deliberately scoped to one coordinator invocation.
// It gives same-package tests deterministic failure boundaries without making
// production behavior or state-package operations mutable process globals.
type installDependencies struct {
	qualifyCapabilities   func(context.Context, []agents.Adapter, time.Time, []capability.CapabilityID) map[model.AgentID][]capability.CapabilityFact
	now                   func() time.Time
	prepareWorkflow       func(context.Context, WorkflowRequest) (PreparedWorkflowInstall, error)
	applyWorkflow         func(PreparedWorkflowInstall) (sddinstall.Receipt, error)
	invokeComponent       func(model.ComponentID, func() ([]string, error)) ([]string, error)
	saveInstallStatus     func(string, state.InstallStatus) error
	clearInstallStatus    func(string) error
	saveState             func(string, state.State) error
	saveLock              func(string, state.Lockfile) error
	beginJournal          func(string, string, []ManagedTarget) (*InstallJournal, error)
	attachWorkflowReceipt func(*InstallJournal, sddinstall.Receipt) error
	recordJournalOutcome  func(*InstallJournal, MutationOutcome) error
	commitJournal         func(*InstallJournal) error
	restoreAndVerify      func(*InstallJournal) error
	buildInstallPlan      func(context.Context, string, *agents.Registry, model.Selection, []model.ComponentID) (InstallPlan, error)
	applyRegistryPlan     func(string, sdd.GlobalInstallPlan) (sdd.GlobalApplyResult, error)
}

func defaultInstallDependencies() installDependencies {
	return installDependencies{
		qualifyCapabilities: qualifyCapabilities,
		now:                 func() time.Time { return time.Now().UTC() },
		prepareWorkflow:     PrepareWorkflow,
		applyWorkflow: func(workflow PreparedWorkflowInstall) (sddinstall.Receipt, error) {
			return workflow.Apply()
		},
		invokeComponent: func(_ model.ComponentID, invoke func() ([]string, error)) ([]string, error) {
			return invoke()
		},
		saveInstallStatus:  state.SaveInstallStatus,
		clearInstallStatus: state.ClearInstallStatus,
		saveState:          state.Save,
		saveLock:           state.SaveLock,
		beginJournal:       BeginInstallJournal,
		attachWorkflowReceipt: func(journal *InstallJournal, receipt sddinstall.Receipt) error {
			return journal.AttachWorkflowReceipt(receipt)
		},
		recordJournalOutcome: func(journal *InstallJournal, outcome MutationOutcome) error { return journal.Record(outcome) },
		commitJournal:        func(journal *InstallJournal) error { return journal.Commit() },
		restoreAndVerify:     func(journal *InstallJournal) error { return journal.RestoreAndVerify() },
		buildInstallPlan:     BuildInstallPlan,
		applyRegistryPlan:    sdd.ApplyGlobalInstallPlan,
	}
}

// Repair reapplies the previously installed configuration from lock/state metadata.
func Repair(homeDir string, registry *agents.Registry, version string, dryRun bool) (InstallResult, error) {
	s, err := state.Load(homeDir)
	if err != nil {
		return InstallResult{}, err
	}
	lock, err := state.LoadLock(homeDir)
	if err != nil {
		return InstallResult{}, err
	}

	registryIntent, err := registrySelectionFromMetadata(s, lock)
	if err != nil {
		return InstallResult{}, err
	}
	if registryIntent == nil {
		// Legacy receipt-without-intent guard (REQ-REM-B1): an empty
		// overlay over a committed registry receipt would plan stale
		// retirement of prior custom outputs, so Repair fails closed
		// before BuildInstallPlan or any mutation instead of guessing
		// intent from receipt evidence.
		hasReceipt, receiptErr := hasCommittedRegistryReceipt(homeDir)
		if receiptErr != nil {
			return InstallResult{}, receiptErr
		}
		if hasReceipt {
			return InstallResult{}, ErrRepairRegistryIntentMissing
		}
	}

	selection, err := selectionFromMetadata(s, lock)
	if err != nil {
		return InstallResult{}, err
	}
	selection.DryRun = dryRun

	return Install(homeDir, registry, selection, version, dryRun)
}

// Rollback restores managed files from a previous backup manifest.
func Rollback(homeDir, backupID string) (backup.Manifest, error) {
	if backupID == "" {
		s, err := state.Load(homeDir)
		if err != nil {
			return backup.Manifest{}, err
		}
		lock, err := state.LoadLock(homeDir)
		if err != nil {
			return backup.Manifest{}, err
		}
		backupID = firstNonEmptyString(lock.LastBackupID, s.LastBackupID)
	}

	if backupID == "" {
		return backup.Manifest{}, fmt.Errorf("no backup available for rollback")
	}
	if !validBackupID.MatchString(backupID) {
		return backup.Manifest{}, fmt.Errorf("invalid backup ID format: %q", backupID)
	}

	manifestPath := filepath.Join(homeDir, ".cortex-ia", "backups", backupID, backup.ManifestFilename)
	manifest, err := backup.ReadManifest(manifestPath)
	if err != nil {
		return backup.Manifest{}, err
	}

	restore := backup.RestoreService{}
	if err := restore.Restore(manifest); err != nil {
		return backup.Manifest{}, err
	}

	return manifest, nil
}

// Install runs the full installation pipeline using a 2-stage orchestrator:
// Stage 1 (Prepare): validate agents + create backup (stops on error, rolls back)
// Stage 2 (Apply): inject components per agent + save state (continues on error)
func Install(homeDir string, registry *agents.Registry, selection model.Selection, version string, dryRun bool, onProgress ...ProgressFunc) (InstallResult, error) {
	return installWithDependencies(homeDir, registry, selection, version, dryRun, defaultInstallDependencies(), onProgress...)
}

func installWithDependencies(homeDir string, registry *agents.Registry, selection model.Selection, version string, dryRun bool, deps installDependencies, onProgress ...ProgressFunc) (InstallResult, error) {
	var progress ProgressFunc
	if len(onProgress) > 0 {
		progress = onProgress[0]
	}

	result := InstallResult{}
	if err := selection.ValidateCurrent(); err != nil {
		return result, err
	}

	// 1. Resolve components with dependencies.
	components := selection.Components
	if len(components) == 0 {
		components = catalog.ComponentsForPreset(selection.Preset)
	}
	resolved := catalog.ResolveDeps(components)

	// Global preflight (design D7): every selected adapter is validated and
	// the complete registry overlay resolves and plans before any directory
	// creation, backup, or status write is allowed to happen. A preflight
	// failure is terminal and pure: zero homes change.
	preflight, preflightErr := deps.buildInstallPlan(context.Background(), homeDir, registry, selection, resolved)
	if preflightErr != nil {
		return result, preflightErr
	}
	resolved = preflight.Resolved
	adapters := preflight.Adapters

	var preparedWorkflow PreparedWorkflowInstall
	if slicesContainsComponent(resolved, model.ComponentSDD) {
		qualificationTime := deps.now().UTC()
		qualifiedFacts := deps.qualifyCapabilities(context.Background(), adapters, qualificationTime, nil)
		evaluationTime := deps.now().UTC()
		var prepareErr error
		preparedWorkflow, prepareErr = deps.prepareWorkflow(context.Background(), WorkflowRequest{
			HomeDir: homeDir, Adapters: adapters, GeneratorVersion: version,
			CapabilityEvaluationTime: evaluationTime, QualifiedCapabilityFacts: qualifiedFacts,
		})
		if prepareErr != nil {
			return result, prepareErr
		}
		result.WorkflowFingerprint = preparedWorkflow.Fingerprint
		result.WorkflowPlan = preparedWorkflow.Plan
		result.WorkflowDoctor = preparedWorkflow.Doctor
		result.WorkflowCutover = preparedWorkflow.Cutover
	}

	if dryRun {
		if progress == nil {
			fmt.Println("=== DRY RUN ===")
			fmt.Printf("Agents: %v\n", selection.Agents)
			fmt.Printf("Preset: %s\n", selection.Preset)
			fmt.Printf("Components (resolved): %v\n", resolved)
			fmt.Println("No changes will be made.")
		}
		result.ComponentsDone = resolved
		return result, nil
	}

	// 2. Build prepare steps. Prepare stays sequential and RunStage preserves its
	// reverse rollback semantics. All post-backup writers are assembled below but
	// are not allowed to run until their complete target set has been captured.
	bkStep := &backupStep{
		homeDir: homeDir, registry: registry,
		agentIDs: selection.Agents, resolved: resolved, version: version,
		progress: progress,
	}
	prepareSteps := []Step{
		&validateStep{registry: registry, agentIDs: selection.Agents},
		bkStep,
	}

	// 3. Build apply steps: one sequential chain per agent, agents run in parallel.
	componentSet := make(map[model.ComponentID]bool)
	for _, c := range resolved {
		componentSet[c] = true
	}

	var allComponentSteps []*componentStep
	var workflowOnce sync.Once
	var workflowFiles []string
	var workflowErr error
	workflowWriter := newPreparedWriter(workflowManagedTargets(preparedWorkflow))
	applyWorkflow := func() ([]string, error) {
		workflowOnce.Do(func() {
			workflowErr = workflowWriter.run(func() error {
				receipt, applyErr := deps.applyWorkflow(preparedWorkflow)
				result.WorkflowReceipt = receipt
				result.WorkflowRollback = receipt.RestoreAvailable && receipt.BackupVerified
				workflowFiles = append(workflowFiles, receipt.Applied...)
				if workflowWriter.journal != nil && receipt.ID != "" {
					if attachErr := deps.attachWorkflowReceipt(workflowWriter.journal, receipt); attachErr != nil {
						return errors.Join(applyErr, fmt.Errorf("attach workflow receipt: %w", attachErr))
					}
				}
				return applyErr
			})
		})
		return workflowFiles, workflowErr
	}

	// The registry overlay plan applies exactly once across the agent chains.
	// The planner only returns a plan when it carries operations or records
	// declared intent, so empty overlays never reach this writer. Its targets
	// join the shared journal, so any later failure restores the same
	// pre-apply snapshot as every other writer (design D10).
	var registryFiles []string
	var registryOnce sync.Once
	var registryErr error
	var registryWriter *preparedWriter
	if preflight.Registry != nil {
		registryWriter = newPreparedWriter(registryManagedTargets(homeDir, preflight.Registry.Plan))
	}
	applyRegistry := func() ([]string, error) {
		registryOnce.Do(func() {
			outcome, applyErr := deps.applyRegistryPlan(homeDir, preflight.Registry.Plan)
			registryFiles = append(registryFiles, outcome.Applied...)
			registryFiles = append(registryFiles, outcome.Deleted...)
			registryErr = applyErr
		})
		return registryFiles, registryErr
	}

	// Build one sequential step chain per agent. Each chain applies
	// components in dependency order for that agent.
	var agentChains [][]Step
	for _, agentID := range selection.Agents {
		adapter, err := registry.Get(agentID)
		if err != nil {
			continue // preflight already catches this
		}

		if progress == nil {
			fmt.Printf("\nConfiguring %s...\n", agentID)
		}
		var chain []Step
		for _, inj := range buildInjectors(homeDir, adapter, selection, applyWorkflow, componentSet[model.ComponentSDD]) {
			if !componentSet[inj.id] {
				continue
			}
			writer := newPreparedWriter(componentManagedTargets(homeDir, adapter, selection, inj.id, preparedWorkflow))
			if len(writer.ManagedTargets()) == 0 {
				// A component that declares no targets (for example the skills
				// component with no community skills, or an idempotent workflow
				// with nothing to write) is a no-op for this adapter.
				continue
			}
			cs := &componentStep{
				homeDir: homeDir, adapter: adapter,
				componentID: inj.id,
				injectorFn:  func() ([]string, error) { return deps.invokeComponent(inj.id, inj.fn) },
				writer:      writer,
				progress:    progress,
			}
			chain = append(chain, cs)
			allComponentSteps = append(allComponentSteps, cs)
		}
		if registryWriter != nil {
			// The registry overlay is install-level, not a component: its
			// once-guarded apply is the final sequential operation inside
			// every agent chain, preserving the parallel-chain boundary and
			// the sequential order with that agent's component writes.
			chain = append(chain, &registryPlanStep{
				agent: adapter, apply: applyRegistry, writer: registryWriter, progress: progress,
			})
		}
		if len(chain) > 0 {
			agentChains = append(agentChains, chain)
		}
	}

	// 4. Run Prepare before creating the journal. The backup directory is the
	// durable checkpoint root, so journal creation cannot become an untracked
	// managed write.
	// Within each agent, components run sequentially (same config files).
	// Different agents run in parallel (different config dirs).
	prepResult := RunStage(prepareSteps)
	if prepResult.Error != nil {
		result.BackupID = bkStep.BackupID
		result.ComponentsDone = resolved
		return result, prepResult.Error
	}
	result.BackupID = bkStep.BackupID
	result.ComponentsDone = resolved

	// Capture every writer target before the first post-backup write, then bind
	// the same journal to each writer. A child workflow receipt remains typed.
	statusTarget := relativeManagedTarget(homeDir, state.InstallStatusPath(homeDir), "install-status")
	stateTarget := relativeManagedTarget(homeDir, state.StatePath(homeDir), "state")
	lockTarget := relativeManagedTarget(homeDir, state.LockPath(homeDir), "lock")
	statusWriter := newPreparedWriter(withParentDirectories(homeDir, []ManagedTarget{statusTarget}))
	stateWriter := newPreparedWriter(withParentDirectories(homeDir, []ManagedTarget{stateTarget}))
	lockWriter := newPreparedWriter(withParentDirectories(homeDir, []ManagedTarget{lockTarget}))
	allWriters := []*preparedWriter{workflowWriter, statusWriter, stateWriter, lockWriter}
	if registryWriter != nil {
		allWriters = append(allWriters, registryWriter)
	}
	for _, step := range allComponentSteps {
		allWriters = append(allWriters, step.writer)
	}
	journal, journalErr := deps.beginJournal(homeDir, filepath.Join(bkStep.BackupDir, "journal"), managedTargets(allWriters))
	if journalErr != nil {
		return result, fmt.Errorf("capture install journal: %w", journalErr)
	}
	for _, writer := range allWriters {
		writer.bindJournal(journal)
		writer.recordJournalOutcome = deps.recordJournalOutcome
	}
	priorLock, err := state.LoadLock(homeDir)
	if err != nil {
		return result, err
	}

	fail := func(stage string, err error) (InstallResult, error) {
		for _, cs := range allComponentSteps {
			result.FilesChanged = append(result.FilesChanged, cs.Files...)
		}
		result.FilesChanged = append(result.FilesChanged, workflowFiles...)
		result.FilesChanged = append(result.FilesChanged, registryFiles...)
		result.FilesChanged = dedupeStrings(result.FilesChanged)
		result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", stage, err))
		if restoreErr := deps.restoreAndVerify(journal); restoreErr != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("restore install journal: %v", restoreErr))
			return result, fmt.Errorf("%s: %w; restore install journal: %v", stage, err, restoreErr)
		}
		return result, fmt.Errorf("%s: %w", stage, err)
	}

	if err := statusWriter.run(func() error {
		return deps.saveInstallStatus(homeDir, state.InstallStatus{
			Status: "in-progress", StartedAt: time.Now().UTC().Format(time.RFC3339), BackupID: bkStep.BackupID,
		})
	}); err != nil {
		return fail("write install status", err)
	}

	// Active agent writers join here before any failure can begin reverse restore.
	applyResult := RunParallelChains(agentChains)
	if applyResult.Error != nil {
		return fail("apply agent chains", applyResult.Error)
	}

	// 5. Persist terminal metadata, clear status, and commit. Every failure is
	// terminal and restores the precise preimages through the shared journal.
	result.BackupID = bkStep.BackupID
	result.ComponentsDone = resolved
	for _, cs := range allComponentSteps {
		result.FilesChanged = append(result.FilesChanged, cs.Files...)
	}
	result.FilesChanged = append(result.FilesChanged, registryFiles...)
	s := state.State{
		InstalledAgents:   selection.Agents,
		Preset:            selection.Preset,
		Components:        resolved,
		LastInstall:       time.Now(),
		LastBackupID:      result.BackupID,
		Version:           version,
		StrictTDD:         selection.StrictTDD,
		RegistrySelection: copyRegistrySelection(selection.Registry),
	}
	if err := stateWriter.run(func() error { return deps.saveState(homeDir, s) }); err != nil {
		return fail("save state", err)
	}

	lock := state.Lockfile{
		InstalledAgents:   selection.Agents,
		Preset:            selection.Preset,
		Components:        resolved,
		Files:             durableInstallFiles(priorLock.Files, preparedWorkflow, result.FilesChanged),
		GeneratedAt:       time.Now(),
		LastBackupID:      result.BackupID,
		Version:           version,
		RegistrySelection: copyRegistrySelection(selection.Registry),
	}
	if err := lockWriter.run(func() error { return deps.saveLock(homeDir, lock) }); err != nil {
		return fail("save lock", err)
	}

	if err := statusWriter.run(func() error { return deps.clearInstallStatus(homeDir) }); err != nil {
		return fail("clear install status", err)
	}
	if err := deps.commitJournal(journal); err != nil {
		return fail("commit install journal", err)
	}

	result.FilesChanged = dedupeStrings(result.FilesChanged)
	return result, nil
}

type injectorEntry struct {
	id model.ComponentID
	fn func() ([]string, error)
}

// buildInjectors returns the ordered list of component injectors for an agent.
func buildInjectors(homeDir string, adapter agents.Adapter, selection model.Selection, applyWorkflow func() ([]string, error), workflowOwnership ...bool) []injectorEntry {
	workflowOwnsInstructions := len(workflowOwnership) != 0 && workflowOwnership[0]
	entries := []injectorEntry{
		{model.ComponentCortex, func() ([]string, error) {
			r, err := cortexcomp.Inject(homeDir, adapter)
			return r.Files, err
		}},
		{model.ComponentForgeSpec, func() ([]string, error) {
			r, err := forgespeccomp.Inject(homeDir, adapter)
			return r.Files, err
		}},
		{model.ComponentMailbox, func() ([]string, error) {
			r, err := mailbox.Inject(homeDir, adapter)
			return r.Files, err
		}},
		{model.ComponentContext7, func() ([]string, error) {
			r, err := context7.Inject(homeDir, adapter)
			return r.Files, err
		}},
		{model.ComponentSDD, func() ([]string, error) {
			return applyWorkflow()
		}},
		{model.ComponentSkills, func() ([]string, error) {
			r, err := skillscomp.Inject(homeDir, adapter, selection.CommunitySkills)
			return r.Files, err
		}},
	}
	if !workflowOwnsInstructions {
		entries = append(entries, injectorEntry{model.ComponentConventions, func() ([]string, error) {
			r, err := conventions.Inject(homeDir, adapter)
			return r.Files, err
		}})
	}
	return entries
}

// componentManagedTargets declares the concrete files a component can mutate.
// The coordinator converts these declarations into journal preimages before any
// post-backup writer begins; components remain unaware of transaction policy.
func componentManagedTargets(homeDir string, adapter agents.Adapter, selection model.Selection, component model.ComponentID, workflow PreparedWorkflowInstall) []ManagedTarget {
	var paths []string
	switch component {
	case model.ComponentCortex, model.ComponentForgeSpec, model.ComponentMailbox, model.ComponentContext7:
		paths = append(paths, adapter.MCPConfigPath(homeDir, "cortex"))
	case model.ComponentSDD:
		for _, path := range workflow.Plan.Backup.Paths {
			paths = append(paths, filepath.Join(homeDir, filepath.FromSlash(path)))
		}
	case model.ComponentSkills:
		for _, skill := range selection.CommunitySkills {
			paths = append(paths, filepath.Join(adapter.SkillsDir(homeDir), string(skill), "SKILL.md"))
		}
	case model.ComponentConventions:
		paths = append(paths,
			filepath.Join(state.SharedSkillsDir(homeDir), "_shared", "cortex-convention.md"),
			filepath.Join(state.SharedSkillsDir(homeDir), "_shared", "cortex-advanced.md"),
			adapter.SystemPromptFile(homeDir),
		)
	}
	return withParentDirectories(homeDir, managedFileTargets(homeDir, paths, string(component)))
}

func workflowManagedTargets(workflow PreparedWorkflowInstall) []ManagedTarget {
	// The backup scope is the exact superset of every path the plan may create,
	// update, or delete — including the ownership sidecars (".cortex-ia.base" and
	// ".cortex-ia.json") written by the SDD applier. Declaring them keeps journal
	// rollback able to restore an initially absent parent directory.
	var targets []ManagedTarget
	for _, path := range workflow.Plan.Backup.Paths {
		targets = append(targets, ManagedTarget{Path: filepath.ToSlash(path), Kind: TargetFile, Owner: "workflow"})
	}
	return targets
}

// ---------------------------------------------------------------------------
// Global install preflight (design D7)
// ---------------------------------------------------------------------------

// RulePreflightAdapterLayout is the pipeline-level rule for adapters that
// cannot represent a custom skill overlay. The registry and bundle packages
// own every other executable rule cited by preflight diagnostics.
const RulePreflightAdapterLayout = "pipeline.adapter-skill-layout"

// InstallPlan is the pure result of the global preflight: validated adapters
// for every selected agent, the effective component set after registry
// overlay disables, and the registry plan when an overlay or a prior
// committed registry receipt requires one. Building it performs reads only.
type InstallPlan struct {
	// Adapters lists every selected agent's adapter in selection order; an
	// unknown agent fails the preflight before any write can happen.
	Adapters []agents.Adapter
	// Resolved is the effective component set: dependency-resolved components
	// minus overlay-disabled optional components.
	Resolved []model.ComponentID
	// Registry is non-nil when the selection declared an overlay or a prior
	// committed registry receipt exists; nil keeps the legacy no-overlay path
	// byte-for-byte unchanged.
	Registry *GlobalRegistryPlan
}

// GlobalRegistryPlan pairs the pure global install plan with whether the
// current selection declared an overlay. A plan without operations (an empty
// overlay, or a converged repeat install) applies nothing.
type GlobalRegistryPlan struct {
	Plan    sdd.GlobalInstallPlan
	Overlay bool
}

// BuildInstallPlan performs the global pure preflight for one install: it
// validates every selected adapter, resolves the registry overlay (custom
// skill provenance, additive merge, protected disables) against the embedded
// baseline catalog, lowers verified custom skills through each adapter's
// declared layout, and plans every write and stale-managed retirement with
// destination collision checks and the rollback inventory. It runs before
// any EnsureDir, backup, or status write, so a non-empty diagnostic report is
// always a pre-write rejection with a deterministic primary cause and zero
// homes changed.
func BuildInstallPlan(ctx context.Context, homeDir string, agentRegistry *agents.Registry, selection model.Selection, resolved []model.ComponentID) (InstallPlan, error) {
	plan := InstallPlan{Resolved: resolved}
	plan.Adapters = make([]agents.Adapter, 0, len(selection.Agents))
	for _, agentID := range selection.Agents {
		adapter, err := agentRegistry.Get(agentID)
		if err != nil {
			return InstallPlan{}, fmt.Errorf("unknown agent %q: %w", agentID, err)
		}
		plan.Adapters = append(plan.Adapters, adapter)
	}

	priorReceipt, priorErr := sdd.LoadCommittedRegistryReceipt(homeDir)
	hasPriorReceipt := priorErr == nil
	if priorErr != nil && !errors.Is(priorErr, fs.ErrNotExist) {
		return InstallPlan{}, fmt.Errorf("load committed registry receipt: %w", priorErr)
	}
	overlay := selection.Registry
	hasOverlay := overlay != nil
	if !hasOverlay && !hasPriorReceipt {
		// Legacy input: no overlay declared and no prior overlay evidence.
		// The existing pipeline path applies unchanged (REQ-COMPAT-001).
		return plan, nil
	}

	registrySelection := model.RegistrySelection{}
	if hasOverlay {
		registrySelection = *overlay
	}
	retained := withoutComponents(resolved, registrySelection.DisabledComponents)
	policy := registryDisablePolicy(retained)
	embedded, err := operationalassets.BuildOperationalCatalog()
	if err != nil {
		return InstallPlan{}, fmt.Errorf("materialize embedded baseline catalog: %w", err)
	}

	// The registry receives the retained selection explicitly (design D4):
	// resolved components minus declared disables. Resolve revalidates the
	// disables and seals EffectiveComponents from this retained set, never
	// from the policy classification map (REQ-REM-B3).
	resolvedRegistry, diagnostics := sddregistry.Resolve(ctx, sddregistry.Request{
		Selection:          registrySelection,
		RetainedComponents: retained,
	}, embedded, policy)
	if len(diagnostics) > 0 {
		return InstallPlan{}, &sddregistry.InstallError{Primary: diagnostics[0], All: diagnostics}
	}
	plan.Resolved = withoutComponents(resolved, resolvedRegistry.Disabled)

	bundles, err := lowerRegistryBundles(plan.Adapters, resolvedRegistry.EffectiveSkills)
	if err != nil {
		return InstallPlan{}, err
	}
	request := sdd.GlobalInstallPlanRequest{
		HomeDir: homeDir,
		Bundles: bundles,
		Receipt: resolvedRegistry.CanonicalReceipt,
	}
	if hasPriorReceipt {
		request.PriorManagedPaths = priorReceipt.HostOutputs
	}
	globalPlan, planDiagnostics := sdd.BuildGlobalInstallPlan(request)
	if len(planDiagnostics) > 0 {
		return InstallPlan{}, &sddregistry.InstallError{Primary: planDiagnostics[0], All: planDiagnostics}
	}
	if !registryPlanHasOperations(globalPlan) &&
		len(registrySelection.CustomSkillPaths) == 0 && len(registrySelection.DisabledComponents) == 0 {
		// A converged baseline input with no declared intent plans nothing
		// and must not churn evidence: empty overlays stay byte-for-byte the
		// baseline and leave no receipt residue (SC-BASE-E).
		return plan, nil
	}
	plan.Registry = &GlobalRegistryPlan{Plan: globalPlan, Overlay: hasOverlay}
	return plan, nil
}

// lowerRegistryBundles lowers the resolved custom skills onto every selected
// adapter through its declared skill layout. A custom overlay without a
// declared layout is unrepresentable and fails closed; embedded skills are
// never re-lowered here (the workflow bundle owns those).
func lowerRegistryBundles(adapters []agents.Adapter, skills []skillcore.Skill) ([]sdd.CompiledInjectionBundle, error) {
	customs := make([]skillcore.Skill, 0, len(skills))
	for _, skill := range skills {
		if skill.Origin == skillcore.OriginCustom {
			customs = append(customs, skill)
		}
	}
	if len(customs) == 0 {
		return nil, nil
	}
	bundles := make([]sdd.CompiledInjectionBundle, 0, len(adapters))
	for _, adapter := range adapters {
		layout, ok := adapter.(agents.SkillLayoutProvider)
		if !ok {
			diagnostic := sddregistry.Diagnostic{
				Class:           sddregistry.ErrorUnsupported,
				Stage:           sddregistry.StagePlan,
				Rule:            RulePreflightAdapterLayout,
				Cause:           fmt.Errorf("adapter %q declares no custom skill layout", adapter.Agent()),
				SafeRemediation: "install custom skills only on adapters that declare a skill layout",
			}
			return nil, &sddregistry.InstallError{Primary: diagnostic, All: []sddregistry.Diagnostic{diagnostic}}
		}
		assets, err := renderers.LowerCustomSkills(layout, customs)
		if err != nil {
			diagnostic := sddregistry.Diagnostic{
				Class:           sddregistry.ErrorInvalid,
				Stage:           sddregistry.StagePlan,
				Rule:            RulePreflightAdapterLayout,
				Cause:           fmt.Errorf("lower custom skills for %q: %w", adapter.Agent(), err),
				SafeRemediation: "fix the declared custom skill so it lowers to exactly one safe destination",
			}
			return nil, &sddregistry.InstallError{Primary: diagnostic, All: []sddregistry.Diagnostic{diagnostic}}
		}
		bundles = append(bundles, sdd.CompiledInjectionBundle{
			Target: renderers.TargetID(adapter.Agent()), Bundle: renderers.Bundle{Assets: assets},
		})
	}
	return bundles, nil
}

// registryDisablePolicy derives the registry disable policy from the catalog's
// explicit disable-class descriptors given the retained component selection
// (design D4). Transitive dependencies of retained components resolve as
// protected-required; IDs absent from the policy map stay protected fail-closed.
func registryDisablePolicy(retained []model.ComponentID) sddregistry.Policy {
	classes := catalog.DisableClasses(retained)
	componentClasses := make(map[model.ComponentID]sddregistry.DisableClass, len(classes))
	for id, class := range classes {
		switch class {
		case catalog.Optional:
			componentClasses[id] = sddregistry.Optional
		case catalog.ProtectedAuthority:
			componentClasses[id] = sddregistry.ProtectedAuthority
		case catalog.ProtectedWorkflow:
			componentClasses[id] = sddregistry.ProtectedWorkflow
		case catalog.ProtectedRequired:
			componentClasses[id] = sddregistry.ProtectedRequired
		default:
			// Unclassified stays absent from the map: the registry treats a
			// missing classification as protected (fail-closed).
		}
	}
	return sddregistry.Policy{
		SchemaVersion:    "1.0.0",
		PolicyVersion:    "catalog-disable-classes-1",
		ComponentClasses: componentClasses,
	}
}

// withoutComponents returns resolved with every disabled entry removed,
// preserving dependency order.
func withoutComponents(resolved, disabled []model.ComponentID) []model.ComponentID {
	if len(disabled) == 0 {
		return resolved
	}
	off := make(map[model.ComponentID]bool, len(disabled))
	for _, id := range disabled {
		off[id] = true
	}
	kept := make([]model.ComponentID, 0, len(resolved))
	for _, id := range resolved {
		if !off[id] {
			kept = append(kept, id)
		}
	}
	return kept
}

// registryPlanHasOperations reports whether the plan carries any write or
// stale-managed delete. A converged overlay — an empty one, or a repeat
// install whose outputs already match — plans nothing; the planner pairs
// this with declared-intent detection so empty overlays stay byte-for-byte
// the baseline (REQ-BASE-001) while declared disables still commit their
// canonical evidence.
func registryPlanHasOperations(plan sdd.GlobalInstallPlan) bool {
	if len(plan.Shared.Writes) > 0 || len(plan.Shared.Deletes) > 0 {
		return true
	}
	for _, adapter := range plan.Adapters {
		if len(adapter.Ops) > 0 {
			return true
		}
	}
	return false
}

// registryManagedTargets declares the journal-managed surface of one registry
// plan: every rollback path plus the committed canonical receipt location, so
// a failed apply restores the pre-apply snapshot and never leaves a committed
// receipt behind.
func registryManagedTargets(homeDir string, plan sdd.GlobalInstallPlan) []ManagedTarget {
	targets := make([]ManagedTarget, 0, len(plan.RollbackPaths)+1)
	for _, path := range plan.RollbackPaths {
		if strings.TrimSpace(path) == "" {
			continue
		}
		targets = append(targets, ManagedTarget{Path: path, Kind: TargetFile, Owner: "registry"})
	}
	targets = append(targets, relativeManagedTarget(homeDir, sdd.CommittedRegistryReceiptPath(homeDir), "registry"))
	return withParentDirectories(homeDir, targets)
}

// registryPlanStep applies the registry overlay plan inside one agent's
// sequential chain. Every chain shares the once-guarded apply and the same
// journal writer, so the plan executes exactly once while each chain keeps
// the transactional boundary of a component step.
type registryPlanStep struct {
	agent    agents.Adapter
	apply    func() ([]string, error)
	writer   *preparedWriter
	progress ProgressFunc

	// Output: managed paths written or deleted by the apply.
	Files []string
}

func (s *registryPlanStep) Name() string {
	return fmt.Sprintf("%s/registry-overlay", s.agent.Agent())
}

func (s *registryPlanStep) Run() error {
	if s.progress != nil {
		s.progress(s.Name(), "running", nil)
	}
	var files []string
	run := func() error {
		var err error
		files, err = s.apply()
		return err
	}
	err := s.writer.run(run)
	if err != nil {
		if s.progress != nil {
			s.progress(s.Name(), "failed", err)
		}
		return err
	}
	s.Files = files
	if s.progress != nil {
		s.progress(s.Name(), "succeeded", nil)
	}
	return nil
}

func managedFileTargets(homeDir string, paths []string, owner string) []ManagedTarget {
	targets := make([]ManagedTarget, 0, len(paths))
	for _, path := range paths {
		if strings.TrimSpace(path) == "" {
			continue
		}
		targets = append(targets, relativeManagedTarget(homeDir, path, owner))
	}
	return targets
}

func relativeManagedTarget(homeDir, path, owner string) ManagedTarget {
	relative, err := filepath.Rel(homeDir, path)
	if err != nil {
		return ManagedTarget{Path: path, Kind: TargetFile, Owner: owner}
	}
	return ManagedTarget{Path: filepath.ToSlash(relative), Kind: TargetFile, Owner: owner}
}

func withParentDirectories(homeDir string, targets []ManagedTarget) []ManagedTarget {
	result := append([]ManagedTarget(nil), targets...)
	for _, target := range targets {
		if target.Kind != TargetFile {
			continue
		}
		path := filepath.Dir(filepath.FromSlash(target.Path))
		for path != "." && path != string(filepath.Separator) {
			result = append(result, ManagedTarget{Path: filepath.ToSlash(path), Kind: TargetDirectory, Owner: target.Owner})
			path = filepath.Dir(path)
		}
	}
	return result
}

func managedTargets(writers []*preparedWriter) []ManagedTarget {
	byPath := make(map[string]ManagedTarget)
	for _, writer := range writers {
		if writer == nil {
			continue
		}
		for _, target := range writer.ManagedTargets() {
			if target.Path == "" {
				continue
			}
			if prior, exists := byPath[target.Path]; exists {
				if prior.Kind == target.Kind {
					continue
				}
				// A file target is more specific than its parent directory target.
				if target.Kind == TargetFile {
					byPath[target.Path] = target
				}
				continue
			}
			byPath[target.Path] = target
		}
	}
	result := make([]ManagedTarget, 0, len(byPath))
	for _, target := range byPath {
		result = append(result, target)
	}
	return result
}

func slicesContainsComponent(components []model.ComponentID, target model.ComponentID) bool {
	for _, component := range components {
		if component == target {
			return true
		}
	}
	return false
}

func collectBackupPaths(homeDir string, registry *agents.Registry, agentIDs []model.AgentID, components []model.ComponentID) []string {
	var paths []string
	componentSet := make(map[model.ComponentID]bool)
	for _, c := range components {
		componentSet[c] = true
	}

	for _, agentID := range agentIDs {
		adapter, err := registry.Get(agentID)
		if err != nil {
			continue
		}

		// System prompt file.
		if f := adapter.SystemPromptFile(homeDir); f != "" {
			paths = append(paths, f)
		}
		// Settings file.
		if f := adapter.SettingsPath(homeDir); f != "" {
			paths = append(paths, f)
		}
		// MCP config files.
		for _, name := range []string{"cortex", "forgespec", "context7"} {
			if f := adapter.MCPConfigPath(homeDir, name); f != "" {
				if _, err := os.Stat(f); err == nil {
					paths = append(paths, f)
				}
			}
		}
		// SDD files.
		if componentSet[model.ComponentSDD] {
			paths = append(paths, sdd.FilesToBackup(homeDir, adapter)...)
		}
	}

	return paths
}

func dedupeStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

// durableInstallFiles reconstructs managed inventory exclusively from prior
// ownership metadata, the desired workflow bundle, and current component
// reports. Explicit workflow deletes are authoritative removals. It does not
// inspect the target tree, so unrelated files cannot become managed by mere
// presence on disk.
func durableInstallFiles(prior []string, workflow PreparedWorkflowInstall, reported []string) []string {
	files := append([]string(nil), prior...)
	for _, asset := range workflow.Plan.Inventory {
		files = append(files, asset.Path)
	}
	files = append(files, reported...)

	deleted := make(map[string]struct{}, len(workflow.Plan.Deletes)*3)
	for _, effect := range workflow.Plan.Deletes {
		for _, path := range []string{effect.Path, effect.OwnershipPath, effect.BasePath} {
			if path != "" {
				deleted[path] = struct{}{}
			}
		}
	}
	collapsed := dedupeStrings(files)
	if len(deleted) == 0 {
		return collapsed
	}
	kept := collapsed[:0]
	for _, path := range collapsed {
		if _, remove := deleted[path]; !remove {
			kept = append(kept, path)
		}
	}
	if len(kept) == 0 {
		return nil
	}
	return kept
}

// SelectionFromState reconstructs a Selection from persisted state/lock metadata.
func SelectionFromState(s state.State, lock state.Lockfile) (model.Selection, error) {
	return selectionFromMetadata(s, lock)
}

func selectionFromMetadata(s state.State, lock state.Lockfile) (model.Selection, error) {
	selection := model.Selection{
		Agents:     dedupeAgents(lock.InstalledAgents, s.InstalledAgents),
		Preset:     firstNonEmptyPreset(lock.Preset, s.Preset, model.PresetFull),
		Components: dedupeComponents(lock.Components, s.Components),
	}

	if len(selection.Agents) == 0 {
		return model.Selection{}, fmt.Errorf("no cortex-ia installation metadata found")
	}

	registryIntent, err := registrySelectionFromMetadata(s, lock)
	if err != nil {
		return model.Selection{}, err
	}
	selection.Registry = registryIntent

	if err := selection.ValidateCurrent(); err != nil {
		return model.Selection{}, fmt.Errorf("repair selection: %w", err)
	}

	return selection, nil
}

// ErrRepairRegistryIntentMissing is the stable fail-closed error returned when
// a committed registry receipt exists but neither state nor lock carries the
// persisted registry intent of the install that produced it.
var ErrRepairRegistryIntentMissing = errors.New("committed registry receipt without persisted registry intent")

// ErrRepairRegistryIntentConflict is the stable fail-closed error returned
// when state and lock carry disagreeing non-nil registry intent copies.
var ErrRepairRegistryIntentConflict = errors.New("state and lock disagree on persisted registry intent")

// registrySelectionFromMetadata reconstructs registry intent exclusively from
// persisted metadata (design D2): equal state/lock copies are accepted, a
// single surviving copy recovers the other, and conflicting non-nil copies
// fail closed. Nil, nil means no overlay was ever declared.
func registrySelectionFromMetadata(s state.State, lock state.Lockfile) (*model.RegistrySelection, error) {
	switch {
	case s.RegistrySelection == nil && lock.RegistrySelection == nil:
		return nil, nil
	case s.RegistrySelection == nil:
		return copyRegistrySelection(lock.RegistrySelection), nil
	case lock.RegistrySelection == nil:
		return copyRegistrySelection(s.RegistrySelection), nil
	case !equalRegistrySelection(s.RegistrySelection, lock.RegistrySelection):
		return nil, ErrRepairRegistryIntentConflict
	default:
		return copyRegistrySelection(s.RegistrySelection), nil
	}
}

// copyRegistrySelection returns a deep copy of a registry selection so
// persisted metadata can never alias caller-owned transport state.
func copyRegistrySelection(selection *model.RegistrySelection) *model.RegistrySelection {
	if selection == nil {
		return nil
	}
	copied := *selection
	copied.CustomSkillPaths = append([]string(nil), selection.CustomSkillPaths...)
	copied.DisabledComponents = append([]model.ComponentID(nil), selection.DisabledComponents...)
	return &copied
}

// equalRegistrySelection compares two registry selections semantically,
// treating nil and empty slices as equivalent.
func equalRegistrySelection(a, b *model.RegistrySelection) bool {
	if a.ConfigFile != b.ConfigFile {
		return false
	}
	if len(a.CustomSkillPaths) != len(b.CustomSkillPaths) {
		return false
	}
	for i, path := range a.CustomSkillPaths {
		if path != b.CustomSkillPaths[i] {
			return false
		}
	}
	if len(a.DisabledComponents) != len(b.DisabledComponents) {
		return false
	}
	for i, id := range a.DisabledComponents {
		if id != b.DisabledComponents[i] {
			return false
		}
	}
	return true
}

// hasCommittedRegistryReceipt reports whether the canonical receipt of a
// prior successful registry apply exists, mirroring the preflight's read-only
// receipt loading semantics.
func hasCommittedRegistryReceipt(homeDir string) (bool, error) {
	_, err := sdd.LoadCommittedRegistryReceipt(homeDir)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return false, fmt.Errorf("load committed registry receipt: %w", err)
}

func dedupeAgents(groups ...[]model.AgentID) []model.AgentID {
	seen := make(map[model.AgentID]struct{})
	result := make([]model.AgentID, 0)
	for _, group := range groups {
		for _, value := range group {
			if value == "" {
				continue
			}
			if _, ok := seen[value]; ok {
				continue
			}
			seen[value] = struct{}{}
			result = append(result, value)
		}
	}
	return result
}

func dedupeComponents(groups ...[]model.ComponentID) []model.ComponentID {
	seen := make(map[model.ComponentID]struct{})
	result := make([]model.ComponentID, 0)
	for _, group := range groups {
		for _, value := range group {
			if value == "" {
				continue
			}
			if _, ok := seen[value]; ok {
				continue
			}
			seen[value] = struct{}{}
			result = append(result, value)
		}
	}
	return result
}

func firstNonEmptyPreset(values ...model.PresetID) model.PresetID {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return model.PresetFull
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
