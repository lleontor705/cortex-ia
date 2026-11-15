package targets

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/lleontor705/cortex-ia/internal/assets"
)

type agyRole struct {
	name  string
	mode  string
	tools []string
}

var agyRoles = []agyRole{
	{"orchestrator", "primary", []string{"view_file", "invoke_subagent", "send_message", "manage_subagents", "ask_question", "call_mcp_tool"}},
	{"discovery", "subagent", []string{"run_command", "view_file", "grep_search", "find_by_name", "list_dir", "write_to_file", "call_mcp_tool"}},
	{"investigate", "subagent", []string{"run_command", "view_file", "grep_search", "find_by_name", "list_dir", "call_mcp_tool"}},
	{"planner", "subagent", []string{"view_file", "grep_search", "find_by_name", "list_dir", "call_mcp_tool"}},
	{"implement", "subagent", []string{"run_command", "write_to_file", "replace_file_content", "view_file", "grep_search", "find_by_name", "list_dir", "call_mcp_tool"}},
	{"reviewer", "subagent", []string{"run_command", "view_file", "grep_search", "find_by_name", "list_dir", "call_mcp_tool"}},
}

const agyPluginName = "cortex-ia"

const agyPreamble = `## Host adaptation: Antigravity
This role prompt is shared with OpenCode. In Antigravity:
- ` + "`cortex_ia_*`" + ` tools come from the ` + "`cortex_ia`" + ` MCP server and memory tools (` + "`cortex_*`" + `) from the ` + "`cortex`" + ` MCP server. Invoke both through ` + "`call_mcp_tool`" + `.
- ` + "`task({ subagent_type, ... })`" + ` dispatch is ` + "`invoke_subagent`" + `: the role name selects the subagent and the dispatch envelope goes in the prompt.
- ` + "`read`, `grep`, `glob`, `edit`, `write`, `shell`" + ` map to ` + "`view_file`, `grep_search`, `find_by_name`, `replace_file_content` or `write_to_file`, `run_command`" + `.
- Shared contracts live in ` + "`~/.cortex-ia/agy/contracts/`" + `.
- A pre-tool hook enforces file leases and this role's permission rules. When a call is denied, the reason states what to do next.`

const agyRulesStub = `# Cortex-IA for Antigravity

This plugin installs the Cortex-IA work-control bundle:

- Six role agents: ` + "`orchestrator`" + ` (primary) and ` + "`discovery`, `investigate`, `planner`, `implement`, `reviewer`" + ` (subagents).
- Skills under ` + "`skills/`" + `, including ` + "`cmd-*`" + ` skills that mirror the Cortex-IA slash commands.
- MCP servers ` + "`cortex_ia`" + ` (work authority, boards, leases) and ` + "`cortex`" + ` (memory and code graph).
- A pre-tool guard (` + "`cortex-ia-guard`" + `) that enforces file leases and role permission rules.

Coordinated work starts with the orchestrator: ` + "`agy --agent orchestrator`" + `.

Shared workflow contracts live in ` + "`~/.cortex-ia/agy/contracts/`" + `. Read them on demand; this file intentionally carries no protocol text.
`

type agyPluginManifest struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
}

type agyHookHandler struct {
	Type    string `json:"type"`
	Command string `json:"command"`
	Timeout int    `json:"timeout"`
}

type agyHookGroup struct {
	Matcher string           `json:"matcher"`
	Hooks   []agyHookHandler `json:"hooks"`
}

