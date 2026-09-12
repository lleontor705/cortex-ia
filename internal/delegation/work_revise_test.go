package delegation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestReviseWorkDefinitionHappyReadyAndBacklog(t *testing.T) {
	ctx := context.Background()
	store, workspace := newReviseTestStore(t)
	pinPath := writeRevisePin(t, workspace, "openspec/changes/c/plan.md", "old")
	old := reviseContract(workspace, pinPath, []byte("old"))
	ready := createReviseTask(t, ctx, store, workspace, "ready", nil, old)
	if err := os.WriteFile(filepath.Join(workspace, filepath.FromSlash(pinPath)), []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}

	revised, err := store.ReviseWorkDefinition(ctx, revisePlan(workspace, ready, reviseContract(workspace, pinPath, []byte("new"))))
	if err != nil {
		t.Fatalf("ReviseWorkDefinition ready failed: %v", err)
	}
	if revised.Revision != ready.Revision+1 || revised.Status != WorkReady || revised.Title != "revised title" || revised.Objective != "revised objective" {
		t.Fatalf("unexpected revised ready: %+v", revised)
	}
	if len(revised.Dependencies) != 0 || revised.BoardID != ready.BoardID || revised.Workspace != ready.Workspace {
		t.Fatalf("identity/dependencies changed: %+v", revised)
	}
	assertReviseEvent(t, store, ready.ID)

	pinPath = writeRevisePin(t, workspace, "openspec/changes/d/plan.md", "old-b")
	dep := createReviseTask(t, ctx, store, workspace, "dep", nil, nil)
	backlog := createReviseTask(t, ctx, store, workspace, "backlog", []string{dep.ID}, reviseContract(workspace, pinPath, []byte("old-b")))
	if err := os.WriteFile(filepath.Join(workspace, filepath.FromSlash(pinPath)), []byte("new-b"), 0o600); err != nil {
		t.Fatal(err)
	}
	revised, err = store.ReviseWorkDefinition(ctx, revisePlan(workspace, backlog, reviseContract(workspace, pinPath, []byte("new-b"))))
	if err != nil {
		t.Fatalf("ReviseWorkDefinition backlog failed: %v", err)
	}
	if revised.Status != WorkBacklog || len(revised.Dependencies) != 1 || revised.Dependencies[0] != dep.ID {
		t.Fatalf("backlog/dependencies not preserved: %+v", revised)
	}
}

