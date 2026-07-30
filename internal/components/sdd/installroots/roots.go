// Package installroots resolves adapter workflow destinations without mixing
// home-relative workflow files with externally-owned configuration.
package installroots

import (
	"fmt"
	"path/filepath"
	"strings"

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
	case "gemini":
		root = ".gemini"
	case "cursor":
		root = ".cursor"
	case "vscode":
		root = ".copilot"
	case "codex":
		root = ".codex"
	case "windsurf":
		root = ".codeium/windsurf"
	case "antigravity":
		root = ".gemini/antigravity"
	case "kilocode":
		root = ".config/kilo"
	case "kimi":
		root = ".kimi"
	case "qwen":
		root = ".qwen"
	case "kiro":
		root = ".kiro"
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
	if adapter == "opencode" || adapter == "kilocode" || adapter == "qwen" {
		commands := workflowRoot(ir.SemanticID("root/"+adapter+"/commands"), filepath.ToSlash(filepath.Join(root, "commands")))
		result.Commands = &commands
	}
	if adapter == "kiro" {
		if externalConfig == "" || !filepath.IsAbs(externalConfig) {
			return ir.AdapterInstallRoots{}, fmt.Errorf("kiro external config root must be absolute")
		}
		externalAbsolute := filepath.Clean(externalConfig)
		workflowAbsolute, err := filepath.Abs(filepath.Join(homeDir, root))
		if err != nil {
			return ir.AdapterInstallRoots{}, fmt.Errorf("resolve kiro workflow root: %w", err)
		}
		if samePath(externalAbsolute, workflowAbsolute) {
			return ir.AdapterInstallRoots{}, fmt.Errorf("kiro external config root must remain separate from workflow root")
		}
		external := ir.AssetPathRoot{Scope: ir.ScopeExternalConfig, RootID: "root/kiro/external", Absolute: externalAbsolute}
		result.ExternalConfig = &external
	}
	return result, nil
}

func samePath(left, right string) bool {
	left = filepath.Clean(left)
	right = filepath.Clean(right)
	if filepath.VolumeName(left) != "" || filepath.VolumeName(right) != "" {
		return strings.EqualFold(left, right)
	}
	return left == right
}
