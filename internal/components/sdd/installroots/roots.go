// Package installroots resolves adapter workflow destinations without mixing
// home-relative workflow files with externally-owned configuration.
package installroots

import (
	"fmt"
	"path/filepath"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
)

func workflowRoot(id ir.SemanticID, relative string) ir.AssetPathRoot {
	return ir.AssetPathRoot{Scope: ir.ScopeWorkflowRoot, RootID: id, Relative: relative}
}

// Resolve returns typed roots for all supported adapters. externalConfig is
// used only for adapters whose settings/MCP data is outside the workflow root.
func Resolve(adapter, homeDir, externalConfig string) (ir.AdapterInstallRoots, error) {
	if homeDir == "" {
		return ir.AdapterInstallRoots{}, fmt.Errorf("home directory is required")
	}
	var root string
	switch adapter {
	case "claude":
		root = ".claude"
	case "opencode":
		root = ".config/opencode"
	case "vscode":
		root = ".copilot"
	case "codex":
		root = ".codex"
	default:
		return ir.AdapterInstallRoots{}, fmt.Errorf("unsupported adapter %q", adapter)
	}
	base := workflowRoot(ir.SemanticID("root/"+adapter), filepath.ToSlash(root))
	result := ir.AdapterInstallRoots{
		Workflow: base,
		Prompt:   base, Skills: base, Agents: base,
		Commands: nil,
	}
	result.Prompt.Relative = filepath.ToSlash(filepath.Join(root, "steering"))
	result.Skills.Relative = filepath.ToSlash(filepath.Join(root, "skills"))
	result.Agents.Relative = filepath.ToSlash(filepath.Join(root, "agents"))
	if adapter == "opencode" {
		commands := workflowRoot(ir.SemanticID("root/"+adapter+"/commands"), filepath.ToSlash(filepath.Join(root, "commands")))
		result.Commands = &commands
	}
	return result, nil
}
