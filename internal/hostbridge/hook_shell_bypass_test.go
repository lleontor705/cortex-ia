package hostbridge

import "testing"

// Every spelling below reaches a work-authority verb through the shell and
// must be denied on both hosts.
var shellAuthorityBypasses = []string{
	"cortex-ia work approve T --reviewer x --revision 1 --verdict PASS",
	"cortex-ia work Approve T --reviewer x --revision 1 --verdict PASS",
	`"cortex-ia" work approve T --reviewer x`,
	"cortex-ia 'work' approve T --reviewer x",
	"cortex-ia --debug work approve T --reviewer x",
	"`which cortex-ia` work approve T --reviewer x",
	"$(which cortex-ia) work approve T --reviewer x",
	"C=cortex-ia; $C work approve T --reviewer x",
	"C=cortex-ia; ${C} work approve T --reviewer x",
	"/usr/local/bin/cortex-ia work approve T --reviewer x",
	"env cortex-ia work approve T --reviewer x",
	`bash -c "cortex-ia work approve T --reviewer x"`,
	"echo ok && cortex-ia work claim T --owner x",
	"cortex-ia work claim T --owner x",
	"cortex-ia board delete b1",
	"cortex-ia Board ARCHIVE b1",
	`sqlite3 ~/.cortex-ia/delegation.db "update tasks set status='done'"`,
	"SQLITE3 ~/.cortex-ia/Delegation.db 'select 1'",
	"CORTEX_IA_AGENT_SHELL= cortex-ia work status T",
	"unset CORTEX_IA_AGENT_SHELL; cortex-ia work status T",
	"echo $'cortex_ia_agent_shell'",
}

// Look-alikes that must stay allowed. approve-x is not an authority verb: the
// CLI rejects it as an unknown subcommand, so the hook lets it through too.
var shellLookalikes = []string{
	"cortex-ia work status T",
	"cortex-ia work list",
	"cortex-ia work approvals T",
	"cortex-ia board list",
	"cortex-ia board status b1",
	"cortex-ia work approve-x T",
	"echo approve",
	"go test ./...",
	"grep approve README.md",
}

func TestShellAuthorityBypassesDenied(t *testing.T) {
	for _, cmd := range shellAuthorityBypasses {
		t.Run("claude/"+cmd, func(t *testing.T) {
			d := mustDecide(t, claudeInput("investigate", "Bash", map[string]any{"command": cmd}), Deps{})
			assertVerdict(t, d, Deny, "")
		})
		t.Run("agy/"+cmd, func(t *testing.T) {
			d := mustDecide(t, agyInput("run_command", map[string]any{"CommandLine": cmd}), Deps{})
			assertVerdict(t, d, Deny, "")
		})
	}
}

func TestShellLookalikesAllowed(t *testing.T) {
	for _, cmd := range shellLookalikes {
		t.Run("claude/"+cmd, func(t *testing.T) {
			d := mustDecide(t, claudeInput("investigate", "Bash", map[string]any{"command": cmd}), Deps{})
			assertVerdict(t, d, PassThrough, "")
		})
		t.Run("agy/"+cmd, func(t *testing.T) {
			d := mustDecide(t, agyInput("run_command", map[string]any{"CommandLine": cmd}), Deps{})
			assertVerdict(t, d, PassThrough, "")
		})
	}
}

func TestAGYEmptyRoleMayImplementButNotApprove(t *testing.T) {
	denied := map[string]string{
		"cortex_ia_work_approve":   `{"task_id":"T","verdict":"PASS"}`,
		"cortex_ia_work_recover":   `{}`,
		"cortex_ia_work_retry":     `{"task_id":"T"}`,
		"cortex_ia_work_reconcile": `{"task_id":"T"}`,
		"cortex_ia_work_decompose": `{"task_id":"T"}`,
	}
	for tool, args := range denied {
		d := mustDecide(t, agyMCPCall(tool, args), Deps{StateHome: t.TempDir()})
		assertVerdict(t, d, Deny, "does not identify the calling role")
	}
	for _, tool := range []string{"cortex_ia_work_claim", "cortex_ia_work_transition", "cortex_ia_work_create"} {
		d := mustDecide(t, agyMCPCall(tool, `{"task_id":"T"}`), Deps{StateHome: t.TempDir()})
		assertVerdict(t, d, PassThrough, "")
	}
	open := mustDecide(t, agyMCPCall("cortex_ia_work_status", `{}`), Deps{})
	assertVerdict(t, open, PassThrough, "")
}

// agyMCPCall builds an agy call_mcp_tool input for the cortex_ia server.
func agyMCPCall(tool, argsJSON string) HookInput {
	return agyInput("call_mcp_tool", map[string]any{
		"ServerName": MCPServerName,
		"ToolName":   tool,
		"Arguments":  argsJSON,
	})
}
