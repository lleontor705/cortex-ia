package pipeline

import (
	"errors"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/agents"
	"github.com/lleontor705/cortex-ia/internal/model"
	"github.com/lleontor705/cortex-ia/internal/state"
	"github.com/lleontor705/cortex-ia/internal/system"
)

// mockAdapter implements agents.Adapter with minimal stubs for testing.
type mockAdapter struct {
	agentID model.AgentID
}

func (m *mockAdapter) Agent() model.AgentID                         { return m.agentID }
func (m *mockAdapter) Tier() model.SupportTier                      { return model.TierFull }
func (m *mockAdapter) Detect(string) (bool, string, string, bool, error) {
	return false, "", "", false, nil
}
func (m *mockAdapter) GlobalConfigDir(string) string                { return "" }
func (m *mockAdapter) SystemPromptDir(string) string                { return "" }
func (m *mockAdapter) SystemPromptFile(string) string               { return "" }
func (m *mockAdapter) SkillsDir(string) string                      { return "" }
func (m *mockAdapter) SettingsPath(string) string                   { return "" }
func (m *mockAdapter) SystemPromptStrategy() model.SystemPromptStrategy {
	return model.StrategyMarkdownSections
}
func (m *mockAdapter) MCPStrategy() model.MCPStrategy               { return model.StrategySeparateMCPFiles }
func (m *mockAdapter) MCPConfigPath(string, string) string          { return "" }
func (m *mockAdapter) SupportsSkills() bool                         { return false }
func (m *mockAdapter) SupportsSystemPrompt() bool                   { return false }
func (m *mockAdapter) SupportsMCP() bool                            { return false }
func (m *mockAdapter) SupportsSlashCommands() bool                  { return false }
func (m *mockAdapter) CommandsDir(string) string                    { return "" }
func (m *mockAdapter) SupportsTaskDelegation() bool                 { return false }
func (m *mockAdapter) SupportsSubAgents() bool                      { return false }
func (m *mockAdapter) SubAgentsDir(string) string                   { return "" }
func (m *mockAdapter) SupportsAutoInstall() bool                    { return false }
func (m *mockAdapter) InstallCommands(_ system.PlatformProfile) [][]string { return nil }

// --- validateStep tests ---

func TestValidateStep_Name(t *testing.T) {
	step := &validateStep{
		registry: agents.NewRegistry(),
		agentIDs: nil,
	}
	if got := step.Name(); got != "validate-agents" {
		t.Errorf("Name() = %q, want %q", got, "validate-agents")
	}
}

func TestValidateStep_AllValid(t *testing.T) {
	reg := agents.NewRegistry()
	reg.Register(&mockAdapter{agentID: "test-agent"})
	reg.Register(&mockAdapter{agentID: "other-agent"})

	step := &validateStep{
		registry: reg,
		agentIDs: []model.AgentID{"test-agent", "other-agent"},
	}
	if err := step.Run(); err != nil {
		t.Errorf("Run() unexpected error: %v", err)
	}
}

func TestValidateStep_UnknownAgent(t *testing.T) {
	reg := agents.NewRegistry()
	reg.Register(&mockAdapter{agentID: "known-agent"})

	step := &validateStep{
		registry: reg,
		agentIDs: []model.AgentID{"known-agent", "ghost-agent"},
	}
	err := step.Run()
	if err == nil {
		t.Fatal("Run() expected error for unknown agent, got nil")
	}
	if got := err.Error(); got != `unknown agent "ghost-agent"` {
		t.Errorf("Run() error = %q, want %q", got, `unknown agent "ghost-agent"`)
	}
}

// --- componentStep tests ---

func TestComponentStep_Name(t *testing.T) {
	step := &componentStep{
		adapter:     &mockAdapter{agentID: "claude-code"},
		componentID: "cortex",
	}
	if got := step.Name(); got != "claude-code/cortex" {
		t.Errorf("Name() = %q, want %q", got, "claude-code/cortex")
	}
}

func TestComponentStep_RunSuccess(t *testing.T) {
	expectedFiles := []string{"/tmp/file1.md", "/tmp/file2.json"}
	step := &componentStep{
		homeDir:     "/tmp",
		adapter:     &mockAdapter{agentID: "test-agent"},
		componentID: "test-component",
		injectorFn: func() ([]string, error) {
			return expectedFiles, nil
		},
	}

	err := step.Run()
	if err != nil {
		t.Fatalf("Run() unexpected error: %v", err)
	}
	if len(step.Files) != len(expectedFiles) {
		t.Fatalf("Files length = %d, want %d", len(step.Files), len(expectedFiles))
	}
	for i, f := range step.Files {
		if f != expectedFiles[i] {
			t.Errorf("Files[%d] = %q, want %q", i, f, expectedFiles[i])
		}
	}
}

func TestComponentStep_RunError(t *testing.T) {
	injErr := errors.New("injection failed")
	step := &componentStep{
		homeDir:     "/tmp",
		adapter:     &mockAdapter{agentID: "test-agent"},
		componentID: "test-component",
		injectorFn: func() ([]string, error) {
			return nil, injErr
		},
	}

	err := step.Run()
	if err == nil {
		t.Fatal("Run() expected error, got nil")
	}
	if !errors.Is(err, injErr) {
		t.Errorf("Run() error = %v, want %v", err, injErr)
	}
	if step.Files != nil {
		t.Errorf("Files should be nil on error, got %v", step.Files)
	}
}

