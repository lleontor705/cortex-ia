package sdd

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/agents"
	"github.com/lleontor705/cortex-ia/internal/assets"
	"github.com/lleontor705/cortex-ia/internal/components/filemerge"
	"github.com/lleontor705/cortex-ia/internal/model"
	"github.com/lleontor705/cortex-ia/internal/state"
)

// InjectionResult describes the outcome of SDD injection.
type InjectionResult struct {
	Changed bool
	Files   []string
}

// sddSkillIDs are the core SDD skills written by this component.
var sddSkillIDs = []string{
	"bootstrap", "investigate", "draft-proposal", "write-specs",
	"architect", "decompose", "team-lead", "implement", "validate", "finalize",
	"debate", "debug", "execute-plan", "ideate", "monitor",
	"open-pr", "file-issue", "parallel-dispatch", "scan-registry",
}

// sddSkillDescriptions maps each SDD skill ID to a short description used in
// sub-agent frontmatter so that OpenCode recognises the agent type.
var sddSkillDescriptions = map[string]string{
	"bootstrap":         "Detects tech stack, conventions, and initialises SDD context",
	"investigate":       "Explores codebase, diagnoses bugs, and compares approaches",
	"draft-proposal":    "Creates change proposals with intent, scope, and rollback plan",
	"write-specs":       "Transforms proposals into Given/When/Then specifications",
	"architect":         "Designs technical architecture and data flows for a change",
	"decompose":         "Breaks designs into phased, dependency-ordered implementation tasks",
	"team-lead":         "Coordinates the apply phase, launching implement agents in parallel",
	"implement":         "Executes implementation tasks, writing production code",
	"validate":          "Verifies implementation satisfies specs with execution evidence",
	"finalize":          "Merges delta specs, archives changes, and generates retrospective",
	"debate":            "Moderates adversarial debates between competing approaches",
	"debug":             "Systematic root-cause debugging before proposing fixes",
	"execute-plan":      "Executes written implementation plans with review checkpoints",
	"ideate":            "Collaborative brainstorming to explore intent and requirements",
	"monitor":           "Generates dashboard visualising SDD pipeline state and tasks",
	"open-pr":           "Creates pull requests following issue-first enforcement",
	"file-issue":        "Creates GitHub issues using required templates",
	"parallel-dispatch": "Dispatches independent tasks to parallel sub-agents",
	"scan-registry":     "Scans skill directories and builds unified skill registry",
}

// coordinatorSkills lists skills that are allowed to delegate to sub-agents
// via the task tool. All other skills are leaf agents with task disabled.
var coordinatorSkills = map[string]bool{
	"team-lead":         true,
	"debate":            true,
	"parallel-dispatch": true,
}

// isCoordinator reports whether a skill is allowed to delegate to sub-agents.
func isCoordinator(skillID string) bool {
	return coordinatorSkills[skillID]
}

// agentRole classifies each SDD skill for tool matrix generation.
type agentRole int

const (
	roleLeafReader  agentRole = iota // read-only exploration (investigate, ideate)
	roleLeafPlanner                  // read-only planning (draft-proposal, write-specs, architect, decompose)
	roleLeafWriter                   // code-writing leaf (implement, bootstrap, finalize, debug, execute-plan)
	roleLeafOps                      // git/CI operations (open-pr, file-issue, scan-registry, monitor)
	roleLeafVerify                   // verification with execution (validate)
	roleCoordinator                  // can delegate via task() (team-lead, debate, parallel-dispatch)
)

// agentRoles maps each SDD skill to its role.
var agentRoles = map[string]agentRole{
	"bootstrap":         roleLeafWriter,
	"investigate":       roleLeafReader,
	"draft-proposal":    roleLeafPlanner,
	"write-specs":       roleLeafPlanner,
	"architect":         roleLeafPlanner,
	"decompose":         roleLeafPlanner,
	"team-lead":         roleCoordinator,
	"implement":         roleLeafWriter,
	"validate":          roleLeafVerify,
	"finalize":          roleLeafWriter,
	"debate":            roleCoordinator,
	"debug":             roleLeafWriter,
	"execute-plan":      roleLeafWriter,
	"ideate":            roleLeafReader,
	"monitor":           roleLeafOps,
	"open-pr":           roleLeafOps,
	"file-issue":        roleLeafOps,
	"parallel-dispatch": roleCoordinator,
	"scan-registry":     roleLeafOps,
}

