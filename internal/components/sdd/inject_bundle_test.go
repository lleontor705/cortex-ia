package sdd

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/agents"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/compiler"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/prompt"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/registry"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/renderers"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/skillcore"
	"github.com/lleontor705/cortex-ia/internal/model"
)

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

const (
	bundleBaselinePath = ".config/opencode/AGENTS.md"
	bundleCustomPath   = ".config/opencode/skills/review-checklist/SKILL.md"
)

var (
	baselineContent = []byte("# OpenCode Workflow Bundle\n\nManaged baseline index.\n")
	customContent   = []byte("---\nname: review-checklist\ndescription: Checklist discipline\n---\n\nFollow the checklist.\n")
)

func bundleAsset(path string, semanticID ir.SemanticID, kind renderers.AssetKind, content []byte, mode fs.FileMode) renderers.Asset {
	return renderers.Asset{Path: path, SemanticID: semanticID, Kind: kind, Content: content, Mode: mode}
}

func baselineBundleAsset() renderers.Asset {
	return bundleAsset(bundleBaselinePath, "asset/opencode/instruction/root", renderers.AssetInstruction, baselineContent, 0o644)
}

func customBundleAsset() renderers.Asset {
	return bundleAsset(bundleCustomPath, "asset/skill/review-checklist", renderers.AssetSkill, customContent, 0o644)
}

func overlayBundle() CompiledInjectionBundle {
	return CompiledInjectionBundle{
		Target:      "opencode",
		Profile:     ProfilePortableFlat,
		Fingerprint: "fixture-overlay-1",
		Bundle:      renderers.Bundle{Assets: []renderers.Asset{baselineBundleAsset(), customBundleAsset()}},
	}
}

func baselineOnlyBundle() CompiledInjectionBundle {
	return CompiledInjectionBundle{
		Target:      "opencode",
		Profile:     ProfilePortableFlat,
		Fingerprint: "fixture-baseline-1",
		Bundle:      renderers.Bundle{Assets: []renderers.Asset{baselineBundleAsset()}},
	}
}

// sealedBundleReceipt builds a sealed canonical registry receipt carrying one
// embedded skill per requested ID plus the custom overlay skill, mirroring the
// shape registry.Resolve seals for the same effective input.
func sealedBundleReceipt(t *testing.T, customIDs ...string) registry.Receipt {
	t.Helper()
	embedded := []skillcore.Skill{bundleSkillRecord("orchestrator", skillcore.OriginEmbedded)}
	for _, id := range customIDs {
		embedded = append(embedded, bundleSkillRecord(id, skillcore.OriginCustom))
	}
	return registry.SealReceipt(registry.Receipt{
		SchemaVersion:   registry.ReceiptSchemaVersion,
		PolicyDigest:    "policy-fixture",
		BaselineDigest:  "baseline-fixture",
		EffectiveSkills: registry.BuildSkillSet(embedded),
		EffectiveComponents: []model.ComponentID{
			"authority/forgespec", "workflow/sdd",
		},
	})
}

func bundleSkillRecord(id string, origin skillcore.OriginKind) skillcore.Skill {
	content := []byte("# " + id + "\n\nManaged body.\n")
	return skillcore.Skill{ID: model.SkillID(id), Content: content, ContentSHA256: ir.FingerprintContent(content), Origin: origin}
}

func requirePlan(t *testing.T, request GlobalInstallPlanRequest) GlobalInstallPlan {
	t.Helper()
	plan, diags := BuildGlobalInstallPlan(request)
	if len(diags) != 0 {
		t.Fatalf("BuildGlobalInstallPlan returned diagnostics: %v", diags)
	}
	return plan
}

func requireApply(t *testing.T, homeDir string, plan GlobalInstallPlan) GlobalApplyResult {
	t.Helper()
	result, err := ApplyGlobalInstallPlan(homeDir, plan)
	if err != nil {
		t.Fatalf("ApplyGlobalInstallPlan: %v", err)
	}
	return result
}

func assertFileBytes(t *testing.T, homeDir, path string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(filepath.Join(homeDir, filepath.FromSlash(path)))
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("content of %s:\n got %q\nwant %q", path, got, want)
	}
}

func assertPathAbsent(t *testing.T, homeDir, path string) {
	t.Helper()
	if _, err := os.Lstat(filepath.Join(homeDir, filepath.FromSlash(path))); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("path %s still exists (or is unreadable): %v", path, err)
	}
}

func opPaths(ops []BundleOperation) []string {
	paths := make([]string, 0, len(ops))
	for _, op := range ops {
		paths = append(paths, op.Path)
	}
	return paths
}

// ---------------------------------------------------------------------------
// AC: plan purity (no EnsureDir/backup before BuildInstallPlan returns)
// ---------------------------------------------------------------------------

// TestBundle_BuildInstallPlanIsPure proves the global plan is built strictly
// before any write: planning against a home directory that does not exist
// reads current state, computes ops, and leaves the filesystem untouched.
func TestBundle_BuildInstallPlanIsPure(t *testing.T) {
	outer := t.TempDir()
	home := filepath.Join(outer, "absent-home")

	plan := requirePlan(t, GlobalInstallPlanRequest{
		HomeDir: home,
		Bundles: []CompiledInjectionBundle{overlayBundle()},
		Receipt: sealedBundleReceipt(t, "review-checklist"),
	})

	if _, err := os.Lstat(home); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("planning created filesystem state under %s (Lstat err = %v)", home, err)
	}
	if len(plan.Adapters) != 1 || len(plan.Adapters[0].Ops) != 2 {
		t.Fatalf("adapter ops = %+v, want exactly the two planned creates", plan.Adapters)
	}
	if got := plan.RollbackPaths; len(got) != 2 {
		t.Errorf("RollbackPaths = %v, want both planned targets", got)
	}
	if err := registry.ValidateReceipt(plan.Receipt); err != nil {
		t.Errorf("planned receipt is not sealed correctly: %v", err)
	}
	want := []string{bundleBaselinePath, bundleCustomPath}
	if got := plan.Receipt.HostOutputs; !equalStrings(got, want) {
		t.Errorf("receipt HostOutputs = %v, want %v", got, want)
	}
}

