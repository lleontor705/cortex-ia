package pipeline

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"

	"github.com/lleontor705/cortex-ia/internal/agents/opencode"
	"github.com/lleontor705/cortex-ia/internal/assets"
	"github.com/lleontor705/cortex-ia/internal/backup"
	"github.com/lleontor705/cortex-ia/internal/components/filemerge"
	"github.com/lleontor705/cortex-ia/internal/installmeta"
	"github.com/lleontor705/cortex-ia/internal/mcpmanager"
	"github.com/lleontor705/cortex-ia/internal/state"
)

// Apply transactionally executes a plan: it revalidates every precondition,
// captures and verifies a restorable backup of all declared targets, journals
// their preimages, applies the mutations atomically, verifies the postimages,
// commits the v2 metadata last, and on any failure restores the exact
// preimages in reverse. A converged plan is a pure no-op.
func Apply(req Request, plan *Plan) (*Receipt, error) {
	if plan == nil {
		return nil, errors.New("apply requires a plan")
	}
	if plan.Converged {
		return &Receipt{PlanDigest: plan.Digest, Converged: true}, nil
	}
	if len(plan.Conflicts) > 0 {
		return &Receipt{PlanDigest: plan.Digest, Conflicts: plan.Conflicts}, &ConflictError{Conflicts: plan.Conflicts}
	}
	if err := revalidatePreconditions(plan); err != nil {
		return &Receipt{PlanDigest: plan.Digest}, err
	}

	home := plan.HomeDir
	now := req.now()
	backupID := fmt.Sprintf("install-%s-%09d", now.UTC().Format("20060102T150405"), now.Nanosecond())
	txnID := fmt.Sprintf("txn-%s-%s", now.UTC().Format("20060102T150405"), planDigestPrefix(plan.Digest))

	receipt := &Receipt{PlanDigest: plan.Digest, TransactionID: txnID}

	// Backup before any mutation: the verified manifest is the durable
	// restoration proof, including for explicitly authorized overwrites.
	snapshotDir := filepath.Join(home, ".cortex-ia", "backups", backupID)
	if _, err := captureVerifiedBackup(plan, snapshotDir); err != nil {
		return receipt, fmt.Errorf("capture verified backup: %w", err)
	}
	receipt.BackupID = backupID
	receipt.BackupVerified = true

	// The journal captures every declared preimage before the first write.
	// Its checkpoint root lives inside the backup directory so journal
	// creation can never become an untracked managed write.
	journal, err := BeginInstallJournal(home, filepath.Join(snapshotDir, "journal"), journalTargets(plan))
	if err != nil {
		return receipt, fmt.Errorf("capture install journal: %w", err)
	}

	fail := func(stage string, err error) (*Receipt, error) {
		wrapped := fmt.Errorf("%s: %w", stage, err)
		if restoreErr := journal.RestoreAndVerify(); restoreErr != nil {
			receipt.RestoreError = restoreErr.Error()
			return receipt, fmt.Errorf("%w; reverse restore failed, journal retained for safe retry: %v", wrapped, restoreErr)
		}
		receipt.Restored = true
		return receipt, wrapped
	}

	// runTarget executes one managed mutation and records its verified
	// postimage in the journal.
	runTarget := func(rel string, createdDirs []string, mutate func() error) error {
		if err := mutate(); err != nil {
			return err
		}
		abs := filepath.Join(home, filepath.FromSlash(rel))
		image, err := inspectPath(abs, rel)
		if err != nil {
			return err
		}
		image.CreatedDirs = createdDirs
		return journal.Record(image)
	}

	expected := make(map[string]string, len(plan.Effects))

	// File effects execute in sorted destination order.
	for _, effect := range plan.Effects {
		if !isFileEffect(effect.Kind) || !effect.mutating() {
			continue
		}
		abs := filepath.Join(home, filepath.FromSlash(effect.Dest))
		switch effect.Kind {
		case EffectCreate, EffectManagedUpdate, EffectOverwrite:
			content, readErr := assets.ReadBytes(effect.Source)
			if readErr != nil {
				return fail("read embedded source", readErr)
			}
			created := missingDirs(home, abs)
			if err := runTarget(effect.Dest, created, func() error {
				if _, err := filemerge.WriteFileAtomic(abs, content, 0o644); err != nil {
					return fmt.Errorf("write %q: %w", effect.Dest, err)
				}
				return nil
			}); err != nil {
				return fail("apply copy effect", err)
			}
			expected[effect.Dest] = effect.SourceSHA
		case EffectSafeMerge:
			base, readErr := os.ReadFile(abs)
			if readErr != nil {
				return fail("read settings file", readErr)
			}
			overlay, overlayErr := settingsOverlay(effect.Source)
			if overlayErr != nil {
				return fail("prepare embedded settings template", overlayErr)
			}
			merged, mergeErr := filemerge.MutateJSONDocument(abs, base, filemerge.JSONMutation{Overlay: overlay})
			if mergeErr != nil {
				return fail("merge settings template", mergeErr)
			}
			if err := runTarget(effect.Dest, nil, func() error {
				if _, err := filemerge.WriteFileAtomic(abs, merged, 0o644); err != nil {
					return fmt.Errorf("write merged settings %q: %w", effect.Dest, err)
				}
				return nil
			}); err != nil {
				return fail("apply settings merge", err)
			}
			expected[effect.Dest] = journalSHA256(merged)
		case EffectDelete:
			if err := runTarget(effect.Dest, nil, func() error {
				if err := os.Remove(abs); err != nil && !errors.Is(err, os.ErrNotExist) {
					return fmt.Errorf("remove stale artifact %q: %w", effect.Dest, err)
				}
				return nil
			}); err != nil {
				return fail("apply stale deletion", err)
			}
			expected[effect.Dest] = ""
		}
		receipt.Changes = append(receipt.Changes, fmt.Sprintf("%s %s", effect.Kind, effect.Dest))
	}

	// MCP effects run after file effects so the manager resolves the same
	// config file the settings template just materialized: when the template
	// creates opencode.jsonc mid-apply, the manager's load precedence
	// switches to it, so the effective config path is resolved here, at
	// apply time, never reused from planning. Its own writes are atomic; the
	// config preimage is journaled like every other target.
	manager := mcpmanager.New(home)
	configRel := relFromHome(home, manager.ConfigPath())
	// The MCP config's final postimage legitimately extends the merged
	// template with managed entries; its file-effect digest expectation is
	// dropped and verification is ownership-based instead.
	delete(expected, configRel)
	records := keptMCPRecords(plan)
	for _, effect := range plan.Effects {
		switch effect.Kind {
		case EffectMCPAdd:
			preset, ok := mcpmanager.Lookup(effect.Dest)
			if !ok {
				return fail("resolve managed preset", fmt.Errorf("preset %q vanished from the catalog", effect.Dest))
			}
			result, addErr := manager.Add(effect.Dest, plan.evidence, req.Probes[effect.Dest]...)
			if addErr != nil {
				return fail("add managed MCP entry", addErr)
			}
			record, recErr := mcpRecord(preset, result.ConfigPath, plan)
			if recErr != nil {
				return fail("record managed MCP ownership", recErr)
			}
			records = append(records, record)
			receipt.Changes = append(receipt.Changes, fmt.Sprintf("%s %s", effect.Kind, effect.Dest))
			if !result.Qualified {
				receipt.Warnings = append(receipt.Warnings, fmt.Sprintf("MCP %q configured without valid qualification evidence", effect.Dest))
			}
		case EffectMCPRemove:
			if _, removeErr := manager.Remove(effect.Dest, plan.evidence); removeErr != nil {
				return fail("remove managed MCP entry", removeErr)
			}
			receipt.Changes = append(receipt.Changes, fmt.Sprintf("%s %s", effect.Kind, effect.Dest))
		}
		if effect.Kind == EffectMCPAdd || effect.Kind == EffectMCPRemove {
			image, inspectErr := inspectPath(manager.ConfigPath(), configRel)
			if inspectErr != nil {
				return fail("inspect MCP config postimage", inspectErr)
			}
			if err := journal.Record(image); err != nil {
				return fail("record MCP config outcome", err)
			}
		}
	}

	// Postimage verification: every mutation must own its expected bytes.
	if err := verifyPostimages(plan, expected); err != nil {
		return fail("verify postimages", err)
	}
	if err := verifyMCPPostimages(plan, req, records); err != nil {
		return fail("verify managed MCP selection", err)
	}

	// Metadata commits last: state first, then the derived lock, both
	// journaled so a failure restores the exact pre-transaction metadata.
	meta := buildMetadata(req, plan, records, backupID, txnID)
	if err := runTarget(stateRel(home, state.StatePath(home)), nil, func() error {
		return state.SaveMetadataV2(home, meta)
	}); err != nil {
		return fail("commit state v2", err)
	}
	lock := state.NewLockFromMetadataV2(meta)
	if err := runTarget(stateRel(home, state.LockPath(home)), nil, func() error {
		return state.SaveLockV2(home, lock)
	}); err != nil {
		return fail("commit lock v2", err)
	}

	if err := journal.Commit(); err != nil {
		return fail("commit install journal", err)
	}
	sort.Strings(receipt.Changes)
	return receipt, nil
}

