package delegation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func drTestStore(t *testing.T) (*Store, string) {
	t.Helper()
	t.Setenv("CORTEX_IA_HOME", t.TempDir())
	workspace := t.TempDir()
	store, err := OpenStore(filepath.Join(t.TempDir(), "delegation.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if _, err := store.CreateBoard(context.Background(), "dr-board", "Direct Rebind Board", ""); err != nil {
		t.Fatal(err)
	}
	return store, workspace
}

func drWriteFile(t *testing.T, workspace, rel, content string) {
	t.Helper()
	abs := filepath.Join(workspace, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func drDirectTask(t *testing.T, ctx context.Context, store *Store, workspace, id, rel, content string) WorkItem {
	t.Helper()
	drWriteFile(t, workspace, rel, content)
	item, err := store.CreateWorkInBoardWithDefinition(ctx, "dr-board", id, id+" title", nil, WorkDefinition{
		Project: workspace, Objective: "objective", Acceptance: "acceptance", Verification: "verification", AllowedFiles: []string{rel},
	})
	if err != nil {
		t.Fatalf("create direct task %s: %v", id, err)
	}
	if item.Contract != nil {
		t.Fatalf("direct task %s unexpectedly carries an SDD contract", id)
	}
	return item
}

func drDoneDirectTask(t *testing.T, ctx context.Context, store *Store, id string) WorkItem {
	t.Helper()
	claim, err := store.ClaimWork(ctx, id, "impl-owner", time.Minute)
	if err != nil {
		t.Fatalf("claim %s: %v", id, err)
	}
	if _, err := store.TransitionWork(ctx, id, claim.Token, claim.Revision, WorkInReview); err != nil {
		t.Fatalf("transition %s in_review: %v", id, err)
	}
	if _, err := store.ApproveWork(ctx, id, "independent-reviewer", "PASS", "verified", 0); err != nil {
		t.Fatalf("approve direct task %s: %v", id, err)
	}
	done, err := store.GetWork(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if done.Status != WorkDone {
		t.Fatalf("task %s status = %s, want %s", id, done.Status, WorkDone)
	}
	return done
}

func drTaskState(t *testing.T, store *Store, id string) string {
	t.Helper()
	var status string
	var revision, reviews, approvals int64
	if err := store.db.QueryRow(`SELECT status,revision,
		(SELECT COUNT(*) FROM work_reviews WHERE item_id=work_items.id),
		(SELECT COUNT(*) FROM work_approvals WHERE item_id=work_items.id) FROM work_items WHERE id=?`, id).
		Scan(&status, &revision, &reviews, &approvals); err != nil {
		t.Fatal(err)
	}
	return status + "|" + strconv.FormatInt(revision, 10) + "|" + strconv.FormatInt(reviews, 10) + "|" + strconv.FormatInt(approvals, 10)
}

func TestDirectTaskArchivePassesOnlyWithCurrentFingerprintBinding(t *testing.T) {
	ctx := context.Background()
	store, workspace := drTestStore(t)
	item := drDirectTask(t, ctx, store, workspace, "direct-1", "src/impl.go", "package impl")
	done := drDoneDirectTask(t, ctx, store, item.ID)

	approvals, err := store.ListWorkApprovals(ctx, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(approvals) != 1 || approvals[0].Binding == nil || approvals[0].Binding.Contract != nil ||
		len(approvals[0].Binding.DefinitionSHA256) != 64 || len(approvals[0].Binding.ChangeSHA256) != 64 {
		t.Fatalf("direct approval must carry a fingerprints-only binding with complete digests: %+v", approvals)
	}
	binding, err := store.ValidateArchiveBoard(ctx, "dr-board", done.Workspace, "c", "sdd-lite", "hybrid")
	if err != nil || len(binding.TaskIDs) != 1 || binding.TaskIDs[0] != item.ID {
		t.Fatalf("current direct fingerprints must pass the archive gate: %v %+v", err, binding)
	}
	drWriteFile(t, workspace, "src/impl.go", "package impl // drifted")
	if _, err := store.ValidateArchiveBoard(ctx, "dr-board", done.Workspace, "c", "sdd-lite", "hybrid"); err == nil {
		t.Fatal("stale direct fingerprints must not pass the archive gate")
	}
}

func TestDirectTaskReviewRefreshRebindsFingerprintOnlyBinding(t *testing.T) {
	ctx := context.Background()
	store, workspace := drTestStore(t)
	item := drDirectTask(t, ctx, store, workspace, "direct-2", "src/other.go", "package other")
	done := drDoneDirectTask(t, ctx, store, item.ID)
	if done.LatestApproval == nil || done.LatestApproval.Binding == nil {
		t.Fatalf("completed direct task lost its approval binding: %+v", done.LatestApproval)
	}
	stale := done.LatestApproval.Binding.ChangeSHA256

	drWriteFile(t, workspace, "src/other.go", "package other // drifted")
	if _, err := store.ValidateArchiveBoard(ctx, "dr-board", done.Workspace, "c", "sdd-lite", "hybrid"); err == nil {
		t.Fatal("drifted direct task must not pass the archive gate")
	}
	before := drTaskState(t, store, item.ID)
	if _, err := store.RefreshWorkReview(ctx, item.ID, done.Revision+5); !errors.Is(err, ErrWorkConflict) {
		t.Fatalf("stale revision refresh must be refused, got %v", err)
	}
	if after := drTaskState(t, store, item.ID); after != before {
		t.Fatalf("refused refresh left side effects: %s -> %s", before, after)
	}
	refreshed, err := store.RefreshWorkReview(ctx, item.ID, done.Revision)
	if err != nil || refreshed.Status != WorkInReview {
		t.Fatalf("refresh direct review: %v %+v", err, refreshed)
	}
	if refreshed.Review == nil || refreshed.Review.Binding == nil || refreshed.Review.Binding.Contract != nil {
		t.Fatalf("refreshed direct review binding must be fingerprints-only: %+v", refreshed.Review)
	}
	if _, err := store.ApproveWork(ctx, item.ID, "independent-reviewer-2", "PASS", "reverified", refreshed.Revision); err != nil {
		t.Fatalf("re-approve refreshed direct task: %v", err)
	}
	reapproved, err := store.GetWork(ctx, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reapproved.LatestApproval == nil || reapproved.LatestApproval.Binding == nil || reapproved.LatestApproval.Binding.Contract != nil {
		t.Fatalf("fresh direct approval must be fingerprints-only: %+v", reapproved.LatestApproval)
	}
	if reapproved.LatestApproval.Binding.ChangeSHA256 == stale {
		t.Fatal("fresh direct approval did not re-bind to the current file fingerprints")
	}
	if _, err := store.ValidateArchiveBoard(ctx, "dr-board", done.Workspace, "c", "sdd-lite", "hybrid"); err != nil {
		t.Fatalf("re-bound direct task must pass the archive gate: %v", err)
	}
}

func TestLegacyDirectTaskRefreshWritesFingerprintBinding(t *testing.T) {
	ctx := context.Background()
	store, workspace := drTestStore(t)
	item := drDirectTask(t, ctx, store, workspace, "direct-legacy", "src/legacy.go", "package legacy")
	if _, err := store.db.Exec(`INSERT INTO work_approvals(item_id,reviewer,verdict,evidence,created_at) VALUES(?,?,?,?,?)`,
		item.ID, "legacy-reviewer", "PASS", "legacy evidence", "2026-01-01T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(`UPDATE work_items SET status='done',revision=revision+1 WHERE id=?`, item.ID); err != nil {
		t.Fatal(err)
	}
	legacy, err := store.GetWork(ctx, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if legacy.LatestApproval == nil || legacy.LatestApproval.Binding != nil {
		t.Fatalf("legacy fixture must carry an approval without a binding: %+v", legacy.LatestApproval)
	}
	if _, err := store.ValidateArchiveBoard(ctx, "dr-board", legacy.Workspace, "c", "sdd-lite", "hybrid"); err == nil {
		t.Fatal("unbound legacy direct task must not pass the archive gate")
	}
	refreshed, err := store.RefreshWorkReview(ctx, item.ID, legacy.Revision)
	if err != nil || refreshed.Status != WorkInReview || refreshed.Review == nil || refreshed.Review.Binding == nil {
		t.Fatalf("legacy direct refresh must reopen with a fingerprints-only binding: %v %+v", err, refreshed)
	}
	if refreshed.Review.Binding.Contract != nil {
		t.Fatal("legacy direct refresh must not invent contract pins")
	}
	if _, err := store.ApproveWork(ctx, item.ID, "independent-reviewer", "PASS", "reverified", refreshed.Revision); err != nil {
		t.Fatalf("fresh approve of legacy direct task: %v", err)
	}
	if _, err := store.ValidateArchiveBoard(ctx, "dr-board", legacy.Workspace, "c", "sdd-lite", "hybrid"); err != nil {
		t.Fatalf("re-bound legacy direct task must pass the archive gate: %v", err)
	}
}

func TestSDDContractTaskBindingRulesUnchanged(t *testing.T) {
	ctx := context.Background()
	store, workspace := drTestStore(t)
	pinRel := "openspec/changes/c/plan.md"
	drWriteFile(t, workspace, pinRel, "plan-content")
	pinHash := sha256.Sum256([]byte("plan-content"))
	contract := &SDDContract{
		Version: 1, Workflow: "sdd-lite", ChangeID: "c", SpecPlane: "hybrid",
		Pins: []ContractPin{
			{Transport: "workspace_file", Project: workspace, Locator: pinRel, SHA256: hex.EncodeToString(pinHash[:])},
		},
		RequirementIDs: []string{"REQ-WA-004"},
	}
	drWriteFile(t, workspace, "src/sdd.go", "package sdd")
	item, err := store.CreateWorkInBoardWithDefinition(ctx, "dr-board", "sdd-1", "SDD title", nil, WorkDefinition{
		Project: workspace, Objective: "objective", Acceptance: "acceptance", Verification: "verification", AllowedFiles: []string{"src/sdd.go"}, Contract: contract,
	})
	if err != nil {
		t.Fatalf("create SDD task: %v", err)
	}
	claim, err := store.ClaimWork(ctx, item.ID, "impl-owner", time.Minute)
	if err != nil {
		t.Fatalf("claim SDD task: %v", err)
	}
	inReview, err := store.TransitionWork(ctx, item.ID, claim.Token, claim.Revision, WorkInReview)
	if err != nil {
		t.Fatalf("transition SDD task in_review: %v", err)
	}
	if _, err := store.ApproveWork(ctx, item.ID, "independent-reviewer", "PASS", "evidence", 0); err == nil || !strings.Contains(err.Error(), "explicit positive revision") {
		t.Fatalf("SDD approval without an explicit revision must stay refused, got %v", err)
	}
	if _, err := store.ApproveWork(ctx, item.ID, "independent-reviewer", "PASS", "evidence", inReview.Revision); err != nil {
		t.Fatalf("approve SDD task: %v", err)
	}
	approvals, err := store.ListWorkApprovals(ctx, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(approvals) != 1 || approvals[0].Binding == nil || approvals[0].Binding.Contract == nil {
		t.Fatalf("SDD approval must stay pin-bound with a single approval record: %+v", approvals)
	}
	if len(approvals[0].Binding.Contract.Pins) != 1 || approvals[0].Binding.Contract.Pins[0].Locator != pinRel {
		t.Fatalf("SDD approval binding lost its contract pins: %+v", approvals[0].Binding.Contract)
	}
	done, err := store.GetWork(ctx, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	binding, err := store.ValidateArchiveBoard(ctx, "dr-board", done.Workspace, "c", "sdd-lite", "hybrid")
	if err != nil || len(binding.TaskIDs) != 1 || binding.TaskIDs[0] != item.ID {
		t.Fatalf("SDD task archive gate regressed: %v %+v", err, binding)
	}
}
