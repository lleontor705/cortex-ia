package delegation

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestIndexedQueryPlanAndPerformance(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "delegation.db")

	store, err := OpenStore(dbPath)
	if err != nil {
		t.Fatalf("OpenStore failed: %v", err)
	}
	defer func() { _ = store.Close() }()

	ctx := context.Background()

	// 1. Verify that SQLite uses the composite indexes via EXPLAIN QUERY PLAN
	indexChecks := []struct {
		query     string
		wantIndex string
	}{
		{
			query:     "SELECT id FROM work_approvals WHERE item_id = 'test-id' ORDER BY id DESC",
			wantIndex: "work_approvals_item_id_idx",
		},
		{
			query:     "SELECT item_id FROM work_dependencies WHERE depends_on = 'dep-id'",
			wantIndex: "work_dependencies_depends_on_idx",
		},
		{
			query:     "SELECT item_id FROM work_claims WHERE expires_at > '2026-01-01'",
			wantIndex: "work_claims_expires_at_idx",
		},
		{
			query:     "SELECT path FROM work_leases WHERE expires_at <= '2026-01-01'",
			wantIndex: "work_leases_expires_at_idx",
		},
	}

	for _, tc := range indexChecks {
		var id int
		var parent int
		var notused int
		var detail string
		err := store.db.QueryRowContext(ctx, "EXPLAIN QUERY PLAN "+tc.query).Scan(&id, &parent, &notused, &detail)
		if err != nil {
			t.Fatalf("EXPLAIN QUERY PLAN failed for %q: %v", tc.query, err)
		}
		if !strings.Contains(detail, tc.wantIndex) {
			t.Errorf("query %q: detail = %q, want index %q", tc.query, detail, tc.wantIndex)
		}
	}

	// 2. Functional and regression latency check
	boardID := "perf-board"
	if _, err := store.CreateBoard(ctx, boardID, "Perf Board", "Benchmark latency"); err != nil {
		t.Fatalf("CreateBoard failed: %v", err)
	}

	for i := 1; i <= 20; i++ {
		taskID := fmt.Sprintf("task-perf-%03d", i)
		var deps []string
		if i > 1 {
			deps = []string{fmt.Sprintf("task-perf-%03d", i-1)}
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
	}

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

	start := time.Now()
	items, err := store.ListWorkByBoard(ctx, boardID)
	if err != nil {
		t.Fatalf("ListWorkByBoard failed: %v", err)
	}
	if len(items) != 20 {
		t.Fatalf("expected 20 items, got %d", len(items))
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Errorf("ListWorkByBoard took %v, want < 500ms", elapsed)
	}

	start = time.Now()
	item, err := store.GetWork(ctx, "task-perf-001")
	if err != nil {
		t.Fatalf("GetWork failed: %v", err)
	}
	if item.ID != "task-perf-001" {
		t.Fatalf("unexpected item ID: %s", item.ID)
	}
	if elapsed := time.Since(start); elapsed > 200*time.Millisecond {
		t.Errorf("GetWork took %v, want < 200ms", elapsed)
	}

	start = time.Now()
	snap, err := store.BoardSnapshot(ctx, boardID)
	if err != nil {
		t.Fatalf("BoardSnapshot failed: %v", err)
	}
	if snap.Board.ID != boardID {
		t.Fatalf("unexpected board snapshot ID: %s", snap.Board.ID)
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Errorf("BoardSnapshot took %v, want < 500ms", elapsed)
	}
}
