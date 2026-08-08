package pipeline

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/model"
)

func TestInstallDependencies_DefaultsAreComplete(t *testing.T) {
	deps := defaultInstallDependencies()
	if deps.prepareWorkflow == nil || deps.applyWorkflow == nil || deps.invokeComponent == nil || deps.invokePersona == nil ||
		deps.saveInstallStatus == nil || deps.clearInstallStatus == nil || deps.saveState == nil || deps.saveLock == nil ||
		deps.beginJournal == nil || deps.attachWorkflowReceipt == nil || deps.recordJournalOutcome == nil ||
		deps.commitJournal == nil || deps.restoreAndVerify == nil {
		t.Fatal("default install dependencies must supply every coordinator operation")
	}
}

func TestInstallDependencies_BeginJournalHookIsScoped(t *testing.T) {
	boom := errors.New("injected begin journal failure")
	deps := defaultInstallDependencies()
	deps.beginJournal = func(string, string, []ManagedTarget) (*InstallJournal, error) {
		return nil, boom
	}

	selection := model.Selection{
		Agents:     []model.AgentID{model.AgentCodex},
		Components: []model.ComponentID{model.ComponentCortex},
	}
	result, err := installWithDependencies(t.TempDir(), newTestRegistry(), selection, "test-v1", false, deps)
	if !errors.Is(err, boom) || !strings.Contains(err.Error(), "capture install journal") {
		t.Fatalf("injected Install() error = %v, want begin-journal failure", err)
	}
	if result.BackupID == "" {
		t.Fatal("injected post-backup failure lost backup evidence")
	}

	if _, err := Install(t.TempDir(), newTestRegistry(), selection, "test-v1", false); err != nil {
		t.Fatalf("public Install() inherited per-call hook: %v", err)
	}
}

func TestInstallDependencies_PreparedWriterUsesScopedRecordHook(t *testing.T) {
	homeDir := t.TempDir()
	target := ManagedTarget{Path: "target.txt", Kind: TargetFile}
	if err := os.WriteFile(filepath.Join(homeDir, target.Path), []byte("after"), 0o600); err != nil {
		t.Fatal(err)
	}
	journal := &InstallJournal{TargetRoot: homeDir, Targets: []ManagedTarget{target}}
	called := false
	writer := newPreparedWriter([]ManagedTarget{target})
	writer.bindJournal(journal)
	writer.recordJournalOutcome = func(got *InstallJournal, outcome MutationOutcome) error {
		called = got == journal && outcome.Path == target.Path
		return nil
	}

	if err := writer.run(func() error { return nil }); err != nil {
		t.Fatalf("prepared writer error = %v", err)
	}
	if !called {
		t.Fatal("prepared writer did not use its scoped journal record hook")
	}
}
