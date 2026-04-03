package conventions

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/lleontor705/cortex-ia/internal/agents"
	"github.com/lleontor705/cortex-ia/internal/assets"
	"github.com/lleontor705/cortex-ia/internal/components/filemerge"
	"github.com/lleontor705/cortex-ia/internal/state"
)

// sharedOnce ensures cortex-convention.md and cortex-advanced.md are written
// only once across all parallel agent goroutines, preventing Windows file-lock
// race conditions.
var (
	sharedOnce    sync.Once
	sharedOnceErr error
	sharedResult  struct {
		changed bool
		paths   []string
	}
)

// ResetSharedWrite resets the sync.Once for testing. Must be called between test runs.
func ResetSharedWrite() {
	sharedOnce = sync.Once{}
	sharedOnceErr = nil
	sharedResult = struct {
		changed bool
		paths   []string
	}{}
}

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
	// Written once across all agents to avoid Windows rename race conditions.
	sharedOnce.Do(func() {
		sharedDir := filepath.Join(state.SharedSkillsDir(homeDir), "_shared")

		// Write cortex-convention.md (core protocols).
		content, err := assets.Read("skills/_shared/cortex-convention.md")
		if err != nil {
			sharedOnceErr = fmt.Errorf("read cortex-convention asset: %w", err)
			return
		}
		conventionPath := filepath.Join(sharedDir, "cortex-convention.md")
		wr, err := filemerge.WriteFileAtomic(conventionPath, []byte(content), 0o644)
		if err != nil {
			sharedOnceErr = fmt.Errorf("write cortex-convention: %w", err)
			return
		}
		sharedResult.changed = wr.Changed
		sharedResult.paths = append(sharedResult.paths, conventionPath)

		// Write cortex-advanced.md (low-frequency tools reference).
		advContent, err := assets.Read("skills/_shared/cortex-advanced.md")
		if err != nil {
			sharedOnceErr = fmt.Errorf("read cortex-advanced asset: %w", err)
			return
		}
		advPath := filepath.Join(sharedDir, "cortex-advanced.md")
		advWr, err := filemerge.WriteFileAtomic(advPath, []byte(advContent), 0o644)
		if err != nil {
			sharedOnceErr = fmt.Errorf("write cortex-advanced: %w", err)
			return
		}
		sharedResult.changed = sharedResult.changed || advWr.Changed
		sharedResult.paths = append(sharedResult.paths, advPath)
	})
	if sharedOnceErr != nil {
		return InjectionResult{}, sharedOnceErr
	}
	changed = changed || sharedResult.changed
	files = append(files, sharedResult.paths...)

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