// ---------------------------------------------------------------------------
// AC-DET-2: converged state produces no rewrites
// ---------------------------------------------------------------------------

func TestBundle_IdempotentSecondRun(t *testing.T) {
	home := t.TempDir()
	receipt := sealedBundleReceipt(t, "review-checklist")
	request := GlobalInstallPlanRequest{
		HomeDir: home,
		Bundles: []CompiledInjectionBundle{overlayBundle()},
		Receipt: receipt,
	}

	first := requirePlan(t, request)
	firstResult := requireApply(t, home, first)
	if !firstResult.Changed {
		t.Fatal("first apply reported no changes")
	}
	assertFileBytes(t, home, bundleBaselinePath, baselineContent)
	assertFileBytes(t, home, bundleCustomPath, customContent)

	committed, err := LoadCommittedRegistryReceipt(home)
	if err != nil {
		t.Fatalf("load committed receipt: %v", err)
	}
	if err := registry.ValidateReceipt(committed); err != nil {
		t.Fatalf("committed receipt is invalid: %v", err)
	}
	if committed.Fingerprint != first.Receipt.Fingerprint {
		t.Fatalf("committed fingerprint %s != planned %s", committed.Fingerprint, first.Receipt.Fingerprint)
	}

	receiptBytesBefore, err := os.ReadFile(CommittedRegistryReceiptPath(home))
	if err != nil {
		t.Fatalf("read committed receipt file: %v", err)
	}

	second := requirePlan(t, GlobalInstallPlanRequest{
		HomeDir:           home,
		Bundles:           []CompiledInjectionBundle{overlayBundle()},
		PriorManagedPaths: committed.HostOutputs,
		Receipt:           receipt,
	})
	if len(second.Adapters) != 1 || len(second.Adapters[0].Ops) != 0 {
		t.Fatalf("second-run adapter ops = %v, want none (converged)", second.Adapters)
	}
	if len(second.Shared.Writes) != 0 || len(second.Shared.Deletes) != 0 {
		t.Fatalf("second-run shared ops = %v, want none (converged)", second.Shared)
	}
	if len(second.RollbackPaths) != 0 {
		t.Fatalf("second-run RollbackPaths = %v, want none (no planned mutation)", second.RollbackPaths)
	}
	if second.Receipt.Fingerprint != first.Receipt.Fingerprint {
		t.Fatalf("second-run receipt fingerprint %s != first %s", second.Receipt.Fingerprint, first.Receipt.Fingerprint)
	}

	secondResult := requireApply(t, home, second)
	if secondResult.Changed {
		t.Errorf("second apply rewrote files: applied %v deleted %v", secondResult.Applied, secondResult.Deleted)
	}

	receiptBytesAfter, err := os.ReadFile(CommittedRegistryReceiptPath(home))
	if err != nil {
		t.Fatalf("re-read committed receipt file: %v", err)
	}
	if !bytes.Equal(receiptBytesBefore, receiptBytesAfter) {
		t.Error("committed receipt bytes changed on the idempotent second run")
	}
}

// ---------------------------------------------------------------------------
// AC-ROLL-1, AC-ROLL-2: stale managed delete restores baseline
// ---------------------------------------------------------------------------

func TestBundle_StaleDeleteRestoresBaseline(t *testing.T) {
	home := t.TempDir()

	overlayReceipt := sealedBundleReceipt(t, "review-checklist")
	first := requirePlan(t, GlobalInstallPlanRequest{
		HomeDir: home,
		Bundles: []CompiledInjectionBundle{overlayBundle()},
		Receipt: overlayReceipt,
	})
	requireApply(t, home, first)
	assertFileBytes(t, home, bundleCustomPath, customContent)

	committed, err := LoadCommittedRegistryReceipt(home)
	if err != nil {
		t.Fatalf("load committed receipt after overlay install: %v", err)
	}

	// Removing the overlay and reinstalling must plan deletion of the formerly
	// managed custom output and verify the exact baseline afterwards.
	baselineReceipt := sealedBundleReceipt(t)
	second := requirePlan(t, GlobalInstallPlanRequest{
		HomeDir:           home,
		Bundles:           []CompiledInjectionBundle{baselineOnlyBundle()},
		PriorManagedPaths: committed.HostOutputs,
		Receipt:           baselineReceipt,
	})
	if len(second.Adapters) != 1 || len(second.Adapters[0].Ops) != 0 {
		t.Fatalf("baseline restore planned adapter writes %v, want none", second.Adapters)
	}
	if got := opPaths(second.Shared.Deletes); !equalStrings(got, []string{bundleCustomPath}) {
		t.Fatalf("planned stale deletes = %v, want the formerly managed custom skill", got)
	}
	stale := second.Shared.Deletes[0]
	if !stale.Delete || !stale.Existed || stale.BeforeSHA256 != ir.FingerprintContent(customContent) {
		t.Fatalf("stale delete op = %+v, want a delete of the existing managed bytes", stale)
	}

	result := requireApply(t, home, second)
	if !result.Changed || !equalStrings(result.Deleted, []string{bundleCustomPath}) {
		t.Fatalf("baseline restore result = %+v, want the custom output deleted", result)
	}
	assertPathAbsent(t, home, bundleCustomPath)
	assertFileBytes(t, home, bundleBaselinePath, baselineContent)

	restored, err := LoadCommittedRegistryReceipt(home)
	if err != nil {
		t.Fatalf("load committed receipt after baseline restore: %v", err)
	}
	if got := restored.HostOutputs; !equalStrings(got, []string{bundleBaselinePath}) {
		t.Errorf("restored receipt HostOutputs = %v, want exactly the baseline output", got)
	}
	if err := registry.ValidateReceipt(restored); err != nil {
		t.Errorf("restored receipt is invalid: %v", err)
	}
	if restored.Fingerprint == committed.Fingerprint {
		t.Error("baseline receipt fingerprint equals the overlay receipt fingerprint; removal is not visible")
	}
}

