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
// 1. Orchestrator prompt into system prompt
// 2. SDD skill files into skills directory
// 3. Shared conventions into _shared/
// 4. Slash commands (OpenCode only)
func Inject(homeDir string, adapter agents.Adapter) (InjectionResult, error) {
	files := make([]string, 0)
	changed := false

	// 1. Inject SDD orchestrator into system prompt.
	if adapter.SupportsSystemPrompt() {
		result, err := injectOrchestratorPrompt(homeDir, adapter)
		if err != nil {
			return InjectionResult{}, fmt.Errorf("sdd orchestrator prompt: %w", err)
		}
		changed = changed || result.Changed
		files = append(files, result.Files...)
	}

	// 2. Write SDD skill files.
	if adapter.SupportsSkills() {
		result, err := injectSkillFiles(homeDir, adapter)
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
// agent's system prompt using the appropriate strategy.
func injectOrchestratorPrompt(homeDir string, adapter agents.Adapter) (InjectionResult, error) {
	// Choose the right orchestrator variant based on task delegation support.
	assetPath := "generic/sdd-orchestrator-single.md"
	if adapter.SupportsTaskDelegation() {
		assetPath = "generic/sdd-orchestrator.md"
	}

	content, err := assets.Read(assetPath)
	if err != nil {
		return InjectionResult{}, err
	}

	// Template in the absolute skills directory so sub-agents know where to
	// find SKILL.md files. Use forward slashes for cross-platform consistency.
	if skillsDir := adapter.SkillsDir(homeDir); skillsDir != "" {
		normalised := filepath.ToSlash(skillsDir)
		content = strings.ReplaceAll(content, "{{SKILLS_DIR}}", normalised)
	}

	promptFile := adapter.SystemPromptFile(homeDir)
	if promptFile == "" {
		return InjectionResult{}, nil
	}

	switch adapter.SystemPromptStrategy() {
	case model.StrategyMarkdownSections:
		existing, _ := os.ReadFile(promptFile)
		updated := filemerge.InjectMarkdownSection(string(existing), "sdd-orchestrator", content)
		wr, err := filemerge.WriteFileAtomic(promptFile, []byte(updated), 0o644)
		if err != nil {
			return InjectionResult{}, err
		}
		return InjectionResult{Changed: wr.Changed, Files: []string{promptFile}}, nil

	case model.StrategyFileReplace:
		existing, _ := os.ReadFile(promptFile)
		var updated string
		if len(existing) > 0 {
			updated = string(existing) + "\n\n" + content
		} else {
			updated = content
		}
		wr, err := filemerge.WriteFileAtomic(promptFile, []byte(updated), 0o644)
		if err != nil {
			return InjectionResult{}, err
		}
		return InjectionResult{Changed: wr.Changed, Files: []string{promptFile}}, nil

	case model.StrategyAppendToFile:
		existing, _ := os.ReadFile(promptFile)
		if strings.Contains(string(existing), "SDD Workflow") {
			return InjectionResult{Files: []string{promptFile}}, nil
		}
		updated := string(existing) + "\n\n" + content
		wr, err := filemerge.WriteFileAtomic(promptFile, []byte(updated), 0o644)
		if err != nil {
			return InjectionResult{}, err
		}
		return InjectionResult{Changed: wr.Changed, Files: []string{promptFile}}, nil

	default:
		return InjectionResult{}, fmt.Errorf("unsupported system prompt strategy for %s", adapter.Agent())
	}
}

// injectSkillFiles writes SDD skill files to the skills directory.
// Convention content is inlined into each SKILL.md so sub-agents never need to
// resolve external file paths (which fail because the agent's CWD is the
// project root, not the skills directory).
func injectSkillFiles(homeDir string, adapter agents.Adapter) (InjectionResult, error) {
	skillsDir := adapter.SkillsDir(homeDir)
	if skillsDir == "" {
		return InjectionResult{}, nil
	}

	// Read convention content once — it will be inlined into every skill.
	conventionContent, conventionErr := assets.Read("skills/_shared/cortex-convention.md")

	files := make([]string, 0)
	changed := false

	// Write SDD skill files with convention inlined.
	for _, skillID := range sddSkillIDs {
		assetPath := "skills/" + skillID + "/SKILL.md"
		content, err := assets.Read(assetPath)
		if err != nil {
			log.Printf("sdd: skipping skill %q — asset not found: %v", skillID, err)
			continue
		}

		// Inline convention content into each SKILL.md, replacing file references.
		if conventionErr == nil {
			content = inlineConvention(content, conventionContent)
		}

		path := filepath.Join(skillsDir, skillID, "SKILL.md")
		wr, err := filemerge.WriteFileAtomic(path, []byte(content), 0o644)
		if err != nil {
			return InjectionResult{}, fmt.Errorf("write skill %q: %w", skillID, err)
		}
		changed = changed || wr.Changed
		files = append(files, path)
	}

	return InjectionResult{Changed: changed, Files: files}, nil
}

// inlineConvention replaces references to the external convention file with its
// actual content so sub-agents never need to read a separate file.
//
// Two reference patterns exist in SKILL.md files:
//  1. Persistence section: "Follow the shared Cortex convention in `../_shared/cortex-convention.md` ..."
//     → replaced with a brief note + full convention content
//  2. Step instructions: "Follow the Skill Loading Protocol in `../_shared/cortex-convention.md`:"
//     → replaced with the Skill Loading Protocol excerpt inlined
func inlineConvention(skillContent, conventionContent string) string {
	lines := strings.Split(skillContent, "\n")
	protocol := extractSection(conventionContent, "## Skill Loading Protocol")

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Pattern 1: persistence reference — replace with full convention.
		if strings.HasPrefix(trimmed, "Follow the shared Cortex convention in `../_shared/cortex-convention.md`") {
			lines[i] = "## Cortex Convention (inlined)\n\n" + conventionContent
			continue
		}

		// Pattern 2: step reference with colon — replace with Skill Loading Protocol section.
		if strings.HasPrefix(trimmed, "Follow the Skill Loading Protocol in `../_shared/cortex-convention.md`") {
			if protocol != "" {
				lines[i] = protocol
			}
			continue
		}

		// Pattern 3: short reference — replace with Skill Loading Protocol section.
		if strings.Contains(trimmed, "../_shared/cortex-convention.md") {
			if protocol != "" {
				lines[i] = protocol
			} else {
				lines[i] = "Load skill registry from Cortex: mem_search(query: \"skill-registry\"). Fallback: read .sdd/skill-registry.md from project root."
			}
			continue
		}
	}

	return strings.Join(lines, "\n")
}

// extractSection extracts a markdown section (from heading to next same-level heading or EOF).
func extractSection(content, heading string) string {
	idx := strings.Index(content, heading)
	if idx < 0 {
		return ""
	}
	section := content[idx:]

	// Find the next heading at the same level (## for ##).
	prefix := heading[:strings.Index(heading, " ")+1] // e.g. "## "
	rest := section[len(heading):]
	end := strings.Index(rest, "\n"+prefix)
	if end >= 0 {
		return strings.TrimSpace(section[:len(heading)+end])
	}
	return strings.TrimSpace(section)
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

	// Generate sub-agent stubs with absolute skill paths so agents can find
	// their SKILL.md regardless of working directory.
	skillsDir := filepath.ToSlash(adapter.SkillsDir(homeDir))
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
			baseJSON, _ := os.ReadFile(settingsPath)
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

	if adapter.SupportsSystemPrompt() {
		if f := adapter.SystemPromptFile(homeDir); f != "" {
			paths = append(paths, f)
		}
	}

	if adapter.SupportsSkills() {
		skillsDir := adapter.SkillsDir(homeDir)
		for _, id := range sddSkillIDs {
			paths = append(paths, filepath.Join(skillsDir, id, "SKILL.md"))
		}
	}

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
