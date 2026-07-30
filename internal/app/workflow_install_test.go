package app

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/backup"
	sddinstall "github.com/lleontor705/cortex-ia/internal/components/sdd/install"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
	"github.com/lleontor705/cortex-ia/internal/verify"
)

func TestWriteWorkflowInstallPlanDisclosesEveryEffectAndBlocker(t *testing.T) {
	plan := sddinstall.Plan{
		Profile:           "portable-sequential",
		Degradations:      []string{"delegation -> sequential"},
		Creates:           []sddinstall.Effect{{Path: "create.md", SemanticID: ir.SemanticID("asset.create"), AfterSHA256: "new-create"}},
		Updates:           []sddinstall.Effect{{Path: "update.md", SemanticID: ir.SemanticID("asset.update"), BeforeSHA256: "old-update", AfterSHA256: "new-update"}},
		Deletes:           []sddinstall.Effect{{Path: "delete.md", SemanticID: ir.SemanticID("asset.delete"), BeforeSHA256: "old-delete"}},
		PermissionChanges: []sddinstall.PermissionChange{{Path: "update.md", From: fs.FileMode(0o600), To: fs.FileMode(0o644)}},
		Conflicts:         []sddinstall.PlanConflict{{Path: "conflict.md", SemanticID: ir.SemanticID("asset.conflict"), State: sddinstall.OwnershipUnknown, Reason: "ownership metadata is absent", CurrentHash: "current", DesiredHash: "desired"}},
		Backup:            sddinstall.BackupScope{Required: true, Paths: []string{"delete.md", "update.md"}},
	}

	var output bytes.Buffer
	WriteWorkflowInstallPlan(&output, plan)
	got := output.String()
	for _, want := range []string{
		"Profile: portable-sequential",
		"Degradation: delegation -> sequential",
		"CREATE create.md semantic=asset.create after=new-create",
		"UPDATE update.md semantic=asset.update before=old-update after=new-update",
		"DELETE delete.md semantic=asset.delete before=old-delete",
		"PERMISSION update.md 0600 -> 0644",
		"BACKUP required: delete.md, update.md",
		"CONFLICT conflict.md semantic=asset.conflict state=unknown reason=ownership metadata is absent current=current desired=desired",
		"Install blocked: 1 conflict(s)",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q:\n%s", want, got)
		}
	}
}

func TestRunWorkflowInstallUsesExactPlanAndOffersExplicitRestoration(t *testing.T) {
	plan := sddinstall.Plan{
		Profile: "portable-flat",
		Creates: []sddinstall.Effect{{Path: "asset.md", SemanticID: ir.SemanticID("asset.create")}},
		Backup:  sddinstall.BackupScope{Required: true, Paths: []string{"asset.md"}},
	}
	wantErr := errors.New("write failed")
	applied := false
	command := WorkflowInstallCommand{
		Plan:   plan,
		Doctor: verify.DoctorReport{Profile: "portable-flat", Qualified: true},
		Apply: func(got sddinstall.Plan) (sddinstall.Receipt, error) {
			applied = true
			if got.Profile != plan.Profile || len(got.Creates) != len(plan.Creates) {
				t.Fatalf("Apply() plan = %+v, want exact planned value %+v", got, plan)
			}
			return sddinstall.Receipt{Backup: backup.Manifest{ID: "backup-42"}, BackupVerified: true, RestoreAvailable: true}, wantErr
		},
	}

	var output bytes.Buffer
	receipt, err := RunWorkflowInstall(&output, command)
	if !errors.Is(err, wantErr) {
		t.Fatalf("RunWorkflowInstall() error = %v, want %v", err, wantErr)
	}
	if !applied {
		t.Fatal("RunWorkflowInstall() did not apply the disclosed plan")
	}
	if !receipt.RestoreAvailable {
		t.Fatal("RunWorkflowInstall() discarded restoration evidence")
	}
	if got := output.String(); !strings.Contains(got, "Restoration available: cortex-ia rollback --backup backup-42") {
		t.Fatalf("failure output did not offer exact restoration command:\n%s", got)
	}
}

func TestRunWorkflowInstallBlocksOnDoctorFindingsBeforeApply(t *testing.T) {
	called := false
	var output bytes.Buffer
	_, err := RunWorkflowInstall(&output, WorkflowInstallCommand{
		Plan: sddinstall.Plan{Profile: "portable-sequential"},
		Doctor: verify.DoctorReport{Profile: "portable-sequential", Findings: []verify.Finding{{
			Code: verify.FindingManifest, Target: "opencode", Path: "manifest.json", Remediation: "regenerate bundle", Blocking: true,
		}}},
		Apply: func(sddinstall.Plan) (sddinstall.Receipt, error) {
			called = true
			return sddinstall.Receipt{}, nil
		},
	})
	if err == nil {
		t.Fatal("RunWorkflowInstall() accepted blocking doctor finding")
	}
	if called {
		t.Fatal("RunWorkflowInstall() applied after blocking doctor finding")
	}
	if got := output.String(); !strings.Contains(got, "doctor.manifest.consistency") || !strings.Contains(got, "remediation=regenerate bundle") {
		t.Fatalf("doctor finding was not surfaced before blocking:\n%s", got)
	}
}