// agentColors assigns a hex color per skill for the OpenCode UI.
var agentColors = map[string]string{
	"bootstrap":         "#607D8B",
	"investigate":       "#78909C",
	"draft-proposal":    "#90A4AE",
	"write-specs":       "#B0BEC5",
	"architect":         "#546E7A",
	"decompose":         "#455A64",
	"team-lead":         "#FF8F00",
	"implement":         "#2E7D32",
	"validate":          "#F57F17",
	"finalize":          "#37474F",
	"debate":            "#6A1B9A",
	"debug":             "#C62828",
	"execute-plan":      "#1565C0",
	"ideate":            "#00838F",
	"monitor":           "#4527A0",
	"open-pr":           "#2E7D32",
	"file-issue":        "#EF6C00",
	"parallel-dispatch": "#00695C",
	"scan-registry":     "#795548",
}

// agentSteps returns the max agentic iterations for a skill.
func agentSteps(skillID string) int {
	switch agentRoles[skillID] {
	case roleCoordinator:
		return 50
	case roleLeafWriter:
		return 60
	case roleLeafVerify:
		return 40
	case roleLeafReader:
		return 40
	default:
		return 30
	}
}

// agentTemperature returns the LLM temperature for a skill.
func agentTemperature(skillID string) float64 {
	switch skillID {
	case "validate":
		return 0.1
	case "investigate", "draft-proposal", "ideate":
		return 0.3
	default:
		return 0.2
	}
}

// toolsForRole returns the full OpenCode tools matrix for a given role.
func toolsForRole(role agentRole) map[string]any {
	// Base tools shared by all sub-agents.
	base := map[string]any{
		"bash": true, "read": true, "glob": true, "grep": true, "list": true,
		"question": true, "cortex_*": true, "sdd_*": true, "msg_*": true, "tb_*": true,
		"cli_*": true,
		// Disabled by default for sub-agents.
		"skill": false, "todoread": false, "todowrite": false, "playwright_*": false,
	}
	switch role {
	case roleLeafReader:
		base["edit"] = false
		base["write"] = false
		base["patch"] = false
		base["task"] = false
		base["lsp"] = true
		base["webfetch"] = true
		base["websearch"] = true
	case roleLeafPlanner:
		base["edit"] = false
		base["write"] = false
		base["patch"] = false
		base["task"] = false
		base["lsp"] = false
		base["webfetch"] = false
		base["websearch"] = false
	case roleLeafWriter:
		base["edit"] = true
		base["write"] = true
		base["patch"] = true
		base["task"] = false
		base["lsp"] = true
		base["file_*"] = true
		base["webfetch"] = true
		base["websearch"] = false
	case roleLeafOps:
		base["edit"] = true
		base["write"] = true
		base["patch"] = false
		base["task"] = false
		base["lsp"] = false
		base["webfetch"] = false
		base["websearch"] = false
	case roleLeafVerify:
		base["edit"] = false
		base["write"] = false
		base["patch"] = false
		base["task"] = false
		base["lsp"] = false
		base["webfetch"] = true
		base["websearch"] = true
	case roleCoordinator:
		base["edit"] = false
		base["write"] = false
		base["patch"] = false
		base["task"] = true
		base["lsp"] = false
		base["file_*"] = true
		base["webfetch"] = false
		base["websearch"] = false
	}
	return base
}

// Inject injects the full SDD workflow into the given agent:
// 1. Orchestrator prompt into system prompt (+ copy to shared dir)
// 2. SDD skill files into shared ~/.cortex-ia/skills/ directory
// 3. Slash commands (OpenCode only)
// 4. Sub-agent stubs (OpenCode only)
func Inject(homeDir string, adapter agents.Adapter, assignments model.ModelAssignments) (InjectionResult, error) {
	files := make([]string, 0)
	changed := false

	// 1. Inject SDD orchestrator into system prompt.
	if adapter.SupportsSystemPrompt() {
		result, err := injectOrchestratorPrompt(homeDir, adapter, assignments)
		if err != nil {
			return InjectionResult{}, fmt.Errorf("sdd orchestrator prompt: %w", err)
		}
		changed = changed || result.Changed
		files = append(files, result.Files...)
	}

	// 2. Write SDD skill files to shared directory (~/.cortex-ia/skills/).
	{
		result, err := injectSkillFiles(homeDir)
		if err != nil {
			return InjectionResult{}, fmt.Errorf("sdd skills: %w", err)
		}
		changed = changed || result.Changed
		files = append(files, result.Files...)
	}

	// 3. Write slash commands (OpenCode only).
	if adapter.SupportsSlashCommands() {
		result, err := injectCommands(homeDir, adapter)
		if err != nil {
			return InjectionResult{}, fmt.Errorf("sdd commands: %w", err)
		}
		changed = changed || result.Changed
		files = append(files, result.Files...)
	}

	// 4. Write sub-agent files if supported.
	if adapter.SupportsSubAgents() {
		result, err := injectSubAgents(homeDir, adapter)
		if err != nil {
			return InjectionResult{}, fmt.Errorf("sdd sub-agents: %w", err)
		}
		changed = changed || result.Changed
		files = append(files, result.Files...)
	}

	return InjectionResult{Changed: changed, Files: files}, nil
}

