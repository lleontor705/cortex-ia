package delegation

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestReviseWorkDefinitionHappyReadyAndBacklog(t *testing.T) {
	ctx := context.Background()
	store, workspace := newReviseTestStore(t)
	ready := createReviseTask(t, ctx, store, workspace, "ready", nil)

	revised, err := store.ReviseWorkDefinition(ctx, revisePlan(workspace, ready))
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

	dep := createReviseTask(t, ctx, store, workspace, "dep", nil)
	backlog := createReviseTask(t, ctx, store, workspace, "backlog", []string{dep.ID})
	revised, err = store.ReviseWorkDefinition(ctx, revisePlan(workspace, backlog))
	if err != nil {
		t.Fatalf("ReviseWorkDefinition backlog failed: %v", err)
	}
	if revised.Status != WorkBacklog || len(revised.Dependencies) != 1 || revised.Dependencies[0] != dep.ID {
		t.Fatalf("backlog/dependencies not preserved: %+v", revised)
	}
}

func TestReviseWorkDefinitionRejectsStaleRevisionWithoutEffects(t *testing.T) {
	ctx := context.Background()
	store, workspace := newReviseTestStore(t)
	item := createReviseTask(t, ctx, store, workspace, "ready", nil)
	before := snapshotReviseTask(t, store, item.ID)
	plan := revisePlan(workspace, item)
	plan.ExpectedRevision++
	if _, err := store.ReviseWorkDefinition(ctx, plan); !errors.Is(err, ErrWorkConflict) {
		t.Fatalf("expected stale revision conflict, got %v", err)
	}
	assertSameSnapshot(t, before, snapshotReviseTask(t, store, item.ID))
}

func TestReviseWorkDefinitionRejectsActiveLifecycleAndBoardMismatch(t *testing.T) {
	ctx := context.Background()
	store, workspace := newReviseTestStore(t)
	item := createReviseTask(t, ctx, store, workspace, "ready", nil)
	before := snapshotReviseTask(t, store, item.ID)

	bad := revisePlan(workspace, item)
	bad.BoardID = "other-board"
	if _, err := store.ReviseWorkDefinition(ctx, bad); !errors.Is(err, ErrWorkConflict) {
		t.Fatalf("expected board identity conflict, got %v", err)
	}
	assertSameSnapshot(t, before, snapshotReviseTask(t, store, item.ID))

	claimed, err := store.ClaimWork(ctx, item.ID, "agent", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.ReviseWorkDefinition(ctx, revisePlan(workspace, item)); !errors.Is(err, ErrWorkConflict) {
		t.Fatalf("expected active lifecycle conflict, got %v", err)
	}
	_, _ = store.TransitionWork(ctx, item.ID, claimed.Token, claimed.Revision, WorkBlocked)
}

func TestReviseWorkDefinitionDecodeRejectsUnknownAndRetiredFields(t *testing.T) {
	if _, err := DecodeWorkRevisionPlan(strings.NewReader(`{"version":1,"task_id":"t","board_id":"b","project":".","expected_revision":1,"expected_status":"ready","extra":true,"definition":{}}`)); err == nil {
		t.Fatal("expected unknown top-level field decode failure")
	}
	if _, err := DecodeWorkRevisionPlan(strings.NewReader(`{"version":1,"task_id":"t","board_id":"b","project":".","expected_revision":1,"expected_status":"ready","expected_workflow":"sdd-lite","definition":{}}`)); err == nil {
		t.Fatal("retired expected_workflow field must fail closed")
	}
	if _, err := DecodeWorkRevisionPlan(strings.NewReader(`{"version":1,"task_id":"t","board_id":"b","project":".","expected_revision":1,"expected_status":"ready","definition":{"title":"t","objective":"o","acceptance_criteria":"a","verification":"v","allowed_files":[],"sdd_contract":{}}}`)); err == nil {
		t.Fatal("retired sdd_contract field must fail closed")
	}
}

func TestReviseWorkDefinitionRejectsInvalidAllowedFiles(t *testing.T) {
	ctx := context.Background()
	store, workspace := newReviseTestStore(t)
	item := createReviseTask(t, ctx, store, workspace, "ready", nil)
	plan := revisePlan(workspace, item)
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

func createReviseTask(t *testing.T, ctx context.Context, store *Store, workspace, id string, deps []string) WorkItem {
	t.Helper()
	item, err := store.CreateWorkInBoardWithDefinition(ctx, "revise-board", id, id+" title", deps, WorkDefinition{
		Project: workspace, Objective: "objective", Acceptance: "acceptance", Verification: "verification", AllowedFiles: []string{"src/main.go"},
	})
	if err != nil {
		t.Fatalf("create task %s: %v", id, err)
	}
	return item
}

func revisePlan(workspace string, item WorkItem) WorkRevisionPlan {
	return WorkRevisionPlan{Version: 1, TaskID: item.ID, BoardID: item.BoardID, Project: workspace, ExpectedRevision: item.Revision, ExpectedStatus: item.Status, Definition: WorkRevisionDefinition{Title: "revised title", Objective: "revised objective", Acceptance: "revised acceptance", Verification: "revised verification", AllowedFiles: []string{"src/revised.go"}}}
}

type reviseSnapshot struct {
	Title, Objective, Acceptance, Verification, Files string
	Revision                                          int64
	Events                                            int
}

func snapshotReviseTask(t *testing.T, store *Store, id string) reviseSnapshot {
	t.Helper()
	var s reviseSnapshot
	if err := store.db.QueryRow(`SELECT i.title,i.revision,d.objective,d.acceptance_criteria,d.verification,d.allowed_files_json FROM work_items i JOIN work_definitions d ON d.item_id=i.id WHERE i.id=?`, id).Scan(&s.Title, &s.Revision, &s.Objective, &s.Acceptance, &s.Verification, &s.Files); err != nil {
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