// ---------------------------------------------------------------------------
// AC-ROLL-3: non-removable residual prevents false success
// ---------------------------------------------------------------------------

func TestBundle_ResidualPreventsFalseSuccess(t *testing.T) {
	home := t.TempDir()

	// A previously managed output whose desired update applies cleanly, and a
	// later write that fails on a blocked destination (a regular file where a
	// parent directory is required).
	managedFile := filepath.Join(home, filepath.FromSlash(bundleBaselinePath))
	if err := os.MkdirAll(filepath.Dir(managedFile), 0o755); err != nil {
		t.Fatalf("prepare managed parent: %v", err)
	}
	if err := os.WriteFile(managedFile, []byte("old managed bytes\n"), 0o644); err != nil {
		t.Fatalf("write managed file: %v", err)
	}
	blocker := filepath.Join(home, filepath.FromSlash(".config/opencode/skills/blocker"))
	if err := os.MkdirAll(filepath.Dir(blocker), 0o755); err != nil {
		t.Fatalf("prepare blocker parent: %v", err)
	}
	if err := os.WriteFile(blocker, []byte("regular file\n"), 0o644); err != nil {
		t.Fatalf("write blocker file: %v", err)
	}

	desired := CompiledInjectionBundle{
		Target:      "opencode",
		Profile:     ProfilePortableFlat,
		Fingerprint: "fixture-residual-1",
		Bundle: renderers.Bundle{Assets: []renderers.Asset{
			baselineBundleAsset(),
			bundleAsset(".config/opencode/skills/blocker/new.txt", "asset/opencode/instruction/blocked", renderers.AssetInstruction, []byte("never lands\n"), 0o644),
		}},
	}
	plan := requirePlan(t, GlobalInstallPlanRequest{
		HomeDir:           home,
		Bundles:           []CompiledInjectionBundle{desired},
		PriorManagedPaths: []string{bundleBaselinePath},
		Receipt:           sealedBundleReceipt(t),
	})

	// Before the blocked write, damage the already-applied managed update
	// beyond restoration: replace it with a non-empty directory, which cannot
	// be rewritten or removed by the snapshot restoration on any platform.
	t.Cleanup(func() { bundleOpFaultHook = nil })
	bundleOpFaultHook = func(op BundleOperation) error {
		if op.Path != ".config/opencode/skills/blocker/new.txt" {
			return nil
		}
		if err := os.RemoveAll(managedFile); err != nil {
			t.Fatalf("damage managed target: %v", err)
		}
		if err := os.MkdirAll(filepath.Join(managedFile, "residual"), 0o755); err != nil {
			t.Fatalf("damage managed target as directory: %v", err)
		}
		return os.WriteFile(filepath.Join(managedFile, "residual", "stale.txt"), []byte("residual\n"), 0o644)
	}

	result, err := ApplyGlobalInstallPlan(home, plan)
	if err == nil {
		t.Fatalf("apply with a non-removable residual claimed success: %+v", result)
	}
	var installErr *registry.InstallError
	if !errors.As(err, &installErr) {
		t.Fatalf("error is %T (%v), want *registry.InstallError", err, err)
	}
	if installErr.Primary.Class != registry.ErrorWrite || installErr.Primary.Stage != registry.StageApply {
		t.Errorf("primary diagnostic = %s/%s, want %s/%s", installErr.Primary.Class, installErr.Primary.Stage, registry.ErrorWrite, registry.StageApply)
	}
	if installErr.Rollback == nil {
		t.Fatal("restoration did not converge but no rollback diagnostic was reported")
	}
	if installErr.Rollback.Class != registry.ErrorRollback || installErr.Rollback.Stage != registry.StageRollback {
		t.Errorf("rollback diagnostic = %s/%s, want %s/%s", installErr.Rollback.Class, installErr.Rollback.Stage, registry.ErrorRollback, registry.StageRollback)
	}
	if !strings.Contains(installErr.Rollback.Cause.Error(), bundleBaselinePath) {
		t.Errorf("rollback cause %q does not list the residual path %q", installErr.Rollback.Cause, bundleBaselinePath)
	}

	// No false baseline receipt: nothing may be committed, the blocked write
	// must be absent, and the damaged residual must still be present because
	// restoration could not converge.
	if _, statErr := os.Lstat(CommittedRegistryReceiptPath(home)); !errors.Is(statErr, fs.ErrNotExist) {
		t.Errorf("committed receipt exists after a failed apply (statErr = %v)", statErr)
	}
	assertPathAbsent(t, home, ".config/opencode/skills/blocker/new.txt")
	if info, statErr := os.Lstat(managedFile); statErr != nil || !info.IsDir() {
		t.Errorf("residual %s was converged away (statErr = %v, isDir = %t)", bundleBaselinePath, statErr, info.IsDir())
	}
	if result.Receipt.Fingerprint != "" {
		t.Errorf("failed apply returned a receipt fingerprint %q", result.Receipt.Fingerprint)
	}
}