// injectOrchestratorPrompt injects the SDD orchestrator instructions into the
// agent's system prompt and writes a copy to the shared prompts directory.
func injectOrchestratorPrompt(homeDir string, adapter agents.Adapter, assignments model.ModelAssignments) (InjectionResult, error) {
	// Choose the right orchestrator variant based on task delegation support.
	assetPath := "generic/sdd-orchestrator-single.md"
	if adapter.SupportsTaskDelegation() {
		assetPath = "generic/sdd-orchestrator.md"
	}

	content, err := assets.Read(assetPath)
	if err != nil {
		return InjectionResult{}, err
	}

	// Template {{SKILLS_DIR}} with the shared skills directory (~/.cortex-ia/skills/).
	sharedSkillsDir := filepath.ToSlash(state.SharedSkillsDir(homeDir))
	content = strings.ReplaceAll(content, "{{SKILLS_DIR}}", sharedSkillsDir)

	// Template {{MODEL_ASSIGNMENTS}} with per-phase model routing table.
	if assignments == nil {
		assignments = model.ModelsForPreset(model.ModelPresetBalanced)
	}
	content = strings.ReplaceAll(content, "{{MODEL_ASSIGNMENTS}}", model.FormatModelAssignments(assignments))

	files := make([]string, 0, 2)
	changed := false

	// Write orchestrator prompt to shared dir for inspection/reference.
	sharedPromptPath := filepath.Join(state.SharedPromptsDir(homeDir), "orchestrator.md")
	wr, err := filemerge.WriteFileAtomic(sharedPromptPath, []byte(content), 0o644)
	if err != nil {
		return InjectionResult{}, fmt.Errorf("write shared orchestrator prompt: %w", err)
	}
	changed = changed || wr.Changed
	files = append(files, sharedPromptPath)

	// Inject into agent's system prompt file.
	promptFile := adapter.SystemPromptFile(homeDir)
	if promptFile == "" {
		return InjectionResult{Changed: changed, Files: files}, nil
	}

	existing, err := os.ReadFile(promptFile)
	if err != nil && !os.IsNotExist(err) {
		return InjectionResult{}, fmt.Errorf("read system prompt: %w", err)
	}
	updated := filemerge.InjectMarkdownSection(string(existing), "sdd-orchestrator", content)
	wr, err = filemerge.WriteFileAtomic(promptFile, []byte(updated), 0o644)
	if err != nil {
		return InjectionResult{}, err
	}
	changed = changed || wr.Changed
	files = append(files, promptFile)

	return InjectionResult{Changed: changed, Files: files}, nil
}

