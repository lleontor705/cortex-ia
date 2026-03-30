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

// injectSkillFiles writes SDD skill files and shared conventions to the skills directory.
func injectSkillFiles(homeDir string, adapter agents.Adapter) (InjectionResult, error) {
	skillsDir := adapter.SkillsDir(homeDir)
	if skillsDir == "" {
		return InjectionResult{}, nil
	}

	files := make([]string, 0)
	changed := false

	// Write SDD skill files.
	for _, skillID := range sddSkillIDs {
		assetPath := "skills/" + skillID + "/SKILL.md"
		content, err := assets.Read(assetPath)
		if err != nil {
			log.Printf("sdd: skipping skill %q — asset not found: %v", skillID, err)
			continue
		}

		path := filepath.Join(skillsDir, skillID, "SKILL.md")
		wr, err := filemerge.WriteFileAtomic(path, []byte(content), 0o644)
		if err != nil {
			return InjectionResult{}, fmt.Errorf("write skill %q: %w", skillID, err)
		}
		changed = changed || wr.Changed
		files = append(files, path)
	}

	// Write shared convention file.
	conventionContent, err := assets.Read("skills/_shared/cortex-convention.md")
	if err == nil {
		path := filepath.Join(skillsDir, "_shared", "cortex-convention.md")
		wr, err := filemerge.WriteFileAtomic(path, []byte(conventionContent), 0o644)
		if err != nil {
			return InjectionResult{}, fmt.Errorf("write convention: %w", err)
		}
		changed = changed || wr.Changed
		files = append(files, path)
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

// injectSubAgents writes sub-agent definition files for agents that support them.
func injectSubAgents(homeDir string, adapter agents.Adapter) (InjectionResult, error) {
	subAgentsDir := adapter.SubAgentsDir(homeDir)
	if subAgentsDir == "" {
		return InjectionResult{}, nil
	}

	// For now, generate basic sub-agent stubs from skill names.
	// Full implementation will come with agent-specific overlays.
	files := make([]string, 0)
	changed := false

	for _, skillID := range sddSkillIDs {
		content := fmt.Sprintf("# SDD Agent: %s\n\nLoad skill from skills/%s/SKILL.md and follow its instructions.\n", skillID, skillID)
		path := filepath.Join(subAgentsDir, skillID+".md")
		wr, err := filemerge.WriteFileAtomic(path, []byte(content), 0o644)
		if err != nil {
			return InjectionResult{}, fmt.Errorf("write sub-agent %q: %w", skillID, err)
		}
		changed = changed || wr.Changed
		files = append(files, path)
	}

	return InjectionResult{Changed: changed, Files: files}, nil
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
		paths = append(paths, filepath.Join(skillsDir, "_shared", "cortex-convention.md"))
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
