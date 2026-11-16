package targets

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/assets"
)

var renderRoles = []string{"orchestrator", "discovery", "investigate", "planner", "implement", "reviewer"}

func findFile(t *testing.T, b Bundle, path string) []byte {
	t.Helper()
	for _, f := range b.Files {
		if f.Path == path {
			return f.Content
		}
	}
	t.Fatalf("bundle has no file %s", path)
	return nil
}

func countUnder(b Bundle, dir string) int {
	n := 0
	for _, f := range b.Files {
		if strings.HasPrefix(f.Path, dir+string(filepath.Separator)) {
			n++
		}
	}
	return n
}

func assertNoOpenCodeContracts(t *testing.T, b Bundle) {
	t.Helper()
	for _, f := range b.Files {
		if strings.Contains(string(f.Content), "~/.cortex-ia/opencode/contracts") {
			t.Errorf("%s still references the OpenCode contracts path", f.Path)
		}
	}
}

func TestClaudeBundleRender(t *testing.T) {
	home := t.TempDir()
	b, err := ClaudeBundle(home)
	if err != nil {
		t.Fatalf("ClaudeBundle: %v", err)
	}
	if n := countUnder(b, filepath.Join(home, ".claude", "agents")); n != len(renderRoles) {
		t.Fatalf("agents = %d, want %d", n, len(renderRoles))
	}
	for _, role := range renderRoles {
		content := string(findFile(t, b, filepath.Join(home, ".claude", "agents", role+".md")))
		if !strings.HasPrefix(content, "---\n") {
			t.Errorf("%s: no frontmatter", role)
		}
		want := "cortex-ia hook pre-tool --host claude --role " + role
		if !strings.Contains(content, want) {
			t.Errorf("%s: missing hook command %q", role, want)
		}
		if !strings.Contains(content, "name: "+role+"\n") {
			t.Errorf("%s: missing name key", role)
		}
	}
	globalSrc, err := assets.ReadBytes("AGENTS.md")
	if err != nil {
		t.Fatal(err)
	}
	wantGlobal := strings.TrimRight(string(rewriteContractsBytes(globalSrc, string(TargetClaude))), "\n")
	for _, role := range renderRoles {
		content := string(findFile(t, b, filepath.Join(home, ".claude", "agents", role+".md")))
		if !strings.Contains(content, "\n# Global protocol\n\n") {
			t.Errorf("%s lacks the global protocol appendix", role)
		}
		if !strings.Contains(content, wantGlobal) {
			t.Errorf("%s does not carry the rewritten installed AGENTS.md", role)
		}
	}

	protocol := string(findFile(t, b, filepath.Join(home, ".claude", "skills", "cortex-protocol", "SKILL.md")))
	front, _, _ := splitFrontmatter([]byte(protocol))
	if !strings.Contains(front, "disable-model-invocation: true") || strings.Contains(front, "autoinvoke") {
		t.Error("autoinvoke skill not converted to disable-model-invocation")
	}
	planCmd := string(findFile(t, b, filepath.Join(home, ".claude", "commands", "plan.md")))
	if !strings.HasPrefix(planCmd, "---\ndescription: \"Plan a larger change") || strings.Contains(planCmd, "agent: ") {
		t.Errorf("command frontmatter not reduced to description")
	}
	findFile(t, b, filepath.Join(home, ".cortex-ia", "claude", "contracts", "workflow-map.md"))
	assertNoOpenCodeContracts(t, b)
	if len(b.JSONSets) != 3 {
		t.Errorf("JSON sets = %d, want 3", len(b.JSONSets))
	}
	assertAgentShellSet(t, b, filepath.Join(home, ".claude", "settings.json"))
}

