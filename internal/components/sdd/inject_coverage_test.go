package sdd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/agents/codex"
	"github.com/lleontor705/cortex-ia/internal/agents/opencode"
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

func (s *stubAdapter) Agent() model.AgentID                               { return s.agentID }
func (s *stubAdapter) Tier() model.SupportTier                            { return model.TierFull }
func (s *stubAdapter) Detect(string) (bool, string, string, bool, error)  { return false, "", "", false, nil }
func (s *stubAdapter) GlobalConfigDir(string) string                      { return "" }
func (s *stubAdapter) SystemPromptDir(string) string                      { return "" }
func (s *stubAdapter) SystemPromptFile(string) string                     { return s.promptFileVal }
func (s *stubAdapter) SkillsDir(string) string                            { return s.skillsDirVal }
func (s *stubAdapter) SettingsPath(string) string                         { return s.settingsPathVal }
func (s *stubAdapter) SystemPromptStrategy() model.SystemPromptStrategy   { return model.StrategyMarkdownSections }
func (s *stubAdapter) MCPStrategy() model.MCPStrategy                     { return 0 }
func (s *stubAdapter) MCPConfigPath(string, string) string                { return "" }
func (s *stubAdapter) SupportsSkills() bool                               { return s.supportsSkills }
func (s *stubAdapter) SupportsSystemPrompt() bool                         { return s.supportsPrompt }
func (s *stubAdapter) SupportsMCP() bool                                  { return false }
func (s *stubAdapter) SupportsSlashCommands() bool                        { return s.supportsCommands }
func (s *stubAdapter) CommandsDir(string) string                          { return s.commandsDirVal }
func (s *stubAdapter) SupportsTaskDelegation() bool                       { return s.supportsDelegation }
func (s *stubAdapter) SupportsSubAgents() bool                            { return s.supportsSubAgents }
func (s *stubAdapter) SubAgentsDir(string) string                         { return s.subAgentsDirVal }
func (s *stubAdapter) SupportsAutoInstall() bool                          { return false }
func (s *stubAdapter) InstallCommands(_ system.PlatformProfile) [][]string { return nil }

// ---------------------------------------------------------------------------
// OpenCode injection: covers injectCommands, injectSubAgents, buildAgentHiddenOverlay
// ---------------------------------------------------------------------------