// ---------------------------------------------------------------------------
// AC-DIAG-3: write failure is distinguished from rollback-incomplete
// ---------------------------------------------------------------------------

func TestBundle_WriteFailureNoConvergence(t *testing.T) {
	home := t.TempDir()

	// A previously managed output whose update applies cleanly, so the later
	// failure forces a real restoration.
	managedPath := filepath.Join(home, filepath.FromSlash(bundleBaselinePath))
	if err := os.MkdirAll(filepath.Dir(managedPath), 0o755); err != nil {
		t.Fatalf("prepare managed parent: %v", err)
	}
	if err := os.WriteFile(managedPath, []byte("old managed bytes\n"), 0o644); err != nil {
		t.Fatalf("write managed file: %v", err)
	}
	// A regular file blocking the parent of a desired write: the plan sees an
	// absent target (nothing exists under a non-directory), and the apply-time
	// write fails after the update has already been applied.
	blocker := filepath.Join(home, filepath.FromSlash(".config/opencode/skills/blocker"))
	if err := os.MkdirAll(filepath.Dir(blocker), 0o755); err != nil {
		t.Fatalf("prepare blocker parent: %v", err)
	}
	if err := os.WriteFile(blocker, []byte("regular file\n"), 0o644); err != nil {
		t.Fatalf("write blocker file: %v", err)
	}

	updated := append([]byte(nil), baselineContent...)
	desired := CompiledInjectionBundle{
		Target:      "opencode",
		Profile:     ProfilePortableFlat,
		Fingerprint: "fixture-write-failure-1",
		Bundle: renderers.Bundle{Assets: []renderers.Asset{
			bundleAsset(bundleBaselinePath, "asset/opencode/instruction/root", renderers.AssetInstruction, updated, 0o644),
			bundleAsset(".config/opencode/skills/blocker/new.txt", "asset/opencode/instruction/blocked", renderers.AssetInstruction, []byte("never lands\n"), 0o644),
		}},
	}
	plan := requirePlan(t, GlobalInstallPlanRequest{
		HomeDir:           home,
		Bundles:           []CompiledInjectionBundle{desired},
		PriorManagedPaths: []string{bundleBaselinePath},
		Receipt:           sealedBundleReceipt(t),
	})

	result, err := ApplyGlobalInstallPlan(home, plan)
	if err == nil {
		t.Fatalf("apply with a post-preflight write failure claimed success: %+v", result)
	}
	var installErr *registry.InstallError
	if !errors.As(err, &installErr) {
		t.Fatalf("error is %T (%v), want *registry.InstallError", err, err)
	}
	if installErr.Primary.Class != registry.ErrorWrite || installErr.Primary.Stage != registry.StageApply {
		t.Errorf("primary diagnostic = %s/%s, want %s/%s", installErr.Primary.Class, installErr.Primary.Stage, registry.ErrorWrite, registry.StageApply)
	}
	// The restoration completed, so this is a write failure, not an incomplete
	// rollback: the distinction the diagnostics contract requires.
	if installErr.Rollback != nil {
		t.Fatalf("complete restoration was reported as rollback-incomplete: %+v", installErr.Rollback)
	}

	// No convergence: the managed update was restored to its prior bytes and
	// no receipt was committed.
	assertFileBytes(t, home, bundleBaselinePath, []byte("old managed bytes\n"))
	assertPathAbsent(t, home, ".config/opencode/skills/blocker/new.txt")
	if _, statErr := os.Lstat(CommittedRegistryReceiptPath(home)); !errors.Is(statErr, fs.ErrNotExist) {
		t.Errorf("committed receipt exists after a failed apply (statErr = %v)", statErr)
	}
}

// ---------------------------------------------------------------------------
// Receipt lifecycle: committed only after verification succeeds
// ---------------------------------------------------------------------------

