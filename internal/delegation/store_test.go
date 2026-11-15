package delegation

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestBoardsAndDashboard(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "delegation.db")

	store, err := OpenStore(dbPath)
	if err != nil {
		t.Fatalf("OpenStore failed: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// 1. Create Board
	board, err := store.CreateBoard(ctx, "b-test", "Test Board", "A test board description")
	if err != nil {
		t.Fatalf("CreateBoard failed: %v", err)
	}
	if board.ID != "b-test" || board.Title != "Test Board" {
		t.Fatalf("board created mismatch: %+v", board)
	}

	// 2. List Boards
	boards, err := store.ListBoards(ctx)
	if err != nil {
		t.Fatalf("ListBoards failed: %v", err)
	}
	if len(boards) < 1 {
		t.Fatalf("expected at least 1 board, got %d", len(boards))
	}

	// 3. Board Snapshot
	snap, err := store.BoardSnapshot(ctx, "b-test")
	if err != nil {
		t.Fatalf("BoardSnapshot failed: %v", err)
	}
	if snap.Board.ID != "b-test" {
		t.Errorf("snapshot board mismatch: %+v", snap.Board)
	}

	// 4. Dashboard
	dash, err := store.Dashboard(ctx)
	if err != nil {
		t.Fatalf("Dashboard failed: %v", err)
	}
	if len(dash.Sessions) < 1 {
		t.Errorf("expected at least 1 session in dashboard, got %d", len(dash.Sessions))
	}

	// 5. Board Archive, Unarchive, and Delete Lifecycle
	// Cannot archive or delete default board
	if _, err := store.ArchiveBoard(ctx, "default"); err == nil {
		t.Errorf("expected error archiving default board, got nil")
	}
	if err := store.DeleteBoard(ctx, "default"); err == nil {
		t.Errorf("expected error deleting default board, got nil")
	}

	// Cannot delete active board
	if err := store.DeleteBoard(ctx, "b-test"); err == nil {
		t.Errorf("expected error deleting active board, got nil")
	}

	// Archive custom board
	archived, err := store.ArchiveBoard(ctx, "b-test")
	if err != nil {
		t.Fatalf("ArchiveBoard failed: %v", err)
	}
	if archived.Status != "archived" {
		t.Errorf("expected archived status, got %s", archived.Status)
	}

	// Unarchive custom board
	restored, err := store.UnarchiveBoard(ctx, "b-test")
	if err != nil {
		t.Fatalf("UnarchiveBoard failed: %v", err)
	}
	if restored.Status != "active" {
		t.Errorf("expected active status, got %s", restored.Status)
	}

	// Re-archive and delete
	if _, err := store.ArchiveBoard(ctx, "b-test"); err != nil {
		t.Fatalf("re-archive failed: %v", err)
	}
	if err := store.DeleteBoard(ctx, "b-test"); err != nil {
		t.Fatalf("DeleteBoard failed: %v", err)
	}
	if _, err := store.GetBoard(ctx, "b-test"); err == nil {
		t.Errorf("expected GetBoard to fail for deleted board, got nil")
	}

	// 6. List Delegations & Activity
	_, _ = store.ListDelegations(ctx, 10)
	_, _ = store.ListActivity(ctx, 10)
}

func TestWorkItemsClaimsAndLeases(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "delegation.db")

	store, err := OpenStore(dbPath)
	if err != nil {
		t.Fatalf("OpenStore failed: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Setup board
	_, _ = store.CreateBoard(ctx, "default", "Default Board", "")

	// 1. Create Work Item with dependency
	_, err = store.CreateWorkInBoard(ctx, "default", "t-dep", "Dependency task", nil)
	if err != nil {
		t.Fatalf("CreateWorkInBoard dep failed: %v", err)
	}
	item, err := store.CreateWorkInBoard(ctx, "default", "t-1", "Main task", []string{"t-dep"})
	if err != nil {
		t.Fatalf("CreateWorkInBoard failed: %v", err)
	}
	if item.ID != "t-1" || item.Status != WorkBacklog {
		t.Fatalf("expected item t-1 to be in backlog due to dependency, got status: %s", item.Status)
	}

	// 2. Complete dependency to unblock t-1
	claimDep, err := store.ClaimWork(ctx, "t-dep", "agent-dep", 5*time.Minute)
	if err != nil {
		t.Fatalf("ClaimWork dep failed: %v", err)
	}
	depClaimed, _ := store.GetWork(ctx, "t-dep")
	depReview, err := store.TransitionWork(ctx, "t-dep", claimDep.Token, depClaimed.Revision, WorkInReview)
	if err != nil {
		t.Fatalf("TransitionWork in_review failed: %v", err)
	}
	_, err = store.ApproveWork(ctx, "t-dep", "rev-1", "PASS", "evidence:1", depReview.Revision)
	if err != nil {
		t.Fatalf("ApproveWork dep failed: %v", err)
	}

	// Now t-1 should be ready
	t1, err := store.GetWork(ctx, "t-1")
	if err != nil {
		t.Fatalf("GetWork t-1 failed: %v", err)
	}
	if t1.Status != WorkReady {
		t.Fatalf("expected t-1 to become ready, got %s", t1.Status)
	}

	// 3. Claim Task t-1
	claim, err := store.ClaimWork(ctx, "t-1", "agent-impl-1", 5*time.Minute)
	if err != nil {
		t.Fatalf("ClaimWork failed: %v", err)
	}
	if claim.Token == "" {
		t.Fatal("expected non-empty claim token")
	}

	// 4. Renew Claim
	_, err = store.RenewWorkClaim(ctx, "t-1", claim.Token, 10*time.Minute)
	if err != nil {
		t.Fatalf("RenewWorkClaim failed: %v", err)
	}

	// 5. Reserve Lease
	lease, err := store.ReserveWorkLease(ctx, "t-1", claim.Token, "src/main.go", 5*time.Minute)
	if err != nil {
		t.Fatalf("ReserveWorkLease failed: %v", err)
	}
	if lease.Token == "" {
		t.Fatal("expected non-empty lease token")
	}

	// 6. Renew Lease
	_, err = store.RenewWorkLease(ctx, "src/main.go", lease.Token, 10*time.Minute)
	if err != nil {
		t.Fatalf("RenewWorkLease failed: %v", err)
	}

	// 6b. Extend Task Authority (claim + all leases)
	if err := store.ExtendTaskAuthority(ctx, "t-1", 15*time.Minute); err != nil {
		t.Fatalf("ExtendTaskAuthority failed: %v", err)
	}

	// 6c. Verify Work Lease against SQLite
	verified, err := store.VerifyWorkLease(ctx, "src/main.go", "t-1", "agent-impl-1")
	if err != nil || !verified.Valid {
		t.Fatalf("expected valid lease verification, got: %+v, err: %v", verified, err)
	}
	unverified, _ := store.VerifyWorkLease(ctx, "src/other.go", "t-1", "agent-impl-1")
	if unverified.Valid {
		t.Errorf("expected invalid lease for unleased file")
	}

	// 7. Transition to in_review with CAS
	itemClaimed, _ := store.GetWork(ctx, "t-1")
	itemReview, err := store.TransitionWork(ctx, "t-1", claim.Token, itemClaimed.Revision, WorkInReview)
	if err != nil {
		t.Fatalf("TransitionWork to in_review failed: %v", err)
	}

	// 8. Delivery has already released the lease atomically.
	if len(itemReview.Leases) != 0 {
		t.Fatal("transition retained file leases")
	}
	if err := store.ReleaseWorkLease(ctx, "src/main.go", lease.Token); err == nil {
		t.Fatal("released lease token remained usable")
	}

	// 9. Approve Task
	approval, err := store.ApproveWork(ctx, "t-1", "agent-rev-1", "PASS", "evidence:ok", itemReview.Revision)
	if err != nil {
		t.Fatalf("ApproveWork failed: %v", err)
	}
	if approval.Verdict != "PASS" {
		t.Errorf("expected PASS, got %s", approval.Verdict)
	}

	// 9b. ListWorkApprovals & LatestApproval
	approvals, err := store.ListWorkApprovals(ctx, "t-1")
	if err != nil {
		t.Fatalf("ListWorkApprovals failed: %v", err)
	}
	if len(approvals) != 1 || approvals[0].Verdict != "PASS" {
		t.Fatalf("expected 1 PASS approval, got %+v", approvals)
	}
	itemDone, err := store.GetWork(ctx, "t-1")
	if err != nil {
		t.Fatalf("GetWork failed: %v", err)
	}
	if itemDone.LatestApproval == nil || itemDone.LatestApproval.Verdict != "PASS" {
		t.Fatalf("expected LatestApproval PASS, got %+v", itemDone.LatestApproval)
	}

	// 9c. ComputeWorkFingerprint
	fp, err := store.ComputeWorkFingerprint(ctx, "t-1")
	if err != nil {
		t.Fatalf("ComputeWorkFingerprint failed: %v", err)
	}
	if fp.TaskID != "t-1" {
		t.Fatalf("unexpected fingerprint result: %+v", fp)
	}

	// 10. List Work
	workList, err := store.ListWork(ctx)
	if err != nil {
		t.Fatalf("ListWork failed: %v", err)
	}
	if len(workList) < 2 {
		t.Errorf("expected at least 2 work items, got %d", len(workList))
	}

	// 11. ListWorkByBoard
	boardWork, err := store.ListWorkByBoard(ctx, "default")
	if err != nil {
		t.Fatalf("ListWorkByBoard failed: %v", err)
	}
	if len(boardWork) < 2 {
		t.Errorf("expected at least 2 work items in board, got %d", len(boardWork))
	}

	// 12. Recover and Retry tests
	_, _ = store.RecoverWork(ctx)
	_, _ = store.RetryWork(ctx, "t-1", 4)
}

func TestOpenStoreReadOnly_ConcurrentWithImmediateTransaction(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "delegation.db")

	// Initialize writer store
	writer, err := OpenStore(dbPath)
	if err != nil {
		t.Fatalf("OpenStore failed: %v", err)
	}
	defer writer.Close()

	ctx := context.Background()
	_, err = writer.CreateBoard(ctx, "b-test", "Test Board", "")
	if err != nil {
		t.Fatalf("CreateBoard failed: %v", err)
	}

	// Open a second store in ReadOnly mode
	reader, err := OpenStoreReadOnly(dbPath)
	if err != nil {
		t.Fatalf("OpenStoreReadOnly failed: %v", err)
	}
	defer reader.Close()

	// Verify reader can query boards
	boards, err := reader.ListBoards(ctx)
	if err != nil || len(boards) == 0 {
		t.Fatalf("reader ListBoards failed: %v", err)
	}

	// Start an immediate transaction on writer
	conn, err := writer.db.Conn(ctx)
	if err != nil {
		t.Fatalf("writer db.Conn failed: %v", err)
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		t.Fatalf("BEGIN IMMEDIATE failed: %v", err)
	}

	// ReadOnly store should be able to read during WAL mode without hanging
	readDone := make(chan bool)
	go func() {
		b, err := reader.ListBoards(ctx)
		if err == nil && len(b) > 0 {
			readDone <- true
		} else {
			readDone <- false
		}
	}()

	select {
	case ok := <-readDone:
		if !ok {
			t.Errorf("reader failed to read during active transaction")
		}
	case <-time.After(2 * time.Second):
		t.Errorf("OpenStoreReadOnly was blocked by active immediate transaction (WAL query_only failed)")
	}

	_, _ = conn.ExecContext(ctx, "ROLLBACK")
}

func TestVerifySessionWorkLeases_Batch(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "delegation.db")

	store, err := OpenStore(dbPath)
	if err != nil {
		t.Fatalf("OpenStore failed: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	sessionID := "ses_batchtest123"
	workspace := tempDir

	_, _ = store.CreateBoard(ctx, "default", "Default", "")
	item, err := store.CreateWorkInBoardWithDefinition(ctx, "default", "task-batch", "Batch Task", nil, WorkDefinition{
		ConversationOwnership: ConversationOwnership{
			OpenCodeSessionID:     sessionID,
			OpenCodeRootSessionID: sessionID,
		},
		Project: workspace,
	})
	if err != nil {
		t.Fatalf("CreateWorkInBoardWithDefinition failed: %v", err)
	}

	claim, err := store.ClaimWork(ctx, item.ID, "opencode-session:"+sessionID, 10*time.Minute)
	if err != nil {
		t.Fatalf("ClaimWork failed: %v", err)
	}

	_, err = store.ReserveWorkLease(ctx, item.ID, claim.Token, "src/file1.go", 10*time.Minute)
	if err != nil {
		t.Fatalf("ReserveWorkLease file1 failed: %v", err)
	}
	_, err = store.ReserveWorkLease(ctx, item.ID, claim.Token, "src/file2.go", 10*time.Minute)
	if err != nil {
		t.Fatalf("ReserveWorkLease file2 failed: %v", err)
	}

	reader, err := OpenStoreReadOnly(dbPath)
	if err != nil {
		t.Fatalf("OpenStoreReadOnly failed: %v", err)
	}
	defer reader.Close()

	// Batch verify: file1 and file2 are valid, file3 is not leased
	results, err := reader.VerifySessionWorkLeases(ctx, []string{"src/file1.go", "src/file2.go", "src/unleased.go"}, workspace, sessionID)
	if err != nil {
		t.Fatalf("VerifySessionWorkLeases failed: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	if !results[0].Valid || results[0].Path != "src/file1.go" {
		t.Errorf("expected file1 valid, got: %+v", results[0])
	}
	if !results[1].Valid || results[1].Path != "src/file2.go" {
		t.Errorf("expected file2 valid, got: %+v", results[1])
	}
	if results[2].Valid || results[2].Path != "src/unleased.go" {
		t.Errorf("expected unleased invalid, got: %+v", results[2])
	}
}

func TestDecomposeWithReadOnlyGate(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "delegation.db")

	store, err := OpenStore(dbPath)
	if err != nil {
		t.Fatalf("OpenStore failed: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	sessionID := "ses_decomptest123"

	_, _ = store.CreateBoard(ctx, "test-board", "Test Board", "")
	item, err := store.CreateWorkInBoardWithDefinition(ctx, "test-board", "task-parent", "Parent Task", nil, WorkDefinition{
		ConversationOwnership: ConversationOwnership{
			OpenCodeSessionID:     sessionID,
			OpenCodeRootSessionID: sessionID,
		},
		Project:      tempDir,
		Objective:    "Implement feature with large changes",
		Acceptance:   "Pass all tests",
		Verification: "go test ./...",
		AllowedFiles: []string{"src/feature.go"},
	})
	if err != nil {
		t.Fatalf("CreateWorkInBoardWithDefinition failed: %v", err)
	}

	claim, err := store.ClaimWork(ctx, item.ID, "opencode-session:"+sessionID, 10*time.Minute)
	if err != nil {
		t.Fatalf("ClaimWork failed: %v", err)
	}

	itemClaimed, err := store.GetWork(ctx, item.ID)
	if err != nil {
		t.Fatalf("GetWork failed: %v", err)
	}

	blockedItem, err := store.TransitionWork(ctx, item.ID, claim.Token, itemClaimed.Revision, WorkBlocked)
	if err != nil {
		t.Fatalf("TransitionWork to blocked failed: %v", err)
	}

	steps := []WorkStepDefinition{
		{
			ID:           "task-child-impl",
			Title:        "Implement core changes",
			Objective:    "Implement logic",
			Acceptance:   "All core unit tests pass",
			Verification: "go test ./src/core",
			AllowedFiles: []string{"src/feature.go"},
		},
		{
			ID:           "task-child-gate",
			Title:        "Provenance and verification gate",
			Objective:    "Verify aggregate handoff and integration",
			Acceptance:   "All integration tests pass",
			Verification: "go test ./...",
			AllowedFiles: []string{}, // Read-only verification gate
		},
	}

	decomp, err := store.DecomposeWork(ctx, "task-parent", blockedItem.Revision, steps)
	if err != nil {
		t.Fatalf("DecomposeWork failed with read-only gate: %v", err)
	}
	if len(decomp.Children) != 2 {
		t.Fatalf("expected 2 children, got %d", len(decomp.Children))
	}
	if len(decomp.Children[1].AllowedFiles) != 0 {
		t.Errorf("expected child 1 (gate) to have empty allowed files, got %v", decomp.Children[1].AllowedFiles)
	}
}
