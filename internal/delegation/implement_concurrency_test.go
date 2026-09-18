package delegation

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestImplementConcurrencyWithDisjointLeases(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "delegation.db")

	store, err := OpenStore(dbPath)
	if err != nil {
		t.Fatalf("OpenStore failed: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Setup board and two tasks with disjoint files
	_, _ = store.CreateBoard(ctx, "b-test", "Test Board", "")
	_, err = store.CreateWorkInBoardWithDefinition(ctx, "b-test", "task-1", "Task 1", nil, WorkDefinition{
		Project:      tempDir,
		AllowedFiles: []string{"pkg/a.go"},
	})
	if err != nil {
		t.Fatalf("create task-1 failed: %v", err)
	}

	_, err = store.CreateWorkInBoardWithDefinition(ctx, "b-test", "task-2", "Task 2", nil, WorkDefinition{
		Project:      tempDir,
		AllowedFiles: []string{"pkg/b.go"},
	})
	if err != nil {
		t.Fatalf("create task-2 failed: %v", err)
	}

	// Claim task-1 with lease on pkg/a.go
	_, err = store.ClaimWorkWithLeases(ctx, "task-1", "owner-1", []string{"pkg/a.go"}, time.Minute)
	if err != nil {
		t.Fatalf("claim task-1 failed: %v", err)
	}

	// Claim task-2 with lease on pkg/b.go
	_, err = store.ClaimWorkWithLeases(ctx, "task-2", "owner-2", []string{"pkg/b.go"}, time.Minute)
	if err != nil {
		t.Fatalf("claim task-2 failed: %v", err)
	}

	// 1. Create first implement job for task-1
	job1, err := store.Create(ctx, NewJob{
		Role:            "implement",
		TaskID:          "task-1",
		ObjectiveDigest: "sha256:impl-1",
		Transport:       "direct",
		Workspace:       tempDir,
	})
	if err != nil {
		t.Fatalf("expected job1 (task-1) to succeed, got: %v", err)
	}

	// 2. Create second implement job for task-2 concurrently (disjoint files)
	job2, err := store.Create(ctx, NewJob{
		Role:            "implement",
		TaskID:          "task-2",
		ObjectiveDigest: "sha256:impl-2",
		Transport:       "direct",
		Workspace:       tempDir,
	})
	if err != nil {
		t.Fatalf("expected job2 (task-2) to succeed concurrently with disjoint files, got: %v", err)
	}

	if job1.ID == job2.ID {
		t.Fatalf("expected unique job IDs, got: %s", job1.ID)
	}
	if job1.Status != StatusAccepted || job2.Status != StatusAccepted {
		t.Fatalf("expected both jobs to be accepted, got: %s, %s", job1.Status, job2.Status)
	}

	// 3. Attempting to create another job for the SAME task-1 must be blocked
	_, err = store.Create(ctx, NewJob{
		Role:            "implement",
		TaskID:          "task-1",
		ObjectiveDigest: "sha256:impl-1-dup",
		Transport:       "direct",
		Workspace:       tempDir,
	})
	if err == nil || !errors.Is(err, ErrWorkConflict) {
		t.Fatalf("expected duplicate task-1 job to be blocked, got: %v", err)
	}
}

func TestValidateWorkspaceChangesToleratesConcurrentLeases(t *testing.T) {
	tempDir := t.TempDir()

	fileA := filepath.Join(tempDir, "a.txt")
	fileB := filepath.Join(tempDir, "b.txt")
	fileC := filepath.Join(tempDir, "c.txt")

	_ = os.WriteFile(fileA, []byte("initial a"), 0o644)
	_ = os.WriteFile(fileB, []byte("initial b"), 0o644)
	_ = os.WriteFile(fileC, []byte("initial c"), 0o644)

	baseline, err := captureWorkspaceBaseline(tempDir)
	if err != nil {
		t.Fatalf("capture baseline failed: %v", err)
	}

	// Modify a.txt (this worker's file) and b.txt (concurrent worker's file)
	_ = os.WriteFile(fileA, []byte("modified a"), 0o644)
	_ = os.WriteFile(fileB, []byte("modified b"), 0o644)

	// validateWorkspaceChanges with b.txt in toleratedPaths must succeed
	err = validateWorkspaceChanges(tempDir, []string{"a.txt"}, baseline, []string{"b.txt"})
	if err != nil {
		t.Fatalf("expected changes to be valid when b.txt is tolerated, got: %v", err)
	}

	// Now modify c.txt (which is neither allowed nor tolerated)
	_ = os.WriteFile(fileC, []byte("modified c"), 0o644)

	err = validateWorkspaceChanges(tempDir, []string{"a.txt"}, baseline, []string{"b.txt"})
	if err == nil {
		t.Fatalf("expected unleased modification on c.txt to be rejected, got nil")
	}
}

func TestImplementAndInvestigateConcurrency(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "delegation.db")

	store, err := OpenStore(dbPath)
	if err != nil {
		t.Fatalf("OpenStore failed: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// 1. Create investigate job
	invJob, err := store.Create(ctx, NewJob{
		Role:            "investigate",
		ObjectiveDigest: "sha256:inv-1",
		Transport:       "direct",
		Workspace:       tempDir,
	})
	if err != nil {
		t.Fatalf("expected investigate job to succeed, got: %v", err)
	}

	// 2. Create implement job concurrently
	_, _ = store.CreateBoard(ctx, "default", "Default", "")
	_, err = store.CreateWorkInBoardWithDefinition(ctx, "default", "task-x", "Task X", nil, WorkDefinition{
		Project:      tempDir,
		AllowedFiles: []string{"x.go"},
	})
	if err != nil {
		t.Fatalf("create task-x failed: %v", err)
	}
	_, err = store.ClaimWorkWithLeases(ctx, "task-x", "owner-x", []string{"x.go"}, time.Minute)
	if err != nil {
		t.Fatalf("claim task-x failed: %v", err)
	}

	implJob, err := store.Create(ctx, NewJob{
		Role:            "implement",
		TaskID:          "task-x",
		ObjectiveDigest: "sha256:impl-x",
		Transport:       "direct",
		Workspace:       tempDir,
	})
	if err != nil {
		t.Fatalf("expected implement job to succeed concurrently with investigate, got: %v", err)
	}

	if invJob.Status != StatusAccepted || implJob.Status != StatusAccepted {
		t.Fatalf("expected both jobs accepted, got: %s, %s", invJob.Status, implJob.Status)
	}
}
