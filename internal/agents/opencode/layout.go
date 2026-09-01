package opencode

import (
	"os"
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
	PluginRoot      string
	TUIPluginRoot   string
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
		SkillsRoot: ".agents/skills", AgentsRoot: path.Join(config, "agents"), CommandsRoot: path.Join(config, "commands"),
		PluginRoot:     path.Join(config, "plugins"),
		TUIPluginRoot:  path.Join(config, "tui-plugins"),
		RootModuleRoot: path.Join(workflow, "root"), ContractRoot: path.Join(workflow, "contracts"), RoleRoot: path.Join(workflow, "roles"),
		OverlayRoot: path.Join(workflow, "overlays"), QualityRoot: path.Join(workflow, "quality"), ManifestRoot: path.Join(workflow, "manifests"),
		ModelRoot: path.Join(workflow, "models"), PermissionRoot: path.Join(workflow, "permissions"), CompositionPath: path.Join(workflow, "composition.json"),
	}
}

// ResolveConfigRelPath resolves the active OpenCode configuration file path relative
// to the home directory. If opencode.jsonc exists, it takes precedence; otherwise,
// if opencode.json exists, it is used; if neither exists, opencode.jsonc is returned.
func (l Layout) ResolveConfigRelPath(homeDir string) string {
	return path.Join(l.ConfigRoot, l.ResolveConfigFilename(homeDir))
}

// ResolveConfigFilename returns "opencode.json" if it exists and "opencode.jsonc" does not;
// otherwise it returns "opencode.jsonc".
func (l Layout) ResolveConfigFilename(homeDir string) string {
	if homeDir == "" {
		return "opencode.jsonc"
	}
	// Check if opencode.jsonc exists
	jsoncPath := path.Join(homeDir, filepathToSlash(path.Join(l.ConfigRoot, "opencode.jsonc")))
	if fileExists(jsoncPath) {
		return "opencode.jsonc"
	}
	// Check if opencode.json exists
	jsonPath := path.Join(homeDir, filepathToSlash(path.Join(l.ConfigRoot, "opencode.json")))
	if fileExists(jsonPath) {
		return "opencode.json"
	}
	return "opencode.jsonc"
}

func filepathToSlash(p string) string {
	return strings.ReplaceAll(p, "\\", "/")
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func (l Layout) IsWorkflowPath(relative string) bool {
	clean := path.Clean(strings.ReplaceAll(relative, "\\", "/"))
	return clean != "." && clean != ".." && !strings.HasPrefix(clean, "../") &&
		(clean == l.WorkflowRoot || strings.HasPrefix(clean, l.WorkflowRoot+"/"))
}

// IsNativePath reports whether a home-relative path belongs to OpenCode's
// documented discovery surface. Cortex workflow support files are excluded.
// Plugin files are validated against PluginRoot, the single declaration of
// the plugin destination root, so mapping and validation cannot disagree.
func (l Layout) IsNativePath(relative string) bool {
	clean := path.Clean(strings.ReplaceAll(relative, "\\", "/"))
	if clean == path.Join(l.ConfigRoot, "opencode.json") || clean == path.Join(l.ConfigRoot, "opencode.jsonc") || clean == path.Join(l.ConfigRoot, "AGENTS.md") {
		return true
	}
	if nativeMarkdownChild(clean, l.AgentsRoot) || nativeMarkdownChild(clean, l.CommandsRoot) {
		return true
	}
	if plugin := strings.TrimPrefix(clean, l.PluginRoot+"/"); plugin != clean && plugin != "" {
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
