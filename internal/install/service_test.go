package install

import (
	"path/filepath"
	"testing"
)

func TestInstallServiceLifecycle(t *testing.T) {
	tempHome := t.TempDir()

	svc, err := New(tempHome)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	if svc.HomeDir() != filepath.Clean(tempHome) {
		t.Errorf("HomeDir mismatch: %s vs %s", svc.HomeDir(), tempHome)
	}

	// 1. Plan on clean home
	plan, err := svc.Plan(DefaultOptions())
	if err != nil {
		t.Fatalf("Plan failed: %v", err)
	}
	if plan == nil {
		t.Fatal("expected non-nil plan")
	}

	// 2. Install (DryRun)
	dryOpts := DefaultOptions()
	dryOpts.DryRun = true
	dryReceipt, err := svc.Install(dryOpts)
	if err != nil {
		t.Fatalf("DryRun Install failed: %v", err)
	}
	if !dryReceipt.DryRun {
		t.Error("expected DryRun true in receipt")
	}

	// 3. Real Install
	opts := DefaultOptions()
	receipt, err := svc.Install(opts)
	if err != nil {
		t.Fatalf("Real Install failed: %v", err)
	}
	if receipt == nil {
		t.Fatal("expected non-nil receipt")
	}

	// 4. Sync
	syncReceipt, err := svc.Sync(opts)
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}
	if syncReceipt == nil {
		t.Fatal("expected non-nil sync receipt")
	}

	// 5. Doctor
	docResult, err := svc.Doctor()
	if err != nil {
		t.Fatalf("Doctor failed: %v", err)
	}
	if docResult == nil {
		t.Fatal("expected non-nil doctor result")
	}

	// 6. List Backups & Pending Journals
	backups, err := svc.ListBackups()
	if err != nil {
		t.Fatalf("ListBackups failed: %v", err)
	}
	_ = backups

	journals, err := svc.PendingJournals()
	if err != nil {
		t.Fatalf("PendingJournals failed: %v", err)
	}
	_ = journals

	// 7. MCP List
	mcpReport, err := svc.MCPList()
	if err != nil {
		t.Fatalf("MCPList failed: %v", err)
	}
	_ = mcpReport

	// 8. Uninstall (DryRun)
	uninstDry, err := svc.Uninstall(UninstallOptions{DryRun: true})
	if err != nil {
		t.Fatalf("Uninstall DryRun failed: %v", err)
	}
	if !uninstDry.DryRun {
		t.Error("expected DryRun true in uninstall receipt")
	}

	// 9. Real Uninstall
	uninstReceipt, err := svc.Uninstall(UninstallOptions{})
	if err != nil {
		t.Fatalf("Uninstall failed: %v", err)
	}
	if uninstReceipt == nil {
		t.Fatal("expected non-nil uninstall receipt")
	}
}

func TestInstallErrors(t *testing.T) {
	// Empty home rejection
	_, err := New("")
	if err == nil {
		t.Error("expected error for empty home")
	}
}