// ApplyConfirmed re-plans the request with replan while the caller holds the
// canonical home lock, binds the caller-confirmed digest to that freshly
// derived plan, and transactionally applies the fresh plan. It exists so the
// install service can acquire the home lock between the lock-free preview
// planning and the mutation: every backup, journal, write, and metadata
// commit below this call executes under the caller-held lock against a plan
// derived under that same lock, so a home mutated by a concurrent process
// between the preview and the lock is reflected in the fresh plan. The
// digest comparison runs before any side effect — before the backup and the
// journal — so a stale confirmation fails as a typed PlanDriftError with
// zero mutation. An empty expectation keeps the unbound compatibility
// behavior. There is deliberately no plan parameter: a plan derived before
// the lock was acquired is stale by construction, and accepting one would
// reintroduce exactly the time-of-check-to-time-of-use window this entry
// point exists to close. Receipt shapes (converged, conflicts, drift) are
// identical to InstallV2/SyncV2, derived from the fresh plan.
func ApplyConfirmed(req Request, replan func(Request) (*Plan, error)) (*Plan, *Receipt, error) {
	if replan == nil {
		return nil, nil, errors.New("apply requires a planning function")
	}
	plan, err := replan(req)
	if err != nil {
		return plan, &Receipt{DryRun: req.DryRun}, err
	}
	if err := checkPlanDrift(req, plan); err != nil {
		return plan, &Receipt{DryRun: req.DryRun}, err
	}
	if plan.Converged {
		return plan, &Receipt{PlanDigest: plan.Digest, Converged: true}, nil
	}
	if len(plan.Conflicts) > 0 {
		return plan, &Receipt{PlanDigest: plan.Digest, Conflicts: plan.Conflicts}, &ConflictError{Conflicts: plan.Conflicts}
	}
	receipt, err := Apply(req, plan)
	return plan, receipt, err
}