func TestRunWorkflowInstallDryRunNeverApplies(t *testing.T) {
	called := false
	var output bytes.Buffer
	_, err := RunWorkflowInstall(&output, WorkflowInstallCommand{
		DryRun: true,
		Plan:   sddinstall.Plan{Profile: "portable-sequential"},
		Doctor: verify.DoctorReport{Profile: "portable-sequential", Qualified: true},
		Apply: func(sddinstall.Plan) (sddinstall.Receipt, error) {
			called = true
			return sddinstall.Receipt{}, nil
		},
	})
	if err != nil {
		t.Fatalf("RunWorkflowInstall(dry-run) error = %v", err)
	}
	if called {
		t.Fatal("dry-run invoked Apply")
	}
	if got := output.String(); !strings.Contains(got, "DRY RUN: zero persistent mutations") {
		t.Fatalf("dry-run disclosure missing:\n%s", got)
	}
}

func TestRunWorkflowInstallRejectsUnqualifiedOrMismatchedDoctorReport(t *testing.T) {
	for _, tt := range []struct {
		name   string
		report verify.DoctorReport
	}{
		{name: "unqualified", report: verify.DoctorReport{Profile: "portable-sequential"}},
		{name: "different profile", report: verify.DoctorReport{Profile: "native-advanced", Qualified: true}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			_, err := RunWorkflowInstall(&bytes.Buffer{}, WorkflowInstallCommand{
				Plan:   sddinstall.Plan{Profile: "portable-sequential"},
				Doctor: tt.report,
				Apply: func(sddinstall.Plan) (sddinstall.Receipt, error) {
					called = true
					return sddinstall.Receipt{}, nil
				},
			})
			if err == nil || called {
				t.Fatalf("RunWorkflowInstall() error = %v, apply called = %t", err, called)
			}
		})
	}
}

func TestWriteWorkflowDoctorAndExplicitRollback(t *testing.T) {
	report := verify.DoctorReport{
		Profile: "native-advanced",
		Findings: []verify.Finding{{
			Code: verify.FindingEvidenceStale, Severity: verify.SeverityWarning,
			Target: "opencode", Path: "catalog.json", Observed: "expired", Expected: "fresh",
			Evidence: "catalog#fact", Remediation: "run capability probe", Blocking: true,
		}},
	}
	var doctorOutput bytes.Buffer
	WriteWorkflowDoctor(&doctorOutput, report)
	for _, want := range []string{"Profile: native-advanced", "Qualified: false", "doctor.evidence.freshness", "observed=expired", "expected=fresh", "evidence=catalog#fact", "remediation=run capability probe", "blocking=true"} {
		if !strings.Contains(doctorOutput.String(), want) {
			t.Errorf("doctor output missing %q:\n%s", want, doctorOutput.String())
		}
	}

	called := false
	if _, err := RunWorkflowRollback(&bytes.Buffer{}, "", func(string) (sddinstall.RollbackResult, error) {
		called = true
		return sddinstall.RollbackResult{}, nil
	}); err == nil || called {
		t.Fatal("rollback without explicit backup selection should fail before execution")
	}

	var rollbackOutput bytes.Buffer
	result, err := RunWorkflowRollback(&rollbackOutput, "backup-42", func(id string) (sddinstall.RollbackResult, error) {
		called = true
		if id != "backup-42" {
			t.Fatalf("rollback selection = %q, want backup-42", id)
		}
		return sddinstall.RollbackResult{Restored: []string{"managed.md"}, DoctorPassed: true}, nil
	})
	if err != nil || !called || !result.DoctorPassed {
		t.Fatalf("RunWorkflowRollback() = %+v, %v", result, err)
	}
	for _, want := range []string{"Selected backup: backup-42", "RESTORED managed.md", "Doctor qualified restored bundle: true"} {
		if !strings.Contains(rollbackOutput.String(), want) {
			t.Errorf("rollback output missing %q:\n%s", want, rollbackOutput.String())
		}
	}
}

func TestCLIHasNoWorkflowExecutionOrMigrationCommands(t *testing.T) {
	for _, command := range []string{"run", "resume", "schedule", "migrate-state"} {
		if err := runCLI([]string{command}); err == nil {
			t.Errorf("runCLI(%q) unexpectedly accepted prohibited runtime command", command)
		}
	}
}

func TestCLIInstallDryRunPreservesTargetAtShippedBoundary(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)
	target := filepath.Join(homeDir, ".codex", "AGENTS.md")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	want := []byte("operator content\n")
	if err := os.WriteFile(target, want, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := runInstall([]string{"--agent", "codex", "--preset", "minimal", "--dry-run"}); err != nil {
		t.Fatalf("runInstall() dry-run error = %v", err)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("shipped CLI dry-run mutated target: got %q want %q", got, want)
	}
}
