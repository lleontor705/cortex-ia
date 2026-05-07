package state

import (
	"fmt"
	"os"
	"path/filepath"
)

// AgentBuilderRegistryPath returns the path of the persisted agentbuilder
// registry: ~/.cortex-ia/agentbuilder/registry.json.
//
// The function ensures the parent directory exists so callers can write to it
// without a separate MkdirAll step.
func AgentBuilderRegistryPath(homeDir string) (string, error) {
	dir := filepath.Join(homeDir, stateDir, "agentbuilder")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create agentbuilder dir: %w", err)
	}
	return filepath.Join(dir, "registry.json"), nil
}