// revalidatePreconditions re-inspects every planned destination and confirms
// the planning evidence still holds; drift aborts before any mutation.
func revalidatePreconditions(plan *Plan) error {
	for _, effect := range plan.Effects {
		if !isFileEffect(effect.Kind) {
			continue
		}
		abs := filepath.Join(plan.HomeDir, filepath.FromSlash(effect.Dest))
		exists, digest, err := inspectFileTarget(abs)
		if err != nil {
			return fmt.Errorf("revalidate %q: %w", effect.Dest, err)
		}
		if exists != effect.PriorExists || digest != effect.CurrentSHA {
			return fmt.Errorf("precondition drift for %q: planned existed=%t digest=%q, observed existed=%t digest=%q",
				effect.Dest, effect.PriorExists, effect.CurrentSHA, exists, digest)
		}
	}
	return nil
}

// captureVerifiedBackup snapshots every declared target that exists and
// proves the manifest restorable before any mutation may begin.
func captureVerifiedBackup(plan *Plan, snapshotDir string) (backup.Manifest, error) {
	paths := make([]string, 0, len(plan.Effects)+3)
	seen := make(map[string]bool)
	addPath := func(abs string) {
		if abs != "" && !seen[abs] {
			seen[abs] = true
			paths = append(paths, abs)
		}
	}
	home := plan.HomeDir
	for _, effect := range plan.Effects {
		if isFileEffect(effect.Kind) && effect.mutating() {
			addPath(filepath.Join(home, filepath.FromSlash(effect.Dest)))
		}
	}
	if hasMutatingMCPEffect(plan) {
		for _, configRel := range mcpConfigCandidates() {
			addPath(filepath.Join(home, filepath.FromSlash(configRel)))
		}
	}
	addPath(state.StatePath(home))
	addPath(state.LockPath(home))

	manifest, err := backup.NewSnapshotter().Create(snapshotDir, paths)
	if err != nil {
		return backup.Manifest{}, err
	}
	if err := backup.Verify(manifest); err != nil {
		return backup.Manifest{}, fmt.Errorf("backup is not restorable: %w", err)
	}
	return manifest, nil
}

