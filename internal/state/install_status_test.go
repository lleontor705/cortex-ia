package state

import (
	"path/filepath"
	"testing"
)

func TestSaveAndLoadInstallStatus(t *testing.T) {
	homeDir := t.TempDir()

	status := InstallStatus{
		Status:    "in-progress",
		StartedAt: "2026-04-10T12:00:00Z",
		BackupID:  "20260410-120000",
	}

	if err := SaveInstallStatus(homeDir, status); err != nil {
		t.Fatalf("SaveInstallStatus() error: %v", err)
	}

	loaded, err := LoadInstallStatus(homeDir)
	if err != nil {
		t.Fatalf("LoadInstallStatus() error: %v", err)
	}
	if loaded == nil {
		t.Fatal("LoadInstallStatus() returned nil, want non-nil")
	}
	if loaded.Status != "in-progress" {
		t.Errorf("Status = %q, want %q", loaded.Status, "in-progress")
	}
	if loaded.StartedAt != "2026-04-10T12:00:00Z" {
		t.Errorf("StartedAt = %q, want %q", loaded.StartedAt, "2026-04-10T12:00:00Z")
	}
	if loaded.BackupID != "20260410-120000" {
		t.Errorf("BackupID = %q, want %q", loaded.BackupID, "20260410-120000")
	}
}

func TestLoadInstallStatus_NoFile(t *testing.T) {
	homeDir := t.TempDir()

	loaded, err := LoadInstallStatus(homeDir)
	if err != nil {
		t.Fatalf("LoadInstallStatus() error: %v", err)
	}
	if loaded != nil {
		t.Errorf("LoadInstallStatus() = %+v, want nil for missing file", loaded)
	}
}

func TestClearInstallStatus(t *testing.T) {
	homeDir := t.TempDir()

	status := InstallStatus{
		Status:    "in-progress",
		StartedAt: "2026-04-10T12:00:00Z",
	}
	if err := SaveInstallStatus(homeDir, status); err != nil {
		t.Fatalf("SaveInstallStatus() error: %v", err)
	}

	if err := ClearInstallStatus(homeDir); err != nil {
		t.Fatalf("ClearInstallStatus() error: %v", err)
	}

	loaded, err := LoadInstallStatus(homeDir)
	if err != nil {
		t.Fatalf("LoadInstallStatus() after clear error: %v", err)
	}
	if loaded != nil {
		t.Errorf("LoadInstallStatus() after clear = %+v, want nil", loaded)
	}
}

func TestClearInstallStatus_NoFile(t *testing.T) {
	homeDir := t.TempDir()

	// Clearing when no file exists should not error.
	if err := ClearInstallStatus(homeDir); err != nil {
		t.Errorf("ClearInstallStatus() with no file error: %v", err)
	}
}

func TestInstallStatusPath(t *testing.T) {
	got := InstallStatusPath("/home/user")
	want := filepath.Join("/home/user", ".cortex-ia", "install-status.json")
	if got != want {
		t.Errorf("InstallStatusPath() = %q, want %q", got, want)
	}
}

func TestSaveInstallStatus_OverwritesExisting(t *testing.T) {
	homeDir := t.TempDir()

	first := InstallStatus{Status: "in-progress", StartedAt: "2026-04-10T12:00:00Z"}
	if err := SaveInstallStatus(homeDir, first); err != nil {
		t.Fatalf("SaveInstallStatus() first error: %v", err)
	}

	second := InstallStatus{Status: "complete", StartedAt: "2026-04-10T12:00:00Z"}
	if err := SaveInstallStatus(homeDir, second); err != nil {
		t.Fatalf("SaveInstallStatus() second error: %v", err)
	}

	loaded, err := LoadInstallStatus(homeDir)
	if err != nil {
		t.Fatalf("LoadInstallStatus() error: %v", err)
	}
	if loaded.Status != "complete" {
		t.Errorf("Status = %q, want %q after overwrite", loaded.Status, "complete")
	}
}
