package delegation

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBranchToBoardID(t *testing.T) {
	tests := []struct {
		branch   string
		expected string
	}{
		{"", DefaultBoardID},
		{"main", DefaultBoardID},
		{"master", DefaultBoardID},
		{"trunk", DefaultBoardID},
		{"HEAD", DefaultBoardID},
		{"feat/dashboard-improvements", "branch-feat-dashboard-improvements"},
		{"fix/issue-98", "branch-fix-issue-98"},
		{"feature/user_auth.v2", "branch-feature-user-auth-v2"},
		{"refs/heads/feature/cool", "branch-refs-heads-feature-cool"},
	}

	for _, tt := range tests {
		got := BranchToBoardID(tt.branch)
		if got != tt.expected {
			t.Errorf("BranchToBoardID(%q) = %q, expected %q", tt.branch, got, tt.expected)
		}
	}
}

func TestEnsureBoard(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "delegation.db")
	store, err := OpenStore(dbPath)
	if err != nil {
		t.Fatalf("OpenStore failed: %v", err)
	}
	defer func() { _ = store.Close() }()
	ctx := context.Background()

	// 1. Ensure a new board
	board, err := store.EnsureBoard(ctx, "branch-my-feat", "🌿 my-feat", "Auto branch board")
	if err != nil {
		t.Fatalf("EnsureBoard new failed: %v", err)
	}
	if board.ID != "branch-my-feat" || board.Title != "🌿 my-feat" {
		t.Errorf("unexpected board: %+v", board)
	}

	// 2. Ensure the same board again (idempotent)
	board2, err := store.EnsureBoard(ctx, "branch-my-feat", "🌿 my-feat", "Auto branch board")
	if err != nil {
		t.Fatalf("EnsureBoard existing failed: %v", err)
	}
	if board2.ID != board.ID {
		t.Errorf("expected same board id, got %s vs %s", board2.ID, board.ID)
	}

	// 3. Ensure invalid arguments
	if _, err := store.EnsureBoard(ctx, "", "Title", ""); err == nil {
		t.Error("expected error with empty id")
	}
	if _, err := store.EnsureBoard(ctx, "id", "", ""); err == nil {
		t.Error("expected error with empty title")
	}
}

func TestResolveCurrentBranch(t *testing.T) {
	// 1. In non-git directory
	nonGitDir := t.TempDir()
	if b := ResolveCurrentBranch(nonGitDir); b != "" {
		t.Errorf("expected empty branch in non-git directory, got %q", b)
	}

	// 2. In current repository
	wd, err := os.Getwd()
	if err == nil {
		branch := ResolveCurrentBranch(wd)
		if branch == "" {
			t.Error("expected non-empty branch in current git repository")
		}
	}
}

func TestPruneWork(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "delegation.db")
	store, err := OpenStore(dbPath)
	if err != nil {
		t.Fatalf("OpenStore failed: %v", err)
	}
	defer func() { _ = store.Close() }()
	ctx := context.Background()

	// Create tasks
	workspace := t.TempDir()
	task1, err := store.CreateWorkInBoardWithDefinition(ctx, "default", "task-done-1", "Done Task 1", nil, WorkDefinition{Project: workspace})
	if err != nil {
		t.Fatalf("create task-done-1 failed: %v", err)
	}
	task2, err := store.CreateWorkInBoardWithDefinition(ctx, "default", "task-done-2", "Done Task 2", nil, WorkDefinition{Project: workspace})
	if err != nil {
		t.Fatalf("create task-done-2 failed: %v", err)
	}
	taskActive, err := store.CreateWorkInBoardWithDefinition(ctx, "default", "task-active", "Active Task", []string{task2.ID}, WorkDefinition{Project: workspace})
	if err != nil {
		t.Fatalf("create task-active failed: %v", err)
	}

	// Transition task1 and task2 to done via transition/approve
	claim1, err := store.ClaimWork(ctx, task1.ID, "agent-1", 10*time.Minute)
	if err != nil {
		t.Fatalf("claim1 failed: %v", err)
	}
	t1, err := store.GetWork(ctx, task1.ID)
	if err != nil {
		t.Fatalf("get task1 failed: %v", err)
	}
	t1, err = store.TransitionWork(ctx, task1.ID, claim1.Token, t1.Revision, WorkInReview)
	if err != nil {
		t.Fatalf("transition task1 in_review failed: %v", err)
	}
	_, err = store.ApproveWork(ctx, task1.ID, "reviewer", "PASS", "test ok", t1.Revision)
	if err != nil {
		t.Fatalf("approve task1 failed: %v", err)
	}

	claim2, err := store.ClaimWork(ctx, task2.ID, "agent-2", 10*time.Minute)
	if err != nil {
		t.Fatalf("claim2 failed: %v", err)
	}
	t2, err := store.GetWork(ctx, task2.ID)
	if err != nil {
		t.Fatalf("get task2 failed: %v", err)
	}
	t2, err = store.TransitionWork(ctx, task2.ID, claim2.Token, t2.Revision, WorkInReview)
	if err != nil {
		t.Fatalf("transition task2 in_review failed: %v", err)
	}
	_, err = store.ApproveWork(ctx, task2.ID, "reviewer", "PASS", "test ok", t2.Revision)
	if err != nil {
		t.Fatalf("approve task2 failed: %v", err)
	}

	// Verify task statuses
	t1, _ = store.GetWork(ctx, task1.ID)
	t2, _ = store.GetWork(ctx, task2.ID)
	if t1.Status != WorkDone || t2.Status != WorkDone {
		t.Fatalf("expected done status, got %s and %s", t1.Status, t2.Status)
	}

	// Notice: taskActive is in ready/backlog status and depends on task2!
	// Therefore, task2 must NOT be pruned because it is depended on by an active task.
	// Only task1 can be pruned.

	// 1. Dry run prune
	dryRes, err := store.PruneWork(ctx, PruneWorkOptions{
		BoardID: "default",
		DryRun:  true,
	})
	if err != nil {
		t.Fatalf("PruneWork dry run failed: %v", err)
	}
	if dryRes.PrunedCount != 1 || dryRes.PrunedTaskIDs[0] != task1.ID {
		t.Errorf("expected 1 prune candidate (task-done-1), got %+v", dryRes)
	}

	// Verify task1 still exists after dry-run
	if _, err := store.GetWork(ctx, task1.ID); err != nil {
		t.Errorf("task-done-1 should still exist after dry-run")
	}

	// 2. Real prune
	realRes, err := store.PruneWork(ctx, PruneWorkOptions{
		BoardID: "default",
		DryRun:  false,
	})
	if err != nil {
		t.Fatalf("PruneWork real failed: %v", err)
	}
	if realRes.PrunedCount != 1 || realRes.PrunedTaskIDs[0] != task1.ID {
		t.Errorf("expected task-done-1 pruned, got %+v", realRes)
	}

	// Verify task1 was removed, while task2 and taskActive remain
	if _, err := store.GetWork(ctx, task1.ID); err != ErrWorkNotFound {
		t.Errorf("task-done-1 should be deleted, got err=%v", err)
	}
	if _, err := store.GetWork(ctx, task2.ID); err != nil {
		t.Errorf("task-done-2 should still exist because active task depends on it: %v", err)
	}
	if _, err := store.GetWork(ctx, taskActive.ID); err != nil {
		t.Errorf("task-active should still exist: %v", err)
	}
}