func TestBundle_ReceiptCommittedOnlyAfterSuccess(t *testing.T) {
	home := t.TempDir()

	// A failing apply (write blocked by a regular file) commits no receipt.
	blocker := filepath.Join(home, filepath.FromSlash(".config/opencode/skills/blocker"))
	if err := os.MkdirAll(filepath.Dir(blocker), 0o755); err != nil {
		t.Fatalf("prepare blocker parent: %v", err)
	}
	if err := os.WriteFile(blocker, []byte("regular file\n"), 0o644); err != nil {
		t.Fatalf("write blocker file: %v", err)
	}
	failing := CompiledInjectionBundle{
		Target:  "opencode",
		Profile: ProfilePortableFlat,
		Bundle: renderers.Bundle{Assets: []renderers.Asset{
			bundleAsset(".config/opencode/skills/blocker/new.txt", "asset/opencode/instruction/blocked", renderers.AssetInstruction, []byte("nope\n"), 0o644),
		}},
	}
	failingPlan := requirePlan(t, GlobalInstallPlanRequest{
		HomeDir: home,
		Bundles: []CompiledInjectionBundle{failing},
		Receipt: sealedBundleReceipt(t),
	})
	if _, err := ApplyGlobalInstallPlan(home, failingPlan); err == nil {
		t.Fatal("blocked write was accepted")
	}
	if _, statErr := os.Lstat(CommittedRegistryReceiptPath(home)); !errors.Is(statErr, fs.ErrNotExist) {
		t.Fatalf("receipt committed despite apply failure (statErr = %v)", statErr)
	}

	// A successful apply commits a valid, stable, canonical receipt.
	good := requirePlan(t, GlobalInstallPlanRequest{
		HomeDir: home,
		Bundles: []CompiledInjectionBundle{baselineOnlyBundle()},
		Receipt: sealedBundleReceipt(t),
	})
	goodResult := requireApply(t, home, good)
	if goodResult.Receipt.Fingerprint != good.Receipt.Fingerprint {
		t.Fatalf("result receipt fingerprint %s != planned %s", goodResult.Receipt.Fingerprint, good.Receipt.Fingerprint)
	}
	committed, err := LoadCommittedRegistryReceipt(home)
	if err != nil {
		t.Fatalf("load committed receipt: %v", err)
	}
	if err := registry.ValidateReceipt(committed); err != nil {
		t.Fatalf("committed receipt failed canonical validation: %v", err)
	}
	wantBytes := registry.CanonicalReceiptJSON(good.Receipt)
	gotBytes, err := os.ReadFile(CommittedRegistryReceiptPath(home))
	if err != nil {
		t.Fatalf("read committed receipt: %v", err)
	}
	if !bytes.Equal(gotBytes, wantBytes) {
		t.Errorf("committed receipt bytes are not the canonical encoding:\n got %s\nwant %s", gotBytes, wantBytes)
	}

	// A later failing run retains the previously committed receipt verbatim.
	if _, err := ApplyGlobalInstallPlan(home, failingPlan); err == nil {
		t.Fatal("blocked write was accepted on rerun")
	}
	retained, err := os.ReadFile(CommittedRegistryReceiptPath(home))
	if err != nil {
		t.Fatalf("read retained receipt: %v", err)
	}
	if !bytes.Equal(retained, wantBytes) {
		t.Error("failed rerun replaced or corrupted the previously committed receipt")
	}
}

// ---------------------------------------------------------------------------
// Wiring: custom skills and adapter layout reach the renderer (WU-12/14/15)
// ---------------------------------------------------------------------------

type stubSkillLayout struct{}

func (stubSkillLayout) SkillDestinations(skill skillcore.Skill) []string {
	return []string{"skills-custom/" + string(skill.ID) + "/SKILL.md"}
}

var _ agents.SkillLayoutProvider = stubSkillLayout{}

type stubBundleRenderer struct{}

func (stubBundleRenderer) Target() renderers.TargetID { return "stub" }

func (stubBundleRenderer) Render(_ context.Context, resolved renderers.ResolvedWorkflow) (renderers.Bundle, error) {
	assets := []renderers.Asset{{
		Path: "STUB.md", SemanticID: "stub/instruction/root", Kind: renderers.AssetInstruction,
		Content: []byte("# Stub bundle\n"), Mode: 0o644,
	}}
	lowered, err := renderers.LowerCustomSkills(resolved.SkillLayout, resolved.Composition.CustomSkills)
	if err != nil {
		return renderers.Bundle{}, err
	}
	return renderers.Bundle{Assets: append(assets, lowered...)}, nil
}

var _ renderers.Renderer = stubBundleRenderer{}

func wiringCompilation(t *testing.T, overlay []prompt.ComposedCustomSkill) compiler.Result {
	t.Helper()
	return compiler.Result{
		Normalized: compiler.NormalizedInput{Target: "stub"},
		Composition: prompt.CompositionResult{
			CustomSkills: overlay,
		},
		Fingerprint: "fixture-wiring-1",
	}
}

