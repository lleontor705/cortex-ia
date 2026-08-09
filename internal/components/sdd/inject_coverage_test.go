package sdd

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/agents/opencode"
	"github.com/lleontor705/cortex-ia/internal/assets"
	"github.com/lleontor705/cortex-ia/internal/model"
	"github.com/lleontor705/cortex-ia/internal/system"
)

// stubAdapter implements agents.Adapter with configurable capabilities for edge-case testing.
type stubAdapter struct {
	agentID            model.AgentID
	supportsSkills     bool
	supportsPrompt     bool
	supportsCommands   bool
	supportsSubAgents  bool
	supportsDelegation bool
	skillsDirVal       string
	promptFileVal      string
	commandsDirVal     string
	subAgentsDirVal    string
	settingsPathVal    string
}

func (s *stubAdapter) Agent() model.AgentID    { return s.agentID }
func (s *stubAdapter) Tier() model.SupportTier { return model.TierFull }
func (s *stubAdapter) Detect(string) (bool, string, string, bool, error) {
	return false, "", "", false, nil
}
func (s *stubAdapter) GlobalConfigDir(string) string  { return "" }
func (s *stubAdapter) SystemPromptDir(string) string  { return "" }
func (s *stubAdapter) SystemPromptFile(string) string { return s.promptFileVal }
func (s *stubAdapter) SkillsDir(string) string        { return s.skillsDirVal }
func (s *stubAdapter) SettingsPath(string) string     { return s.settingsPathVal }
func (s *stubAdapter) SystemPromptStrategy() model.SystemPromptStrategy {
	return model.StrategyMarkdownSections
}
func (s *stubAdapter) MCPStrategy() model.MCPStrategy                      { return 0 }
func (s *stubAdapter) MCPConfigPath(string, string) string                 { return "" }
func (s *stubAdapter) SupportsSkills() bool                                { return s.supportsSkills }
func (s *stubAdapter) SupportsSystemPrompt() bool                          { return s.supportsPrompt }
func (s *stubAdapter) SupportsMCP() bool                                   { return false }
func (s *stubAdapter) SupportsSlashCommands() bool                         { return s.supportsCommands }
func (s *stubAdapter) CommandsDir(string) string                           { return s.commandsDirVal }
func (s *stubAdapter) SupportsTaskDelegation() bool                        { return s.supportsDelegation }
func (s *stubAdapter) SupportsSubAgents() bool                             { return s.supportsSubAgents }
func (s *stubAdapter) SubAgentsDir(string) string                          { return s.subAgentsDirVal }
func (s *stubAdapter) SupportsAutoInstall() bool                           { return false }
func (s *stubAdapter) InstallCommands(_ system.PlatformProfile) [][]string { return nil }

func TestCurrentOrchestratorAssetsContainNoRetiredCoordinationSurface(t *testing.T) {
	for _, path := range []string{
		"generic/sdd-orchestrator.md",
		"generic/sdd-orchestrator-single.md",
		"generic/sdd-orchestrator-reference.md",
	} {
		content, err := assets.Read(path)
		if err != nil {
			t.Fatal(err)
		}
		lower := strings.ToLower(content)
		for _, forbidden := range []string{"team-lead", "mailbox", "msg_", "a2a_", "resource_", "dlq_", "nested coordinator"} {
			if strings.Contains(lower, forbidden) {
				t.Errorf("%s contains retired coordination surface %q", path, forbidden)
			}
		}
	}
}

func TestCoordinatorSkillsReturnBoundedPlansWithoutNestedDispatch(t *testing.T) {
	for _, id := range []string{"debate", "parallel-dispatch"} {
		content, err := assets.Read("skills/" + id + "/SKILL.md")
		if err != nil {
			t.Fatalf("read %s skill: %v", id, err)
		}
		text := strings.ToLower(content)
		for _, required := range []string{"plan-only", "independent", "bounded", "orchestrator"} {
			if !strings.Contains(text, required) {
				t.Errorf("%s skill missing planning contract %q", id, required)
			}
		}
		for _, forbidden := range []string{"task()", "native `task`"} {
			if strings.Contains(text, forbidden) {
				t.Errorf("%s skill contains nested dispatch instruction %q", id, forbidden)
			}
		}
	}
}

func TestFilesToBackup_WithCommands(t *testing.T) {
	adapter := opencode.NewAdapter()
	paths := FilesToBackup("/home/test", adapter)

	hasCommand := false
	hasSharedSkill := false
	hasLegacyTeamLeadBackup := false
	hasPrompt := false
	for _, p := range paths {
		normalized := filepath.ToSlash(p)
		if strings.Contains(normalized, "commands") && strings.HasSuffix(p, ".md") {
			hasCommand = true
		}
		if strings.Contains(normalized, ".cortex-ia/skills/bootstrap") {
			hasSharedSkill = true
		}
		if strings.Contains(normalized, ".cortex-ia/skills/team-lead/SKILL.md") {
			hasLegacyTeamLeadBackup = true
		}
		if strings.Contains(normalized, "prompts/orchestrator.md") {
			hasPrompt = true
		}
	}
	if !hasCommand {
		t.Error("expected command files")
	}
	if !hasSharedSkill {
		t.Error("expected shared skill files")
	}
	if !hasLegacyTeamLeadBackup {
		t.Error("expected legacy team-lead skill in rollback backup scope")
	}
	if !hasPrompt {
		t.Error("expected shared orchestrator prompt")
	}
}

func TestFilesToBackup_NoPromptNoCommands(t *testing.T) {
	adapter := &stubAdapter{agentID: "test"}
	paths := FilesToBackup("/tmp/test", adapter)
	if len(paths) < 12 {
		t.Errorf("expected at least 12 paths (11 skills + prompt), got %d", len(paths))
	}
}
