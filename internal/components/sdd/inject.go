package sdd

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/lleontor705/cortex-ia/internal/agents"
	"github.com/lleontor705/cortex-ia/internal/assets"
	"github.com/lleontor705/cortex-ia/internal/components/filemerge"
	"github.com/lleontor705/cortex-ia/internal/model"
	"github.com/lleontor705/cortex-ia/internal/state"
)

// sharedWriteOnce ensures shared directory writes (orchestrator prompt, skill files)
// happen only once across parallel agent chains, preventing file lock conflicts on Windows.
var sharedWriteOnce sync.Once
var sharedWriteErr error
var sharedWriteFiles []string

// ResetSharedWrite resets the sync.Once for testing. Must be called between test runs.
func ResetSharedWrite() {
	sharedWriteOnce = sync.Once{}
	sharedWriteErr = nil
	sharedWriteFiles = nil
}

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
	"bootstrap":         "Bootstrap SDD context — detects project stack, conventions, and persistence backend",
	"investigate":       "Investigate codebase, analyze bugs, assess migrations, compare approaches. Supports FOCUS modes: ARCHITECTURE, INVESTIGATION, MIGRATION, GENERAL.",
	"draft-proposal":    "Create a change proposal from exploration results",
	"write-specs":       "Write delta specifications from an approved proposal",
	"architect":         "Create technical design decisions from an approved proposal",
	"decompose":         "Break specs and design into ordered implementation tasks",
	"team-lead":         "Owns the entire apply phase — executes task board groups sequentially, launches @implement sub-agents in parallel within each group, manages file reservations and retries.",
	"implement":         "Implement code changes following SDD specs, design, and task definitions. Supports TASK-TYPE modes: IMPLEMENTATION, REFACTOR, DATABASE, INFRASTRUCTURE, DOCUMENTATION.",
	"validate":          "Validate implementation against specs — runs tests, generates compliance matrix, and applies quality/security/performance review lenses.",
	"finalize":          "Archive completed SDD change — merges delta specs and closes the cycle",
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
	"team-lead":         "#1565C0",
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
	"orchestrator":      "#4A90D9",
}

