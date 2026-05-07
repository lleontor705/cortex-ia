package agentbuilder

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/lleontor705/cortex-ia/internal/components/filemerge"
	"github.com/lleontor705/cortex-ia/internal/model"
)

// InstallToAdapters writes the generated SKILL.md into every adapter that
// supports skills. Failures roll back successful writes so partial installs
// never leave dangling files behind.
//
// Returns one InstallResult per attempted adapter (so callers can render a
// per-target report), plus the first encountered error (if any).
func InstallToAdapters(agent *GeneratedAgent, adapters []AdapterInfo) ([]InstallResult, error) {
	if agent == nil {
		return nil, fmt.Errorf("install: nil GeneratedAgent")
	}
	if agent.Name == "" {
		return nil, fmt.Errorf("install: GeneratedAgent.Name is empty")
	}
	if agent.Content == "" {
		return nil, fmt.Errorf("install: GeneratedAgent.Content is empty")
	}

	results := make([]InstallResult, 0, len(adapters))
	written := make([]string, 0, len(adapters)) // for rollback

	for _, info := range adapters {
		if !info.HasSkills || info.SkillsDir == "" {
			results = append(results, InstallResult{
				AgentID: info.ID,
				Success: false,
				Err:     fmt.Errorf("adapter %q does not support skills", info.ID),
			})
			continue
		}

		path := filepath.Join(info.SkillsDir, agent.Name, "SKILL.md")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			err = fmt.Errorf("install %s: mkdir: %w", info.ID, err)
			rollback(written)
			results = append(results, InstallResult{
				AgentID: info.ID,
				Path:    path,
				Success: false,
				Err:     err,
			})
			return results, err
		}

		if _, err := filemerge.WriteFileAtomic(path, []byte(agent.Content), 0o644); err != nil {
			err = fmt.Errorf("install %s: write: %w", info.ID, err)
			rollback(written)
			results = append(results, InstallResult{
				AgentID: info.ID,
				Path:    path,
				Success: false,
				Err:     err,
			})
			return results, err
		}

		written = append(written, path)
		results = append(results, InstallResult{
			AgentID: info.ID,
			Path:    path,
			Success: true,
		})
	}

	return results, nil
}

// rollback removes any files we just wrote when a later step fails.
// Errors during rollback are intentionally swallowed — the underlying error is
// what we want to surface to the user.
func rollback(paths []string) {
	for _, p := range paths {
		_ = os.Remove(p)
		// Best-effort: also remove the now-empty parent dir (e.g. ~/.claude/skills/<name>/).
		_ = os.Remove(filepath.Dir(p))
	}
}

// AdaptersFromRegistry builds the AdapterInfo list a multi-installer needs
// from a list of cortex-ia adapter ids. Adapters returning empty SkillsDir or
// not supporting skills are filtered out by the caller.
func AdaptersFromRegistry(homeDir string, lookup func(model.AgentID) (skillsDir, promptFile string, hasSkills bool, ok bool), ids []model.AgentID) []AdapterInfo {
	out := make([]AdapterInfo, 0, len(ids))
	for _, id := range ids {
		skillsDir, promptFile, hasSkills, ok := lookup(id)
		if !ok {
			continue
		}
		out = append(out, AdapterInfo{
			ID:         id,
			SkillsDir:  skillsDir,
			HasSkills:  hasSkills,
			PromptFile: promptFile,
		})
	}
	_ = homeDir // homeDir kept for symmetry — lookups are home-aware via the closure
	return out
}
