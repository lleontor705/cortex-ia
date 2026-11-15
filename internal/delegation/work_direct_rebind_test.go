package delegation

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
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
	return item
}

// drAssertCurrent reports whether the live task fingerprints still match its approved binding.
func drAssertCurrent(ctx context.Context, store *Store, id string) error {
	fp, err := store.ComputeWorkFingerprint(ctx, id)
	if err != nil {
		return err
	}
	if fp.MatchesApproved == nil || !*fp.MatchesApproved {
		return errors.New("fingerprints do not match the approved binding")
	}
	return nil
}

func currentWorkRevision(t *testing.T, ctx context.Context, store *Store, id string) int64 {
	t.Helper()
	item, err := store.GetWork(ctx, id)
	if err != nil {
		t.Fatalf("get work %s: %v", id, err)
	}
	return item.Revision
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
	if _, err := store.ApproveWork(ctx, id, "independent-reviewer", "PASS", "verified", currentWorkRevision(t, ctx, store, id)); err != nil {
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

func TestDirectTaskApprovalPassesOnlyWithCurrentFingerprintBinding(t *testing.T) {
	ctx := context.Background()
	store, workspace := drTestStore(t)
	item := drDirectTask(t, ctx, store, workspace, "direct-1", "src/impl.go", "package impl")
	drDoneDirectTask(t, ctx, store, item.ID)

	approvals, err := store.ListWorkApprovals(ctx, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(approvals) != 1 || approvals[0].Binding == nil ||
		len(approvals[0].Binding.DefinitionSHA256) != 64 || len(approvals[0].Binding.ChangeSHA256) != 64 {
		t.Fatalf("direct approval must carry a fingerprints-only binding with complete digests: %+v", approvals)
	}
	if err := drAssertCurrent(ctx, store, item.ID); err != nil {
		t.Fatalf("current direct fingerprints must match the approval: %v", err)
	}
	drWriteFile(t, workspace, "src/impl.go", "package impl // drifted")
	if err := drAssertCurrent(ctx, store, item.ID); err == nil {
		t.Fatal("stale direct fingerprints must not match the approval")
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
	if err := drAssertCurrent(ctx, store, item.ID); err == nil {
		t.Fatal("drifted direct task must not match the approval")
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
	if refreshed.Review == nil || refreshed.Review.Binding == nil {
		t.Fatalf("refreshed direct review binding must carry a fingerprint binding: %+v", refreshed.Review)
	}
	if _, err := store.ApproveWork(ctx, item.ID, "independent-reviewer-2", "PASS", "reverified", refreshed.Revision); err != nil {
		t.Fatalf("re-approve refreshed direct task: %v", err)
	}
	reapproved, err := store.GetWork(ctx, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reapproved.LatestApproval == nil || reapproved.LatestApproval.Binding == nil {
		t.Fatalf("fresh direct approval must carry a fingerprint binding: %+v", reapproved.LatestApproval)
	}
	if reapproved.LatestApproval.Binding.ChangeSHA256 == stale {
		t.Fatal("fresh direct approval did not re-bind to the current file fingerprints")
	}
	if err := drAssertCurrent(ctx, store, item.ID); err != nil {
		t.Fatalf("re-bound direct task must match the approval: %v", err)
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
	if err := drAssertCurrent(ctx, store, item.ID); err == nil {
		t.Fatal("unbound legacy direct task must not match an approval")
	}
	refreshed, err := store.RefreshWorkReview(ctx, item.ID, legacy.Revision)
	if err != nil || refreshed.Status != WorkInReview || refreshed.Review == nil || refreshed.Review.Binding == nil {
		t.Fatalf("legacy direct refresh must reopen with a fingerprints-only binding: %v %+v", err, refreshed)
	}
	if _, err := store.ApproveWork(ctx, item.ID, "independent-reviewer", "PASS", "reverified", refreshed.Revision); err != nil {
		t.Fatalf("fresh approve of legacy direct task: %v", err)
	}
	if err := drAssertCurrent(ctx, store, item.ID); err != nil {
		t.Fatalf("re-bound legacy direct task must match the approval: %v", err)
	}
}
