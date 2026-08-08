// Package sdd owns the Spec-Driven Development workflow profiles, asset
// inventory, and compiled bundle compilation used by the typed installation
// pipeline. The legacy monolithic Inject function has been retired; the
// production path runs through sdd.CompileInjectionBundle and the
// sdd/install package.
package sdd

import (
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/agents"
	"github.com/lleontor705/cortex-ia/internal/assets"
	"github.com/lleontor705/cortex-ia/internal/model"
	"github.com/lleontor705/cortex-ia/internal/state"
)

// InjectionResult describes the outcome of SDD injection.
type InjectionResult struct {
	Changed      bool
	Files        []string
	Profile      WorkflowProfile
	Degradations []string
	Fingerprint  string
}

// sddSkillIDs are the core SDD skills written by this component.
var sddSkillIDs = []string{
	"bootstrap", "investigate", "draft-proposal", "write-specs",
	"architect", "decompose", "implement", "validate", "finalize",
	"debate", "debug", "execute-plan", "ideate", "monitor",
	"open-pr", "file-issue", "parallel-dispatch", "scan-registry",
	"judgment-day", "onboard", "chained-pr", "cognitive-doc-design",
	"comment-writer", "go-testing", "skill-creator", "skill-improver",
	"work-unit-commits",
}

// openCodeSubAgents lists the SDD skills that become sub-agents in opencode.json.
// These are written to the global shared directory (~/.cortex-ia/skills/).
var openCodeSubAgents = []string{
	"bootstrap", "investigate", "draft-proposal", "write-specs",
	"architect", "decompose", "implement", "validate", "finalize",
	"debate", "parallel-dispatch",
}

// openCodeLocalSkills lists utility skills that are written to the agent-local
// skills directory (~/.config/opencode/skills/) instead of the shared directory.
var openCodeLocalSkills = []string{
	"debug", "execute-plan", "file-issue",
	"ideate", "monitor", "open-pr", "scan-registry",
	"judgment-day", "onboard", "chained-pr", "cognitive-doc-design",
	"comment-writer", "go-testing", "skill-creator", "skill-improver",
	"work-unit-commits",
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
	sharedSkillsDir := state.SharedSkillsDir(homeDir)
	paths = append(paths, filepath.Join(sharedSkillsDir, "team-lead", "SKILL.md"))
	for _, id := range openCodeSubAgents {
		paths = append(paths, filepath.Join(sharedSkillsDir, id, "SKILL.md"))
	}

	// Agent-local skills directory.
	if adapter.SupportsSkills() {
		agentSkillsDir := adapter.SkillsDir(homeDir)
		if agentSkillsDir != "" {
			paths = append(paths, filepath.Join(agentSkillsDir, "team-lead", "SKILL.md"))
			if adapter.Agent() == model.AgentOpenCode {
				for _, id := range openCodeLocalSkills {
					paths = append(paths, filepath.Join(agentSkillsDir, id, "SKILL.md"))
				}
			} else {
				paths = append(paths, filepath.Join(agentSkillsDir, "_shared", "cortex-convention.md"))
				for _, id := range sddSkillIDs {
					paths = append(paths, filepath.Join(agentSkillsDir, id, "SKILL.md"))
				}
			}
		}
	}
	if subAgentsDir := adapter.SubAgentsDir(homeDir); subAgentsDir != "" {
		paths = append(paths, filepath.Join(subAgentsDir, "team-lead.md"))
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
