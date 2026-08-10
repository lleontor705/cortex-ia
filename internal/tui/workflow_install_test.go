package tui

import (
	"errors"
	"io/fs"
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/agents"
	"github.com/lleontor705/cortex-ia/internal/agents/opencode"
	"github.com/lleontor705/cortex-ia/internal/backup"
	sddinstall "github.com/lleontor705/cortex-ia/internal/components/sdd/install"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
	"github.com/lleontor705/cortex-ia/internal/model"
	"github.com/lleontor705/cortex-ia/internal/pipeline"
	"github.com/lleontor705/cortex-ia/internal/verify"
)

func TestRenderWorkflowInstallReviewDisclosesPlanDoctorAndRestoration(t *testing.T) {
	view := RenderWorkflowInstallReview(WorkflowInstallReview{
		Plan: sddinstall.Plan{
			Profile:           "portable-sequential",
			Degradations:      []string{"parallel delegation -> sequential"},
			Creates:           []sddinstall.Effect{{Path: "create.md", SemanticID: ir.SemanticID("asset.create")}},
			Updates:           []sddinstall.Effect{{Path: "update.md", SemanticID: ir.SemanticID("asset.update")}},
			Deletes:           []sddinstall.Effect{{Path: "delete.md", SemanticID: ir.SemanticID("asset.delete")}},
			PermissionChanges: []sddinstall.PermissionChange{{Path: "update.md", From: fs.FileMode(0o600), To: fs.FileMode(0o644)}},
			Conflicts:         []sddinstall.PlanConflict{{Path: "conflict.md", State: sddinstall.OwnershipUnknown, Reason: "takeover required"}},
			Backup:            sddinstall.BackupScope{Required: true, Paths: []string{"update.md", "delete.md"}},
		},
		Doctor: verify.DoctorReport{Profile: "portable-sequential", Findings: []verify.Finding{{
			Code: verify.FindingOwnership, Target: "opencode", Path: "conflict.md", Remediation: "resolve ownership", Blocking: true,
		}}},
		InstallErr: errors.New("install stopped"),
		Receipt:    sddinstall.Receipt{Backup: backup.Manifest{ID: "backup-42"}, BackupVerified: true, RestoreAvailable: true},
	})

	for _, want := range []string{
		"Profile: portable-sequential", "Degradation: parallel delegation -> sequential",
		"Create: create.md", "Update: update.md", "Delete: delete.md",
		"Permission: update.md 0600 -> 0644", "Backup: update.md, delete.md",
		"Conflict: conflict.md (takeover required)", "Doctor: doctor.asset.ownership",
		"Install blocked", "Restoration available", "backup-42", "Rollback requires explicit selection",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("view missing %q:\n%s", want, view)
		}
	}
}

func TestTUIInstallCommandReachesShippedPipelineBoundary(t *testing.T) {
	registry := agents.NewRegistry()
	registry.Register(opencode.NewAdapter())
	m := New(registry, t.TempDir(), "test-v1")
	m.Agents = []AgentItem{{ID: model.AgentOpenCode, Selected: true}}
	m.Resolved = []model.ComponentID{model.ComponentSDD}
	m.SDDEnabled = true

	called := false
	m.ExecuteFn = func(selection model.Selection, progress pipeline.ProgressFunc) pipeline.InstallResult {
		called = true
		if len(selection.Agents) != 1 || selection.Agents[0] != model.AgentOpenCode {
			t.Fatalf("TUI selection agents = %v", selection.Agents)
		}
		if len(selection.Components) != 1 || selection.Components[0] != model.ComponentSDD {
			t.Fatalf("TUI selection components = %v", selection.Components)
		}
		return pipeline.InstallResult{ComponentsDone: selection.Components}
	}

	progress := make(chan StepProgressMsg, 1)
	message := m.runInstallWithProgress(progress)()
	if !called {
		t.Fatal("TUI install command bypassed the shipped pipeline boundary")
	}
	done, ok := message.(PipelineDoneMsg)
	if !ok || done.Err != nil {
		t.Fatalf("TUI install result = %#v", message)
	}
}