// journalTargets declares every file the transaction may mutate, including
// the metadata files committed last.
func journalTargets(plan *Plan) []ManagedTarget {
	home := plan.HomeDir
	rels := make([]string, 0, len(plan.Effects)+3)
	seen := make(map[string]bool)
	addRel := func(rel string) {
		if rel != "" && !seen[rel] {
			seen[rel] = true
			rels = append(rels, rel)
		}
	}
	for _, effect := range plan.Effects {
		if isFileEffect(effect.Kind) && effect.mutating() {
			addRel(effect.Dest)
		}
	}
	if hasMutatingMCPEffect(plan) {
		// The manager's load precedence switches to opencode.jsonc as soon
		// as the settings template creates it mid-apply, so both candidate
		// config files are declared before any write can begin.
		for _, configRel := range mcpConfigCandidates() {
			addRel(configRel)
		}
	}
	addRel(stateRel(home, state.StatePath(home)))
	addRel(stateRel(home, state.LockPath(home)))

	targets := make([]ManagedTarget, 0, len(rels))
	for _, rel := range rels {
		targets = append(targets, ManagedTarget{Path: rel, Kind: TargetFile, Owner: "engine"})
	}
	return targets
}

// verifyPostimages confirms every mutating file effect owns its expected
// bytes (copies and merges) or is gone (deletions).
func verifyPostimages(plan *Plan, expected map[string]string) error {
	for _, effect := range plan.Effects {
		if !isFileEffect(effect.Kind) || !effect.mutating() {
			continue
		}
		wantDigest, tracked := expected[effect.Dest]
		if !tracked {
			continue
		}
		abs := filepath.Join(plan.HomeDir, filepath.FromSlash(effect.Dest))
		exists, digest, err := inspectFileTarget(abs)
		if err != nil {
			return err
		}
		if wantDigest == "" {
			if exists {
				return fmt.Errorf("postimage for %q must be absent after deletion", effect.Dest)
			}
			continue
		}
		if !exists || digest != wantDigest {
			return fmt.Errorf("postimage for %q does not match the expected digest", effect.Dest)
		}
	}
	return nil
}

// verifyMCPPostimages re-lists the config with the committed ownership
// evidence and confirms the desired selection is accredited and the
// deselected managed entries are gone.
func verifyMCPPostimages(plan *Plan, req Request, records []state.MCPV2) error {
	home := plan.HomeDir
	manager := mcpmanager.New(home)
	evidence := make([]mcpmanager.OwnershipRecord, 0, len(records))
	for _, record := range records {
		evidence = append(evidence, mcpmanager.OwnershipRecord{
			Name:       record.Name,
			Digest:     record.SemanticDigest,
			ConfigPath: filepath.Join(plan.OpencodeRoot, filepath.FromSlash(record.ConfigPath)),
		})
	}
	listing, err := manager.List(evidence)
	if err != nil {
		return err
	}
	statuses := make(map[string]mcpmanager.EntryStatus, len(listing.Entries))
	for _, report := range listing.Entries {
		statuses[report.Name] = report.Status
	}
	selection := req.mcpSelection()
	for name, desired := range selection {
		if desired && statuses[name] != mcpmanager.StatusManaged {
			return fmt.Errorf("managed MCP %q is not accredited after apply (status %q)", name, statuses[name])
		}
		if !desired && statuses[name] == mcpmanager.StatusManaged {
			return fmt.Errorf("deselected MCP %q is still accredited after apply", name)
		}
	}
	return nil
}

