package delegation

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func TestReadOnlyRolesConcurrency(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "delegation.db")

	store, err := OpenStore(dbPath)
	if err != nil {
		t.Fatalf("OpenStore failed: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// 1. Create first investigate job
	job1, err := store.Create(ctx, NewJob{
		Role:            "investigate",
		ObjectiveDigest: "sha256:investigate-1",
		Transport:       "direct",
		Workspace:       tempDir,
	})
	if err != nil {
		t.Fatalf("expected first investigate job to succeed, got: %v", err)
	}

	// 2. Create second investigate job concurrently in the same workspace
	job2, err := store.Create(ctx, NewJob{
		Role:            "investigate",
		ObjectiveDigest: "sha256:investigate-2",
		Transport:       "direct",
		Workspace:       tempDir,
	})
	if err != nil {
		t.Fatalf("expected second investigate job to succeed concurrently, got: %v", err)
	}

	// 3. Create reviewer job concurrently in the same workspace
	job3, err := store.Create(ctx, NewJob{
		Role:            "reviewer",
		ObjectiveDigest: "sha256:reviewer-1",
		Transport:       "direct",
		Workspace:       tempDir,
	})
	if err != nil {
		t.Fatalf("expected reviewer job to succeed concurrently with investigate jobs, got: %v", err)
	}

	if job1.ID == job2.ID || job1.ID == job3.ID || job2.ID == job3.ID {
		t.Fatalf("expected unique job IDs, got: %s, %s, %s", job1.ID, job2.ID, job3.ID)
	}
	if job1.Status != StatusAccepted || job2.Status != StatusAccepted || job3.Status != StatusAccepted {
		t.Fatalf("expected all jobs to have status accepted, got: %s, %s, %s", job1.Status, job2.Status, job3.Status)
	}
}

func TestPlannerBlocksReadersAndWriters(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "delegation.db")

	store, err := OpenStore(dbPath)
	if err != nil {
		t.Fatalf("OpenStore failed: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// 1. Create active planner job
	plannerJob, err := store.Create(ctx, NewJob{
		Role:            "planner",
		ObjectiveDigest: "sha256:planner-1",
		Transport:       "direct",
		Workspace:       tempDir,
	})
	if err != nil {
		t.Fatalf("expected planner job to succeed, got: %v", err)
	}

	// 2. Reader (investigate) must be blocked by active planner
	_, err = store.Create(ctx, NewJob{
		Role:            "investigate",
		ObjectiveDigest: "sha256:reader-blocked",
		Transport:       "direct",
		Workspace:       tempDir,
	})
	if err == nil || !errors.Is(err, ErrWorkConflict) {
		t.Fatalf("expected investigate to be blocked by active planner, got: %v", err)
	}

	// 3. Writer (implement) must be blocked by active planner
	_, err = store.Create(ctx, NewJob{
		Role:            "implement",
		TaskID:          "task-p",
		ObjectiveDigest: "sha256:implement-blocked",
		Transport:       "direct",
		Workspace:       tempDir,
	})
	if err == nil || !errors.Is(err, ErrWorkConflict) {
		t.Fatalf("expected implement to be blocked by active planner, got: %v", err)
	}

	// 4. Another planner must be blocked by active planner
	_, err = store.Create(ctx, NewJob{
		Role:            "planner",
		ObjectiveDigest: "sha256:planner-2-blocked",
		Transport:       "direct",
		Workspace:       tempDir,
	})
	if err == nil || !errors.Is(err, ErrWorkConflict) {
		t.Fatalf("expected planner to be blocked by active planner, got: %v", err)
	}

	// 5. Complete the planner job
	if err := store.Cancel(ctx, plannerJob.ID); err != nil {
		t.Fatalf("Cancel failed: %v", err)
	}

	// 6. Now reader must succeed
	jobAfter, err := store.Create(ctx, NewJob{
		Role:            "investigate",
		ObjectiveDigest: "sha256:reader-unblocked",
		Transport:       "direct",
		Workspace:       tempDir,
	})
	if err != nil {
		t.Fatalf("expected investigate to succeed after planner completed, got: %v", err)
	}
	if jobAfter.Status != StatusAccepted {
		t.Fatalf("expected status accepted, got: %s", jobAfter.Status)
	}
}

func TestActiveJobsBlockPlanner(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "delegation.db")

	store, err := OpenStore(dbPath)
	if err != nil {
		t.Fatalf("OpenStore failed: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// 1. Create active reader job (investigate)
	readerJob, err := store.Create(ctx, NewJob{
		Role:            "investigate",
		ObjectiveDigest: "sha256:reader-active",
		Transport:       "direct",
		Workspace:       tempDir,
	})
	if err != nil {
		t.Fatalf("expected investigate job to succeed, got: %v", err)
	}

	// 2. Planner must be blocked by active reader
	_, err = store.Create(ctx, NewJob{
		Role:            "planner",
		ObjectiveDigest: "sha256:planner-blocked",
		Transport:       "direct",
		Workspace:       tempDir,
	})
	if err == nil || !errors.Is(err, ErrWorkConflict) {
		t.Fatalf("expected planner to be blocked by active investigate, got: %v", err)
	}

	// 3. Complete the reader job
	if err := store.Cancel(ctx, readerJob.ID); err != nil {
		t.Fatalf("Cancel failed: %v", err)
	}

	// 4. Now planner must succeed
	plannerAfter, err := store.Create(ctx, NewJob{
		Role:            "planner",
		ObjectiveDigest: "sha256:planner-unblocked",
		Transport:       "direct",
		Workspace:       tempDir,
	})
	if err != nil {
		t.Fatalf("expected planner to succeed after reader completed, got: %v", err)
	}
	if plannerAfter.Status != StatusAccepted {
		t.Fatalf("expected status accepted, got: %s", plannerAfter.Status)
	}
}
