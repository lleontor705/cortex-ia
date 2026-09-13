package delegation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestReviewFreshness(t *testing.T) {
	ctx := context.Background()
	store, workspace := newReviseTestStore(t)

	// 1. Setup workspace and contract
	pinRel := "openspec/changes/c/plan.md"
	pinAbs := filepath.Join(workspace, filepath.FromSlash(pinRel))
	if err := os.MkdirAll(filepath.Dir(pinAbs), 0o700); err != nil {
		t.Fatal(err)
	}
	pinBytes := []byte("plan-content")
	if err := os.WriteFile(pinAbs, pinBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	pinHash := sha256.Sum256(pinBytes)

	srcRel := "src/logic.go"
	srcAbs := filepath.Join(workspace, filepath.FromSlash(srcRel))
	if err := os.MkdirAll(filepath.Dir(srcAbs), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(srcAbs, []byte("package logic"), 0o600); err != nil {
		t.Fatal(err)
	}

	contract := &SDDContract{
		Version:   1,
		Workflow:  "sdd-lite",
		ChangeID:  "c",
		SpecPlane: "hybrid",
		Pins: []ContractPin{
			{Transport: "workspace_file", Project: workspace, Locator: pinRel, SHA256: hex.EncodeToString(pinHash[:])},
		},
		RequirementIDs: []string{"REQ-REV-001"},
	}

	item, err := store.CreateWorkInBoardWithDefinition(ctx, "revise-board", "task-rev", "Freshness Task", nil, WorkDefinition{
		Project: workspace, Objective: "objective", Acceptance: "acceptance", Verification: "verification", AllowedFiles: []string{srcRel}, Contract: contract,
	})
	if err != nil {
		t.Fatalf("create work item: %v", err)
	}

	// 2. Claim task
	claim, err := store.ClaimWork(ctx, item.ID, "implement-agent-1", 5*time.Minute)
	if err != nil {
		t.Fatalf("claim work: %v", err)
	}

	// 3. Transition to in_review (captures binding)
	inReview, err := store.TransitionWork(ctx, item.ID, claim.Token, claim.Revision, WorkInReview)
	if err != nil {
		t.Fatalf("transition in_review: %v", err)
	}

	// 4. Test self-approval denial
	_, err = store.ApproveWork(ctx, item.ID, "implement-agent-1", "PASS", "evidence", inReview.Revision)
	if err == nil || !errors.Is(err, ErrWorkConflict) {
		t.Fatalf("expected ErrWorkConflict on self-approval, got: %v", err)
	}

	// 5. Test missing evidence on PASS
	_, err = store.ApproveWork(ctx, item.ID, "independent-rev-1", "PASS", "", inReview.Revision)
	if err == nil {
		t.Fatal("expected error on PASS with empty evidence")
	}

	// 6. Test file modification between transition and approval (binding mismatch)
	if err := os.WriteFile(srcAbs, []byte("package logic // modified"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = store.ApproveWork(ctx, item.ID, "independent-rev-1", "PASS", "evidence", inReview.Revision)
	if err == nil || err.Error() != "SDD review binding changed; fresh review required" {
		t.Fatalf("expected binding changed error, got: %v", err)
	}

	// Restore file so binding matches again
	if err := os.WriteFile(srcAbs, []byte("package logic"), 0o600); err != nil {
		t.Fatal(err)
	}

	// 7. Test stale revision rejection
	_, err = store.ApproveWork(ctx, item.ID, "independent-rev-1", "PASS", "evidence", inReview.Revision+99)
	if err == nil || !errors.Is(err, ErrWorkConflict) {
		t.Fatalf("expected ErrWorkConflict on stale revision, got: %v", err)
	}

	// 8. Legitimate approval succeeds and marks done
	approved, err := store.ApproveWork(ctx, item.ID, "independent-rev-1", "PASS", "evidence-ok", inReview.Revision)
	if err != nil {
		t.Fatalf("approve work: %v", err)
	}
	if approved.Verdict != "PASS" {
		t.Fatalf("expected PASS verdict, got %s", approved.Verdict)
	}
	doneItem, err := store.GetWork(ctx, item.ID)
	if err != nil || doneItem.Status != WorkDone {
		t.Fatalf("expected task status done, got %v (err: %v)", doneItem.Status, err)
	}

	// 9. Test review refresh after completion
	refreshed, err := store.RefreshWorkReview(ctx, item.ID, doneItem.Revision)
	if err != nil {
		t.Fatalf("refresh work review: %v", err)
	}
	if refreshed.Status != WorkInReview || refreshed.Revision != doneItem.Revision+1 {
		t.Fatalf("unexpected refreshed state: %+v", refreshed)
	}

	// Re-approve refreshed task with new review
	reapproved, err := store.ApproveWork(ctx, item.ID, "independent-rev-2", "PASS", "evidence-refreshed", refreshed.Revision)
	if err != nil {
		t.Fatalf("re-approve refreshed task: %v", err)
	}
	if reapproved.Verdict != "PASS" {
		t.Fatalf("expected PASS on reapproval, got %s", reapproved.Verdict)
	}
}