func TestBundle_CompileWiresCustomSkillsThroughAdapterLayout(t *testing.T) {
	document := "---\nname: review-checklist\ndescription: Checklist discipline\n---\n\nFollow the checklist.\n"
	typed := skillcore.Skill{
		ID: model.SkillID("review-checklist"), Content: []byte(document),
		ContentSHA256: ir.FingerprintContent([]byte(document)), Origin: skillcore.OriginCustom,
	}
	overlay := []prompt.ComposedCustomSkill{{
		ID: "review-checklist", ContentSHA256: typed.ContentSHA256, Path: "skills/review-checklist/SKILL.md",
	}}

	t.Run("lowers the overlay to the adapter-declared destination", func(t *testing.T) {
		compiled, err := CompileInjectionBundle(context.Background(), BundleCompilationInput{
			Compilation:       wiringCompilation(t, overlay),
			Renderer:          stubBundleRenderer{},
			ProfileOverride:   "portable-flat",
			CustomSkills:      []skillcore.Skill{typed},
			SkillLayout:       stubSkillLayout{},
			AllowedAssetKinds: []renderers.AssetKind{renderers.AssetInstruction, renderers.AssetSkill},
		})
		if err != nil {
			t.Fatalf("compile bundle: %v", err)
		}
		var skill *renderers.Asset
		for index := range compiled.Bundle.Assets {
			if compiled.Bundle.Assets[index].SemanticID == "asset/skill/review-checklist" {
				skill = &compiled.Bundle.Assets[index]
			}
		}
		if skill == nil {
			t.Fatalf("compiled bundle has no lowered custom skill: %+v", compiled.Bundle.Assets)
		}
		if skill.Path != "skills-custom/review-checklist/SKILL.md" {
			t.Errorf("lowered path = %q, want the adapter-declared destination", skill.Path)
		}
		if !bytes.Equal(skill.Content, []byte(document)) {
			t.Errorf("lowered content was rewritten:\n got %q\nwant %q", skill.Content, document)
		}
	})

	t.Run("overlay without a declared layout fails closed", func(t *testing.T) {
		_, err := CompileInjectionBundle(context.Background(), BundleCompilationInput{
			Compilation:       wiringCompilation(t, overlay),
			Renderer:          stubBundleRenderer{},
			ProfileOverride:   "portable-flat",
			CustomSkills:      []skillcore.Skill{typed},
			AllowedAssetKinds: []renderers.AssetKind{renderers.AssetInstruction, renderers.AssetSkill},
		})
		var validationErr *renderers.ValidationError
		if !errors.As(err, &validationErr) || validationErr.ID != renderers.ErrorUndeclaredSkillLayout {
			t.Fatalf("error = %v, want a renderers validation error %q", err, renderers.ErrorUndeclaredSkillLayout)
		}
	})

	t.Run("composed digest disagreement fails closed", func(t *testing.T) {
		disagree := []prompt.ComposedCustomSkill{{
			ID: "review-checklist", ContentSHA256: strings.Repeat("0", 64), Path: "skills/review-checklist/SKILL.md",
		}}
		_, err := CompileInjectionBundle(context.Background(), BundleCompilationInput{
			Compilation:       wiringCompilation(t, disagree),
			Renderer:          stubBundleRenderer{},
			ProfileOverride:   "portable-flat",
			CustomSkills:      []skillcore.Skill{typed},
			SkillLayout:       stubSkillLayout{},
			AllowedAssetKinds: []renderers.AssetKind{renderers.AssetInstruction, renderers.AssetSkill},
		})
		if err == nil {
			t.Fatal("composed overlay digest disagreeing with the typed record was accepted")
		}
	})

	t.Run("typed record missing from the composition fails closed", func(t *testing.T) {
		_, err := CompileInjectionBundle(context.Background(), BundleCompilationInput{
			Compilation:       wiringCompilation(t, nil),
			Renderer:          stubBundleRenderer{},
			ProfileOverride:   "portable-flat",
			CustomSkills:      []skillcore.Skill{typed},
			SkillLayout:       stubSkillLayout{},
			AllowedAssetKinds: []renderers.AssetKind{renderers.AssetInstruction, renderers.AssetSkill},
		})
		if err == nil {
			t.Fatal("typed custom skill absent from the composed overlay was accepted")
		}
	})
}

// ---------------------------------------------------------------------------
// Containment: reparse/symlink ancestors beneath homeDir are rejected
// ---------------------------------------------------------------------------

// Containment diagnostic rules under test, pinned as stable identifiers so
// the oracle is independent of the production constants.
const (
	testRulePlanUnsafeAncestor  = "bundle.plan.unsafe_ancestor"
	testRuleApplyUnsafeAncestor = "bundle.apply.unsafe_ancestor"
)

// linkDirTo makes link resolve to target: a plain directory symlink on unix,
// an unprivileged junction (a reparse point os.Lstat reports without
// fs.ModeDir) on Windows. Both carry path resolution outside the link.
func linkDirTo(t *testing.T, target, link string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		out, err := exec.Command("cmd", "/c", "mklink", "/J", link, target).CombinedOutput()
		if err != nil {
			t.Skipf("cannot create junction %s -> %s (mklink /J: %v: %s)", link, target, err, strings.TrimSpace(string(out)))
		}
		return
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("cannot create symlink %s -> %s: %v", link, target, err)
	}
}

func diagnosticsHaveRule(t *testing.T, diags registry.Diagnostics, rule string) {
	t.Helper()
	for _, diagnostic := range diags {
		if diagnostic.Rule == rule {
			return
		}
	}
	t.Errorf("diagnostics %v lack rule %q", diags, rule)
}

func writeOutsideTwin(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("prepare outside twin parent: %v", err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write outside twin: %v", err)
	}
}

func assertOutsideBytesUnchanged(t *testing.T, path string, want []byte, context string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, want) {
		t.Errorf("outside twin changed or unreadable after %s (err = %v, got %q, want %q)", context, err, got, want)
	}
}

func assertNoCommittedReceipt(t *testing.T, home string, context string) {
	t.Helper()
	if _, statErr := os.Lstat(CommittedRegistryReceiptPath(home)); !errors.Is(statErr, fs.ErrNotExist) {
		t.Errorf("receipt committed despite %s (statErr = %v)", context, statErr)
	}
}

