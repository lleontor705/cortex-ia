package sdd

import (
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
		content := fmt.Sprintf("# SDD Agent: %s\n\nRead and follow the skill instructions from: %s\n", skillID, skillPath)
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
			overlay := buildAgentHiddenOverlay(sddSkillIDs)
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

// buildAgentHiddenOverlay builds a JSON overlay that sets hidden:true and
// mode:subagent for all SDD skill agents.
func buildAgentHiddenOverlay(skillIDs []string) []byte {
	var b strings.Builder
	b.WriteString(`{"agent":{`)
	for i, id := range skillIDs {
		if i > 0 {
			b.WriteString(",")
		}
		fmt.Fprintf(&b, `%q:{"mode":"subagent","hidden":true}`, id)
	}
	b.WriteString(`}}`)
	return []byte(b.String())
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
