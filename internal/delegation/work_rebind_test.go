package delegation

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestReviseDoneTaskRebindsContractPinsOnly(t *testing.T) {
	ctx := context.Background()
	store, workspace := newReviseTestStore(t)
	pinPath := writeRevisePin(t, workspace, "openspec/changes/c/plan.md", "old")
	item := createReviseTask(t, ctx, store, workspace, "rebind-ok", nil, reviseContract(workspace, pinPath, []byte("old")))
	done := completeReviseTask(t, ctx, store, item.ID)

	writeRevisePin(t, workspace, pinPath, "reconciled")
	rebound, err := store.ReviseWorkDefinition(ctx, pinOnlyRebindPlan(done, reviseContract(workspace, pinPath, []byte("reconciled"))))
	if err != nil {
		t.Fatalf("pin-only rebind on a done task failed: %v", err)
	}
	if rebound.Status != WorkDone || rebound.Revision != done.Revision+1 {
		t.Fatalf("rebind changed status or revision: %+v", rebound)
	}
	if rebound.Title != done.Title || rebound.Objective != done.Objective || rebound.Acceptance != done.Acceptance ||
		rebound.Verification != done.Verification || strings.Join(rebound.AllowedFiles, ",") != strings.Join(done.AllowedFiles, ",") {
		t.Fatalf("frozen definition drifted across a pin-only rebind: %+v", rebound)
	}
	if rebound.Contract.Pins[0].SHA256 == done.Contract.Pins[0].SHA256 {
		t.Fatal("contract pins were not re-bound to the reconciled on-disk digest")
	}
	assertRebindEvent(t, store, item.ID)

	refreshed, err := store.RefreshWorkReview(ctx, item.ID, rebound.Revision)
	if err != nil {
		t.Fatalf("review refresh after a pin-only rebind failed: %v", err)
	}
	if refreshed.Status != WorkInReview {
		t.Fatalf("refreshed status = %s, want %s", refreshed.Status, WorkInReview)
	}
}

func TestReviseDoneTaskRefusesRedefinitionAndUnverifiedPins(t *testing.T) {
	ctx := context.Background()
	store, workspace := newReviseTestStore(t)
	pinPath := writeRevisePin(t, workspace, "openspec/changes/c/plan.md", "old")
	item := createReviseTask(t, ctx, store, workspace, "rebind-guard", nil, reviseContract(workspace, pinPath, []byte("old")))
	done := completeReviseTask(t, ctx, store, item.ID)
	before := snapshotReviseTask(t, store, item.ID)
	writeRevisePin(t, workspace, pinPath, "reconciled")
	live := func() *SDDContract { return reviseContract(workspace, pinPath, []byte("reconciled")) }

	mutations := map[string]func(*WorkRevisionDefinition){
		"title":         func(d *WorkRevisionDefinition) { d.Title = "retitled" },
		"objective":     func(d *WorkRevisionDefinition) { d.Objective = "rewritten" },
		"acceptance":    func(d *WorkRevisionDefinition) { d.Acceptance = "rewritten" },
		"verification":  func(d *WorkRevisionDefinition) { d.Verification = "rewritten" },
		"allowed_files": func(d *WorkRevisionDefinition) { d.AllowedFiles = []string{"src/other.go"} },
	}
	for name, mutate := range mutations {
		plan := pinOnlyRebindPlan(done, live())
		mutate(&plan.Definition)
		if _, err := store.ReviseWorkDefinition(ctx, plan); err == nil || !strings.Contains(err.Error(), "pins only") {
			t.Fatalf("%s change on a done task was not refused: %v", name, err)
		}
	}

	identity := pinOnlyRebindPlan(done, live())
	identity.Definition.Contract.RequirementIDs = []string{"REQ-OTHER-001"}
	if _, err := store.ReviseWorkDefinition(ctx, identity); err == nil {
		t.Fatal("requirement identity change on a done task was not refused")
	}

	unverified := pinOnlyRebindPlan(done, reviseContract(workspace, pinPath, []byte("not-what-is-on-disk")))
	if _, err := store.ReviseWorkDefinition(ctx, unverified); err == nil || !strings.Contains(err.Error(), "fresh contract review") {
		t.Fatalf("pin digest not matching the on-disk file was not refused: %v", err)
	}

	wrongStatus := pinOnlyRebindPlan(done, live())
	wrongStatus.ExpectedStatus = WorkReady
	if _, err := store.ReviseWorkDefinition(ctx, wrongStatus); !errors.Is(err, ErrWorkConflict) {
		t.Fatalf("ready expectation on a done task was not refused: %v", err)
	}

	assertSameSnapshot(t, before, snapshotReviseTask(t, store, item.ID))
}

func completeReviseTask(t *testing.T, ctx context.Context, store *Store, id string) WorkItem {
	t.Helper()
	claim, err := store.ClaimWork(ctx, id, "impl-owner", time.Minute)
	if err != nil {
		t.Fatalf("claim %s: %v", id, err)
	}
	review, err := store.TransitionWork(ctx, id, claim.Token, claim.Revision, WorkInReview)
	if err != nil {
		t.Fatalf("transition %s: %v", id, err)
	}
	if _, err := store.ApproveWork(ctx, id, "independent-reviewer", "PASS", "verified", review.Revision); err != nil {
		t.Fatalf("approve %s: %v", id, err)
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

func pinOnlyRebindPlan(item WorkItem, contract *SDDContract) WorkRevisionPlan {
	return WorkRevisionPlan{
		Version:           1,
		TaskID:            item.ID,
		BoardID:           item.BoardID,
		Project:           item.Workspace,
		ExpectedRevision:  item.Revision,
		ExpectedStatus:    item.Status,
		ExpectedWorkflow:  "sdd-lite",
		ExpectedChangeID:  "c",
		ExpectedSpecPlane: "hybrid",
		Definition: WorkRevisionDefinition{
			Title:        item.Title,
			Objective:    item.Objective,
			Acceptance:   item.Acceptance,
			Verification: item.Verification,
			AllowedFiles: item.AllowedFiles,
			Contract:     contract,
		},
	}
}

func assertRebindEvent(t *testing.T, store *Store, id string) {
	t.Helper()
	var detail string
	if err := store.db.QueryRow(`SELECT detail FROM work_events WHERE item_id=? AND kind='contract_rebind'`, id).Scan(&detail); err != nil {
		t.Fatalf("contract_rebind event missing for %s: %v", id, err)
	}
	if !strings.Contains(detail, "old") || !strings.Contains(detail, "new") {
		t.Fatalf("unexpected contract_rebind audit detail: %s", detail)
	}
}
