package agents

import (
	"testing"

	"github.com/lleontor705/cortex-ia/internal/model"
	"github.com/lleontor705/cortex-ia/internal/system"
)

type mockAdapter struct {
	id model.AgentID
}

func (m *mockAdapter) Agent() model.AgentID                               { return m.id }
func (m *mockAdapter) Tier() model.SupportTier                            { return model.TierFull }
func (m *mockAdapter) Detect(_ string) (bool, string, string, bool, error) { return false, "", "", false, nil }
func (m *mockAdapter) GlobalConfigDir(_ string) string                     { return "" }
func (m *mockAdapter) SystemPromptDir(_ string) string                     { return "" }
func (m *mockAdapter) SystemPromptFile(_ string) string                    { return "" }
func (m *mockAdapter) SkillsDir(_ string) string                           { return "" }
func (m *mockAdapter) SettingsPath(_ string) string                        { return "" }
func (m *mockAdapter) SystemPromptStrategy() model.SystemPromptStrategy    { return 0 }
func (m *mockAdapter) MCPStrategy() model.MCPStrategy                      { return 0 }
func (m *mockAdapter) MCPConfigPath(_ string, _ string) string             { return "" }
func (m *mockAdapter) SupportsSkills() bool                                { return false }
func (m *mockAdapter) SupportsSystemPrompt() bool                          { return false }
func (m *mockAdapter) SupportsMCP() bool                                   { return false }
func (m *mockAdapter) SupportsSlashCommands() bool                         { return false }
func (m *mockAdapter) CommandsDir(_ string) string                         { return "" }
func (m *mockAdapter) SupportsTaskDelegation() bool                        { return false }
func (m *mockAdapter) SupportsSubAgents() bool                             { return false }
func (m *mockAdapter) SubAgentsDir(_ string) string                        { return "" }
func (m *mockAdapter) SupportsAutoInstall() bool                           { return false }
func (m *mockAdapter) InstallCommands(_ system.PlatformProfile) [][]string { return nil }

func TestRegistryRegisterAndGet(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockAdapter{id: model.AgentClaudeCode})
	r.Register(&mockAdapter{id: model.AgentOpenCode})

	a, err := r.Get(model.AgentClaudeCode)
	if err != nil {
		t.Fatal(err)
	}
	if a.Agent() != model.AgentClaudeCode {
		t.Errorf("expected claude-code, got %s", a.Agent())
	}

	_, err = r.Get("nonexistent")
	if err != ErrAgentNotFound {
		t.Errorf("expected ErrAgentNotFound, got %v", err)
	}
}

func TestRegistryAll(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockAdapter{id: model.AgentClaudeCode})
	r.Register(&mockAdapter{id: model.AgentOpenCode})

	all := r.All()
	if len(all) != 2 {
		t.Fatalf("expected 2 adapters, got %d", len(all))
	}
	if all[0].Agent() != model.AgentClaudeCode {
		t.Errorf("expected claude-code first, got %s", all[0].Agent())
	}
	if all[1].Agent() != model.AgentOpenCode {
		t.Errorf("expected opencode second, got %s", all[1].Agent())
	}
}

func TestRegistryDuplicateOverwrites(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockAdapter{id: model.AgentClaudeCode})
	r.Register(&mockAdapter{id: model.AgentClaudeCode}) // overwrite

	if len(r.All()) != 1 {
		t.Errorf("expected 1 adapter after duplicate register, got %d", len(r.All()))
	}
}
