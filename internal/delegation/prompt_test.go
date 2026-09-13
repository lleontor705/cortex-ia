package delegation

import (
	"runtime"
	"strings"
	"testing"
)

func TestExternalPrompt_ImplementRole(t *testing.T) {
	req := Request{
		Role:          "implement",
		TaskID:        "task-auth-01",
		Workspace:     "/repo/app",
		WorkspaceMode: WorkspaceCurrent,
		AllowedFiles:  []string{"internal/auth/auth.go", "internal/auth/auth_test.go"},
		Objective:     "Add token authentication middleware\n\nAcceptance checks:\n- go test ./internal/auth/...",
	}

	prompt := externalPrompt(req)

	// 1. Authority boundary and environment
	if !strings.Contains(prompt, "Autonomous Leaf Execution Task") {
		t.Errorf("expected header Autonomous Leaf Execution Task in prompt")
	}
	if !strings.Contains(prompt, "Authority Boundary") {
		t.Errorf("expected Authority Boundary in prompt")
	}
	if !strings.Contains(prompt, runtime.GOOS) {
		t.Errorf("expected runtime.GOOS %q in prompt", runtime.GOOS)
	}

	// 2. Task context and allowed files
	if !strings.Contains(prompt, "- **Assigned Role**: `implement`") {
		t.Errorf("expected assigned role in prompt")
	}
	if !strings.Contains(prompt, "- **Task ID**: `task-auth-01`") {
		t.Errorf("expected task ID in prompt")
	}
	if !strings.Contains(prompt, "- `internal/auth/auth.go`") || !strings.Contains(prompt, "- `internal/auth/auth_test.go`") {
		t.Errorf("expected allowed files in prompt")
	}

	// 3. Blast radius callout for implement
	if !strings.Contains(prompt, "Blast Radius Enforcement") {
		t.Errorf("expected Blast Radius Enforcement callout for implement role")
	}
	if !strings.Contains(prompt, "status: \"blocked\"") {
		t.Errorf("expected status blocked guidance in prompt")
	}

	// 4. Execution protocol for implement
	if !strings.Contains(prompt, "Execute Verification") {
		t.Errorf("expected Execute Verification step in implement protocol")
	}
	if !strings.Contains(prompt, "Surgical Edits") {
		t.Errorf("expected Surgical Edits step in implement protocol")
	}
}

func TestExternalPrompt_ReviewerRole(t *testing.T) {
	req := Request{
		Role:         "reviewer",
		TaskID:       "task-review-02",
		Workspace:    "/repo/app",
		AllowedFiles: []string{"internal/auth/auth.go"},
		Objective:    "Perform independent audit on auth changes",
	}

	prompt := externalPrompt(req)

	if !strings.Contains(prompt, "- **Assigned Role**: `reviewer`") {
		t.Errorf("expected assigned role reviewer")
	}
	if !strings.Contains(prompt, "Read-Only Scope") {
		t.Errorf("expected Read-Only Scope callout for reviewer")
	}
	if strings.Contains(prompt, "Blast Radius Enforcement") {
		t.Errorf("did not expect Blast Radius Enforcement edit callout for reviewer")
	}
	if !strings.Contains(prompt, "Independent Verification") {
		t.Errorf("expected Independent Verification protocol for reviewer")
	}
}

func TestExternalPrompt_InvestigateRole_EmptyFiles(t *testing.T) {
	req := Request{
		Role:      "investigate",
		TaskID:    "task-inv-03",
		Workspace: "/repo/app",
		Objective: "Investigate database deadlock",
	}

	prompt := externalPrompt(req)

	if !strings.Contains(prompt, "- **Assigned Role**: `investigate`") {
		t.Errorf("expected assigned role investigate")
	}
	if !strings.Contains(prompt, "Read-Only Scope") {
		t.Errorf("expected Read-Only Scope for investigate role with empty files")
	}
	if !strings.Contains(prompt, "Targeted Diagnosis") {
		t.Errorf("expected Targeted Diagnosis protocol for investigate")
	}
}
