package delegation

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStoreAndJobsLifecycle(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "delegation.db")

	store, err := OpenStore(dbPath)
	if err != nil {
		t.Fatalf("OpenStore failed: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// 1. Create Job
	job, err := store.Create(ctx, NewJob{
		Role:            "implement",
		TaskID:          "task-123",
		ObjectiveDigest: "sha256:abc123456",
		Transport:       "direct",
		Workspace:       tempDir,
		Worktree:        filepath.Join(tempDir, "worktree"),
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if job.ID == "" || job.Status != StatusAccepted {
		t.Fatalf("unexpected job initial state: %+v", job)
	}

	// 2. Get Job
	fetched, err := store.Get(ctx, job.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if fetched.ID != job.ID || fetched.Role != "implement" {
		t.Errorf("fetched job mismatch: %+v", fetched)
	}

	// 3. Claim, SetPaneID and Mark Running
	if err := store.SetPaneID(ctx, job.ID, "%42"); err != nil {
		t.Fatalf("SetPaneID failed: %v", err)
	}
	fetchedWithPane, _ := store.Get(ctx, job.ID)
	if fetchedWithPane.PaneID != "%42" {
		t.Errorf("expected pane %%42, got %s", fetchedWithPane.PaneID)
	}
	if err := store.Claim(ctx, job.ID, "worker-1", 1234, 5*time.Minute); err != nil {
		t.Fatalf("Claim failed: %v", err)
	}
	if err := store.MarkRunning(ctx, job.ID); err != nil {
		t.Fatalf("MarkRunning failed: %v", err)
	}

	// 4. Block and Resume
	if err := store.MarkBlocked(ctx, job.ID, "waiting for confirmation"); err != nil {
		t.Fatalf("MarkBlocked failed: %v", err)
	}
	if err := store.MarkResumed(ctx, job.ID); err != nil {
		t.Fatalf("MarkResumed failed: %v", err)
	}

	// 5. Save Receipt and Complete
	receipt := Receipt{
		JobID:      job.ID,
		Status:     StatusSucceeded,
		Output:     json.RawMessage(`{"verdict":"PASS","summary":"done"}`),
		OutputHash: "sha256:1234",
		ExitCode:   0,
	}
	if err := store.Complete(ctx, job.ID, StatusSucceeded, receipt, "", ""); err != nil {
		t.Fatalf("Complete failed: %v", err)
	}

	// 6. Get Result
	savedReceipt, err := store.Result(ctx, job.ID)
	if err != nil {
		t.Fatalf("Result failed: %v", err)
	}
	if savedReceipt.Status != StatusSucceeded {
		t.Errorf("expected succeeded status, got %s", savedReceipt.Status)
	}

	// 7. Recover jobs
	recoveredCount, err := store.Recover(ctx)
	if err != nil {
		t.Fatalf("Recover failed: %v", err)
	}
	_ = recoveredCount

	// 8. Test Cancellation
	job2, err := store.Create(ctx, NewJob{
		Role:            "investigate",
		ObjectiveDigest: "sha256:def456",
		Transport:       "direct",
		Workspace:       tempDir,
	})
	if err != nil {
		t.Fatalf("Create job2 failed: %v", err)
	}
	if err := store.Cancel(ctx, job2.ID); err != nil {
		t.Fatalf("Cancel failed: %v", err)
	}
}

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

	// 5. List Delegations & Activity
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

	// 8. Release Lease
	if err := store.ReleaseWorkLease(ctx, "src/main.go", lease.Token); err != nil {
		t.Fatalf("ReleaseWorkLease failed: %v", err)
	}

	// 9. Approve Task
	approval, err := store.ApproveWork(ctx, "t-1", "agent-rev-1", "PASS", "evidence:ok", itemReview.Revision)
	if err != nil {
		t.Fatalf("ApproveWork failed: %v", err)
	}
	if approval.Verdict != "PASS" {
		t.Errorf("expected PASS, got %s", approval.Verdict)
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

func TestRunnerValidationAndPrompts(t *testing.T) {
	tempDir := t.TempDir()
	wt := filepath.Join(tempDir, "wt")
	_ = os.MkdirAll(wt, 0755)
	_ = os.WriteFile(filepath.Join(wt, ".git"), []byte("gitdir: test\n"), 0600)

	// 1. Valid request
	validReq := Request{
		Role:          "implement",
		TaskID:        "t-valid",
		Objective:     "Implement test feature",
		Workspace:     tempDir,
		Worktree:      wt,
		WorkspaceMode: WorkspaceIsolated,
		AllowedFiles:  []string{"src/main.go"},
	}
	if err := validReq.Validate(); err != nil {
		t.Fatalf("expected valid request, got: %v", err)
	}

	// 2. Invalid role
	invalidRole := validReq
	invalidRole.Role = "unsupported_hacker"
	if err := invalidRole.Validate(); err == nil {
		t.Error("expected error for invalid role")
	}

	// 3. Implement role requires an isolated worktree and writable paths.
	noWorktree := validReq
	noWorktree.Worktree = ""
	if err := noWorktree.Validate(); err == nil {
		t.Error("expected error for implement role without worktree")
	}

	noAllowedFiles := validReq
	noAllowedFiles.AllowedFiles = nil
	if err := noAllowedFiles.Validate(); err == nil {
		t.Error("expected error for implement role without allowed files")
	}

	// 4. Current workspace strategy validation
	currentReq := Request{
		Role:          "implement",
		TaskID:        "t-valid",
		Objective:     "Implement test feature",
		Workspace:     tempDir,
		WorkspaceMode: WorkspaceCurrent,
		AllowedFiles:  []string{"src/main.go"},
	}
	if err := currentReq.Validate(); err != nil {
		t.Fatalf("expected valid current_workspace request, got: %v", err)
	}
	currentReqWithWorktree := currentReq
	currentReqWithWorktree.Worktree = wt
	if err := currentReqWithWorktree.Validate(); err == nil {
		t.Error("expected error for current_workspace with worktree specified")
	}

	// 5. File reading & JSON validation
	reqFile := filepath.Join(tempDir, "request.json")
	data, _ := json.Marshal(validReq)
	_ = os.WriteFile(reqFile, data, 0600)

	parsed, err := ReadRequest(reqFile)
	if err != nil {
		t.Fatalf("ReadRequest failed: %v", err)
	}
	if parsed.TaskID != "t-valid" {
		t.Errorf("task ID mismatch: %s", parsed.TaskID)
	}
}