// injectSkillFiles writes SDD skill files and the shared convention to the
// canonical shared directory (~/.cortex-ia/skills/). Convention references in
// each SKILL.md are replaced with the absolute path so sub-agents can read
// the convention regardless of their working directory.
func injectSkillFiles(homeDir string) (InjectionResult, error) {
	sharedSkillsDir := state.SharedSkillsDir(homeDir)

	files := make([]string, 0)
	changed := false

	// 1. Write convention to _shared/ alongside skills.
	conventionContent, err := assets.Read("skills/_shared/cortex-convention.md")
	if err != nil {
		return InjectionResult{}, fmt.Errorf("read cortex-convention asset: %w", err)
	}
	conventionPath := filepath.Join(sharedSkillsDir, "_shared", "cortex-convention.md")
	wr, err := filemerge.WriteFileAtomic(conventionPath, []byte(conventionContent), 0o644)
	if err != nil {
		return InjectionResult{}, fmt.Errorf("write cortex-convention: %w", err)
	}
	changed = changed || wr.Changed
	files = append(files, conventionPath)

	// Absolute path for convention (forward slashes for cross-platform).
	conventionAbsPath := filepath.ToSlash(conventionPath)

	// 2. Write embedded skills with convention references replaced by absolute path.
	for _, skillID := range sddSkillIDs {
		assetPath := "skills/" + skillID + "/SKILL.md"
		content, err := assets.Read(assetPath)
		if err != nil {
			log.Printf("sdd: skipping skill %q — asset not found: %v", skillID, err)
			continue
		}

		// Replace relative convention references with absolute path.
		content = fixConventionRefs(content, conventionAbsPath)

		path := filepath.Join(sharedSkillsDir, skillID, "SKILL.md")
		wr, err := filemerge.WriteFileAtomic(path, []byte(content), 0o644)
		if err != nil {
			return InjectionResult{}, fmt.Errorf("write skill %q: %w", skillID, err)
		}
		changed = changed || wr.Changed
		files = append(files, path)
	}

	// 3. Load community skills from ~/.cortex-ia/skills-community/.
	communityDir := filepath.Join(homeDir, ".cortex-ia", "skills-community")
	communityFiles, communityChanged := loadExternalSkills(communityDir, sharedSkillsDir, conventionAbsPath)
	files = append(files, communityFiles...)
	changed = changed || communityChanged

	return InjectionResult{Changed: changed, Files: files}, nil
}

// loadExternalSkills copies SKILL.md files from an external directory into the
// shared skills directory, fixing convention references. Returns written files
// and whether any changes occurred.
func loadExternalSkills(sourceDir, targetDir, conventionAbsPath string) ([]string, bool) {
	entries, err := os.ReadDir(sourceDir)
	if err != nil {
		return nil, false // directory doesn't exist — not an error
	}

	var files []string
	changed := false

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		skillID := entry.Name()
		srcPath := filepath.Join(sourceDir, skillID, "SKILL.md")
		content, err := os.ReadFile(srcPath)
		if err != nil {
			continue // no SKILL.md in this dir
		}

		fixed := fixConventionRefs(string(content), conventionAbsPath)
		dstPath := filepath.Join(targetDir, skillID, "SKILL.md")
		wr, err := filemerge.WriteFileAtomic(dstPath, []byte(fixed), 0o644)
		if err != nil {
			log.Printf("sdd: skipping community skill %q: %v", skillID, err)
			continue
		}
		changed = changed || wr.Changed
		files = append(files, dstPath)
	}

	return files, changed
}

// fixConventionRefs replaces relative convention file references with the
// absolute path so sub-agents can read the file from any working directory.
func fixConventionRefs(content, absolutePath string) string {
	content = strings.ReplaceAll(content, "../_shared/cortex-convention.md", absolutePath)
	content = strings.ReplaceAll(content, "skills/_shared/cortex-convention.md", absolutePath)
	return content
}

// injectCommands writes slash command files for agents that support them (OpenCode).
func injectCommands(homeDir string, adapter agents.Adapter) (InjectionResult, error) {
	commandsDir := adapter.CommandsDir(homeDir)
	if commandsDir == "" {
		return InjectionResult{}, nil
	}

	entries, err := assets.ListDir("opencode/commands")
	if err != nil {
		return InjectionResult{}, fmt.Errorf("list command assets: %w", err)
	}

	files := make([]string, 0)
	changed := false

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		content, err := assets.Read("opencode/commands/" + entry.Name())
		if err != nil {
			log.Printf("sdd: skipping command %q: %v", entry.Name(), err)
			continue
		}

		path := filepath.Join(commandsDir, entry.Name())
		wr, err := filemerge.WriteFileAtomic(path, []byte(content), 0o644)
		if err != nil {
			return InjectionResult{}, fmt.Errorf("write command %q: %w", entry.Name(), err)
		}
		changed = changed || wr.Changed
		files = append(files, path)
	}

	return InjectionResult{Changed: changed, Files: files}, nil
}

