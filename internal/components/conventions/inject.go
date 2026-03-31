package conventions

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/lleontor705/cortex-ia/internal/agents"
	"github.com/lleontor705/cortex-ia/internal/assets"
	"github.com/lleontor705/cortex-ia/internal/components/filemerge"
	"github.com/lleontor705/cortex-ia/internal/state"
)

// InjectionResult describes the outcome of conventions injection.
type InjectionResult struct {
	Changed bool
	Files   []string
}

// Inject injects the cortex convention and protocol files into the agent:
// 1. cortex-convention.md into skills/_shared/ (if skills supported)
// 2. cortex-protocol.md into system prompt (if system prompt supported)
func Inject(homeDir string, adapter agents.Adapter) (InjectionResult, error) {
	files := make([]string, 0)
	changed := false

	// 1. Write cortex-convention.md to shared skills dir (~/.cortex-ia/skills/_shared/).
	{
		content, err := assets.Read("skills/_shared/cortex-convention.md")
		if err != nil {
			return InjectionResult{}, fmt.Errorf("read cortex-convention asset: %w", err)
		}
		path := filepath.Join(state.SharedSkillsDir(homeDir), "_shared", "cortex-convention.md")
		wr, err := filemerge.WriteFileAtomic(path, []byte(content), 0o644)
		if err != nil {
			return InjectionResult{}, fmt.Errorf("write cortex-convention: %w", err)
		}
		changed = changed || wr.Changed
		files = append(files, path)
	}

	// 2. Inject cortex-protocol.md into system prompt.
	if adapter.SupportsSystemPrompt() {
		protocol, err := assets.Read("generic/cortex-protocol.md")
		if err != nil {
			return InjectionResult{}, fmt.Errorf("read cortex-protocol: %w", err)
		}

		promptFile := adapter.SystemPromptFile(homeDir)
		if promptFile == "" {
			return InjectionResult{Changed: changed, Files: files}, nil
		}

		// All strategies use marker-based injection to ensure idempotency.
		existing, err := os.ReadFile(promptFile)
		if err != nil && !os.IsNotExist(err) {
			return InjectionResult{}, fmt.Errorf("read system prompt: %w", err)
		}
		updated := filemerge.InjectMarkdownSection(string(existing), "cortex-protocol", protocol)
		wr, err := filemerge.WriteFileAtomic(promptFile, []byte(updated), 0o644)
		if err != nil {
			return InjectionResult{}, err
		}
		changed = changed || wr.Changed
		files = append(files, promptFile)
	}

	return InjectionResult{Changed: changed, Files: files}, nil
}
