package targets

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/assets"
)

type claudeRole struct {
	name    string
	tools   string
	matcher string
}

const claudeHookMatcher = "Write|Edit|MultiEdit|NotebookEdit|Bash|mcp__cortex_ia__.*"

var claudeRoles = []claudeRole{
	{"orchestrator", "Agent(discovery, investigate, planner, implement, reviewer), Read", "Read|mcp__cortex_ia__.*"},
	{"discovery", "Read, Grep, Glob, Bash, Edit, Write", claudeHookMatcher},
	{"investigate", "Read, Grep, Glob, Bash", claudeHookMatcher},
	{"planner", "Read, Grep, Glob", claudeHookMatcher},
	{"implement", "Read, Grep, Glob, Edit, Write, Bash", claudeHookMatcher},
	{"reviewer", "Read, Grep, Glob, Bash", claudeHookMatcher},
}

const claudePreamble = `## Host adaptation: Claude Code
This role prompt is shared with OpenCode. In Claude Code:
- ` + "`cortex_ia_*`" + ` tools are MCP tools of the ` + "`cortex_ia`" + ` server (full name ` + "`mcp__cortex_ia__<tool>`" + `). Memory tools written as ` + "`cortex_*`" + ` or ` + "`cortex_cortex_*`" + ` belong to the ` + "`cortex`" + ` MCP server.
- ` + "`task({ subagent_type, ... })`" + ` dispatch is the ` + "`Agent`" + ` tool: ` + "`subagent_type`" + ` is the role name and the dispatch envelope goes in the prompt.
- ` + "`read`, `grep`, `glob`, `edit`, `write`, `shell`" + ` are ` + "`Read`, `Grep`, `Glob`, `Edit`, `Write`, `Bash`" + `. Skills load through the ` + "`Skill`" + ` tool.
- Shared contracts live in ` + "`~/.cortex-ia/claude/contracts/`" + `.
- The OpenCode permission rules of this role are enforced here by its tool list and by a pre-tool hook. When a call is denied, the reason states what to do next.`

// ClaudeBundle renders the Claude Code install: role agents, skills, commands,
// shared contracts, and the cortex MCP servers in ~/.claude.json.
func ClaudeBundle(home string) (Bundle, error) {
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
	global = rewriteContractsBytes(global, string(TargetClaude))

	root := filepath.Join(home, ".claude")
	b := Bundle{
		Target:    TargetClaude,
		Home:      home,
		StateDir:  stateDirFor(home, TargetClaude),
		OwnedDirs: []string{filepath.Join(home, ".cortex-ia", "claude")},
		Notes: []string{
			"Start the orchestrator with: claude --agent orchestrator",
			"cortex-ia must be on PATH for hooks and the MCP server",
		},
		JSONSets: []JSONSet{
			{Path: filepath.Join(home, ".claude.json"), Key: []string{"mcpServers", "cortex"},
				Value: map[string]any{"command": "cortex", "args": []string{"mcp", "--tools=agent"}}},
			{Path: filepath.Join(home, ".claude.json"), Key: []string{"mcpServers", "cortex_ia"},
				Value: map[string]any{"command": "cortex-ia", "args": []string{"mcp", "serve", "--host", "claude"}}},
		},
	}

	for _, role := range claudeRoles {
		description, body, err := roleParts(role.name)
		if err != nil {
			return Bundle{}, err
		}
		content := claudeAgentFile(role, description, body, global)
		b.Files = append(b.Files, File{Path: filepath.Join(root, "agents", role.name+".md"), Content: content})
	}

	for _, f := range inventory {
		switch f.Kind {
		case assets.KindSkill:
			data, err := assets.ReadBytes(f.Path)
			if err != nil {
				return Bundle{}, err
			}
			if strings.HasSuffix(f.Path, "/SKILL.md") {
				data = rewriteAutoinvoke(data, "disable-model-invocation: true")
			}
			data = rewriteContractsBytes(data, string(TargetClaude))
			rel := strings.TrimPrefix(f.Path, "skills/")
			b.Files = append(b.Files, File{Path: filepath.Join(root, "skills", filepath.FromSlash(rel)), Content: data})
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
			content := "---\ndescription: " + yamlQuote(description) + "\n---\n" + rewriteContractsPath(body, string(TargetClaude))
			b.Files = append(b.Files, File{Path: filepath.Join(root, "commands", name+".md"), Content: []byte(content)})
		}
	}

	contracts, err := contractFiles(inventory, home, TargetClaude)
	if err != nil {
		return Bundle{}, err
	}
	b.Files = append(b.Files, contracts...)
	return b, nil
}

func claudeAgentFile(role claudeRole, description, body string, global []byte) []byte {
	var sb strings.Builder
	sb.WriteString("---\n")
	sb.WriteString("name: " + role.name + "\n")
	sb.WriteString("description: " + yamlQuote(description) + "\n")
	sb.WriteString("tools: " + role.tools + ", Skill, mcp__cortex_ia__*, mcp__cortex__*\n")
	sb.WriteString("hooks:\n")
	sb.WriteString("  PreToolUse:\n")
	sb.WriteString("    - matcher: " + yamlQuote(role.matcher) + "\n")
	sb.WriteString("      hooks:\n")
	sb.WriteString("        - type: command\n")
	sb.WriteString("          command: " + yamlQuote("cortex-ia hook pre-tool --host claude --role "+role.name) + "\n")
	sb.WriteString("          timeout: 15\n")
	sb.WriteString("---\n\n")
	prompt := composeRolePrompt(claudePreamble, rewriteContractsPath(body, string(TargetClaude)), global)
	sb.WriteString(prompt)
	return []byte(sb.String())
}

// InstallClaude installs the Claude Code target with default options.
func InstallClaude(home string, dryRun bool) (*TargetResult, error) {
	return InstallClaudeWith(home, ApplyOptions{DryRun: dryRun})
}

// InstallClaudeWith installs the Claude Code target with explicit options.
func InstallClaudeWith(home string, opts ApplyOptions) (*TargetResult, error) {
	b, err := ClaudeBundle(home)
	if err != nil {
		return nil, err
	}
	return Apply(b, opts)
}

// UninstallClaude removes everything recorded by a Claude Code install.
func UninstallClaude(home string, dryRun bool) (*TargetResult, error) {
	b, err := ClaudeBundle(home)
	if err != nil {
		return nil, err
	}
	return Remove(b, dryRun)
}
