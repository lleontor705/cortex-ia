package sdd

import (
	"encoding/json"
	"fmt"
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

	// Write convention files.
	for _, conventionFile := range []string{"cortex-convention.md", "cortex-advanced.md"} {
		content, err := assets.Read("skills/_shared/" + conventionFile)
		if err != nil {
			continue
		}
		dst := filepath.Join(agentSkillsDir, "_shared", conventionFile)
		wr, err := filemerge.WriteFileAtomic(dst, []byte(content), 0o644)
		if err != nil {
			return InjectionResult{}, fmt.Errorf("write %s to agent: %w", conventionFile, err)
		}
		changed = changed || wr.Changed
		files = append(files, dst)
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
			agent["prompt"] = agentPrompt(id, skillPath)
		}

		agentMap[id] = agent
	}

	data, _ := json.Marshal(map[string]any{"agent": agentMap})
	return data
}

// agentPrompt builds the system prompt for a non-team-lead SDD sub-agent.
func agentPrompt(id, skillPath string) string {
	return fmt.Sprintf(
		"You are the **%s** agent. Read your skill file at %s and follow its instructions exactly.",
		id, skillPath)
}