func TestComponentStep_ProgressCalled(t *testing.T) {
	type call struct {
		stepID string
		status string
		err    error
	}
	var calls []call

	step := &componentStep{
		homeDir:     "/tmp",
		adapter:     &mockAdapter{agentID: "my-agent"},
		componentID: "my-comp",
		injectorFn: func() ([]string, error) {
			return []string{"/f1"}, nil
		},
		progress: func(stepID, status string, err error) {
			calls = append(calls, call{stepID, status, err})
		},
	}

	if err := step.Run(); err != nil {
		t.Fatalf("Run() unexpected error: %v", err)
	}

	if len(calls) != 2 {
		t.Fatalf("progress called %d times, want 2", len(calls))
	}

	wantName := "my-agent/my-comp"
	if calls[0].stepID != wantName || calls[0].status != "running" || calls[0].err != nil {
		t.Errorf("call[0] = %+v, want {%s running <nil>}", calls[0], wantName)
	}
	if calls[1].stepID != wantName || calls[1].status != "succeeded" || calls[1].err != nil {
		t.Errorf("call[1] = %+v, want {%s succeeded <nil>}", calls[1], wantName)
	}
}

func TestComponentStep_ProgressCalledOnError(t *testing.T) {
	type call struct {
		stepID string
		status string
		err    error
	}
	var calls []call
	injErr := errors.New("bad inject")

	step := &componentStep{
		homeDir:     "/tmp",
		adapter:     &mockAdapter{agentID: "agent-x"},
		componentID: "comp-y",
		injectorFn: func() ([]string, error) {
			return nil, injErr
		},
		progress: func(stepID, status string, err error) {
			calls = append(calls, call{stepID, status, err})
		},
	}

	_ = step.Run()

	if len(calls) != 2 {
		t.Fatalf("progress called %d times, want 2", len(calls))
	}

	wantName := "agent-x/comp-y"
	if calls[0].stepID != wantName || calls[0].status != "running" || calls[0].err != nil {
		t.Errorf("call[0] = %+v, want {%s running <nil>}", calls[0], wantName)
	}
	if calls[1].stepID != wantName || calls[1].status != "failed" || !errors.Is(calls[1].err, injErr) {
		t.Errorf("call[1] = {%s %s %v}, want {%s failed %v}", calls[1].stepID, calls[1].status, calls[1].err, wantName, injErr)
	}
}

func TestComponentStep_ProgressNil(t *testing.T) {
	step := &componentStep{
		homeDir:     "/tmp",
		adapter:     &mockAdapter{agentID: "agent"},
		componentID: "comp",
		injectorFn: func() ([]string, error) {
			return []string{"/ok"}, nil
		},
		progress: nil,
	}

	err := step.Run()
	if err != nil {
		t.Fatalf("Run() with nil progress should succeed, got: %v", err)
	}
	if len(step.Files) != 1 || step.Files[0] != "/ok" {
		t.Errorf("Files = %v, want [/ok]", step.Files)
	}
}

// --- backupStep tests ---

func TestBackupStep_Name(t *testing.T) {
	step := &backupStep{}
	if got := step.Name(); got != "backup" {
		t.Errorf("Name() = %q, want %q", got, "backup")
	}
}

func TestBackupStep_Rollback_Noop(t *testing.T) {
	step := &backupStep{BackupDir: ""}
	err := step.Rollback()
	if err != nil {
		t.Errorf("Rollback() with empty BackupDir should be no-op, got: %v", err)
	}
}

// --- installStatusStep tests ---

func TestInstallStatusStep_Name(t *testing.T) {
	step := &installStatusStep{}
	if got := step.Name(); got != "install-status" {
		t.Errorf("Name() = %q, want %q", got, "install-status")
	}
}

func TestInstallStatusStep_Run(t *testing.T) {
	homeDir := t.TempDir()
	step := &installStatusStep{homeDir: homeDir, backupID: "bk-123"}

	if err := step.Run(); err != nil {
		t.Fatalf("Run() unexpected error: %v", err)
	}

	status, err := state.LoadInstallStatus(homeDir)
	if err != nil {
		t.Fatalf("LoadInstallStatus() error: %v", err)
	}
	if status == nil {
		t.Fatal("LoadInstallStatus() returned nil after Run()")
	}
	if status.Status != "in-progress" {
		t.Errorf("Status = %q, want %q", status.Status, "in-progress")
	}
	if status.BackupID != "bk-123" {
		t.Errorf("BackupID = %q, want %q", status.BackupID, "bk-123")
	}
	if status.StartedAt == "" {
		t.Error("StartedAt should not be empty")
	}
}

func TestInstallStatusStep_Rollback(t *testing.T) {
	homeDir := t.TempDir()
	step := &installStatusStep{homeDir: homeDir, backupID: "bk-456"}

	// Run to create the status file.
	if err := step.Run(); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	// Verify file exists.
	status, _ := state.LoadInstallStatus(homeDir)
	if status == nil {
		t.Fatal("expected status file after Run()")
	}

	// Rollback should remove it.
	if err := step.Rollback(); err != nil {
		t.Fatalf("Rollback() error: %v", err)
	}

	status, err := state.LoadInstallStatus(homeDir)
	if err != nil {
		t.Fatalf("LoadInstallStatus() after rollback error: %v", err)
	}
	if status != nil {
		t.Errorf("expected nil after Rollback(), got %+v", status)
	}
}

func TestInstallStatusStep_Rollback_NoFile(t *testing.T) {
	homeDir := t.TempDir()
	step := &installStatusStep{homeDir: homeDir}

	// Rollback when no file exists should not error.
	if err := step.Rollback(); err != nil {
		t.Errorf("Rollback() with no file should be no-op, got: %v", err)
	}
}

func TestInstallStatusStep_ImplementsRollbackStep(t *testing.T) {
	var _ RollbackStep = (*installStatusStep)(nil)
}