// buildMetadata assembles the v2 state document committed after a
// successful apply, preserving prior-presence evidence and migration
// provenance.
func buildMetadata(req Request, plan *Plan, records []state.MCPV2, backupID, txnID string) state.MetadataV2 {
	desired := desiredArtifacts(plan)
	priors := make(map[string]Effect, len(plan.Effects))
	for _, effect := range plan.Effects {
		priors[effect.Dest] = effect
	}

	artifacts := make([]state.ArtifactV2, 0, len(desired))
	for path, artifact := range desired {
		mappingDest := opencode.DestinationForArtifactWithHome(path, string(artifact.Kind), plan.HomeDir)
		if effect, ok := priors[mappingDest]; ok && effect.PriorExists {
			digest := effect.CurrentSHA
			artifact.Prior = &state.ArtifactPrior{Existed: true, Digest: digest}
		}
		artifact.BackupRef = filepath.ToSlash(filepath.Join(".cortex-ia", "backups", backupID))
		artifacts = append(artifacts, artifact)
	}

	meta := state.MetadataV2{
		SchemaVersion: state.MetadataSchemaV2,
		OpencodeRoot:  plan.OpencodeRoot,
		Selection: state.SelectionV2{
			AssetGroups: []string{"native"},
			Cortex:      req.Cortex,
			ForgeSpec:   req.ForgeSpec,
			Context7:    req.Context7,
		},
		Artifacts:     artifacts,
		MCPs:          records,
		TransactionID: txnID,
		BackupID:      backupID,
		UpdatedAt:     req.now(),
	}
	if plan.Migration != nil && plan.Migration.Candidate != nil {
		provenance := plan.Migration.Candidate.Provenance
		meta.Migration = &provenance
	}
	meta.Normalize()
	return meta
}

// keptMCPRecords preserves the records of managed MCP entries that stay
// selected; adds append their fresh records during apply.
func keptMCPRecords(plan *Plan) []state.MCPV2 {
	stay := make(map[string]bool)
	for _, effect := range plan.Effects {
		if effect.Kind == EffectMCPNoop {
			stay[effect.Dest] = true
		}
	}
	records := make([]state.MCPV2, 0, len(plan.Metadata.MCPs))
	for _, mcp := range plan.Metadata.MCPs {
		if stay[mcp.Name] && mcp.Ownership == state.OwnershipManaged {
			records = append(records, mcp)
		}
	}
	return records
}

// mcpRecord derives the sanitized v2 record for one configured preset from
// its canonical secret-free identity.
func mcpRecord(preset mcpmanager.Preset, configAbs string, plan *Plan) (state.MCPV2, error) {
	identity, err := installmeta.MCPServerIdentityFromEntry(preset.Name, preset.Entry)
	if err != nil {
		return state.MCPV2{}, err
	}
	rel, err := filepath.Rel(plan.OpencodeRoot, configAbs)
	if err != nil {
		return state.MCPV2{}, fmt.Errorf("resolve config path relative to the OpenCode root: %w", err)
	}
	return state.NewMCPV2(identity, filepath.ToSlash(rel), state.OwnershipManaged)
}

func isFileEffect(kind EffectKind) bool {
	switch kind {
	case EffectCreate, EffectNoop, EffectManagedUpdate, EffectOverwrite, EffectSafeMerge, EffectDelete:
		return true
	default:
		return false
	}
}

func hasMutatingMCPEffect(plan *Plan) bool {
	for _, effect := range plan.Effects {
		if effect.Kind == EffectMCPAdd || effect.Kind == EffectMCPRemove {
			return true
		}
	}
	return false
}

// mcpConfigCandidates lists both OpenCode global config files the manager
// may resolve, derived from the native layout, never hardcoded per home.
func mcpConfigCandidates() []string {
	root := opencode.NativeLayout().ConfigRoot
	return []string{path.Join(root, "opencode.json"), path.Join(root, "opencode.jsonc")}
}

// missingDirs returns the directory chain that does not exist yet, ordered
// shallow to deep and relative to the home root, so journal restoration can
// remove exactly what the transaction created.
func missingDirs(home, abs string) []string {
	dir := filepath.Dir(abs)
	var missing []string
	for {
		if _, err := os.Stat(dir); err == nil {
			break
		} else if !os.IsNotExist(err) {
			return nil
		}
		missing = append(missing, dir)
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	// missing is deep-to-shallow; reverse for shallow-to-deep and relativize
	// against the home root because journal-created directories must be
	// declared relative to the journal target root.
	for i, j := 0, len(missing)-1; i < j; i, j = i+1, j-1 {
		missing[i], missing[j] = missing[j], missing[i]
	}
	relative := make([]string, 0, len(missing))
	for _, dir := range missing {
		rel, err := filepath.Rel(home, dir)
		if err != nil {
			return nil
		}
		relative = append(relative, filepath.ToSlash(rel))
	}
	return relative
}

// stateRel converts an absolute state path into its home-relative slash
// form for journal declarations.
func stateRel(home, abs string) string {
	rel, err := filepath.Rel(home, abs)
	if err != nil {
		return filepath.ToSlash(abs)
	}
	return filepath.ToSlash(rel)
}

func planDigestPrefix(digest string) string {
	if len(digest) >= 8 {
		return digest[:8]
	}
	return digest
}
