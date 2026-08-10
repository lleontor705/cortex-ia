// Package mcpinject provides shared logic for injecting MCP server configs
// into any supported agent. Each current MCP component (cortex, forgespec,
// context7) defines its own templates and delegates to this
// package for the actual strategy dispatch.
package mcpinject

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/agents"
	"github.com/lleontor705/cortex-ia/internal/components/filemerge"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/services"
	"github.com/lleontor705/cortex-ia/internal/model"
)

// InjectionResult describes the outcome of an MCP injection.
type InjectionResult struct {
	Changed       bool
	Files         []string
	Compatibility CompatibilityResult
}

// ServerTemplates holds the JSON/TOML templates for a single MCP server
// across all strategy variants.
type ServerTemplates struct {
	// Name is the MCP server name (e.g. "cortex", "forgespec").
	Name string

	// Service is the externally owned compatibility and authority manifest for
	// this MCP server. cortex-ia configures the service; it does not implement it.
	Service services.ServiceContract

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

	// TOMLCommand is the command for TOML-based agents (Codex).
	TOMLCommand string
	// TOMLArgs are the arguments for TOML-based agents.
	TOMLArgs []string
}

const tomlOwnershipMarkerPrefix = "# cortex-ia:toml-ownership "

// tomlRegionOwnership is durable evidence for one cortex-ia-managed TOML
// table. It is embedded in that table so config and evidence share the same
// atomic replacement boundary.
type tomlRegionOwnership struct {
	Owner        string   `json:"owner"`
	SemanticID   string   `json:"semantic_id"`
	TablePath    []string `json:"table_path"`
	Command      []string `json:"command"`
	BaseSHA256   string   `json:"base_sha256"`
	OwnershipSHA string   `json:"ownership_sha256"`
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
	updated, err := upsertOwnedTOMLServer(string(existing), tmpl)
	if err != nil {
		return InjectionResult{}, err
	}

	wr, err := filemerge.WriteFileAtomic(settingsPath, []byte(updated), 0o644)
	if err != nil {
		return InjectionResult{}, err
	}
	return InjectionResult{Changed: wr.Changed, Files: []string{settingsPath}}, nil
}

func upsertOwnedTOMLServer(content string, tmpl ServerTemplates) (string, error) {
	tablePath := []string{"mcp_servers", tmpl.Name}
	section := strings.Join(tablePath, ".")
	block := tomlMCPServerBlock(tmpl.TOMLCommand, tmpl.TOMLArgs)
	base := []byte("[" + section + "]\n" + block)
	command := append([]string{tmpl.TOMLCommand}, tmpl.TOMLArgs...)
	ownership := tomlRegionOwnership{
		Owner:      "cortex-ia",
		SemanticID: "mcp/opencode/" + tmpl.Name,
		TablePath:  tablePath,
		Command:    command,
		BaseSHA256: fmt.Sprintf("%x", sha256.Sum256(base)),
	}
	ownership.OwnershipSHA = tomlOwnershipDigest(ownership)
	encoded, err := json.Marshal(ownership)
	if err != nil {
		return "", fmt.Errorf("marshal TOML ownership evidence: %w", err)
	}
	return filemerge.UpsertTOMLBlock(content, section, tomlOwnershipMarkerPrefix+string(encoded)+"\n"+block), nil
}

func tomlMCPServerBlock(command string, args []string) string {
	var block strings.Builder
	fmt.Fprintf(&block, "command = %q\n", command)
	if len(args) > 0 {
		block.WriteString("args = [")
		for i, arg := range args {
			if i > 0 {
				block.WriteString(", ")
			}
			fmt.Fprintf(&block, "%q", arg)
		}
		block.WriteString("]\n")
	}
	return block.String()
}

func tomlOwnershipDigest(ownership tomlRegionOwnership) string {
	values := append([]string{ownership.Owner, ownership.SemanticID}, ownership.TablePath...)
	values = append(values, ownership.Command...)
	values = append(values, ownership.BaseSHA256)
	return fmt.Sprintf("%x", sha256.Sum256([]byte(strings.Join(values, "\x00"))))
}

func mergeJSONFile(path string, overlay []byte) (filemerge.WriteResult, error) {
	result, err := filemerge.MutateJSONFile(path, filemerge.JSONMutation{Overlay: overlay})
	if err != nil {
		return filemerge.WriteResult{}, err
	}
	return result.WriteResult, nil
}