func TestInjectSDD_OpenCode(t *testing.T) {
	ResetSharedWrite()
	tmpDir := t.TempDir()
	adapter := opencode.NewAdapter()

	result, err := Inject(tmpDir, adapter, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed {
		t.Error("expected Changed=true")
	}

	// Verify orchestrator injected.
	promptFile := adapter.SystemPromptFile(tmpDir)
	content, err := os.ReadFile(promptFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "cortex-ia:sdd-orchestrator") {
		t.Error("expected SDD orchestrator marker")
	}
	if !strings.Contains(string(content), "Principal Orchestrator") {
		t.Error("expected multi-agent orchestrator")
	}

	// Verify skills written to shared dir (~/.cortex-ia/skills/).
	sharedSkillsDir := filepath.Join(tmpDir, ".cortex-ia", "skills")
	bootstrapSkill := filepath.Join(sharedSkillsDir, "bootstrap", "SKILL.md")
	if _, err := os.Stat(bootstrapSkill); os.IsNotExist(err) {
		t.Error("expected bootstrap skill in shared dir")
	}

	// Verify convention refs replaced with absolute path in skills.
	implSkill, err := os.ReadFile(filepath.Join(sharedSkillsDir, "implement", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(implSkill), "../_shared/cortex-convention.md") {
		t.Error("expected relative convention refs to be replaced with absolute path")
	}
	if !strings.Contains(string(implSkill), ".cortex-ia/skills/_shared/cortex-convention.md") {
		t.Error("expected absolute convention path in skill content")
	}

	// Verify orchestrator prompt written to shared prompts dir.
	sharedPrompt := filepath.Join(tmpDir, ".cortex-ia", "prompts", "orchestrator.md")
	if _, err := os.Stat(sharedPrompt); os.IsNotExist(err) {
		t.Error("expected orchestrator prompt in shared prompts dir")
	}

	// Verify {{SKILLS_DIR}} replaced with shared path in orchestrator.
	promptData, _ := os.ReadFile(sharedPrompt)
	if strings.Contains(string(promptData), "{{SKILLS_DIR}}") {
		t.Error("expected {{SKILLS_DIR}} to be replaced")
	}
	if !strings.Contains(string(promptData), ".cortex-ia/skills") {
		t.Error("expected shared skills dir in orchestrator prompt")
	}

	// Verify slash commands written.
	commandsDir := adapter.CommandsDir(tmpDir)
	entries, err := os.ReadDir(commandsDir)
	if err != nil {
		t.Fatalf("read commands dir: %v", err)
	}
	if len(entries) < 10 {
		t.Errorf("expected >=10 command files, got %d", len(entries))
	}

	// OpenCode: no .md stubs in agents/ — everything is in opencode.json.
	subAgentsDir := adapter.SubAgentsDir(tmpDir)
	agentEntries, _ := os.ReadDir(subAgentsDir)
	for _, e := range agentEntries {
		if strings.HasSuffix(e.Name(), ".md") {
			t.Errorf("OpenCode should not have .md stubs in agents/, found %s", e.Name())
		}
	}

	// Verify settings merged with full agent configs.
	settingsData, err := os.ReadFile(adapter.SettingsPath(tmpDir))
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}
	var settings map[string]any
	if err := json.Unmarshal(settingsData, &settings); err != nil {
		t.Fatalf("invalid settings JSON: %v", err)
	}
	agentSection, ok := settings["agent"].(map[string]any)
	if !ok {
		t.Fatal("expected 'agent' section in settings")
	}
	// Verify disabled built-in agents.
	for _, disabled := range []string{"build", "plan"} {
		entry, ok := agentSection[disabled].(map[string]any)
		if !ok {
			t.Errorf("missing disabled agent %q", disabled)
			continue
		}
		if entry["disable"] != true {
			t.Errorf("expected %q to be disabled", disabled)
		}
	}

	// Verify orchestrator is primary mode.
	orch, ok := agentSection["orchestrator"].(map[string]any)
	if !ok {
		t.Fatal("missing orchestrator agent")
	}
	if orch["mode"] != "primary" {
		t.Error("orchestrator should be primary mode")
	}

	// Verify sub-agents.
	for _, id := range openCodeSubAgents {
		entry, ok := agentSection[id].(map[string]any)
		if !ok {
			t.Errorf("missing agent entry for %q", id)
			continue
		}
		if entry["mode"] != "subagent" {
			t.Errorf("expected %q to be subagent mode", id)
		}
		// Full config fields must be present.
		for _, field := range []string{"color", "prompt", "description", "steps", "temperature"} {
			if entry[field] == nil {
				t.Errorf("expected %q to have %s", id, field)
			}
		}
		tools, ok := entry["tools"].(map[string]any)
		if !ok {
			t.Errorf("expected %q to have tools section", id)
			continue
		}
		// Writers must have edit+write; readers must not.
		role := agentRoles[id]
		editEnabled, _ := tools["edit"].(bool)
		writeEnabled, _ := tools["write"].(bool)
		switch role {
		case roleLeafWriter, roleLeafOps:
			if !editEnabled || !writeEnabled {
				t.Errorf("writer %q should have edit+write enabled", id)
			}
		case roleLeafReader, roleLeafPlanner, roleLeafVerify:
			if editEnabled || writeEnabled {
				t.Errorf("reader/planner/verifier %q should not have edit+write", id)
			}
		case roleCoordinator:
			if editEnabled || writeEnabled {
				t.Errorf("coordinator %q should not have edit+write", id)
			}
		}
	}

	// team-lead must have permission block.
	tl, _ := agentSection["team-lead"].(map[string]any)
	if tl["permission"] == nil {
		t.Error("team-lead should have permission block")
	}
}

func TestInjectSDD_OpenCode_Idempotent(t *testing.T) {
	ResetSharedWrite()
	tmpDir := t.TempDir()
	adapter := opencode.NewAdapter()

	if _, err := Inject(tmpDir, adapter, nil, false); err != nil {
		t.Fatal(err)
	}
	second, err := Inject(tmpDir, adapter, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if second.Changed {
		t.Error("expected second inject to be idempotent")
	}
}

// ---------------------------------------------------------------------------
// Edge cases
// ---------------------------------------------------------------------------

func TestInject_NoFeatures(t *testing.T) {
	ResetSharedWrite()
	result, err := Inject(t.TempDir(), &stubAdapter{agentID: "test-agent"}, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	// Shared files (skills + prompt) always written, so file list should be non-empty.
	if len(result.Files) == 0 {
		t.Error("expected files to be non-empty (shared skills always written)")
	}
}

func TestInjectAgentPrompt_EmptyPromptFile(t *testing.T) {
	ResetSharedWrite()
	adapter := &stubAdapter{agentID: "test", supportsPrompt: true, promptFileVal: ""}
	result, err := injectAgentPrompt(t.TempDir(), adapter, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	// No prompt file → no agent-level write.
	if result.Changed {
		t.Error("expected no change when prompt file is empty")
	}
}

func TestInjectAgentPrompt_ReadError(t *testing.T) {
	ResetSharedWrite()
	tmpDir := t.TempDir()
	adapter := codex.NewAdapter()

	if err := os.MkdirAll(adapter.SystemPromptFile(tmpDir), 0o755); err != nil {
		t.Fatal(err)
	}

	_, err := injectAgentPrompt(tmpDir, adapter, nil, false)
	if err == nil {
		t.Fatal("expected error when prompt file is a directory")
	}
	if !strings.Contains(err.Error(), "read system prompt") {
		t.Errorf("expected 'read system prompt' error, got: %v", err)
	}
}

func TestInjectSkillFiles_WritesSkills(t *testing.T) {
	tmpDir := t.TempDir()
	result, err := injectSkillFiles(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed {
		t.Error("expected Changed=true")
	}

	// Convention file is NOT written by injectSkillFiles (owned by conventions component).
	// Verify sub-agent skills were written.
	bootstrapSkill := filepath.Join(tmpDir, ".cortex-ia", "skills", "bootstrap", "SKILL.md")
	if _, err := os.Stat(bootstrapSkill); os.IsNotExist(err) {
		t.Error("expected bootstrap skill to be written")
	}
}

func TestInjectSkillFiles_WriteError(t *testing.T) {
	tmpDir := t.TempDir()
	// Block .cortex-ia/skills as a file.
	os.MkdirAll(filepath.Join(tmpDir, ".cortex-ia"), 0o755)
	os.WriteFile(filepath.Join(tmpDir, ".cortex-ia", "skills"), []byte("block"), 0o644)

	_, err := injectSkillFiles(tmpDir)
	if err == nil {
		t.Fatal("expected error when skills dir is blocked")
	}
}

func TestInjectCommands_EmptyDir(t *testing.T) {
	adapter := &stubAdapter{agentID: "test", supportsCommands: true, commandsDirVal: ""}
	result, err := injectCommands(t.TempDir(), adapter)
	if err != nil {
		t.Fatal(err)
	}
	if result.Changed {
		t.Error("expected no change with empty commands dir")
	}
}

func TestInjectSubAgents_EmptyDir(t *testing.T) {
	adapter := &stubAdapter{agentID: "test", supportsSubAgents: true, subAgentsDirVal: ""}
	result, err := injectSubAgents(t.TempDir(), adapter)
	if err != nil {
		t.Fatal(err)
	}
	if result.Changed {
		t.Error("expected no change with empty sub-agents dir")
	}
}

func TestInjectSubAgents_SettingsReadError(t *testing.T) {
	tmpDir := t.TempDir()
	adapter := opencode.NewAdapter()

	if err := os.MkdirAll(adapter.SettingsPath(tmpDir), 0o755); err != nil {
		t.Fatal(err)
	}

	_, err := injectSubAgents(tmpDir, adapter)
	if err == nil {
		t.Fatal("expected error when settings path is a directory")
	}
	if !strings.Contains(err.Error(), "read agent settings") {
		t.Errorf("expected 'read agent settings' error, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Inject error propagation
// ---------------------------------------------------------------------------

func TestInject_OrchestratorError(t *testing.T) {
	ResetSharedWrite()
	tmpDir := t.TempDir()
	promptFile := filepath.Join(tmpDir, "prompt")
	os.MkdirAll(promptFile, 0o755)

	adapter := &stubAdapter{agentID: "test", supportsPrompt: true, promptFileVal: promptFile}
	_, err := Inject(tmpDir, adapter, nil, false)
	if err == nil || !strings.Contains(err.Error(), "sdd orchestrator prompt") {
		t.Fatalf("expected wrapped orchestrator error, got: %v", err)
	}
}

func TestInject_CommandsError(t *testing.T) {
	ResetSharedWrite()
	tmpDir := t.TempDir()
	commandsDir := filepath.Join(tmpDir, "commands")
	os.WriteFile(commandsDir, []byte("block"), 0o644)

	adapter := &stubAdapter{agentID: "test", supportsCommands: true, commandsDirVal: commandsDir}
	_, err := Inject(tmpDir, adapter, nil, false)
	if err == nil || !strings.Contains(err.Error(), "sdd commands") {
		t.Fatalf("expected wrapped commands error, got: %v", err)
	}
}

func TestInject_SubAgentsError(t *testing.T) {
	ResetSharedWrite()
	tmpDir := t.TempDir()
	subAgentsDir := filepath.Join(tmpDir, "agents")
	os.WriteFile(subAgentsDir, []byte("block"), 0o644)

	adapter := &stubAdapter{
		agentID: "test", supportsSubAgents: true,
		subAgentsDirVal: subAgentsDir, settingsPathVal: filepath.Join(tmpDir, "s.json"),
	}
	_, err := Inject(tmpDir, adapter, nil, false)
	if err == nil || !strings.Contains(err.Error(), "sdd sub-agents") {
		t.Fatalf("expected wrapped sub-agents error, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// fixConventionRefs
// ---------------------------------------------------------------------------

func TestFixConventionRefs(t *testing.T) {
	content := "Follow convention in `../_shared/cortex-convention.md`.\nAlso see `skills/_shared/cortex-convention.md`."
	absPath := "/home/user/.cortex-ia/skills/_shared/cortex-convention.md"

	result := fixConventionRefs(content, absPath)

	if strings.Contains(result, "`../_shared/cortex-convention.md`") {
		t.Error("expected relative ref to be replaced")
	}
	// Count occurrences of absPath — should be 2 (both refs replaced).
	if count := strings.Count(result, absPath); count != 2 {
		t.Errorf("expected 2 occurrences of absolute path, got %d", count)
	}
}

// ---------------------------------------------------------------------------
// buildAgentOverlay
// ---------------------------------------------------------------------------

func TestBuildAgentOverlay_ValidJSON(t *testing.T) {
	overlay := buildAgentOverlay([]string{"bootstrap", "investigate", "implement", "team-lead"}, "/test/skills", "/test/prompts")

	var parsed map[string]any
	if err := json.Unmarshal(overlay, &parsed); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, overlay)
	}
	agents, ok := parsed["agent"].(map[string]any)
	if !ok {
		t.Fatal("expected 'agent' key")
	}
	// 4 sub-agents + orchestrator + build (disabled) + plan (disabled) = 7
	if len(agents) != 7 {
		t.Errorf("expected 7 agents, got %d", len(agents))
	}

	// Verify disabled built-in agents.
	for _, disabled := range []string{"build", "plan"} {
		entry, _ := agents[disabled].(map[string]any)
		if entry["disable"] != true {
			t.Errorf("agent %q should be disabled", disabled)
		}
	}

	// Verify orchestrator is primary mode.
	orch, _ := agents["orchestrator"].(map[string]any)
	if orch["mode"] != "primary" {
		t.Error("orchestrator should be primary mode")
	}
	orchPrompt, _ := orch["prompt"].(string)
	if !strings.Contains(orchPrompt, "/test/prompts/orchestrator.md") {
		t.Errorf("orchestrator prompt should reference prompts dir, got: %s", orchPrompt)
	}

	// Every sub-agent must have full config.
	for _, id := range []string{"bootstrap", "investigate", "implement", "team-lead"} {
		entry, _ := agents[id].(map[string]any)
		for _, field := range []string{"color", "prompt", "description", "steps", "temperature", "tools", "mode"} {
			if entry[field] == nil {
				t.Errorf("agent %q missing %s", id, field)
			}
		}
		tools, _ := entry["tools"].(map[string]any)
		if tools["task"] == nil {
			t.Errorf("agent %q missing tools.task", id)
		}
		// Prompt must reference skill path.
		prompt, _ := entry["prompt"].(string)
		if !strings.Contains(prompt, "/test/skills/"+id+"/SKILL.md") {
			t.Errorf("agent %q prompt should reference skill path", id)
		}
	}

	// team-lead should have permission block.
	tl, _ := agents["team-lead"].(map[string]any)
	if tl["permission"] == nil {
		t.Error("team-lead should have permission block")
	}
}

func TestBuildAgentOverlay_Empty(t *testing.T) {
	overlay := buildAgentOverlay(nil, "/test/skills", "/test/prompts")
	var parsed map[string]any
	if err := json.Unmarshal(overlay, &parsed); err != nil {
		t.Fatalf("invalid JSON for empty input: %v", err)
	}
	// Should still have orchestrator + build + plan = 3
	agents, _ := parsed["agent"].(map[string]any)
	if len(agents) != 3 {
		t.Errorf("expected 3 agents (orchestrator+disabled), got %d", len(agents))
	}
}

// ---------------------------------------------------------------------------
// coordinatorSkills
// ---------------------------------------------------------------------------

func TestCoordinatorSkills(t *testing.T) {
	// Only 3 agents are coordinators.
	for _, id := range []string{"team-lead", "debate", "parallel-dispatch"} {
		if !isCoordinator(id) {
			t.Errorf("%q should be a coordinator", id)
		}
	}
	// Leaf agents are NOT coordinators.
	for _, leaf := range []string{"implement", "validate", "bootstrap", "investigate", "architect"} {
		if isCoordinator(leaf) {
			t.Errorf("leaf agent %q should not be a coordinator", leaf)
		}
	}
}

// ---------------------------------------------------------------------------
// FilesToBackup
// ---------------------------------------------------------------------------

func TestFilesToBackup_WithCommands(t *testing.T) {
	adapter := opencode.NewAdapter()
	paths := FilesToBackup("/home/test", adapter)

	hasCommand := false
	hasSharedSkill := false
	hasPrompt := false
	for _, p := range paths {
		normalized := filepath.ToSlash(p)
		if strings.Contains(normalized, "commands") && strings.HasSuffix(p, ".md") {
			hasCommand = true
		}
		if strings.Contains(normalized, ".cortex-ia/skills/bootstrap") {
			hasSharedSkill = true
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
	if !hasPrompt {
		t.Error("expected shared orchestrator prompt")
	}
}

func TestFilesToBackup_NoPromptNoCommands(t *testing.T) {
	adapter := &stubAdapter{agentID: "test"}
	paths := FilesToBackup("/tmp/test", adapter)
	// Should still have shared sub-agent skills (11) + orchestrator prompt = 12.
	// Convention file is owned by the conventions component.
	if len(paths) < 12 {
		t.Errorf("expected at least 12 paths (11 skills + prompt), got %d", len(paths))
	}
}