// The plan must refuse to observe managed outputs through a reparse or
// symlink ancestor beneath homeDir: an outside twin holding the exact
// desired bytes would otherwise be reported as converged (or silently
// deleted when prior-managed), pulling bytes outside the home directory
// into the managed lifecycle.
func TestBundle_PlanRejectsReparseAncestor(t *testing.T) {
	t.Run("desired output behind a reparse ancestor is rejected", func(t *testing.T) {
		home := t.TempDir()
		outside := t.TempDir()
		outsideSkill := filepath.Join(outside, "review-checklist", "SKILL.md")
		writeOutsideTwin(t, outsideSkill, customContent)
		opencodeConfig := filepath.Join(home, ".config", "opencode")
		if err := os.MkdirAll(opencodeConfig, 0o755); err != nil {
			t.Fatalf("prepare home config: %v", err)
		}
		linkDirTo(t, outside, filepath.Join(opencodeConfig, "skills"))

		plan, diags := BuildGlobalInstallPlan(GlobalInstallPlanRequest{
			HomeDir: home,
			Bundles: []CompiledInjectionBundle{overlayBundle()},
			Receipt: sealedBundleReceipt(t, "review-checklist"),
		})
		if len(diags) == 0 {
			t.Fatalf("plan accepted a reparse ancestor escaping homeDir: shared=%+v adapters=%+v", plan.Shared, plan.Adapters)
		}
		diagnosticsHaveRule(t, diags, testRulePlanUnsafeAncestor)
		// Zero mutation: the outside twin keeps its bytes and nothing was committed.
		assertOutsideBytesUnchanged(t, outsideSkill, customContent, "planning")
		assertNoCommittedReceipt(t, home, "planning rejection")
	})

	t.Run("stale-managed observation behind a reparse ancestor is rejected", func(t *testing.T) {
		home := t.TempDir()
		outside := t.TempDir()
		outsideSkill := filepath.Join(outside, "review-checklist", "SKILL.md")
		writeOutsideTwin(t, outsideSkill, customContent)
		opencodeConfig := filepath.Join(home, ".config", "opencode")
		if err := os.MkdirAll(opencodeConfig, 0o755); err != nil {
			t.Fatalf("prepare home config: %v", err)
		}
		linkDirTo(t, outside, filepath.Join(opencodeConfig, "skills"))

		plan, diags := BuildGlobalInstallPlan(GlobalInstallPlanRequest{
			HomeDir:           home,
			Bundles:           []CompiledInjectionBundle{baselineOnlyBundle()},
			PriorManagedPaths: []string{bundleCustomPath},
			Receipt:           sealedBundleReceipt(t),
		})
		if len(diags) == 0 {
			t.Fatalf("plan accepted a stale delete behind a reparse ancestor: deletes=%+v", plan.Shared.Deletes)
		}
		diagnosticsHaveRule(t, diags, testRulePlanUnsafeAncestor)
		assertOutsideBytesUnchanged(t, outsideSkill, customContent, "planning")
	})
}

// A substitution that turns an ancestor into a reparse point after the plan
// was built must stop the apply before os.Remove ever resolves through it,
// and the bytes on the other side must be untouched.
func TestBundle_DeleteSubstitutionRejectedBeforeRemove(t *testing.T) {
	home := t.TempDir()
	outside := t.TempDir()
	skillsDir := filepath.Join(home, ".config", "opencode", "skills")
	skillDir := filepath.Join(skillsDir, "review-checklist")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("prepare managed skill dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), customContent, 0o644); err != nil {
		t.Fatalf("write managed skill: %v", err)
	}

	plan := requirePlan(t, GlobalInstallPlanRequest{
		HomeDir:           home,
		Bundles:           []CompiledInjectionBundle{baselineOnlyBundle()},
		PriorManagedPaths: []string{bundleCustomPath},
		Receipt:           sealedBundleReceipt(t),
	})
	if got := opPaths(plan.Shared.Deletes); !equalStrings(got, []string{bundleCustomPath}) {
		t.Fatalf("planned stale deletes = %v", got)
	}

	outsideSecret := []byte("outside-secret\n")
	outsideSkill := filepath.Join(outside, "review-checklist", "SKILL.md")
	writeOutsideTwin(t, outsideSkill, outsideSecret)
	if err := os.RemoveAll(skillsDir); err != nil {
		t.Fatalf("remove real skills tree: %v", err)
	}
	linkDirTo(t, outside, skillsDir)

	result, err := ApplyGlobalInstallPlan(home, plan)
	if err == nil {
		t.Fatalf("delete through a substituted reparse ancestor claimed success: %+v", result)
	}
	var installErr *registry.InstallError
	if !errors.As(err, &installErr) {
		t.Fatalf("error is %T (%v), want *registry.InstallError", err, err)
	}
	if installErr.Primary.Rule != testRuleApplyUnsafeAncestor {
		t.Errorf("primary rule = %q, want %q", installErr.Primary.Rule, testRuleApplyUnsafeAncestor)
	}
	assertOutsideBytesUnchanged(t, outsideSkill, outsideSecret, "apply rejection")
	assertNoCommittedReceipt(t, home, "containment rejection")
}

