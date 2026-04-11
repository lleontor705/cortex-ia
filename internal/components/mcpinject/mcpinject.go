// Package mcpinject provides shared logic for injecting MCP server configs
// into any supported agent. Each MCP component (cortex, agent-mailbox,
// forgespec, context7) defines its own templates and delegates to this
// package for the actual strategy dispatch.
package mcpinject

import (
	"fmt"
	"os"

	"github.com/lleontor705/cortex-ia/internal/agents"
	"github.com/lleontor705/cortex-ia/internal/components/filemerge"
	"github.com/lleontor705/cortex-ia/internal/model"
)

// InjectionResult describes the outcome of an MCP injection.
type InjectionResult struct {
	Changed bool
	Files   []string
}

// ServerTemplates holds the JSON/TOML templates for a single MCP server
// across all strategy variants.
type ServerTemplates struct {
	// Name is the MCP server name (e.g. "cortex", "forgespec").
	Name string

	// SeparateFileJSON is the standalone JSON for SeparateMCPFiles strategy (Claude Code).
	SeparateFileJSON []byte

	// DefaultOverlayJSON is the mcpServers overlay for MergeIntoSettings/MCPConfigFile.
	DefaultOverlayJSON []byte

	// OpenCodeOverlayJSON is the OpenCode-specific overlay (uses "mcp" key).
	// If nil, DefaultOverlayJSON is used.
	OpenCodeOverlayJSON []byte

	// VSCodeOverlayJSON is the VS Code-specific overlay (uses "servers" key).
	// If nil, DefaultOverlayJSON is used.
	VSCodeOverlayJSON []byte

	// AntigravityOverlayJSON is the Antigravity-specific overlay.
	// If nil, DefaultOverlayJSON is used.
	AntigravityOverlayJSON []byte

	// TOMLCommand is the command for TOML-based agents (Codex).
	TOMLCommand string
	// TOMLArgs are the arguments for TOML-based agents.
	TOMLArgs []string
}

// Inject injects the MCP server config into the agent using the appropriate strategy.
func Inject(homeDir string, adapter agents.Adapter, tmpl ServerTemplates) (InjectionResult, error) {
	if !adapter.SupportsMCP() {
		return InjectionResult{}, nil
	}

	switch adapter.MCPStrategy() {
	case model.StrategySeparateMCPFiles:
		return injectSeparateFile(homeDir, adapter, tmpl)
	case model.StrategyMergeIntoSettings:
		return injectMergeIntoSettings(homeDir, adapter, tmpl)
	case model.StrategyMCPConfigFile:
		return injectMCPConfigFile(homeDir, adapter, tmpl)
	case model.StrategyTOMLFile:
		return injectTOML(homeDir, adapter, tmpl)
	default:
		return InjectionResult{}, fmt.Errorf("unsupported MCP strategy %d for agent %q", adapter.MCPStrategy(), adapter.Agent())
	}
}

func injectSeparateFile(homeDir string, adapter agents.Adapter, tmpl ServerTemplates) (InjectionResult, error) {
	path := adapter.MCPConfigPath(homeDir, tmpl.Name)
	wr, err := filemerge.WriteFileAtomic(path, tmpl.SeparateFileJSON, 0o644)
	if err != nil {
		return InjectionResult{}, err
	}
	return InjectionResult{Changed: wr.Changed, Files: []string{path}}, nil
}

func injectMergeIntoSettings(homeDir string, adapter agents.Adapter, tmpl ServerTemplates) (InjectionResult, error) {
	settingsPath := adapter.SettingsPath(homeDir)
	if settingsPath == "" {
		return InjectionResult{}, nil
	}

	overlay := tmpl.DefaultOverlayJSON
	if adapter.Agent() == model.AgentOpenCode && tmpl.OpenCodeOverlayJSON != nil {
		overlay = tmpl.OpenCodeOverlayJSON
	}

	wr, err := mergeJSONFile(settingsPath, overlay)
	if err != nil {
		return InjectionResult{}, err
	}
	return InjectionResult{Changed: wr.Changed, Files: []string{settingsPath}}, nil
}

func injectMCPConfigFile(homeDir string, adapter agents.Adapter, tmpl ServerTemplates) (InjectionResult, error) {
	path := adapter.MCPConfigPath(homeDir, tmpl.Name)
	if path == "" {
		return InjectionResult{}, nil
	}

	overlay := tmpl.DefaultOverlayJSON
	if adapter.Agent() == model.AgentVSCodeCopilot && tmpl.VSCodeOverlayJSON != nil {
		overlay = tmpl.VSCodeOverlayJSON
	}
	if adapter.Agent() == model.AgentAntigravity && tmpl.AntigravityOverlayJSON != nil {
		overlay = tmpl.AntigravityOverlayJSON
	}

	wr, err := mergeJSONFile(path, overlay)
	if err != nil {
		return InjectionResult{}, err
	}
	return InjectionResult{Changed: wr.Changed, Files: []string{path}}, nil
}

func injectTOML(homeDir string, adapter agents.Adapter, tmpl ServerTemplates) (InjectionResult, error) {
	if tmpl.TOMLCommand == "" {
		return InjectionResult{}, nil
	}

	settingsPath := adapter.SettingsPath(homeDir)
	if settingsPath == "" {
		return InjectionResult{}, nil
	}

	existing, _ := os.ReadFile(settingsPath)
	updated := filemerge.UpsertMCPServerTOML(string(existing), tmpl.Name, tmpl.TOMLCommand, tmpl.TOMLArgs)

	wr, err := filemerge.WriteFileAtomic(settingsPath, []byte(updated), 0o644)
	if err != nil {
		return InjectionResult{}, err
	}
	return InjectionResult{Changed: wr.Changed, Files: []string{settingsPath}}, nil
}

func mergeJSONFile(path string, overlay []byte) (filemerge.WriteResult, error) {
	baseJSON, err := readFileOrEmpty(path)
	if err != nil {
		return filemerge.WriteResult{}, err
	}

	merged, err := filemerge.MergeJSONObjects(baseJSON, overlay)
	if err != nil {
		return filemerge.WriteResult{}, err
	}

	return filemerge.WriteFileAtomic(path, merged, 0o644)
}

func readFileOrEmpty(path string) ([]byte, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read file %q: %w", path, err)
	}
	return content, nil
}