func TestReviseWorkDefinitionRejectsStaleRevisionAndBadNewPinsWithoutEffects(t *testing.T) {
	ctx := context.Background()
	store, workspace := newReviseTestStore(t)
	pinPath := writeRevisePin(t, workspace, "openspec/changes/c/plan.md", "old")
	item := createReviseTask(t, ctx, store, workspace, "ready", nil, reviseContract(workspace, pinPath, []byte("old")))
	before := snapshotReviseTask(t, store, item.ID)
	plan := revisePlan(workspace, item, reviseContract(workspace, pinPath, []byte("old")))
	plan.ExpectedRevision++
	if _, err := store.ReviseWorkDefinition(ctx, plan); !errors.Is(err, ErrWorkConflict) {
		t.Fatalf("expected stale revision conflict, got %v", err)
	}
	assertSameSnapshot(t, before, snapshotReviseTask(t, store, item.ID))

	if err := os.WriteFile(filepath.Join(workspace, filepath.FromSlash(pinPath)), []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ReviseWorkDefinition(ctx, revisePlan(workspace, item, reviseContract(workspace, pinPath, []byte("wrong")))); err == nil || !strings.Contains(err.Error(), "fresh contract review") {
		t.Fatalf("expected bad pin failure, got %v", err)
	}
	assertSameSnapshot(t, before, snapshotReviseTask(t, store, item.ID))
}

func TestReviseWorkDefinitionRejectsLifecycleIdentityAndSDDDrop(t *testing.T) {
	ctx := context.Background()
	store, workspace := newReviseTestStore(t)
	pinPath := writeRevisePin(t, workspace, "openspec/changes/c/plan.md", "old")
	item := createReviseTask(t, ctx, store, workspace, "ready", nil, reviseContract(workspace, pinPath, []byte("old")))
	before := snapshotReviseTask(t, store, item.ID)

	claimed, err := store.ClaimWork(ctx, item.ID, "agent", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.ReviseWorkDefinition(ctx, revisePlan(workspace, item, item.Contract)); !errors.Is(err, ErrWorkConflict) {
		t.Fatalf("expected active lifecycle conflict, got %v", err)
	}
	_, _ = store.TransitionWork(ctx, item.ID, claimed.Token, claimed.Revision, WorkBlocked)

	item2 := createReviseTask(t, ctx, store, workspace, "ready-2", nil, reviseContract(workspace, pinPath, []byte("old")))
	bad := revisePlan(workspace, item2, reviseContract(workspace, pinPath, []byte("old")))
	bad.BoardID = "other-board"
	if _, err = store.ReviseWorkDefinition(ctx, bad); !errors.Is(err, ErrWorkConflict) {
		t.Fatalf("expected board identity conflict, got %v", err)
	}
	bad = revisePlan(workspace, item2, nil)
	if _, err = store.ReviseWorkDefinition(ctx, bad); err == nil || !strings.Contains(err.Error(), "SDD binding cannot be removed") {
		t.Fatalf("expected SDD drop rejection, got %v", err)
	}
	bad = revisePlan(workspace, item2, reviseContract(workspace, pinPath, []byte("old")))
	bad.Definition.Contract.ChangeID = "replacement"
	if _, err = store.ReviseWorkDefinition(ctx, bad); err == nil || !strings.Contains(err.Error(), "immutable") {
		t.Fatalf("expected SDD identity rejection, got %v", err)
	}
	assertSameSnapshot(t, before, before)
}

func TestReviseWorkDefinitionDecodeUnknownFieldsAndInvalidAllowedFiles(t *testing.T) {
	if _, err := DecodeWorkRevisionPlan(strings.NewReader(`{"version":1,"task_id":"t","board_id":"b","project":".","expected_revision":1,"expected_status":"ready","extra":true,"definition":{}}`)); err == nil {
		t.Fatal("expected unknown top-level field decode failure")
	}
	if _, err := DecodeWorkRevisionPlan(strings.NewReader(`{"version":1,"task_id":"t","board_id":"b","project":".","expected_revision":1,"expected_status":"ready","definition":{"title":"t","objective":"o","acceptance_criteria":"a","verification":"v","allowed_files":[],"sdd_contract":{"version":1,"workflow":"sdd-lite","change_id":"c","spec_plane":"hybrid","pins":[],"requirement_ids":[],"extra":true}}}`)); err == nil {
		t.Fatal("expected unknown SDD field decode failure")
	}
	ctx := context.Background()
	store, workspace := newReviseTestStore(t)
	pinPath := writeRevisePin(t, workspace, "openspec/changes/c/plan.md", "old")
	item := createReviseTask(t, ctx, store, workspace, "ready", nil, reviseContract(workspace, pinPath, []byte("old")))
	plan := revisePlan(workspace, item, item.Contract)
	plan.Definition.AllowedFiles = []string{"../escape.go"}
	if _, err := store.ReviseWorkDefinition(ctx, plan); err == nil {
		t.Fatal("expected invalid allowed file rejection")
	}
}

func newReviseTestStore(t *testing.T) (*Store, string) {
	t.Helper()
	workspace := t.TempDir()
	store, err := OpenStore(filepath.Join(t.TempDir(), "delegation.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if _, err := store.CreateBoard(context.Background(), "revise-board", "Revise Board", ""); err != nil {
		t.Fatal(err)
	}
	return store, workspace
}

func createReviseTask(t *testing.T, ctx context.Context, store *Store, workspace, id string, deps []string, contract *SDDContract) WorkItem {
	t.Helper()
	item, err := store.CreateWorkInBoardWithDefinition(ctx, "revise-board", id, id+" title", deps, WorkDefinition{
		Project: workspace, Objective: "objective", Acceptance: "acceptance", Verification: "verification", AllowedFiles: []string{"src/main.go"}, Contract: contract,
	})
	if err != nil {
		t.Fatalf("create task %s: %v", id, err)
	}
	return item
}

func revisePlan(workspace string, item WorkItem, contract *SDDContract) WorkRevisionPlan {
	return WorkRevisionPlan{Version: 1, TaskID: item.ID, BoardID: item.BoardID, Project: workspace, ExpectedRevision: item.Revision, ExpectedStatus: item.Status, ExpectedWorkflow: "sdd-lite", ExpectedChangeID: "c", ExpectedSpecPlane: "hybrid", Definition: WorkRevisionDefinition{Title: "revised title", Objective: "revised objective", Acceptance: "revised acceptance", Verification: "revised verification", AllowedFiles: []string{"src/revised.go"}, Contract: contract}}
}

func reviseContract(workspace, locator string, content []byte) *SDDContract {
	sum := sha256.Sum256(content)
	return &SDDContract{Version: 1, Workflow: "sdd-lite", ChangeID: "c", SpecPlane: "hybrid", Pins: []ContractPin{{Transport: "workspace_file", Project: workspace, Locator: locator, SHA256: hex.EncodeToString(sum[:])}}, RequirementIDs: []string{"REQ-TEST-001"}}
}

func writeRevisePin(t *testing.T, workspace, locator, content string) string {
	t.Helper()
	path := filepath.Join(workspace, filepath.FromSlash(locator))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return locator
}

type reviseSnapshot struct {
	Title, Objective, Acceptance, Verification, Files, Contract string
	Revision                                                    int64
	Events                                                      int
}

func snapshotReviseTask(t *testing.T, store *Store, id string) reviseSnapshot {
	t.Helper()
	var s reviseSnapshot
	if err := store.db.QueryRow(`SELECT i.title,i.revision,d.objective,d.acceptance_criteria,d.verification,d.allowed_files_json,d.contract_json FROM work_items i JOIN work_definitions d ON d.item_id=i.id WHERE i.id=?`, id).Scan(&s.Title, &s.Revision, &s.Objective, &s.Acceptance, &s.Verification, &s.Files, &s.Contract); err != nil {
		t.Fatal(err)
	}
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM work_events WHERE item_id=?`, id).Scan(&s.Events); err != nil {
		t.Fatal(err)
	}
	return s
}

func assertSameSnapshot(t *testing.T, want, got reviseSnapshot) {
	t.Helper()
	if want != got {
		t.Fatalf("snapshot changed\nwant: %+v\n got: %+v", want, got)
	}
}

func assertReviseEvent(t *testing.T, store *Store, id string) {
	t.Helper()
	var detail string
	if err := store.db.QueryRow(`SELECT detail FROM work_events WHERE item_id=? AND kind='definition_revised'`, id).Scan(&detail); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(detail, "old") || !strings.Contains(detail, "new") || strings.Contains(detail, "revised objective") {
		t.Fatalf("unexpected audit detail: %s", detail)
	}
}