// The same substitution performed between the snapshot and the delete must
// be rejected immediately before os.Remove, and the rollback restoration
// must refuse to write through the substituted ancestor instead of
// reporting a false success.
func TestBundle_PostSnapshotDeleteSubstitutionRejectedBeforeRemove(t *testing.T) {
	home := t.TempDir()
	outside := t.TempDir()
	skillsDir := filepath.Join(home, ".config", "opencode", "skills")
	skillDir := filepath.Join(skillsDir, "review-checklist")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("prepare managed skill dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), customContent, 0o644); err != nil {
		t.Fatalf("write managed skill: %v", err)
	}

	plan := requirePlan(t, GlobalInstallPlanRequest{
		HomeDir:           home,
		Bundles:           []CompiledInjectionBundle{baselineOnlyBundle()},
		PriorManagedPaths: []string{bundleCustomPath},
		Receipt:           sealedBundleReceipt(t),
	})
	if got := opPaths(plan.Shared.Deletes); !equalStrings(got, []string{bundleCustomPath}) {
		t.Fatalf("planned stale deletes = %v", got)
	}

	outsideSecret := []byte("outside-secret\n")
	outsideSkill := filepath.Join(outside, "review-checklist", "SKILL.md")
	writeOutsideTwin(t, outsideSkill, outsideSecret)

	t.Cleanup(func() { bundleOpFaultHook = nil })
	substituted := false
	bundleOpFaultHook = func(op BundleOperation) error {
		if op.Path != bundleCustomPath || substituted {
			return nil
		}
		substituted = true
		if err := os.RemoveAll(skillsDir); err != nil {
			t.Fatalf("remove real skills tree inside hook: %v", err)
		}
		linkDirTo(t, outside, skillsDir)
		return nil
	}

	result, err := ApplyGlobalInstallPlan(home, plan)
	if err == nil {
		t.Fatalf("delete through a post-snapshot substituted reparse ancestor claimed success: %+v", result)
	}
	var installErr *registry.InstallError
	if !errors.As(err, &installErr) {
		t.Fatalf("error is %T (%v), want *registry.InstallError", err, err)
	}
	if installErr.Primary.Rule != testRuleApplyUnsafeAncestor {
		t.Errorf("primary rule = %q, want %q", installErr.Primary.Rule, testRuleApplyUnsafeAncestor)
	}
	// os.Remove never resolved through the substituted ancestor.
	assertOutsideBytesUnchanged(t, outsideSkill, outsideSecret, "delete rejection")
	// Restoration refused to converge through the substituted ancestor, so
	// the failure is reported as a rollback residual, never a success.
	if installErr.Rollback == nil {
		t.Fatal("restoration through the substituted ancestor was reported as converged")
	}
	if !strings.Contains(installErr.Rollback.Cause.Error(), bundleCustomPath) {
		t.Errorf("rollback cause %q does not list %q", installErr.Rollback.Cause, bundleCustomPath)
	}
	assertNoCommittedReceipt(t, home, "containment rejection")
}

// Verification observations must not resolve through a reparse ancestor
// either: after the writes land, a substituted ancestor of an already
// written output makes verification fail closed, and the rollback
// restoration must leave the outside bytes untouched instead of rewriting
// them through the substituted ancestor.
func TestBundle_VerifyRejectsReparseAncestorAfterWrites(t *testing.T) {
	home := t.TempDir()
	outside := t.TempDir()
	configDir := filepath.Join(home, ".config")
	managedBaseline := filepath.Join(configDir, "opencode", "AGENTS.md")
	if err := os.MkdirAll(filepath.Dir(managedBaseline), 0o755); err != nil {
		t.Fatalf("prepare managed baseline parent: %v", err)
	}
	if err := os.WriteFile(managedBaseline, []byte("old managed bytes\n"), 0o644); err != nil {
		t.Fatalf("write managed baseline: %v", err)
	}

	desired := CompiledInjectionBundle{
		Target:      "opencode",
		Profile:     ProfilePortableFlat,
		Fingerprint: "fixture-reparse-verify-1",
		Bundle: renderers.Bundle{Assets: []renderers.Asset{
			bundleAsset(bundleBaselinePath, "asset/opencode/instruction/root", renderers.AssetInstruction, baselineContent, 0o644),
			bundleAsset("workspace/extra/out.txt", "asset/opencode/instruction/extra", renderers.AssetInstruction, []byte("extra\n"), 0o644),
		}},
	}
	plan := requirePlan(t, GlobalInstallPlanRequest{
		HomeDir:           home,
		Bundles:           []CompiledInjectionBundle{desired},
		PriorManagedPaths: []string{bundleBaselinePath},
		Receipt:           sealedBundleReceipt(t),
	})

	outsideSecret := []byte("outside-secret\n")
	outsideTwin := filepath.Join(outside, "opencode", "AGENTS.md")
	writeOutsideTwin(t, outsideTwin, outsideSecret)

	t.Cleanup(func() { bundleOpFaultHook = nil })
	bundleOpFaultHook = func(op BundleOperation) error {
		if op.Path != "workspace/extra/out.txt" {
			return nil
		}
		// The baseline write already happened through the real .config; swap
		// it for a junction so verification would have to observe the
		// outside twin.
		if err := os.RemoveAll(configDir); err != nil {
			t.Fatalf("remove real config tree inside hook: %v", err)
		}
		linkDirTo(t, outside, configDir)
		return nil
	}

	result, err := ApplyGlobalInstallPlan(home, plan)
	if err == nil {
		t.Fatalf("verification through a substituted reparse ancestor claimed success: %+v", result)
	}
	var installErr *registry.InstallError
	if !errors.As(err, &installErr) {
		t.Fatalf("error is %T (%v), want *registry.InstallError", err, err)
	}
	if installErr.Primary.Stage != registry.StageVerify || installErr.Primary.Rule != RuleVerifyFailed {
		t.Errorf("primary diagnostic = %s/%s, want %s/%s",
			installErr.Primary.Stage, installErr.Primary.Rule, registry.StageVerify, RuleVerifyFailed)
	}
	// The restoration refused to write the baseline back through the
	// junction, so the residual is reported instead of a false success.
	if installErr.Rollback == nil {
		t.Fatal("restoration through the substituted ancestor was reported as converged")
	}
	if !strings.Contains(installErr.Rollback.Cause.Error(), bundleBaselinePath) {
		t.Errorf("rollback cause %q does not list %q", installErr.Rollback.Cause, bundleBaselinePath)
	}
	assertOutsideBytesUnchanged(t, outsideTwin, outsideSecret, "verify rejection and rollback")
	assertNoCommittedReceipt(t, home, "containment rejection")
}

// ---------------------------------------------------------------------------
// Small helpers
// ---------------------------------------------------------------------------

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for index := range got {
		if got[index] != want[index] {
			return false
		}
	}
	return true
}
