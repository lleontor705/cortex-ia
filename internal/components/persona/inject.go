package persona

import (
	"fmt"
	"os"

	"github.com/lleontor705/cortex-ia/internal/agents"
	"github.com/lleontor705/cortex-ia/internal/assets"
	"github.com/lleontor705/cortex-ia/internal/components/filemerge"
	"github.com/lleontor705/cortex-ia/internal/model"
)

// InjectionResult describes the outcome of persona injection.
type InjectionResult struct {
	Changed bool
	Files   []string
}

// Inject injects the persona instructions into the agent's system prompt.
func Inject(homeDir string, adapter agents.Adapter, persona model.PersonaID) (InjectionResult, error) {
	if !adapter.SupportsSystemPrompt() {
		return InjectionResult{}, nil
	}

	promptFile := adapter.SystemPromptFile(homeDir)
	if promptFile == "" {
		return InjectionResult{}, nil
	}

	if persona == "" {
		persona = model.PersonaProfessional
	}

	assetPath := fmt.Sprintf("generic/persona-%s.md", persona)
	content, err := assets.Read(assetPath)
	if err != nil {
		return InjectionResult{}, fmt.Errorf("read persona asset %q: %w", persona, err)
	}

	existing, err := os.ReadFile(promptFile)
	if err != nil && !os.IsNotExist(err) {
		return InjectionResult{}, fmt.Errorf("read system prompt: %w", err)
	}

	updated := filemerge.InjectMarkdownSection(string(existing), "cortex-persona", content)
	wr, err := filemerge.WriteFileAtomic(promptFile, []byte(updated), 0o644)
	if err != nil {
		return InjectionResult{}, err
	}

	return InjectionResult{Changed: wr.Changed, Files: []string{promptFile}}, nil
}
