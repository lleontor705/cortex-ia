package hostbridge

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

const hookWorkspace = "/repo"

var hookNow = time.Date(2026, 11, 14, 10, 0, 0, 0, time.UTC)

type fakeLeases struct {
	valid  bool
	reason string
	active bool
}

func (f fakeLeases) VerifyLease(relPath, workspace, sessionKey string) (bool, string, error) {
	return f.valid, f.reason, nil
}

func (f fakeLeases) HasActiveWork(workspace string) (bool, error) { return f.active, nil }

func claudeInput(role, tool string, args map[string]any) HookInput {
	return HookInput{Host: HostClaude, Role: role, SessionKey: "cc-s1-a1", Tool: tool, Args: args, Workspace: hookWorkspace}
}

func agyInput(tool string, args map[string]any) HookInput {
	return HookInput{Host: HostAGY, SessionKey: "agy-conv1", Tool: tool, Args: args, Workspace: hookWorkspace}
}

func mustDecide(t *testing.T, in HookInput, deps Deps) Decision {
	t.Helper()
	d, err := Decide(in, deps)
	if err != nil {
		t.Fatalf("Decide(%s %s) error: %v", in.Host, in.Tool, err)
	}
	return d
}

func assertVerdict(t *testing.T, d Decision, want Verdict, reasonPart string) {
	t.Helper()
	if d.Verdict != want {
		t.Fatalf("verdict = %v, want %v (reason %q)", d.Verdict, want, d.Reason)
	}
	if reasonPart != "" && !strings.Contains(d.Reason, reasonPart) {
		t.Fatalf("reason %q does not contain %q", d.Reason, reasonPart)
	}
}

func TestDecideFileMutationWithLeases(t *testing.T) {
	write := map[string]any{"file_path": "/repo/src/a.go"}

	valid := mustDecide(t, claudeInput("implement", "Write", write), Deps{Leases: fakeLeases{valid: true}})
	assertVerdict(t, valid, PassThrough, "")

	missing := mustDecide(t, claudeInput("implement", "Write", write), Deps{Leases: fakeLeases{reason: "not leased"}})
	assertVerdict(t, missing, Deny, "cortex_ia_file_reserve")

	reviewer := mustDecide(t, claudeInput("reviewer", "Write", write), Deps{Leases: fakeLeases{valid: true}})
	assertVerdict(t, reviewer, Deny, "read-only")

	discovery := mustDecide(t, claudeInput("discovery", "Write", map[string]any{"file_path": "/repo/.cortex-ia/discovery.md"}), Deps{})
	assertVerdict(t, discovery, PassThrough, "")

	gitignore := mustDecide(t, claudeInput("discovery", "Edit", map[string]any{"file_path": "sub/.gitignore"}), Deps{})
	assertVerdict(t, gitignore, PassThrough, "")

	discoveryOther := mustDecide(t, claudeInput("discovery", "Write", write), Deps{})
	assertVerdict(t, discoveryOther, Deny, "discovery")

	outside := mustDecide(t, claudeInput("implement", "Write", map[string]any{"file_path": "/etc/passwd"}), Deps{Leases: fakeLeases{valid: true}})
	assertVerdict(t, outside, Deny, "outside the workspace")

	noPath := mustDecide(t, claudeInput("implement", "Write", map[string]any{}), Deps{Leases: fakeLeases{valid: true}})
	assertVerdict(t, noPath, Deny, "cannot verify the target path")

	noDB := mustDecide(t, claudeInput("implement", "Write", write), Deps{})
	assertVerdict(t, noDB, Deny, "claim a task first")
}

func TestDecideAGYFileMutation(t *testing.T) {
	args := map[string]any{"TargetFile": "/repo/src/a.go"}

	inactive := mustDecide(t, agyInput("write_to_file", args), Deps{Leases: fakeLeases{active: false}})
	assertVerdict(t, inactive, PassThrough, "")

	activeNoLease := mustDecide(t, agyInput("write_to_file", args), Deps{Leases: fakeLeases{active: true, reason: "not leased"}})
	assertVerdict(t, activeNoLease, Deny, "not leased")

	outside := mustDecide(t, agyInput("write_to_file", map[string]any{"TargetFile": "/elsewhere/x.go"}), Deps{Leases: fakeLeases{active: true}})
	assertVerdict(t, outside, PassThrough, "")
}

func TestDecideBridgeRoleAndBinding(t *testing.T) {
	stateHome := t.TempDir()
	deps := Deps{StateHome: stateHome, Now: func() time.Time { return hookNow }}
	approve := "mcp__cortex_ia__cortex_ia_work_approve"

	denied := mustDecide(t, claudeInput("implement", approve, map[string]any{"task_id": "T1"}), deps)
	assertVerdict(t, denied, Deny, "available to roles [reviewer, orchestrator]")

	allowed := mustDecide(t, claudeInput("reviewer", approve, map[string]any{"task_id": "T1"}), deps)
	assertVerdict(t, allowed, PassThrough, "")

	caller, err := ResolveCaller(stateHome, "T1", hookNow)
	if err != nil {
		t.Fatalf("ResolveCaller after bridge call: %v", err)
	}
	if caller.Role != "reviewer" || caller.SessionKey != "cc-s1-a1" {
		t.Fatalf("bound caller = %+v", caller)
	}

	agyApprove := agyInput("call_mcp_tool", map[string]any{
		"ServerName": "cortex_ia",
		"ToolName":   "cortex_ia_work_approve",
		"Arguments":  `{"task_id":"T2"}`,
	})
	assertVerdict(t, mustDecide(t, agyApprove, deps), Deny, "does not identify the calling role")
	if _, err := ResolveCaller(stateHome, "T2", hookNow); err == nil {
		t.Fatal("denied agy bridge call must not bind a caller")
	}
}