// agentSteps returns the max agentic iterations for a skill.
func agentSteps(skillID string) int {
	switch skillID {
	case "team-lead":
		return 80
	case "implement":
		return 60
	case "orchestrator":
		return 50
	case "investigate", "validate":
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
// Starts from a base set of tools shared by all sub-agents, then applies
// role-specific overrides for permissions that differ.
func toolsForRole(role agentRole) map[string]any {
	// Base: read-only leaf agent with no write/delegation capabilities.
	tools := map[string]any{
		"bash": true, "read": true, "glob": true, "grep": true, "list": true,
		"question": true, "engram_*": true, "sdd_*": true, "msg_*": true,
		"tb_*": true, "cli_*": true,
		"edit": false, "write": false, "patch": false, "task": false,
		"lsp": false, "webfetch": false, "websearch": false,
		"skill": false, "todoread": false, "todowrite": false, "playwright_*": false,
	}

	switch role {
	case roleLeafReader:
		tools["lsp"] = true
		tools["webfetch"] = true
		tools["websearch"] = true
	case roleLeafPlanner:
		// Base is already correct for planners.
	case roleLeafWriter:
		tools["edit"] = true
		tools["write"] = true
		tools["patch"] = true
		tools["lsp"] = true
		tools["webfetch"] = true
		tools["file_*"] = true
	case roleLeafOps:
		tools["edit"] = true
		tools["write"] = true
	case roleLeafVerify:
		tools["task"] = true
		tools["webfetch"] = true
		tools["websearch"] = true
	case roleCoordinator:
		tools["bash"] = false
		tools["read"] = false
		tools["glob"] = false
		tools["grep"] = false
		tools["list"] = false
		tools["task"] = true
		tools["file_*"] = true
	}

	return tools
}

// Inject injects the full SDD workflow into the given agent:
// 1. Orchestrator prompt into system prompt (+ copy to shared dir)
// 2. SDD skill files into shared ~/.cortex-ia/skills/ directory
// 3. Slash commands (OpenCode only)
// 4. Sub-agent stubs (OpenCode only)
func Inject(homeDir string, adapter agents.Adapter, assignments model.ModelAssignments) (InjectionResult, error) {
	files := make([]string, 0)
	changed := false

	// 1-2. Write shared files (orchestrator prompt + skills) only once across all agents.
	// This prevents file lock conflicts on Windows when agents run in parallel.
	sharedWriteOnce.Do(func() {
		// Write shared orchestrator prompt.
		sharedPromptPath := filepath.Join(state.SharedPromptsDir(homeDir), "orchestrator.md")
		content, err := buildPromptContent("generic/sdd-orchestrator.md", homeDir, assignments)
		if err != nil {
			sharedWriteErr = fmt.Errorf("sdd orchestrator prompt: %w", err)
			return
		}
		if _, err := filemerge.WriteFileAtomic(sharedPromptPath, []byte(content), 0o644); err != nil {
			sharedWriteErr = fmt.Errorf("write shared orchestrator prompt: %w", err)
			return
		}
		sharedWriteFiles = append(sharedWriteFiles, sharedPromptPath)

		// Write shared skill files (only sub-agent skills, not utility skills).
		// Utility skills go to agent-local dirs only.
		// Clean up utility skills from shared dir if left from prior installations.
		for _, utilID := range openCodeLocalSkills {
			staleDir := filepath.Join(state.SharedSkillsDir(homeDir), utilID)
			if _, statErr := os.Stat(staleDir); statErr == nil {
				_ = os.RemoveAll(staleDir)
			}
		}
		skillResult, err := injectSkillFiles(homeDir)
		if err != nil {
			sharedWriteErr = fmt.Errorf("sdd skills: %w", err)
			return
		}
		sharedWriteFiles = append(sharedWriteFiles, skillResult.Files...)
	})
	if sharedWriteErr != nil {
		return InjectionResult{}, sharedWriteErr
	}
	files = append(files, sharedWriteFiles...)

	// Inject orchestrator prompt into agent-specific system prompt file.
	if adapter.SupportsSystemPrompt() {
		result, err := injectAgentPrompt(homeDir, adapter, assignments)
		if err != nil {
			return InjectionResult{}, fmt.Errorf("sdd orchestrator prompt: %w", err)
		}
		changed = changed || result.Changed
		files = append(files, result.Files...)
	}

	// 3. Copy/write skills to the agent-local skills directory.
	// For OpenCode: writes utility skills (not sub-agents) to ~/.config/opencode/skills/.
	// For other agents: copies all 19 skills from shared to agent-local.
	if adapter.SupportsSkills() {
		result, err := copySkillsToAgent(homeDir, adapter)
		if err != nil {
			return InjectionResult{}, fmt.Errorf("sdd agent skills: %w", err)
		}
		changed = changed || result.Changed
		files = append(files, result.Files...)
	}

	// 4. Write slash commands (OpenCode only).
	if adapter.SupportsSlashCommands() {
		result, err := injectCommands(homeDir, adapter)
		if err != nil {
			return InjectionResult{}, fmt.Errorf("sdd commands: %w", err)
		}
		changed = changed || result.Changed
		files = append(files, result.Files...)
	}

	// 5. Write sub-agent files if supported.
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

// buildPromptContent reads an orchestrator asset and applies template substitutions.
func buildPromptContent(assetPath, homeDir string, assignments model.ModelAssignments) (string, error) {
	content, err := assets.Read(assetPath)
	if err != nil {
		return "", err
	}
	content = strings.ReplaceAll(content, "{{SKILLS_DIR}}", filepath.ToSlash(state.SharedSkillsDir(homeDir)))
	if assignments == nil {
		assignments = model.ModelsForPreset(model.ModelPresetBalanced)
	}
	content = strings.ReplaceAll(content, "{{MODEL_ASSIGNMENTS}}", model.FormatModelAssignments(assignments))
	return content, nil
}

// injectAgentPrompt writes the orchestrator prompt to the agent-specific system
// prompt file (and OpenCode local prompts dir). Shared dir writes are handled
// separately via sync.Once in Inject() to prevent Windows file lock conflicts.
func injectAgentPrompt(homeDir string, adapter agents.Adapter, assignments model.ModelAssignments) (InjectionResult, error) {
	assetPath := "generic/sdd-orchestrator-single.md"
	if adapter.SupportsTaskDelegation() {
		assetPath = "generic/sdd-orchestrator.md"
	}

	content, err := buildPromptContent(assetPath, homeDir, assignments)
	if err != nil {
		return InjectionResult{}, err
	}

	files := make([]string, 0, 2)
	changed := false

	// For OpenCode: write to agent-local prompts dir.
	if adapter.Agent() == model.AgentOpenCode {
		agentPromptPath := filepath.Join(adapter.GlobalConfigDir(homeDir), "prompts", "orchestrator.md")
		wr, err := filemerge.WriteFileAtomic(agentPromptPath, []byte(content), 0o644)
		if err != nil {
			return InjectionResult{}, fmt.Errorf("write agent orchestrator prompt: %w", err)
		}
		changed = changed || wr.Changed
		files = append(files, agentPromptPath)
	}

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
	wr, err := filemerge.WriteFileAtomic(promptFile, []byte(updated), 0o644)
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
//
// Only sub-agent skills (openCodeSubAgents) are written to the shared directory.
// Utility skills are written to agent-local directories by copySkillsToAgent.
func injectSkillFiles(homeDir string) (InjectionResult, error) {
	sharedSkillsDir := state.SharedSkillsDir(homeDir)

	files := make([]string, 0)
	changed := false

	// Convention file is written by the conventions component — compute the
	// absolute path here so fixConventionRefs can rewrite relative references.
	conventionAbsPath := filepath.ToSlash(filepath.Join(sharedSkillsDir, "_shared", "cortex-convention.md"))

	// 2. Write sub-agent skills to the shared directory.
	// Utility skills (debate, debug, etc.) are written to agent-local dirs by copySkillsToAgent.
	for _, skillID := range openCodeSubAgents {
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
	// Also replace absolute-looking references that skills may use.
	content = strings.ReplaceAll(content, "~/.cortex-ia/skills/_shared/cortex-convention.md", absolutePath)
	content = strings.ReplaceAll(content, "~/.cortex-ia/_shared/cortex-convention.md", absolutePath)
	content = strings.ReplaceAll(content, "~/.cortex-ia/cortex-convention.md", absolutePath)
	return content
}

// copySkillsToAgent writes skills to the agent-local skills directory.
//
// For OpenCode: writes only utility skills (openCodeLocalSkills) directly from
// embedded assets to ~/.config/opencode/skills/. Sub-agent skills live in the
// global shared directory and are referenced from opencode.json.
//
// For other agents: copies all 19 skills from the shared directory
// (~/.cortex-ia/skills/) to the agent-local directory (e.g. ~/.claude/skills/).
func copySkillsToAgent(homeDir string, adapter agents.Adapter) (InjectionResult, error) {
	agentSkillsDir := adapter.SkillsDir(homeDir)
	if agentSkillsDir == "" {
		return InjectionResult{}, nil
	}

	files := make([]string, 0)
	changed := false

	if adapter.Agent() == model.AgentOpenCode {
		// OpenCode: only utility skills belong in the local dir.
		// Remove sub-agent skills and _shared that may remain from prior installations.
		for _, skillID := range openCodeSubAgents {
			staleDir := filepath.Join(agentSkillsDir, skillID)
			if _, err := os.Stat(staleDir); err == nil {
				_ = os.RemoveAll(staleDir)
			}
		}
		staleShared := filepath.Join(agentSkillsDir, "_shared")
		if _, err := os.Stat(staleShared); err == nil {
			_ = os.RemoveAll(staleShared)
		}

		// Write utility skills from embedded assets to local dir.
		sharedConventionPath := filepath.ToSlash(filepath.Join(
			state.SharedSkillsDir(homeDir), "_shared", "cortex-convention.md"))

		for _, skillID := range openCodeLocalSkills {
			assetPath := "skills/" + skillID + "/SKILL.md"
			content, err := assets.Read(assetPath)
			if err != nil {
				continue
			}
			content = fixConventionRefs(content, sharedConventionPath)
			dst := filepath.Join(agentSkillsDir, skillID, "SKILL.md")
			wr, err := filemerge.WriteFileAtomic(dst, []byte(content), 0o644)
			if err != nil {
				return InjectionResult{}, fmt.Errorf("write local skill %q: %w", skillID, err)
			}
			changed = changed || wr.Changed
			files = append(files, dst)
		}
		return InjectionResult{Changed: changed, Files: files}, nil
	}

	// Other agents: write all 19 skills + convention from embedded assets.
	sharedConventionPath := filepath.ToSlash(filepath.Join(
		state.SharedSkillsDir(homeDir), "_shared", "cortex-convention.md"))

	// Write convention.
	conventionContent, err := assets.Read("skills/_shared/cortex-convention.md")
	if err == nil {
		conventionDst := filepath.Join(agentSkillsDir, "_shared", "cortex-convention.md")
		wr, err := filemerge.WriteFileAtomic(conventionDst, []byte(conventionContent), 0o644)
		if err != nil {
			return InjectionResult{}, fmt.Errorf("write convention to agent: %w", err)
		}
		changed = changed || wr.Changed
		files = append(files, conventionDst)
	}

	// Write each skill from embedded assets.
	for _, skillID := range sddSkillIDs {
		assetPath := "skills/" + skillID + "/SKILL.md"
		content, err := assets.Read(assetPath)
		if err != nil {
			continue
		}
		content = fixConventionRefs(content, sharedConventionPath)
		dst := filepath.Join(agentSkillsDir, skillID, "SKILL.md")
		wr, err := filemerge.WriteFileAtomic(dst, []byte(content), 0o644)
		if err != nil {
			return InjectionResult{}, fmt.Errorf("write skill %q to agent: %w", skillID, err)
		}
		changed = changed || wr.Changed
		files = append(files, dst)
	}

	return InjectionResult{Changed: changed, Files: files}, nil
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

// openCodeSubAgents lists the SDD skills that become sub-agents in opencode.json.
// These are written to the global shared directory (~/.cortex-ia/skills/).
var openCodeSubAgents = []string{
	"bootstrap", "investigate", "draft-proposal", "write-specs",
	"architect", "decompose", "team-lead", "implement", "validate", "finalize",
	"parallel-dispatch",
}

// openCodeLocalSkills lists utility skills that are written to the agent-local
// skills directory (~/.config/opencode/skills/) instead of the shared directory.
// These are NOT sub-agents — they are invoked via slash commands or by the orchestrator.
var openCodeLocalSkills = []string{
	"debate", "debug", "execute-plan", "file-issue",
	"ideate", "monitor", "open-pr", "scan-registry",
}

// injectSubAgents registers SDD sub-agents with the host agent.
//
// For OpenCode: all sub-agents are defined in the "agent" section of opencode.json,
// including the orchestrator (primary), disabled built-in agents, and SDD sub-agents.
// Skills are referenced from the agent-local skills directory.
//
// For Cursor and other agents: sub-agent .md stubs are written to the SubAgentsDir.
func injectSubAgents(homeDir string, adapter agents.Adapter) (InjectionResult, error) {
	files := make([]string, 0)
	changed := false

	if adapter.Agent() == model.AgentOpenCode {
		// OpenCode: merge full agent configs into opencode.json. No .md stubs.
		// Clean up .md stubs from prior installations if SubAgentsDir exists.
		if subAgentsDir := adapter.SubAgentsDir(homeDir); subAgentsDir != "" {
			if _, err := os.Stat(subAgentsDir); err == nil {
				_ = os.RemoveAll(subAgentsDir)
			}
		}

		settingsPath := adapter.SettingsPath(homeDir)
		if settingsPath == "" {
			return InjectionResult{}, nil
		}
		skillsDir := filepath.ToSlash(state.SharedSkillsDir(homeDir))
		promptsDir := filepath.ToSlash(filepath.Join(adapter.GlobalConfigDir(homeDir), "prompts"))
		overlay := buildAgentOverlay(openCodeSubAgents, skillsDir, promptsDir)
		baseJSON, err := os.ReadFile(settingsPath)
		if err != nil && !os.IsNotExist(err) {
			return InjectionResult{}, fmt.Errorf("read agent settings: %w", err)
		}
		merged, err := filemerge.MergeJSONObjects(baseJSON, overlay)
		if err != nil {
			return InjectionResult{}, fmt.Errorf("merge agent configs: %w", err)
		}
		wr, err := filemerge.WriteFileAtomic(settingsPath, merged, 0o644)
		if err != nil {
			return InjectionResult{}, fmt.Errorf("write agent configs: %w", err)
		}
		changed = changed || wr.Changed
		files = append(files, settingsPath)
	} else {
		// Cursor and others: write .md stubs to SubAgentsDir.
		skillsDir := filepath.ToSlash(state.SharedSkillsDir(homeDir))
		subAgentsDir := adapter.SubAgentsDir(homeDir)
		if subAgentsDir == "" {
			return InjectionResult{}, nil
		}
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
	}

	return InjectionResult{Changed: changed, Files: files}, nil
}

// buildAgentOverlay builds a full JSON overlay for all SDD agents in opencode.json,
// including:
//   - orchestrator (primary agent referencing prompts/orchestrator.md)
//   - disabled built-in agents (build, plan)
//   - SDD sub-agents with per-agent tools, steps, temperature, and prompts
func buildAgentOverlay(skillIDs []string, skillsDir, promptsDir string) []byte {
	agentMap := make(map[string]any, len(skillIDs)+3)

	// Orchestrator: primary agent, delegates all work.
	agentMap["orchestrator"] = map[string]any{
		"mode":        "primary",
		"color":       agentColors["orchestrator"],
		"description": "Pure coordinator that delegates ALL work to sub-agents. NEVER reads code or files directly — only receives sub-agent outputs.",
		"prompt":      "{file:" + promptsDir + "/orchestrator.md}",
		"steps":       agentSteps("orchestrator"),
		"temperature": agentTemperature("orchestrator"),
		"permission": map[string]any{
			"task": map[string]any{"*": "allow"},
		},
		"tools": map[string]any{
			"bash": false, "read": false, "glob": false, "grep": false, "list": false,
			"question": true, "engram_*": true, "sdd_*": true, "msg_*": true,
			"tb_*": true, "skill": true, "task": true,
			"todoread": true, "todowrite": true,
			"edit": false, "write": false, "patch": false, "lsp": false,
			"webfetch": false, "websearch": false, "playwright_*": false,
		},
	}

	// Disable built-in agents that conflict with the SDD workflow.
	agentMap["build"] = map[string]any{"disable": true}
	agentMap["plan"] = map[string]any{"disable": true}

	// SDD sub-agents.
	for _, id := range skillIDs {
		role := agentRoles[id]
		skillPath := skillsDir + "/" + id + "/SKILL.md"

		agent := map[string]any{
			"mode":        "subagent",
			"color":       agentColors[id],
			"description": sddSkillDescriptions[id],
			"steps":       agentSteps(id),
			"temperature": agentTemperature(id),
			"tools":       toolsForRole(role),
		}

		// team-lead has a specialized prompt and task permission.
		if id == "team-lead" {
			agent["prompt"] = fmt.Sprintf(
				"You are the **team-lead** agent. Read your skill file at %s and follow its instructions exactly. "+
					"You are a COORDINATOR — you NEVER write code. You own the task board and execute all groups, "+
					"launching @implement sub-agents via the task tool for each task.",
				skillPath)
			agent["permission"] = map[string]any{
				"task": map[string]any{"*": "allow"},
			}
		} else {
			agent["prompt"] = fmt.Sprintf(
				"You are the **%s** agent. Read your skill file at %s and follow its instructions exactly. "+
					"When ENABLED CLIs are specified in your task prompt, you MUST use them for validation, "+
					"cross-checking, or code generation. Run at least ONE CLI consultation per task unless CLIs are set to 'none'.",
				id, skillPath)
		}

		agentMap[id] = agent
	}

	data, _ := json.Marshal(map[string]any{"agent": agentMap})
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

	// Shared skills directory (~/.cortex-ia/skills/) — only sub-agent skills.
	// Convention file is owned by the conventions component.
	sharedSkillsDir := state.SharedSkillsDir(homeDir)
	for _, id := range openCodeSubAgents {
		paths = append(paths, filepath.Join(sharedSkillsDir, id, "SKILL.md"))
	}

	// Agent-local skills directory.
	if adapter.SupportsSkills() {
		agentSkillsDir := adapter.SkillsDir(homeDir)
		if agentSkillsDir != "" {
			if adapter.Agent() == model.AgentOpenCode {
				// OpenCode: only utility skills in local dir.
				for _, id := range openCodeLocalSkills {
					paths = append(paths, filepath.Join(agentSkillsDir, id, "SKILL.md"))
				}
			} else {
				// Other agents: all skills + convention copied to local.
				paths = append(paths, filepath.Join(agentSkillsDir, "_shared", "cortex-convention.md"))
				for _, id := range sddSkillIDs {
					paths = append(paths, filepath.Join(agentSkillsDir, id, "SKILL.md"))
				}
			}
		}
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
