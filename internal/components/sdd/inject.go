// Package sdd injects the Spec-Driven Development pipeline skills, roles, and orchestrator prompt into agents.
package sdd

import (
	"fmt"
	"io/fs"
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

// Inject injects the full SDD workflow into the given agent:
// 1. Orchestrator prompt into system prompt (+ copy to shared dir)
// 2. SDD skill files into shared ~/.cortex-ia/skills/ directory
// 3. Slash commands (OpenCode only)
// 4. Sub-agent stubs (OpenCode only)
func Inject(homeDir string, adapter agents.Adapter, assignments model.ModelAssignments, strictTDD bool) (InjectionResult, error) {
	files := make([]string, 0)
	changed := false

	// 1-2. Write shared files (orchestrator prompt + skills) only once across all agents.
	// This prevents file lock conflicts on Windows when agents run in parallel.
	sharedWriteOnce.Do(func() {
		// Write shared orchestrator prompt.
		sharedPromptPath := filepath.Join(state.SharedPromptsDir(homeDir), "orchestrator.md")
		content, err := buildPromptContent("generic/sdd-orchestrator.md", homeDir, assignments, strictTDD)
		if err != nil {
			sharedWriteErr = fmt.Errorf("sdd orchestrator prompt: %w", err)
			return
		}
		if _, err := filemerge.WriteFileAtomic(sharedPromptPath, []byte(content), 0o644); err != nil {
			sharedWriteErr = fmt.Errorf("write shared orchestrator prompt: %w", err)
			return
		}
		sharedWriteFiles = append(sharedWriteFiles, sharedPromptPath)

		// Write orchestrator reference file (detailed protocols loaded on demand).
		refPath := filepath.Join(state.SharedPromptsDir(homeDir), "sdd-orchestrator-reference.md")
		refContent, err := assets.Read("generic/sdd-orchestrator-reference.md")
		if err != nil {
			sharedWriteErr = fmt.Errorf("sdd orchestrator reference: %w", err)
			return
		}
		if _, err := filemerge.WriteFileAtomic(refPath, []byte(refContent), 0o644); err != nil {
			sharedWriteErr = fmt.Errorf("write orchestrator reference: %w", err)
			return
		}
		sharedWriteFiles = append(sharedWriteFiles, refPath)

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
		result, err := injectAgentPrompt(homeDir, adapter, assignments, strictTDD)
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

	// Shared orchestrator prompt and reference file.
	paths = append(paths, filepath.Join(state.SharedPromptsDir(homeDir), "orchestrator.md"))
	paths = append(paths, filepath.Join(state.SharedPromptsDir(homeDir), "sdd-orchestrator-reference.md"))

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
