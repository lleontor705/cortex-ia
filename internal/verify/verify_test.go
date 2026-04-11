package verify

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/model"
	"github.com/lleontor705/cortex-ia/internal/state"
)

func TestRun_AllPass(t *testing.T) {
	checks := []Check{
		{ID: "a", Name: "Check A", Severity: SeverityError, Fn: func(*Context) error { return nil }},
		{ID: "b", Name: "Check B", Severity: SeverityWarning, Fn: func(*Context) error { return nil }},
	}
	report := Run(&Context{}, checks)
	if report.Passed != 2 || report.Failed != 0 || report.Warned != 0 {
		t.Errorf("expected 2 passed, got passed=%d failed=%d warned=%d", report.Passed, report.Failed, report.Warned)
	}
	if report.HasErrors() {
		t.Error("expected no errors")
	}
}

func TestRun_ErrorAndWarning(t *testing.T) {
	checks := []Check{
		{ID: "pass", Name: "OK", Severity: SeverityError, Fn: func(*Context) error { return nil }},
		{ID: "fail", Name: "Fail", Severity: SeverityError, Fn: func(*Context) error { return fmt.Errorf("broken") }},
		{ID: "warn", Name: "Warn", Severity: SeverityWarning, Fn: func(*Context) error { return fmt.Errorf("degraded") }},
	}
	report := Run(&Context{}, checks)
	if report.Passed != 1 {
		t.Errorf("expected 1 passed, got %d", report.Passed)
	}
	if report.Failed != 1 {
		t.Errorf("expected 1 failed, got %d", report.Failed)
	}
	if report.Warned != 1 {
		t.Errorf("expected 1 warned, got %d", report.Warned)
	}
	if !report.HasErrors() {
		t.Error("expected HasErrors=true")
	}
}

func TestRun_Empty(t *testing.T) {
	report := Run(&Context{}, nil)
	if report.Passed != 0 || report.Failed != 0 {
		t.Error("expected empty report")
	}
}

func TestCheckFilesExist_AllPresent(t *testing.T) {
	tmpDir := t.TempDir()
	f := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(f, []byte("ok"), 0o644)

	ctx := &Context{Lock: state.Lockfile{Files: []string{f}}}
	if err := checkFilesExist(ctx); err != nil {
		t.Errorf("expected pass, got: %v", err)
	}
}

func TestCheckFilesExist_Missing(t *testing.T) {
	ctx := &Context{Lock: state.Lockfile{Files: []string{"/nonexistent/file"}}}
	if err := checkFilesExist(ctx); err == nil {
		t.Error("expected error for missing file")
	}
}

func TestCheckSkillsPresent(t *testing.T) {
	tmpDir := t.TempDir()
	skillsDir := filepath.Join(tmpDir, ".cortex-ia", "skills")
	for _, id := range []string{"bootstrap", "implement", "validate", "architect", "investigate"} {
		os.MkdirAll(filepath.Join(skillsDir, id), 0o755)
		os.WriteFile(filepath.Join(skillsDir, id, "SKILL.md"), []byte("skill"), 0o644)
	}

	ctx := &Context{HomeDir: tmpDir}
	if err := checkSkillsPresent(ctx); err != nil {
		t.Errorf("expected pass, got: %v", err)
	}
}

func TestCheckSkillsPresent_Missing(t *testing.T) {
	ctx := &Context{HomeDir: t.TempDir()}
	if err := checkSkillsPresent(ctx); err == nil {
		t.Error("expected error for missing skills")
	}
}

func TestCheckConventionPresent(t *testing.T) {
	tmpDir := t.TempDir()
	convDir := filepath.Join(tmpDir, ".cortex-ia", "skills", "_shared")
	os.MkdirAll(convDir, 0o755)
	os.WriteFile(filepath.Join(convDir, "cortex-convention.md"), []byte("conv"), 0o644)

	ctx := &Context{HomeDir: tmpDir}
	if err := checkConventionPresent(ctx); err != nil {
		t.Errorf("expected pass, got: %v", err)
	}
}

func TestCheckConventionPresent_Missing(t *testing.T) {
	ctx := &Context{HomeDir: t.TempDir()}
	if err := checkConventionPresent(ctx); err == nil {
		t.Error("expected error for missing convention")
	}
}

func TestCheckStateLockConsistent_Match(t *testing.T) {
	ctx := &Context{
		State: state.State{InstalledAgents: []model.AgentID{"codex"}},
		Lock:  state.Lockfile{InstalledAgents: []model.AgentID{"codex"}},
	}
	if err := checkStateLockConsistent(ctx); err != nil {
		t.Errorf("expected pass, got: %v", err)
	}
}

func TestCheckStateLockConsistent_Mismatch(t *testing.T) {
	ctx := &Context{
		State: state.State{InstalledAgents: []model.AgentID{"codex"}},
		Lock:  state.Lockfile{InstalledAgents: []model.AgentID{"claude-code"}},
	}
	if err := checkStateLockConsistent(ctx); err == nil {
		t.Error("expected error for mismatch")
	}
}

func TestCheckStateLockConsistent_BothEmpty(t *testing.T) {
	ctx := &Context{}
	if err := checkStateLockConsistent(ctx); err != nil {
		t.Errorf("expected pass for both empty, got: %v", err)
	}
}

func TestCheckInstallStatus_NoFile(t *testing.T) {
	ctx := &Context{HomeDir: t.TempDir()}
	if err := checkInstallStatus(ctx); err != nil {
		t.Errorf("expected pass when no install status file, got: %v", err)
	}
}

func TestCheckInstallStatus_InProgress(t *testing.T) {
	tmpDir := t.TempDir()
	status := state.InstallStatus{
		Status:    "in-progress",
		StartedAt: "2026-04-10T12:00:00Z",
		BackupID:  "20260410-120000",
	}
	if err := state.SaveInstallStatus(tmpDir, status); err != nil {
		t.Fatalf("SaveInstallStatus() error: %v", err)
	}

	ctx := &Context{HomeDir: tmpDir}
	err := checkInstallStatus(ctx)
	if err == nil {
		t.Fatal("expected error for in-progress install status")
	}
	msg := err.Error()
	if !strings.Contains(msg, "did not complete cleanly") {
		t.Errorf("error message %q should mention incomplete install", msg)
	}
	if !strings.Contains(msg, "20260410-120000") {
		t.Errorf("error message %q should mention backup ID", msg)
	}
}

func TestCheckInstallStatus_Complete(t *testing.T) {
	tmpDir := t.TempDir()
	status := state.InstallStatus{
		Status:    "complete",
		StartedAt: "2026-04-10T12:00:00Z",
	}
	if err := state.SaveInstallStatus(tmpDir, status); err != nil {
		t.Fatalf("SaveInstallStatus() error: %v", err)
	}

	ctx := &Context{HomeDir: tmpDir}
	if err := checkInstallStatus(ctx); err != nil {
		t.Errorf("expected pass for complete status, got: %v", err)
	}
}
