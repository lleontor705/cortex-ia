package pipeline

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/lleontor705/cortex-ia/internal/agents"
	"github.com/lleontor705/cortex-ia/internal/backup"
	"github.com/lleontor705/cortex-ia/internal/model"
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
	writer      *preparedWriter

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
	var files []string
	run := func() error {
		var err error
		files, err = s.injectorFn()
		return err
	}
	var err error
	if s.writer != nil {
		err = s.writer.run(run)
	} else {
		err = run()
	}
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

// preparedWriter wraps a post-backup mutation. Its targets are relative to the
// journal root and must already have been captured by the InstallJournal. It
// records each target's terminal postimage even when the wrapped writer fails.
// Pipeline assembly uses this wrapper for component, persona, and metadata
// writers; it deliberately contains no agent-specific policy.
type preparedWriter struct {
	targets              []ManagedTarget
	journal              *InstallJournal
	recordJournalOutcome func(*InstallJournal, MutationOutcome) error
}

// newPreparedWriter freezes a writer's target inventory before the caller
// begins journal capture. The same generic wrapper serves component, persona,
// and configuration writers without selecting behavior by agent ID.
func newPreparedWriter(targets []ManagedTarget) *preparedWriter {
	inventory := append([]ManagedTarget(nil), targets...)
	sort.Slice(inventory, func(i, j int) bool {
		if inventory[i].Path != inventory[j].Path {
			return inventory[i].Path < inventory[j].Path
		}
		if inventory[i].Kind != inventory[j].Kind {
			return inventory[i].Kind < inventory[j].Kind
		}
		return inventory[i].Owner < inventory[j].Owner
	})
	return &preparedWriter{targets: inventory}
}

// ManagedTargets returns an immutable copy of the declared target inventory.
func (w *preparedWriter) ManagedTargets() []ManagedTarget {
	if w == nil {
		return nil
	}
	return append([]ManagedTarget(nil), w.targets...)
}

// bindJournal attaches the already captured journal immediately before the
// writer enters its post-backup phase.
func (w *preparedWriter) bindJournal(journal *InstallJournal) {
	w.journal = journal
}

func (w *preparedWriter) run(run func() error) error {
	if err := w.beforeRun(); err != nil {
		return err
	}

	err := run()
	if recordErr := w.recordAttempt(err); recordErr != nil {
		return errors.Join(err, fmt.Errorf("record writer outcome: %w", recordErr))
	}
	return err
}

func (w *preparedWriter) beforeRun() error {
	if w == nil || w.journal == nil {
		return errors.New("prepared writer requires a captured install journal")
	}
	if len(w.targets) == 0 {
		return errors.New("prepared writer requires declared targets")
	}
	for _, target := range w.targets {
		kind, declared := w.journal.targetKind(target.Path)
		if !declared || kind != target.Kind {
			return fmt.Errorf("prepared writer target %q was not captured", target.Path)
		}
	}
	return nil
}

func (w *preparedWriter) recordAttempt(writeErr error) error {
	targets := append([]ManagedTarget(nil), w.targets...)
	sort.Slice(targets, func(i, j int) bool { return targets[i].Path < targets[j].Path })
	for _, target := range targets {
		path, err := journalPath(w.journal.TargetRoot, target.Path)
		if err != nil {
			return err
		}
		outcome, err := inspectPath(path, target.Path)
		if err != nil {
			return err
		}
		if writeErr != nil {
			outcome.Error = writeErr.Error()
		}
		record := w.recordJournalOutcome
		if record == nil {
			record = func(journal *InstallJournal, outcome MutationOutcome) error { return journal.Record(outcome) }
		}
		if err := record(w.journal, outcome); err != nil {
			return err
		}
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