// AGYBundle renders the Antigravity plugin under ~/.gemini/config/plugins/cortex-ia
// together with its shared contracts and the plugin enablement in config.json.
func AGYBundle(home string) (Bundle, error) {
	home, err := resolveHome(home)
	if err != nil {
		return Bundle{}, err
	}
	inventory, err := assets.Inventory()
	if err != nil {
		return Bundle{}, err
	}
	global, err := assets.ReadBytes("AGENTS.md")
	if err != nil {
		return Bundle{}, err
	}
	global = rewriteContractsBytes(global, string(TargetAGY))

	plugin := filepath.Join(home, ".gemini", "config", "plugins", agyPluginName)
	b := Bundle{
		Target:    TargetAGY,
		Home:      home,
		StateDir:  stateDirFor(home, TargetAGY),
		OwnedDirs: []string{plugin, filepath.Join(home, ".cortex-ia", "agy")},
		Validate:  func() (string, error) { return validateAGYPlugin(plugin) },
		Notes:     []string{"Start the orchestrator with: agy --agent orchestrator"},
		JSONSets: []JSONSet{{
			Path:  filepath.Join(home, ".gemini", "config", "config.json"),
			Key:   []string{"plugins", agyPluginName},
			Value: map[string]any{"enabled": true},
		}},
	}

	jsonFiles := []struct {
		name  string
		value any
	}{
		{"plugin.json", agyPluginManifest{Name: agyPluginName, Version: "1.0.0", Description: "Cortex-IA work authority, role agents and skills for Antigravity"}},
		{"mcp_config.json", map[string]any{"mcpServers": map[string]any{
			"cortex":    map[string]any{"command": "cortex", "args": []string{"mcp", "--tools=agent"}},
			"cortex_ia": map[string]any{"command": "cortex-ia", "args": []string{"mcp", "serve", "--host", "agy"}},
		}}},
		{"hooks.json", map[string]any{"cortex-ia-guard": map[string]any{"PreToolUse": []agyHookGroup{{
			Matcher: "write_to_file|replace_file_content|multi_replace_file_content|edit|apply_patch|run_command|call_mcp_tool",
			Hooks:   []agyHookHandler{{Type: "command", Command: "cortex-ia hook pre-tool --host agy", Timeout: 15}},
		}}}}},
	}
	for _, jf := range jsonFiles {
		data, err := json.MarshalIndent(jf.value, "", "  ")
		if err != nil {
			return Bundle{}, fmt.Errorf("encode %s: %w", jf.name, err)
		}
		b.Files = append(b.Files, File{Path: filepath.Join(plugin, jf.name), Content: append(data, '\n')})
	}
	b.Files = append(b.Files, File{Path: filepath.Join(plugin, "rules", "AGENTS.md"), Content: []byte(agyRulesStub)})

	for _, role := range agyRoles {
		description, body, err := roleParts(role.name)
		if err != nil {
			return Bundle{}, err
		}
		b.Files = append(b.Files, File{
			Path:    filepath.Join(plugin, "agents", role.name+".md"),
			Content: agyAgentFile(role, description, body, global),
		})
	}

	for _, f := range inventory {
		switch f.Kind {
		case assets.KindSkill:
			data, err := assets.ReadBytes(f.Path)
			if err != nil {
				return Bundle{}, err
			}
			data = rewriteAutoinvoke(data, "")
			data = rewriteContractsBytes(data, string(TargetAGY))
			rel := strings.TrimPrefix(f.Path, "skills/")
			b.Files = append(b.Files, File{Path: filepath.Join(plugin, "skills", filepath.FromSlash(rel)), Content: data})
		case assets.KindCommand:
			data, err := assets.ReadBytes(f.Path)
			if err != nil {
				return Bundle{}, err
			}
			description, body, err := commandParts(data)
			if err != nil {
				return Bundle{}, fmt.Errorf("command %s: %w", f.Path, err)
			}
			name := strings.TrimSuffix(path.Base(f.Path), ".md")
			content := agyCommandSkill(name, description, rewriteContractsPath(body, string(TargetAGY)))
			b.Files = append(b.Files, File{Path: filepath.Join(plugin, "skills", "cmd-"+name, "SKILL.md"), Content: []byte(content)})
		}
	}

	contracts, err := contractFiles(inventory, home, TargetAGY)
	if err != nil {
		return Bundle{}, err
	}
	b.Files = append(b.Files, contracts...)
	return b, nil
}

func agyAgentFile(role agyRole, description, body string, global []byte) []byte {
	var sb strings.Builder
	sb.WriteString("---\n")
	sb.WriteString("name: " + role.name + "\n")
	sb.WriteString("description: " + yamlQuote(description) + "\n")
	sb.WriteString("mode: " + role.mode + "\n")
	sb.WriteString("tools:\n")
	for _, t := range role.tools {
		sb.WriteString("  - " + t + "\n")
	}
	sb.WriteString("---\n\n")
	sb.WriteString(composeRolePrompt(agyPreamble, rewriteContractsPath(body, string(TargetAGY)), global))
	return []byte(sb.String())
}

func agyCommandSkill(name, description, body string) string {
	desc := description + " (Use when the user asks to run the " + name + " command.)"
	body = strings.ReplaceAll(body, "$ARGUMENTS", "the user's request")
	return "---\nname: cmd-" + name + "\ndescription: " + yamlQuote(desc) + "\n---\n" + body
}

// validateAGYPlugin runs the agy plugin validator when the CLI is installed.
func validateAGYPlugin(pluginDir string) (string, error) {
	bin, err := exec.LookPath("agy")
	if err != nil {
		return "agy not found on PATH; plugin validation skipped", nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, bin, "plugin", "validate", pluginDir, "--json").CombinedOutput()
	text := strings.TrimSpace(string(out))
	if err != nil {
		return text, fmt.Errorf("agy plugin validate: %w", err)
	}
	return text, nil
}

// InstallAGY installs the Antigravity target with default options.
func InstallAGY(home string, dryRun bool) (*TargetResult, error) {
	return InstallAGYWith(home, ApplyOptions{DryRun: dryRun})
}

// InstallAGYWith installs the Antigravity target with explicit options.
func InstallAGYWith(home string, opts ApplyOptions) (*TargetResult, error) {
	b, err := AGYBundle(home)
	if err != nil {
		return nil, err
	}
	return Apply(b, opts)
}

// UninstallAGY removes everything recorded by an Antigravity install.
func UninstallAGY(home string, dryRun bool) (*TargetResult, error) {
	b, err := AGYBundle(home)
	if err != nil {
		return nil, err
	}
	return Remove(b, dryRun)
}
