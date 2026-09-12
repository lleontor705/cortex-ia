package delegation

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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

	// 1. Valid request using current_workspace
	validReq := Request{
		Role:          "implement",
		TaskID:        "t-valid",
		Objective:     "Implement test feature",
		Workspace:     tempDir,
		WorkspaceMode: WorkspaceCurrent,
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

	// 3. Implement role rejects isolated_worktree with actionable retirement error
	isolatedReq := validReq
	isolatedReq.WorkspaceMode = WorkspaceIsolated
	isolatedReq.Worktree = wt
	if err := isolatedReq.Validate(); err == nil || !strings.Contains(err.Error(), "isolated_worktree strategy is retired; use current_workspace") {
		t.Fatalf("expected isolated_worktree retirement error, got: %v", err)
	}

	// 4. Implement role requires allowed files
	noAllowedFiles := validReq
	noAllowedFiles.AllowedFiles = nil
	if err := noAllowedFiles.Validate(); err == nil {
		t.Error("expected error for implement role without allowed files")
	}

	// 5. Current workspace strategy with worktree specified must fail
	currentReqWithWorktree := validReq
	currentReqWithWorktree.Worktree = wt
	if err := currentReqWithWorktree.Validate(); err == nil || !strings.Contains(err.Error(), "current_workspace strategy must not include worktree") {
		t.Fatalf("expected error for current_workspace with worktree specified, got: %v", err)
	}

	// 6. Implement role requires explicit workspace strategy
	emptyStrategy := validReq
	emptyStrategy.WorkspaceMode = ""
	if err := emptyStrategy.Validate(); err == nil || !strings.Contains(err.Error(), "implement delegation requires explicit workspace_strategy") {
		t.Fatalf("expected error for missing workspace strategy, got: %v", err)
	}

	// 7. File reading & JSON validation
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

	// 8. Investigate role allows absolute paths and outside workspace paths in allowed files
	investigateReq := Request{
		Role:         "investigate",
		Objective:    "Inspect contract",
		Workspace:    tempDir,
		AllowedFiles: []string{"C:/contracts/workflow-map.md", filepath.Join(tempDir, "relative.md")},
	}
	if err := investigateReq.Validate(); err != nil {
		t.Fatalf("expected investigate role to allow paths in allowed files, got: %v", err)
	}
	if len(investigateReq.AllowedFiles) != 2 || investigateReq.AllowedFiles[1] != "relative.md" {
		t.Errorf("expected relative.md to be normalized, got: %v", investigateReq.AllowedFiles)
	}

	// 9. Implement role normalizes absolute paths inside workspace
	implementAbsReq := validReq
	implementAbsReq.AllowedFiles = []string{filepath.Join(tempDir, "src", "main.go")}
	if err := implementAbsReq.Validate(); err != nil {
		t.Fatalf("expected implement role to accept and normalize absolute path inside workspace, got: %v", err)
	}
	if implementAbsReq.AllowedFiles[0] != "src/main.go" {
		t.Errorf("expected normalized src/main.go, got %s", implementAbsReq.AllowedFiles[0])
	}

	// 10. Implement role rejects paths outside workspace
	implementOutsideReq := validReq
	implementOutsideReq.AllowedFiles = []string{"/outside/file.go"}
	if runtime.GOOS == "windows" {
		implementOutsideReq.AllowedFiles = []string{"C:\\outside\\file.go"}
		if strings.HasPrefix(strings.ToLower(tempDir), "c:") {
			implementOutsideReq.AllowedFiles = []string{"Z:\\outside\\file.go"}
		}
	}
	if err := implementOutsideReq.Validate(); err == nil {
		t.Error("expected error for implement role with path outside workspace")
	}
}

func TestRequestModelAndEffortValidation(t *testing.T) {
	tempDir := t.TempDir()
	req := Request{
		Role:      "investigate",
		Objective: "Diagnose system issue",
		Workspace: tempDir,
		Model:     "custom-dynamic-model",
		Effort:    "high",
	}
	if err := req.Validate(); err != nil {
		t.Fatalf("expected valid request with custom model and effort, got: %v", err)
	}

	req.Effort = "invalid-effort"
	if err := req.Validate(); err == nil {
		t.Error("expected error for invalid effort")
	}

	req.Effort = "low"
	if err := req.Validate(); err != nil {
		t.Errorf("expected valid low effort, got: %v", err)
	}
}

func TestParseModelsOutput(t *testing.T) {
	sampleOutput := []byte(`
Fetching available models...
gemini-3.8-flash-high	Gemini 3.8 Flash (High)
gemini-3.7-flash-medium	Gemini 3.7 Flash (Medium)
claude-sonnet-4-6       Claude Sonnet 4.6 (Thinking)
`)
	models := ParseModelsOutput(sampleOutput)
	if len(models) != 3 {
		t.Fatalf("expected 3 models, got %d", len(models))
	}
	if models[0].ID != "gemini-3.8-flash-high" || models[0].Name != "Gemini 3.8 Flash (High)" {
		t.Errorf("unexpected model 0: %+v", models[0])
	}
	if models[1].ID != "gemini-3.7-flash-medium" || models[1].Name != "Gemini 3.7 Flash (Medium)" {
		t.Errorf("unexpected model 1: %+v", models[1])
	}
	if models[2].ID != "claude-sonnet-4-6" || models[2].Name != "Claude Sonnet 4.6 (Thinking)" {
		t.Errorf("unexpected model 2: %+v", models[2])
	}
}