func TestAGYBundleRender(t *testing.T) {
	home := t.TempDir()
	b, err := AGYBundle(home)
	if err != nil {
		t.Fatalf("AGYBundle: %v", err)
	}
	plugin := filepath.Join(home, ".gemini", "config", "plugins", "cortex-ia")
	if n := countUnder(b, filepath.Join(plugin, "agents")); n != len(renderRoles) {
		t.Fatalf("agents = %d, want %d", n, len(renderRoles))
	}
	if n := countUnder(b, filepath.Join(plugin, "commands")); n != 0 {
		t.Errorf("agy plugin must not have commands, found %d", n)
	}
	findFile(t, b, filepath.Join(plugin, "skills", "cmd-plan", "SKILL.md"))
	cmd := string(findFile(t, b, filepath.Join(plugin, "skills", "cmd-plan", "SKILL.md")))
	if !strings.Contains(cmd, "name: cmd-plan\n") || !strings.Contains(cmd, "(Use when the user asks to run the plan command.)") {
		t.Errorf("cmd-plan skill frontmatter incomplete:\n%s", cmd)
	}
	if strings.Contains(cmd, "$ARGUMENTS") {
		t.Error("$ARGUMENTS not replaced in cmd skill")
	}

	rules := findFile(t, b, filepath.Join(plugin, "rules", "AGENTS.md"))
	if len(rules) >= 24000 {
		t.Errorf("rules/AGENTS.md is %d bytes, limit is 24000", len(rules))
	}
	if strings.HasPrefix(string(rules), "---") {
		t.Error("rules/AGENTS.md must not have frontmatter")
	}

	for _, name := range []string{"hooks.json", "mcp_config.json", "plugin.json"} {
		raw := findFile(t, b, filepath.Join(plugin, name))
		var v any
		if err := json.Unmarshal(raw, &v); err != nil {
			t.Errorf("%s is not valid JSON: %v", name, err)
		}
	}
	agent := string(findFile(t, b, filepath.Join(plugin, "agents", "orchestrator.md")))
	if !strings.Contains(agent, "mode: primary") || !strings.Contains(agent, "  - invoke_subagent\n") {
		t.Error("orchestrator agent frontmatter incomplete")
	}
	impl := string(findFile(t, b, filepath.Join(plugin, "agents", "implement.md")))
	if !strings.Contains(impl, "# Global protocol\n\n") {
		t.Error("agy implement agent lacks the global protocol appendix")
	}
	if !strings.Contains(impl, "  - replace_file_content\n") || !strings.Contains(impl, "mode: subagent") {
		t.Error("implement agent frontmatter incomplete")
	}
	assertNoOpenCodeContracts(t, b)
	findFile(t, b, filepath.Join(home, ".cortex-ia", "agy", "contracts", "workflow-map.md"))
}

func TestRenderHelpers(t *testing.T) {
	front, body, ok := splitFrontmatter([]byte("---\nname: x\ndescription: \"A: b\"\n---\nbody\n"))
	if !ok || body != "body\n" {
		t.Fatalf("splitFrontmatter = %q %q %v", front, body, ok)
	}
	if got := frontmatterValue(front, "description"); got != "A: b" {
		t.Errorf("frontmatterValue = %q", got)
	}
	if _, _, ok := splitFrontmatter([]byte("no frontmatter")); ok {
		t.Error("splitFrontmatter accepted text without frontmatter")
	}
	if got := rewriteContractsPath("see ~/.cortex-ia/opencode/contracts/x.md", "agy"); got != "see ~/.cortex-ia/agy/contracts/x.md" {
		t.Errorf("rewriteContractsPath = %q", got)
	}
	src := []byte("---\nname: s\nmetadata:\n  author: a\n  opencode/autoinvoke: false\n---\nbody")
	if out := string(rewriteAutoinvoke(src, "")); strings.Contains(out, "autoinvoke") || !strings.Contains(out, "author: a") {
		t.Errorf("rewriteAutoinvoke remove = %q", out)
	}
	if out := string(rewriteAutoinvoke(src, "disable-model-invocation: true")); !strings.Contains(out, "\ndisable-model-invocation: true\n---\nbody") {
		t.Errorf("rewriteAutoinvoke replace = %q", out)
	}
}

func TestHostBundlesResolveHomeAndRejectRelative(t *testing.T) {
	home := t.TempDir()
	for _, build := range []func(string) (Bundle, error){ClaudeBundle, AGYBundle} {
		b, err := build(home)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := Apply(Bundle{Target: b.Target, Home: "relative/home", Files: b.Files}, ApplyOptions{DryRun: true}); err == nil {
			t.Error("relative home accepted")
		}
	}
}

func assertAgentShellSet(t *testing.T, b Bundle, settings string) {
	t.Helper()
	for _, s := range b.JSONSets {
		if s.Path == settings && len(s.Key) == 2 && s.Key[0] == "env" && s.Key[1] == "CORTEX_IA_AGENT_SHELL" && s.Value == "1" {
			return
		}
	}
	t.Errorf("bundle lacks env.CORTEX_IA_AGENT_SHELL in %s", settings)
}
