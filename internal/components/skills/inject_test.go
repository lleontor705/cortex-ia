package skills

import (
	"testing"

	"github.com/lleontor705/cortex-ia/internal/model"
	"github.com/lleontor705/cortex-ia/internal/system"
)

// mockAdapter implements agents.Adapter for testing skill injection.
type mockAdapter struct {
	supportsSkills bool
	skillsDir      string
}

func (m *mockAdapter) Agent() model.AgentID                               { return "test-agent" }
func (m *mockAdapter) Tier() model.SupportTier                            { return model.TierFull }
func (m *mockAdapter) Detect(_ string) (bool, string, string, bool, error) { return false, "", "", false, nil }
func (m *mockAdapter) GlobalConfigDir(_ string) string                     { return "" }
func (m *mockAdapter) SystemPromptDir(_ string) string                     { return "" }
func (m *mockAdapter) SystemPromptFile(_ string) string                    { return "" }
func (m *mockAdapter) SkillsDir(_ string) string                           { return m.skillsDir }
func (m *mockAdapter) SettingsPath(_ string) string                        { return "" }
func (m *mockAdapter) SystemPromptStrategy() model.SystemPromptStrategy    { return 0 }
func (m *mockAdapter) MCPStrategy() model.MCPStrategy                      { return 0 }
func (m *mockAdapter) MCPConfigPath(_ string, _ string) string             { return "" }
func (m *mockAdapter) SupportsSkills() bool                                { return m.supportsSkills }
func (m *mockAdapter) SupportsSystemPrompt() bool                          { return false }
func (m *mockAdapter) SupportsMCP() bool                                   { return false }
func (m *mockAdapter) SupportsSlashCommands() bool                         { return false }
func (m *mockAdapter) CommandsDir(_ string) string                         { return "" }
func (m *mockAdapter) SupportsTaskDelegation() bool                        { return false }
func (m *mockAdapter) SupportsSubAgents() bool                             { return false }
func (m *mockAdapter) SubAgentsDir(_ string) string                        { return "" }
func (m *mockAdapter) SupportsAutoInstall() bool                           { return false }
func (m *mockAdapter) InstallCommands(_ system.PlatformProfile) [][]string { return nil }

// ---------------------------------------------------------------------------
// isSDDSkill
// ---------------------------------------------------------------------------

func TestIsSDDSkill_True(t *testing.T) {
	sddSkills := []model.SkillID{
		"bootstrap", "investigate", "draft-proposal", "write-specs",
		"architect", "decompose", "team-lead", "implement",
		"validate", "finalize", "debate", "debug",
		"execute-plan", "ideate", "monitor", "open-pr",
		"file-issue", "parallel-dispatch", "scan-registry",
		"sdd-custom-thing",
	}
	for _, id := range sddSkills {
		if !isSDDSkill(id) {
			t.Errorf("isSDDSkill(%q) = false, want true", id)
		}
	}
}

func TestIsSDDSkill_False(t *testing.T) {
	nonSDD := []model.SkillID{
		"my-custom-skill", "linter", "formatter", "deploy-helper",
	}
	for _, id := range nonSDD {
		if isSDDSkill(id) {
			t.Errorf("isSDDSkill(%q) = true, want false", id)
		}
	}
}

// ---------------------------------------------------------------------------
// Inject
// ---------------------------------------------------------------------------

func TestInject_UnsupportedAgent(t *testing.T) {
	adapter := &mockAdapter{supportsSkills: false, skillsDir: "/some/dir"}
	ids := []model.SkillID{"my-skill", "other-skill"}

	result, err := Inject("/home/test", adapter, ids)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Skipped) != len(ids) {
		t.Errorf("Skipped = %d, want %d", len(result.Skipped), len(ids))
	}
	if result.Changed {
		t.Error("Changed should be false for unsupported agent")
	}
}

func TestInject_EmptySkillDir(t *testing.T) {
	adapter := &mockAdapter{supportsSkills: true, skillsDir: ""}
	ids := []model.SkillID{"my-skill"}

	result, err := Inject("/home/test", adapter, ids)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Skipped) != len(ids) {
		t.Errorf("Skipped = %d, want %d", len(result.Skipped), len(ids))
	}
}

func TestInject_SkipsSDDSkills(t *testing.T) {
	adapter := &mockAdapter{supportsSkills: true, skillsDir: t.TempDir()}
	ids := []model.SkillID{"bootstrap", "validate", "implement"}

	result, err := Inject("/home/test", adapter, ids)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// All provided skills are SDD skills, so none should be written.
	if len(result.Files) != 0 {
		t.Errorf("Files = %v, want empty (SDD skills should be skipped)", result.Files)
	}
}
