package opencode

import (
	"path"
	"strings"
)

// Layout describes OpenCode's native discovery surface separately from the
// Cortex workflow files that OpenCode does not load directly.
type Layout struct {
	ConfigRoot      string
	WorkflowRoot    string
	SkillsRoot      string
	AgentsRoot      string
	CommandsRoot    string
	RootModuleRoot  string
	ContractRoot    string
	RoleRoot        string
	OverlayRoot     string
	QualityRoot     string
	ManifestRoot    string
	ModelRoot       string
	PermissionRoot  string
	CompositionPath string
}

// NativeLayout returns the home-relative OpenCode file layout.
func NativeLayout() Layout {
	const (
		config   = ".config/opencode"
		workflow = ".cortex-ia/opencode"
	)
	return Layout{
		ConfigRoot: config, WorkflowRoot: workflow,
		SkillsRoot: path.Join(config, "skills"), AgentsRoot: path.Join(config, "agents"), CommandsRoot: path.Join(config, "commands"),
		RootModuleRoot: path.Join(workflow, "root"), ContractRoot: path.Join(workflow, "contracts"), RoleRoot: path.Join(workflow, "roles"),
		OverlayRoot: path.Join(workflow, "overlays"), QualityRoot: path.Join(workflow, "quality"), ManifestRoot: path.Join(workflow, "manifests"),
		ModelRoot: path.Join(workflow, "models"), PermissionRoot: path.Join(workflow, "permissions"), CompositionPath: path.Join(workflow, "composition.json"),
	}
}

func (l Layout) IsWorkflowPath(relative string) bool {
	clean := path.Clean(strings.ReplaceAll(relative, "\\", "/"))
	return clean != "." && clean != ".." && !strings.HasPrefix(clean, "../") &&
		(clean == l.WorkflowRoot || strings.HasPrefix(clean, l.WorkflowRoot+"/"))
}

// IsNativePath reports whether a home-relative path belongs to OpenCode's
// documented discovery surface. Cortex workflow support files are excluded.
func (l Layout) IsNativePath(relative string) bool {
	clean := path.Clean(strings.ReplaceAll(relative, "\\", "/"))
	if clean == path.Join(l.ConfigRoot, "opencode.json") || clean == path.Join(l.ConfigRoot, "opencode.jsonc") || clean == path.Join(l.ConfigRoot, "AGENTS.md") {
		return true
	}
	if nativeMarkdownChild(clean, l.AgentsRoot) || nativeMarkdownChild(clean, l.CommandsRoot) {
		return true
	}
	value := strings.TrimPrefix(clean, l.SkillsRoot+"/")
	parts := strings.Split(value, "/")
	return value != clean && len(parts) == 2 && parts[0] != "" && parts[1] == "SKILL.md"
}

func nativeMarkdownChild(relative, root string) bool {
	value := strings.TrimPrefix(relative, root+"/")
	return value != relative && value != "" && !strings.Contains(value, "/") && strings.HasSuffix(value, ".md")
}
