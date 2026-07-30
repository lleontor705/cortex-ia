package pipeline

import (
	"os"
	"path/filepath"
	"testing"

	sddinstall "github.com/lleontor705/cortex-ia/internal/components/sdd/install"
)

func TestPlannedInstallReceiptOffersVerifiedRestoration(t *testing.T) {
	root := t.TempDir()
	backupRoot := filepath.Join(t.TempDir(), "backups")
	target := filepath.Join(root, "agents", "implement.md")
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("before\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	receipt, err := ApplyPlannedInstall(root, backupRoot, sddinstall.Plan{
		Updates: []sddinstall.Effect{{Path: "agents/implement.md", Content: []byte("after\n"), AfterMode: 0o600}},
		Backup:  sddinstall.BackupScope{Required: true, Paths: []string{"agents/implement.md"}},
	})
	if err != nil {
		t.Fatalf("ApplyPlannedInstall() error = %v", err)
	}
	if !receipt.BackupVerified || !receipt.RestoreAvailable {
		t.Fatalf("receipt = %+v, want verified restoration", receipt)
	}
	if err := RestorePlannedInstall(receipt); err != nil {
		t.Fatalf("RestorePlannedInstall() error = %v", err)
	}
	content, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "before\n" {
		t.Fatalf("restored content = %q, want before", content)
	}
}

func TestRestorePlannedInstallRunsDoctorOnRestoredBundle(t *testing.T) {
	root := t.TempDir()
	backupRoot := filepath.Join(t.TempDir(), "backups")
	target := filepath.Join(root, "agents", "implement.md")
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("prior\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	receipt, err := ApplyPlannedInstall(root, backupRoot, sddinstall.Plan{
		Updates: []sddinstall.Effect{{Path: "agents/implement.md", SemanticID: "asset/agent/implement", Content: []byte("installed\n"), AfterMode: 0o600}},
		Backup:  sddinstall.BackupScope{Required: true, Paths: []string{"agents/implement.md"}},
	})
	if err != nil {
		t.Fatalf("ApplyPlannedInstall() error = %v", err)
	}
	doctorCalled := false
	result, err := RestorePlannedInstallWithDoctor(receipt, func() error {
		doctorCalled = true
		content, readErr := os.ReadFile(target)
		if readErr != nil {
			return readErr
		}
		if string(content) != "prior\n" {
			t.Fatalf("doctor saw content = %q, want prior", content)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("RestorePlannedInstallWithDoctor() error = %v", err)
	}
	if !doctorCalled || !result.DoctorPassed {
		t.Fatalf("doctorCalled = %v, result = %+v", doctorCalled, result)
	}
}