func TestBuildAGYArgs_ModelNotSelectableByAgent(t *testing.T) {
	// 1. role has a specific model configured in TUI; agent request sends a different model.
	role := RoleConfig{
		Delegate: true,
		CLI:      "agy",
		Mode:     "accept-edits",
		Model:    "gemini-3.7-flash-high",
		Effort:   "medium",
	}
	req := Request{
		Role:      "implement",
		Objective: "Test task",
		Workspace: "/test/workspace",
		Model:     "agent-chosen-model-should-be-ignored",
		Effort:    "low",
	}
	args := buildAGYArgs(req, role, "1h", "/test/workspace", false)

	// Verify --model uses role.Model ("gemini-3.7-flash-high"), NOT req.Model
	foundModel := ""
	foundEffort := ""
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "--model" {
			foundModel = args[i+1]
		}
		if args[i] == "--effort" {
			foundEffort = args[i+1]
		}
	}
	if foundModel != "gemini-3.7-flash-high" {
		t.Fatalf("expected --model to be role model 'gemini-3.7-flash-high', got %q (args: %v)", foundModel, args)
	}
	if foundEffort != "medium" {
		t.Fatalf("expected --effort to be role effort 'medium', got %q (args: %v)", foundEffort, args)
	}

	// 2. role has empty model; should default to DefaultAGYModel.
	roleEmpty := RoleConfig{
		Delegate: true,
		CLI:      "agy",
	}
	argsDefault := buildAGYArgs(req, roleEmpty, "1h", "/test/workspace", false)
	foundModelDefault := ""
	for i := 0; i < len(argsDefault)-1; i++ {
		if argsDefault[i] == "--model" {
			foundModelDefault = argsDefault[i+1]
		}
	}
	if foundModelDefault != DefaultAGYModel {
		t.Fatalf("expected --model to fallback to DefaultAGYModel %q, got %q", DefaultAGYModel, foundModelDefault)
	}
}

func TestDelegationAllFourRoles(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "delegation.db")
	store, err := OpenStore(dbPath)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	workspace, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working dir: %v", err)
	}

	// Setup board and task for implement
	_, _ = store.CreateBoard(ctx, "default", "Default Board", "")
	_, err = store.CreateWorkInBoard(ctx, "default", "task-impl", "Implement task", nil)
	if err != nil {
		t.Fatalf("failed to create work item: %v", err)
	}
	claim, err := store.ClaimWork(ctx, "task-impl", "implement-agent", 5*time.Minute)
	if err != nil {
		t.Fatalf("failed to claim work item: %v", err)
	}
	_, err = store.ReserveWorkLease(ctx, "task-impl", claim.Token, "pkg/foo.go", 5*time.Minute)
	if err != nil {
		t.Fatalf("failed to reserve lease: %v", err)
	}

	testCases := []struct {
		role              string
		taskID            string
		allowedFiles      []string
		workspaceStrategy string
		expectedMode      string
	}{
		{
			role:              "implement",
			taskID:            "task-impl",
			allowedFiles:      []string{"pkg/foo.go"},
			workspaceStrategy: "current_workspace",
			expectedMode:      "accept-edits",
		},
		{
			role:              "investigate",
			taskID:            "",
			allowedFiles:      nil,
			workspaceStrategy: "",
			expectedMode:      "plan",
		},
		{
			role:              "planner",
			taskID:            "",
			allowedFiles:      nil,
			workspaceStrategy: "",
			expectedMode:      "plan",
		},
		{
			role:              "reviewer",
			taskID:            "task-impl",
			allowedFiles:      nil,
			workspaceStrategy: "",
			expectedMode:      "plan",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.role, func(t *testing.T) {
			req := Request{
				Role:          tc.role,
				TaskID:        tc.taskID,
				Objective:     "Execute objective for " + tc.role,
				Workspace:     workspace,
				WorkspaceMode: tc.workspaceStrategy,
				AllowedFiles:  tc.allowedFiles,
			}
			if err := req.Validate(); err != nil {
				t.Fatalf("request validation failed for role %s: %v", tc.role, err)
			}

			job, err := CreateFromRequest(ctx, store, req, "direct")
			if err != nil {
				t.Fatalf("failed to create job for role %s: %v", tc.role, err)
			}
			if job.Role != tc.role {
				t.Fatalf("expected job role %s, got %s", tc.role, job.Role)
			}
			if job.Status != StatusAccepted {
				t.Fatalf("expected job status accepted, got %s", job.Status)
			}

			// Verify AGY args building for this role
			roleCfg := RoleConfig{
				Delegate: true,
				CLI:      "agy",
				Mode:     tc.expectedMode,
				Model:    DefaultAGYModel,
			}
			args := buildAGYArgs(req, roleCfg, "1h", workspace, true)
			hasModel := false
			hasMode := false
			for i := 0; i < len(args)-1; i++ {
				if args[i] == "--model" && args[i+1] == DefaultAGYModel {
					hasModel = true
				}
				if args[i] == "--mode" && args[i+1] == tc.expectedMode {
					hasMode = true
				}
			}
			if !hasModel {
				t.Errorf("args for %s missing --model %s: %v", tc.role, DefaultAGYModel, args)
			}
			if tc.expectedMode != "plan" && !hasMode {
				t.Errorf("args for %s missing --mode %s: %v", tc.role, tc.expectedMode, args)
			}
		})
	}
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
