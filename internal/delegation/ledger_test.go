package delegation

import (
	"context"
	"path/filepath"
	"testing"
)

func TestStoreLedgerLifecycle(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "delegation.db")

	store, err := OpenStore(dbPath)
	if err != nil {
		t.Fatalf("OpenStore failed: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// 1. Add facts
	f1, err := store.AddFact(ctx, "board-alpha", "Go 1.26.1 verified", "orchestrator")
	if err != nil {
		t.Fatalf("AddFact 1 failed: %v", err)
	}
	if f1.ID == 0 || f1.Fact != "Go 1.26.1 verified" || f1.BoardID != "board-alpha" {
		t.Fatalf("unexpected fact: %+v", f1)
	}

	f2, err := store.AddFact(ctx, "board-alpha", "SQLite WAL enabled", "investigate")
	if err != nil {
		t.Fatalf("AddFact 2 failed: %v", err)
	}
	if f2.ID <= f1.ID {
		t.Fatalf("expected f2 ID > f1 ID, got %d <= %d", f2.ID, f1.ID)
	}

	// 2. List facts
	facts, err := store.ListFacts(ctx, "board-alpha")
	if err != nil {
		t.Fatalf("ListFacts failed: %v", err)
	}
	if len(facts) != 2 {
		t.Fatalf("expected 2 facts, got %d", len(facts))
	}
	if facts[0].Fact != "Go 1.26.1 verified" || facts[1].Fact != "SQLite WAL enabled" {
		t.Fatalf("unexpected facts list: %+v", facts)
	}

	// 3. Record progress evaluations
	p1, err := store.RecordProgress(ctx, "board-alpha", 1, "Completed phase 1 setup", false, "continue")
	if err != nil {
		t.Fatalf("RecordProgress 1 failed: %v", err)
	}
	if p1.Cycle != 1 || p1.DriftDetected {
		t.Fatalf("unexpected progress 1: %+v", p1)
	}

	// Automatic cycle auto-increment
	p2, err := store.RecordProgress(ctx, "board-alpha", 0, "Discovered dependency drift", true, "replan")
	if err != nil {
		t.Fatalf("RecordProgress 2 failed: %v", err)
	}
	if p2.Cycle != 2 || !p2.DriftDetected || p2.Action != "replan" {
		t.Fatalf("unexpected progress 2: %+v", p2)
	}

	// 4. List progress evaluations
	evals, err := store.ListProgress(ctx, "board-alpha")
	if err != nil {
		t.Fatalf("ListProgress failed: %v", err)
	}
	if len(evals) != 2 {
		t.Fatalf("expected 2 progress records, got %d", len(evals))
	}

	// 5. Get complete ledger report
	report, err := store.GetLedger(ctx, "board-alpha")
	if err != nil {
		t.Fatalf("GetLedger failed: %v", err)
	}
	if len(report.Facts) != 2 || len(report.Progress) != 2 {
		t.Fatalf("unexpected ledger report: %+v", report)
	}
}
