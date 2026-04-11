package pipeline

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/lleontor705/cortex-ia/internal/agents"
	"github.com/lleontor705/cortex-ia/internal/backup"
	"github.com/lleontor705/cortex-ia/internal/model"
	"github.com/lleontor705/cortex-ia/internal/state"
)

// backupStep creates a config snapshot before installation.
type backupStep struct {
	homeDir  string
	registry *agents.Registry
	agentIDs []model.AgentID
	resolved []model.ComponentID
	version  string
	progress ProgressFunc

	// Output: set during Run().
	BackupID  string
	BackupDir string
}

func (s *backupStep) Name() string { return "backup" }

func (s *backupStep) Run() error {
	s.BackupDir = filepath.Join(s.homeDir, ".cortex-ia", "backups", time.Now().Format("20060102-150405"))
	snap := backup.NewSnapshotter()

	paths := collectBackupPaths(s.homeDir, s.registry, s.agentIDs, s.resolved)
	manifest, err := snap.Create(s.BackupDir, paths)
	if err != nil {
		return fmt.Errorf("backup: %w", err)
	}
	manifest.Source = backup.BackupSourceInstall
	manifest.CreatedByVersion = s.version
	if err := backup.WriteManifest(filepath.Join(s.BackupDir, backup.ManifestFilename), manifest); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}
	s.BackupID = manifest.ID
	if s.progress == nil {
		fmt.Printf("Backup created: %s (%d files)\n", manifest.ID, manifest.FileCount)
	}
	return nil
}

// validateStep checks that all requested agents exist in the registry.
type validateStep struct {
	registry *agents.Registry
	agentIDs []model.AgentID
}

func (s *validateStep) Name() string { return "validate-agents" }

func (s *validateStep) Run() error {
	for _, id := range s.agentIDs {
		if _, err := s.registry.Get(id); err != nil {
			return fmt.Errorf("unknown agent %q", id)
		}
	}
	return nil
}

// componentStep applies one component to one agent.
type componentStep struct {
	homeDir     string
	adapter     agents.Adapter
	componentID model.ComponentID
	injectorFn  func() ([]string, error)
	progress    ProgressFunc

	// Output: files written.
	Files []string
}

func (s *componentStep) Name() string {
	return fmt.Sprintf("%s/%s", s.adapter.Agent(), s.componentID)
}

func (s *componentStep) Run() error {
	if s.progress != nil {
		s.progress(s.Name(), "running", nil)
	}
	files, err := s.injectorFn()
	if err != nil {
		if s.progress != nil {
			s.progress(s.Name(), "failed", err)
		}
		if s.progress == nil {
			fmt.Printf("  [!] %s/%s: %v\n", s.adapter.Agent(), s.componentID, err)
		}
		return err
	}
	s.Files = files
	if s.progress != nil {
		s.progress(s.Name(), "succeeded", nil)
	}
	if s.progress == nil {
		fmt.Printf("  [+] %s\n", s.componentID)
	}
	return nil
}

// Ensure backupStep output dir can be cleaned up on rollback.
func (s *backupStep) Rollback() error {
	if s.BackupDir != "" {
		return os.RemoveAll(s.BackupDir)
	}
	return nil
}

// installStatusStep writes an install-status marker so that partial failures
// can be detected by the doctor/verify system. On Run() it writes
// status "in-progress"; the caller is responsible for updating to "complete"
// after all components succeed. Rollback() clears the marker.
type installStatusStep struct {
	homeDir  string
	backupID string // set by the caller after backupStep.Run()
}

func (s *installStatusStep) Name() string { return "install-status" }

func (s *installStatusStep) Run() error {
	status := state.InstallStatus{
		Status:    "in-progress",
		StartedAt: time.Now().UTC().Format(time.RFC3339),
		BackupID:  s.backupID,
	}
	return state.SaveInstallStatus(s.homeDir, status)
}

func (s *installStatusStep) Rollback() error {
	return state.ClearInstallStatus(s.homeDir)
}
