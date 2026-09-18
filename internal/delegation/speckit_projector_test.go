package delegation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDetectHighRiskPaths(t *testing.T) {
	cases := []struct {
		paths    []string
		expected bool
	}{
		{paths: []string{"src/domain/user.go", "docs/readme.md"}, expected: false},
		{paths: []string{"src/auth/login.go"}, expected: true},
		{paths: []string{"src/crypto/token.go"}, expected: true},
		{paths: []string{"db/migrations/001_init.sql"}, expected: true},
		{paths: []string{"infra/secrets.yaml"}, expected: true},
		{paths: []string{"pkg/oauth/client.go"}, expected: true},
	}

	for _, c := range cases {
		got := DetectHighRiskPaths(c.paths)
		if got != c.expected {
			t.Errorf("DetectHighRiskPaths(%v) = %v; want %v", c.paths, got, c.expected)
		}
	}
}

func TestProjectSpecKitState_Lifecycle(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "delegation.db")
	store, err := OpenStore(dbPath)
	if err != nil {
		t.Fatalf("OpenStore failed: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	boardID := "board-speckit-test"
	changeID := "001-user-lockout"

	if _, err := store.CreateBoard(ctx, boardID, "SpecKit Test Board", ""); err != nil {
		t.Fatalf("CreateBoard failed: %v", err)
	}

	contract := &SDDContract{
		Version:   1,
		Workflow:  "sdd-lite",
		ChangeID:  changeID,
		SpecPlane: "speckit",
		Pins: []ContractPin{
			{
				Transport: "workspace_file",
				Project:   tempDir,
				Locator:   ".specify/specs/001-user-lockout/spec.md",
				SHA256:    strings.Repeat("a", 64),
			},
		},
		RequirementIDs: []string{"REQ-USER-001"},
	}

	// Create dummy spec file for pin verification
	specPath := filepath.Join(tempDir, ".specify", "specs", changeID, "spec.md")
	if err := os.MkdirAll(filepath.Dir(specPath), 0o755); err != nil {
		t.Fatalf("mkdir spec dir: %v", err)
	}
	specBytes := []byte("test spec content")
	if err := os.WriteFile(specPath, specBytes, 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	h := sha256.Sum256(specBytes)
	contract.Pins[0].SHA256 = hex.EncodeToString(h[:])

	def := WorkDefinition{
		Contract:     contract,
		Project:      tempDir,
		Objective:    "Implement lockout after 5 failed attempts",
		Acceptance:   "AC-001: user locked out",
		Verification: "go test ./...",
		AllowedFiles: []string{"src/auth/service.go"},
	}

	// 1. Create work item
	item, err := store.createWorkInBoardWithDefinition(ctx, tempDir, boardID, "TASK-001", "User Lockout", nil, def)
	if err != nil {
		t.Fatalf("createWorkInBoardWithDefinition failed: %v", err)
	}
	if item.ID != "TASK-001" {
		t.Fatalf("unexpected task ID: %s", item.ID)
	}

	// Verify state.yaml was generated
	statePath := filepath.Join(tempDir, ".specify", "specs", changeID, "state.yaml")
	stateData, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read state.yaml failed: %v", err)
	}
	if !strings.Contains(string(stateData), `feature_id: "001-user-lockout"`) {
		t.Errorf("state.yaml missing feature_id: %s", string(stateData))
	}

	// Verify tasks.md was generated
	tasksPath := filepath.Join(tempDir, ".specify", "specs", changeID, "tasks.md")
	tasksData, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatalf("read tasks.md failed: %v", err)
	}
	if !strings.Contains(string(tasksData), "TASK-001") {
		t.Errorf("tasks.md missing TASK-001: %s", string(tasksData))
	}

	// 2. Claim work item
	claim, err := store.ClaimWork(ctx, "TASK-001", "worker-1", 10*time.Minute)
	if err != nil {
		t.Fatalf("ClaimWork failed: %v", err)
	}

	// Verify active-task.json was generated
	activeTaskPath := filepath.Join(tempDir, ".specify", "runtime", "active-task.json")
	activeData, err := os.ReadFile(activeTaskPath)
	if err != nil {
		t.Fatalf("read active-task.json failed: %v", err)
	}
	if !strings.Contains(string(activeData), `"taskId": "TASK-001"`) {
		t.Errorf("active-task.json missing taskId: %s", string(activeData))
	}

	// 3. Transition to in_review
	item, err = store.TransitionWork(ctx, "TASK-001", claim.Token, claim.Revision, WorkInReview)
	if err != nil {
		t.Fatalf("TransitionWork failed: %v", err)
	}

	// 4. Approve work
	_, err = store.ApproveWork(ctx, "TASK-001", "reviewer-1", "PASS", "all tests pass", item.Revision)
	if err != nil {
		t.Fatalf("ApproveWork failed: %v", err)
	}

	// Verify state.yaml is now DONE
	stateData, _ = os.ReadFile(statePath)
	if !strings.Contains(string(stateData), `phase: "DONE"`) {
		t.Errorf("state.yaml not DONE: %s", string(stateData))
	}

	// Test validateSpecKitArchiveStructure
	taskIDs, err := validateSpecKitArchiveStructure(filepath.Join(tempDir, ".specify", "specs", changeID))
	if err != nil {
		t.Fatalf("validateSpecKitArchiveStructure failed: %v", err)
	}
	if len(taskIDs) != 1 || taskIDs[0] != "TASK-001" {
		t.Errorf("unexpected task IDs: %v", taskIDs)
	}

	// 5. Test ArchiveChange with speckit
	receipt, err := store.ArchiveChange(ctx, ArchiveOptions{
		BoardID:   boardID,
		Workspace: tempDir,
		ChangeID:  changeID,
		Workflow:  "sdd-lite",
		SpecPlane: "speckit",
	})
	if err != nil {
		t.Fatalf("ArchiveChange failed: %v", err)
	}
	if receipt.ArchiveID == "" {
		t.Errorf("expected non-empty ArchiveID")
	}

	// Verify the spec directory was moved to archive destination
	if _, err := os.Stat(receipt.Destination); err != nil {
		t.Errorf("archive destination does not exist: %v", err)
	}
}
