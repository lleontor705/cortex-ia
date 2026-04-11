package sdd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/agents"
	"github.com/lleontor705/cortex-ia/internal/assets"
	"github.com/lleontor705/cortex-ia/internal/components/filemerge"
	"github.com/lleontor705/cortex-ia/internal/model"
	"github.com/lleontor705/cortex-ia/internal/state"
)

// strictTDDDirective is appended to the orchestrator prompt when Strict TDD mode is enabled.
const strictTDDDirective = `

## Strict TDD Mode

All implementation tasks MUST follow strict Test-Driven Development:
1. Write failing tests FIRST — no production code before tests exist
2. Run tests to confirm they fail for the right reason
3. Write minimal production code to pass the tests
4. Refactor while keeping tests green

Sub-agents in the implement phase MUST produce test files before source files.
`

// buildPromptContent reads an orchestrator asset and applies template substitutions.
func buildPromptContent(assetPath, homeDir string, assignments model.ModelAssignments, strictTDD bool) (string, error) {
	content, err := assets.Read(assetPath)
	if err != nil {
		return "", err
	}
	content = strings.ReplaceAll(content, "{{SKILLS_DIR}}", filepath.ToSlash(state.SharedSkillsDir(homeDir)))
	if assignments == nil {
		assignments = model.ModelsForPreset(model.ModelPresetBalanced)
	}
	content = strings.ReplaceAll(content, "{{MODEL_ASSIGNMENTS}}", model.FormatModelAssignments(assignments))
	if strictTDD {
		content += strictTDDDirective
	}
	return content, nil
}

// injectAgentPrompt writes the orchestrator prompt to the agent-specific system
// prompt file (and OpenCode local prompts dir). Shared dir writes are handled
// separately via sync.Once in Inject() to prevent Windows file lock conflicts.
func injectAgentPrompt(homeDir string, adapter agents.Adapter, assignments model.ModelAssignments, strictTDD bool) (InjectionResult, error) {
	assetPath := "generic/sdd-orchestrator-single.md"
	if adapter.SupportsTaskDelegation() {
		assetPath = "generic/sdd-orchestrator.md"
	}

	content, err := buildPromptContent(assetPath, homeDir, assignments, strictTDD)
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