// injectSubAgents writes sub-agent definition files for agents that support them.
// For OpenCode, it also merges hidden:true into opencode.json so sub-agents
// don't appear in the UI — only the orchestrator is visible.
func injectSubAgents(homeDir string, adapter agents.Adapter) (InjectionResult, error) {
	subAgentsDir := adapter.SubAgentsDir(homeDir)
	if subAgentsDir == "" {
		return InjectionResult{}, nil
	}

	// Generate sub-agent stubs with absolute skill paths pointing to the
	// shared directory (~/.cortex-ia/skills/).
	skillsDir := filepath.ToSlash(state.SharedSkillsDir(homeDir))
	files := make([]string, 0)
	changed := false

	for _, skillID := range sddSkillIDs {
		skillPath := skillsDir + "/" + skillID + "/SKILL.md"
		desc := sddSkillDescriptions[skillID]
		if desc == "" {
			desc = "SDD sub-agent: " + skillID
		}
		content := fmt.Sprintf("---\ndescription: %s\nmode: subagent\n---\n\nRead and follow the skill instructions from: %s\n", desc, skillPath)
		path := filepath.Join(subAgentsDir, skillID+".md")
		wr, err := filemerge.WriteFileAtomic(path, []byte(content), 0o644)
		if err != nil {
			return InjectionResult{}, fmt.Errorf("write sub-agent %q: %w", skillID, err)
		}
		changed = changed || wr.Changed
		files = append(files, path)
	}

	// For OpenCode: merge hidden:true into settings so sub-agents are not
	// visible in the UI. Only the orchestrator remains selectable.
	if adapter.Agent() == model.AgentOpenCode {
		settingsPath := adapter.SettingsPath(homeDir)
		if settingsPath != "" {
			overlay := buildAgentOverlay(sddSkillIDs, skillsDir)
			baseJSON, err := os.ReadFile(settingsPath)
			if err != nil && !os.IsNotExist(err) {
				return InjectionResult{}, fmt.Errorf("read agent settings: %w", err)
			}
			merged, err := filemerge.MergeJSONObjects(baseJSON, overlay)
			if err != nil {
				return InjectionResult{}, fmt.Errorf("merge agent hidden flags: %w", err)
			}
			wr, err := filemerge.WriteFileAtomic(settingsPath, merged, 0o644)
			if err != nil {
				return InjectionResult{}, fmt.Errorf("write agent hidden flags: %w", err)
			}
			changed = changed || wr.Changed
			files = append(files, settingsPath)
		}
	}

	return InjectionResult{Changed: changed, Files: files}, nil
}

// buildAgentOverlay builds a full JSON overlay for all SDD skill agents,
// including mode, hidden, color, description, prompt, steps, temperature,
// and the complete tools matrix based on each agent's role.
func buildAgentOverlay(skillIDs []string, skillsDir string) []byte {
	agents := make(map[string]any, len(skillIDs))
	for _, id := range skillIDs {
		role := agentRoles[id]
		skillPath := skillsDir + "/" + id + "/SKILL.md"
		prompt := fmt.Sprintf(
			"You are the **%s** agent. Read your skill file at %s and follow its instructions exactly. "+
				"When ENABLED CLIs are specified in your task prompt, you MUST use them for validation, "+
				"cross-checking, or code generation. Run at least ONE CLI consultation per task unless CLIs are set to 'none'.",
			id, skillPath)
		agents[id] = map[string]any{
			"mode":        "subagent",
			"hidden":      true,
			"color":       agentColors[id],
			"description": sddSkillDescriptions[id],
			"prompt":      prompt,
			"steps":       agentSteps(id),
			"temperature": agentTemperature(id),
			"tools":       toolsForRole(role),
		}
	}
	data, _ := json.Marshal(map[string]any{"agent": agents})
	return data
}

// FilesToBackup returns all file paths that SDD injection would modify for the given agent.
// Used by the backup system to snapshot before injection.
func FilesToBackup(homeDir string, adapter agents.Adapter) []string {
	paths := make([]string, 0)

	// Agent-specific system prompt.
	if adapter.SupportsSystemPrompt() {
		if f := adapter.SystemPromptFile(homeDir); f != "" {
			paths = append(paths, f)
		}
	}

	// Shared skills directory (~/.cortex-ia/skills/).
	sharedSkillsDir := state.SharedSkillsDir(homeDir)
	paths = append(paths, filepath.Join(sharedSkillsDir, "_shared", "cortex-convention.md"))
	for _, id := range sddSkillIDs {
		paths = append(paths, filepath.Join(sharedSkillsDir, id, "SKILL.md"))
	}

	// Shared orchestrator prompt.
	paths = append(paths, filepath.Join(state.SharedPromptsDir(homeDir), "orchestrator.md"))

	// Agent-specific slash commands (OpenCode).
	if adapter.SupportsSlashCommands() {
		commandsDir := adapter.CommandsDir(homeDir)
		entries, _ := fs.ReadDir(assets.FS, "opencode/commands")
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
				paths = append(paths, filepath.Join(commandsDir, e.Name()))
			}
		}
	}

	return paths
}
