package delegation

import (
	"context"
	"path/filepath"
	"testing"
)

func TestDashboardForConversation_WorkspaceScope(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "delegation.db")

	store, err := OpenStore(dbPath)
	if err != nil {
		t.Fatalf("OpenStore failed: %v", err)
	}
	defer func() { _ = store.Close() }()

	ctx := context.Background()

	// Create board
	if _, err := store.CreateBoard(ctx, "feat-test", "Feature Test", "Board for test"); err != nil {
		t.Fatalf("CreateBoard failed: %v", err)
	}

	// Task 1 created in session ses_planner
	plannerOwnership := ConversationOwnership{
		OpenCodeSessionID:     "ses_planner",
		OpenCodeRootSessionID: "ses_planner",
	}
	_, err = store.CreateWorkInBoardWithDefinition(ctx, "feat-test", "task-1", "Task 1", nil, WorkDefinition{
		Project:               tempDir,
		ConversationOwnership: plannerOwnership,
		Objective:             "Obj 1",
		Acceptance:            "Acc 1",
		Verification:          "go test",
		AllowedFiles:          []string{"test.go"},
	})
	if err != nil {
		t.Fatalf("CreateWorkInBoardWithDefinition task-1 failed: %v", err)
	}

	// Task 2 created in same session
	_, err = store.CreateWorkInBoardWithDefinition(ctx, "feat-test", "task-2", "Task 2", nil, WorkDefinition{
		Project:               tempDir,
		ConversationOwnership: plannerOwnership,
		Objective:             "Obj 2",
		Acceptance:            "Acc 2",
		Verification:          "go test",
		AllowedFiles:          []string{"test.go"},
	})
	if err != nil {
		t.Fatalf("CreateWorkInBoardWithDefinition task-2 failed: %v", err)
	}

	// 1. Query with empty session IDs: should default to global and return workspace tasks
	dashGlobal, err := store.DashboardForConversation(ctx, tempDir, "", "")
	if err != nil {
		t.Fatalf("DashboardForConversation empty session failed: %v", err)
	}
	if dashGlobal.RequestedSessionID != "global" || dashGlobal.RootSessionID != "global" {
		t.Errorf("expected session IDs to default to global, got %q, %q", dashGlobal.RequestedSessionID, dashGlobal.RootSessionID)
	}
	if dashGlobal.Summary["total_tasks"] != 2 {
		t.Errorf("expected 2 total tasks for global scope, got %d", dashGlobal.Summary["total_tasks"])
	}
	if len(dashGlobal.Tasks) != 2 {
		t.Errorf("expected 2 tasks in list, got %d", len(dashGlobal.Tasks))
	}

	// 2. Query with a worker subagent whose root session is ses_planner: must see tasks
	dashWorker, err := store.DashboardForConversation(ctx, tempDir, "ses_worker", "ses_planner")
	if err != nil {
		t.Fatalf("DashboardForConversation worker session failed: %v", err)
	}
	if dashWorker.Summary["total_tasks"] != 2 {
		t.Errorf("expected 2 total tasks for worker session under ses_planner, got %d", dashWorker.Summary["total_tasks"])
	}
	if len(dashWorker.Tasks) != 2 {
		t.Errorf("expected 2 tasks in list for worker session under ses_planner, got %d", len(dashWorker.Tasks))
	}

	// 3. Query with an isolated new session: must NOT see tasks from ses_planner
	dashOther, err := store.DashboardForConversation(ctx, tempDir, "ses_other", "ses_other")
	if err != nil {
		t.Fatalf("DashboardForConversation isolated session failed: %v", err)
	}
	if dashOther.Summary["total_tasks"] != 0 {
		t.Errorf("expected 0 total tasks for isolated session, got %d", dashOther.Summary["total_tasks"])
	}
	if len(dashOther.Tasks) != 0 {
		t.Errorf("expected 0 tasks in list for isolated session, got %d", len(dashOther.Tasks))
	}
}