func TestDecideShell(t *testing.T) {
	cases := []struct {
		command string
		want    Verdict
	}{
		{"cortex-ia work approve T --verdict PASS", Deny},
		{"go test ./... && cortex-ia work approve T", Deny},
		{"cortex-ia board delete b1", Deny},
		{"sqlite3 ~/.cortex-ia/delegation.db 'select 1'", Deny},
		{"go test ./...", PassThrough},
		{"/usr/local/bin/cortex-ia work list", PassThrough},
	}
	for _, tc := range cases {
		d := mustDecide(t, claudeInput("implement", "Bash", map[string]any{"command": tc.command}), Deps{})
		if d.Verdict != tc.want {
			t.Fatalf("Bash %q verdict = %v, want %v", tc.command, d.Verdict, tc.want)
		}
	}

	agy := mustDecide(t, agyInput("run_command", map[string]any{"CommandLine": "cortex-ia work approve X --verdict PASS"}), Deps{})
	assertVerdict(t, agy, Deny, "cortex_ia_*")
}

func TestDecideRejectsUnknownRole(t *testing.T) {
	if _, err := Decide(claudeInput("admin", "Bash", map[string]any{"command": "ls"}), Deps{}); err == nil {
		t.Fatal("unknown role must fail closed with an error")
	}
}

func TestParseHookPayloadClaude(t *testing.T) {
	t.Setenv("CLAUDE_PROJECT_DIR", "")
	raw := []byte(`{"session_id":"s1","agent_id":"a1","tool_name":"Write","tool_input":{"file_path":"/tmp/ws/a.go"},"cwd":"/tmp/ws"}`)
	in, err := ParseHookPayload(HostClaude, "implement", raw)
	if err != nil {
		t.Fatalf("ParseHookPayload: %v", err)
	}
	if in.SessionKey != "cc-s1-a1" || in.Workspace != "/tmp/ws" || in.Tool != "Write" || in.Role != "implement" {
		t.Fatalf("claude input = %+v", in)
	}

	t.Setenv("CLAUDE_PROJECT_DIR", "/projects/x")
	in, err = ParseHookPayload(HostClaude, "", raw)
	if err != nil || in.Workspace != "/projects/x" {
		t.Fatalf("CLAUDE_PROJECT_DIR not preferred: %+v %v", in, err)
	}

	if _, err := ParseHookPayload(HostClaude, "", []byte("not json")); err == nil {
		t.Fatal("invalid JSON must be an error")
	}
	if _, err := ParseHookPayload(HostClaude, "", []byte(`{"session_id":"s1","cwd":"/tmp/ws"}`)); err == nil {
		t.Fatal("payload without tool_name must be an error")
	}
}

func TestParseHookPayloadAGY(t *testing.T) {
	raw := []byte(`{"conversationId":"conv1","toolCall":{"name":"write_to_file","args":{"TargetFile":"/ws/a.go"}},"workspacePaths":["/ws"],"cwd":"/other"}`)
	in, err := ParseHookPayload(HostAGY, "", raw)
	if err != nil {
		t.Fatalf("ParseHookPayload: %v", err)
	}
	if in.SessionKey != "agy-conv1" || in.Workspace != "/ws" || in.Tool != "write_to_file" || in.Args["TargetFile"] != "/ws/a.go" {
		t.Fatalf("agy input = %+v", in)
	}
	if _, err := ParseHookPayload(HostAGY, "", []byte(`{"toolCall":{"name":"edit"}}`)); err == nil {
		t.Fatal("agy payload without conversationId must be an error")
	}
}

func TestRenderDecisionShapes(t *testing.T) {
	if out, code := RenderDecision(HostClaude, pass()); out != nil || code != 0 {
		t.Fatalf("claude pass-through = %q, %d", out, code)
	}

	out, code := RenderDecision(HostClaude, deny("nope"))
	if code != 0 {
		t.Fatalf("claude deny exit = %d", code)
	}
	var claude struct {
		HookSpecificOutput struct {
			HookEventName            string `json:"hookEventName"`
			PermissionDecision       string `json:"permissionDecision"`
			PermissionDecisionReason string `json:"permissionDecisionReason"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal(out, &claude); err != nil {
		t.Fatalf("claude deny is not JSON: %v (%s)", err, out)
	}
	if claude.HookSpecificOutput.HookEventName != "PreToolUse" || claude.HookSpecificOutput.PermissionDecision != "deny" || claude.HookSpecificOutput.PermissionDecisionReason != "nope" {
		t.Fatalf("claude deny shape = %s", out)
	}

	out, code = RenderDecision(HostAGY, pass())
	if string(out) != `{"decision":"ask"}` || code != 0 {
		t.Fatalf("agy pass-through = %s, %d", out, code)
	}
	out, code = RenderDecision(HostAGY, deny("nope"))
	if string(out) != `{"decision":"deny","reason":"nope"}` || code != 0 {
		t.Fatalf("agy deny = %s, %d", out, code)
	}
}

func TestRenderFailureFailsClosed(t *testing.T) {
	if out, code := RenderFailure(HostClaude, errors.New("boom")); out != nil || code != 2 {
		t.Fatalf("claude failure = %q, %d", out, code)
	}
	if out, code := RenderFailure(HostAGY, errors.New("boom")); code != 0 || string(out) != `{"decision":"deny","reason":"cortex-ia hook error: boom"}` {
		t.Fatalf("agy failure = %s, %d", out, code)
	}
}
