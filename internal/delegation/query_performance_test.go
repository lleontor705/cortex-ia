package delegation

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

func TestIndexedQueryLatency(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "delegation.db")

	store, err := OpenStore(dbPath)
	if err != nil {
		t.Fatalf("OpenStore failed: %v", err)
	}
	defer func() { _ = store.Close() }()

	ctx := context.Background()
	boardID := "perf-board"
	if _, err := store.CreateBoard(ctx, boardID, "Perf Board", "Benchmark latency"); err != nil {
		t.Fatalf("CreateBoard failed: %v", err)
	}

	// Seed 20 items with dependencies and approvals
	var prevID string
	for i := 1; i <= 20; i++ {
		taskID := fmt.Sprintf("task-perf-%03d", i)
		var deps []string
		if prevID != "" {
			deps = []string{prevID}
		}
		if _, err := store.CreateWorkInBoardWithDefinition(ctx, boardID, taskID, fmt.Sprintf("Task %d", i), deps, WorkDefinition{
			Project:      tempDir,
			Objective:    fmt.Sprintf("Objective %d", i),
			Acceptance:   "PASS",
			Verification: "exit 0",
			AllowedFiles: []string{fmt.Sprintf("file_%d.go", i)},
		}); err != nil {
			t.Fatalf("CreateWork failed for %s: %v", taskID, err)
		}
		prevID = taskID
	}

	// Claim and approve some items
	claim, err := store.ClaimWorkWithLeases(ctx, "task-perf-001", "owner-perf", []string{"file_1.go"}, 5*time.Minute)
	if err != nil {
		t.Fatalf("ClaimWork failed: %v", err)
	}
	trans, err := store.TransitionWork(ctx, "task-perf-001", claim.Token, claim.Revision, WorkInReview)
	if err != nil {
		t.Fatalf("TransitionWork failed: %v", err)
	}
	if _, err := store.ApproveWork(ctx, "task-perf-001", "reviewer-perf", "PASS", "evidence", trans.Revision); err != nil {
		t.Fatalf("ApproveWork failed: %v", err)
	}

	// 1. Measure ListWorkByBoard latency
	start := time.Now()
	items, err := store.ListWorkByBoard(ctx, boardID)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("ListWorkByBoard failed: %v", err)
	}
	if len(items) != 20 {
		t.Fatalf("expected 20 items, got %d", len(items))
	}
	if elapsed > 20*time.Millisecond {
		t.Errorf("ListWorkByBoard took %v, want < 20ms", elapsed)
	}

	// 2. Measure single item lookup latency (must be < 5ms)
	start = time.Now()
	item, err := store.GetWork(ctx, "task-perf-001")
	elapsed = time.Since(start)
	if err != nil {
		t.Fatalf("GetWork failed: %v", err)
	}
	if item.ID != "task-perf-001" {
		t.Fatalf("unexpected item ID: %s", item.ID)
	}
	if elapsed > 5*time.Millisecond {
		t.Errorf("GetWork took %v, want < 5ms", elapsed)
	}

	// 3. Measure board snapshot latency
	start = time.Now()
	snap, err := store.BoardSnapshot(ctx, boardID)
	elapsed = time.Since(start)
	if err != nil {
		t.Fatalf("BoardSnapshot failed: %v", err)
	}
	if snap.Board.ID != boardID {
		t.Fatalf("unexpected board snapshot ID: %s", snap.Board.ID)
	}
	if elapsed > 15*time.Millisecond {
		t.Errorf("BoardSnapshot took %v, want < 15ms", elapsed)
	}
}
