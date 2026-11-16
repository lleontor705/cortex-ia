package hostbridge

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
)

type HookInput struct {
	Host       Host
	Role       string
	SessionKey string
	Tool       string
	Args       map[string]any
	Workspace  string
}

type Verdict int

const (
	PassThrough Verdict = iota
	Deny
)

type Decision struct {
	Verdict Verdict
	Reason  string
}

// LeaseChecker is implemented by an adapter over *delegation.Store so this package stays testable.
type LeaseChecker interface {
	VerifyLease(relPath, workspace, sessionKey string) (valid bool, reason string, err error)
	HasActiveWork(workspace string) (bool, error)
}

type Deps struct {
	StateHome string
	Leases    LeaseChecker
	Now       func() time.Time
}

const discoveryDocPath = ".cortex-ia/discovery.md"

// plannerPlanPath matches docs/plans/<board-id>.md with a single flat name segment.
var plannerPlanPath = regexp.MustCompile(`^docs/plans/[a-z0-9][a-z0-9._-]{0,127}\.md$`)

var orchestratorReadTools = map[string]bool{"Read": true, "Grep": true, "Glob": true}

var (
	claudeFileMutationTools = map[string]bool{"Write": true, "Edit": true, "MultiEdit": true, "NotebookEdit": true}
	claudeFilePathKeys      = []string{"file_path", "notebook_path"}
	agyFileMutationTools    = map[string]bool{"write_to_file": true, "replace_file_content": true, "multi_replace_file_content": true, "edit": true, "apply_patch": true, "write": true}
	agyFilePathKeys         = []string{"TargetFile", "targetFile", "AbsolutePath", "absolutePath", "file_path", "filePath", "path", "file"}
	agyMCPServerKeys        = []string{"ServerName", "serverName", "server_name", "server"}
	agyMCPToolKeys          = []string{"ToolName", "toolName", "tool_name", "tool"}
	agyMCPArgumentKeys      = []string{"Arguments", "arguments", "args", "input"}
	agyShellKeys            = []string{"CommandLine", "commandLine", "command"}
)

type claudeHookPayload struct {
	SessionID string         `json:"session_id"`
	AgentID   string         `json:"agent_id"`
	ToolName  string         `json:"tool_name"`
	ToolInput map[string]any `json:"tool_input"`
	Cwd       string         `json:"cwd"`
}

type agyHookPayload struct {
	ConversationID string `json:"conversationId"`
	ToolCall       struct {
		Name string         `json:"name"`
		Args map[string]any `json:"args"`
	} `json:"toolCall"`
	WorkspacePaths []string `json:"workspacePaths"`
	Cwd            string   `json:"cwd"`
}

func ParseHookPayload(host Host, role string, raw []byte) (HookInput, error) {
	role = strings.ToLower(strings.TrimSpace(role))
	switch host {
	case HostClaude:
		var payload claudeHookPayload
		if err := json.Unmarshal(raw, &payload); err != nil {
			return HookInput{}, fmt.Errorf("decode claude hook payload: %w", err)
		}
		key, err := SessionKey(HostClaude, payload.SessionID, payload.AgentID)
		if err != nil {
			return HookInput{}, err
		}
		workspace := strings.TrimSpace(os.Getenv("CLAUDE_PROJECT_DIR"))
		if workspace == "" {
			workspace = payload.Cwd
		}
		return newHookInput(HostClaude, role, key, payload.ToolName, payload.ToolInput, workspace)
	case HostAGY:
		var payload agyHookPayload
		if err := json.Unmarshal(raw, &payload); err != nil {
			return HookInput{}, fmt.Errorf("decode agy hook payload: %w", err)
		}
		key, err := SessionKey(HostAGY, payload.ConversationID, "")
		if err != nil {
			return HookInput{}, err
		}
		workspace := payload.Cwd
		if len(payload.WorkspacePaths) > 0 && strings.TrimSpace(payload.WorkspacePaths[0]) != "" {
			workspace = payload.WorkspacePaths[0]
		}
		return newHookInput(HostAGY, role, key, payload.ToolCall.Name, payload.ToolCall.Args, workspace)
	}
	return HookInput{}, fmt.Errorf("unsupported hook host %q", host)
}

func newHookInput(host Host, role, key, tool string, args map[string]any, workspace string) (HookInput, error) {
	if strings.TrimSpace(tool) == "" {
		return HookInput{}, fmt.Errorf("%s hook payload has no tool name", host)
	}
	workspace = strings.TrimSpace(workspace)
	if !filepath.IsAbs(workspace) {
		return HookInput{}, fmt.Errorf("%s hook workspace must be an absolute path", host)
	}
	return HookInput{
		Host:       host,
		Role:       role,
		SessionKey: key,
		Tool:       tool,
		Args:       args,
		Workspace:  filepath.Clean(workspace),
	}, nil
}

func Decide(in HookInput, deps Deps) (Decision, error) {
	if in.Role != "" && !slices.Contains(Roles, in.Role) {
		return Decision{}, fmt.Errorf("unknown hook role %q", in.Role)
	}
	if tool, args, ok := bridgeCall(in); ok {
		return decideBridge(in, deps, tool, args)
	}
	if isFileMutation(in) {
		return decideFileMutation(in, deps, stringArg(in.Args, filePathKeys(in)...))
	}
	if command, ok := shellCommand(in); ok {
		if reason := shellDenyReason(command); reason != "" {
			return deny(reason), nil
		}
		return pass(), nil
	}
	if in.Host == HostClaude && in.Role == "orchestrator" && orchestratorReadTools[in.Tool] {
		return decideOrchestratorRead(in), nil
	}
	return pass(), nil
}

// RenderFailure is the fail-closed output for an internal hook error. Claude
// blocks a call only through exit code 2 with the reason on stderr; agy reads
// its verdict from stdout.
func RenderFailure(host Host, err error) ([]byte, int) {
	if host == HostAGY {
		return renderAGY(Decision{Verdict: Deny, Reason: "cortex-ia hook error: " + err.Error()}), 0
	}
	return nil, 2
}

func RenderDecision(host Host, d Decision) ([]byte, int) {
	if host == HostAGY {
		return renderAGY(d), 0
	}
	if d.Verdict != Deny {
		return nil, 0
	}
	out, err := json.Marshal(claudeHookOutput{HookSpecificOutput: claudeHookSpecific{
		HookEventName:            "PreToolUse",
		PermissionDecision:       "deny",
		PermissionDecisionReason: d.Reason,
	}})
	if err != nil {
		return nil, 2
	}
	return out, 0
}

type claudeHookOutput struct {
	HookSpecificOutput claudeHookSpecific `json:"hookSpecificOutput"`
}

type claudeHookSpecific struct {
	HookEventName            string `json:"hookEventName"`
	PermissionDecision       string `json:"permissionDecision"`
	PermissionDecisionReason string `json:"permissionDecisionReason"`
}

type agyHookOutput struct {
	Decision string `json:"decision"`
	Reason   string `json:"reason,omitempty"`
}

func renderAGY(d Decision) []byte {
	out := agyHookOutput{Decision: "ask"}
	if d.Verdict == Deny {
		out = agyHookOutput{Decision: "deny", Reason: d.Reason}
	}
	// agy has no neutral verdict; "ask" keeps its normal permission prompt and
	// honors the user's always-allow cache.
	raw, err := json.Marshal(out)
	if err != nil {
		return []byte(`{"decision":"deny","reason":"cortex-ia hook could not render its decision"}`)
	}
	return raw
}

func pass() Decision { return Decision{Verdict: PassThrough} }

func deny(reason string) Decision { return Decision{Verdict: Deny, Reason: reason} }

func bridgeCall(in HookInput) (string, map[string]any, bool) {
	switch in.Host {
	case HostClaude:
		tool, ok := BridgeTool(in.Tool)
		return tool, in.Args, ok
	case HostAGY:
		if !strings.EqualFold(in.Tool, "call_mcp_tool") {
			return "", nil, false
		}
		if stringArg(in.Args, agyMCPServerKeys...) != MCPServerName {
			return "", nil, false
		}
		tool := stringArg(in.Args, agyMCPToolKeys...)
		if !strings.HasPrefix(tool, "cortex_ia_") {
			return "", nil, false
		}
		return tool, objectArg(in.Args, agyMCPArgumentKeys...), true
	}
	return "", nil, false
}

func decideBridge(in HookInput, deps Deps, tool string, args map[string]any) (Decision, error) {
	if !RoleMayCall(in.Role, tool) {
		if in.Role == "" {
			return deny(fmt.Sprintf("%s is restricted to roles [%s] and this host does not identify the calling role; approve or recover from a terminal with cortex-ia work", tool, strings.Join(AllowedRoles(tool), ", "))), nil
		}
		return deny(fmt.Sprintf("%s is available to roles [%s]; this agent's role is %q", tool, strings.Join(AllowedRoles(tool), ", "), in.Role)), nil
	}
	// Only restricted tools need a caller identity. Binding read-only calls
	// would let them overwrite the binding a pending restricted call relies on.
	if AllowedRoles(tool) == nil {
		return pass(), nil
	}
	taskID := stringArg(args, "task_id")
	if taskID == "" {
		return pass(), nil
	}
	if deps.StateHome == "" {
		return Decision{}, errors.New("state home is required to bind a caller")
	}
	now := time.Now()
	if deps.Now != nil {
		now = deps.Now()
	}
	if err := BindCaller(deps.StateHome, Caller{
		Host:       in.Host,
		SessionKey: in.SessionKey,
		Role:       in.Role,
		TaskID:     taskID,
		Tool:       tool,
		BoundAt:    now,
	}); err != nil {
		return Decision{}, fmt.Errorf("bind caller: %w", err)
	}
	return pass(), nil
}

func isFileMutation(in HookInput) bool {
	switch in.Host {
	case HostClaude:
		return claudeFileMutationTools[in.Tool]
	case HostAGY:
		return agyFileMutationTools[strings.ToLower(in.Tool)]
	}
	return false
}

func filePathKeys(in HookInput) []string {
	if in.Host == HostClaude {
		return claudeFilePathKeys
	}
	return agyFilePathKeys
}

func decideFileMutation(in HookInput, deps Deps, target string) (Decision, error) {
	if target == "" {
		return deny(fmt.Sprintf("cannot verify the target path of %s", in.Tool)), nil
	}
	rel, inside := workspaceRelative(in.Workspace, target)
	if !inside {
		if in.Host == HostAGY {
			return pass(), nil
		}
		return deny("target is outside the workspace"), nil
	}
	switch in.Role {
	case "discovery":
		if rel == discoveryDocPath || path.Base(rel) == ".gitignore" {
			return pass(), nil
		}
		return deny(fmt.Sprintf("role discovery may write only %s and .gitignore files; target is %q", discoveryDocPath, rel)), nil
	case "orchestrator", "investigate", "planner", "reviewer":
		// planner's only sanctioned mutation is its plan file
		if in.Role == "planner" && plannerPlanPath.MatchString(rel) {
			return pass(), nil
		}
		return deny(fmt.Sprintf("role %s is read-only; route the change to an implement task", in.Role)), nil
	}
	return decideLease(in, deps, rel)
}

func decideLease(in HookInput, deps Deps, rel string) (Decision, error) {
	if deps.Leases == nil {
		if in.Host == HostAGY {
			return pass(), nil
		}
		return deny("no cortex-ia work database; claim a task first"), nil
	}
	if in.Host == HostAGY {
		active, err := deps.Leases.HasActiveWork(in.Workspace)
		if err != nil {
			return Decision{}, fmt.Errorf("check active work: %w", err)
		}
		if !active {
			return pass(), nil
		}
	}
	valid, reason, err := deps.Leases.VerifyLease(rel, in.Workspace, in.SessionKey)
	if err != nil {
		return Decision{}, fmt.Errorf("verify lease: %w", err)
	}
	if valid {
		return pass(), nil
	}
	if reason == "" {
		reason = "no live lease covers this path"
	}
	return deny(fmt.Sprintf(`%s Reserve it with cortex_ia_file_reserve({ task_id, paths: ["%s"] }) under your claim, or claim the task with cortex_ia_work_claim({ task_id, paths: ["%s"] }).`, reason, rel, rel)), nil
}

// decideOrchestratorRead lets the orchestrator read and search inside the
// workspace. Env files stay unreadable, including through a glob filter.
func decideOrchestratorRead(in HookInput) Decision {
	target := stringArg(in.Args, "file_path")
	glob := ""
	switch in.Tool {
	case "Grep":
		target, glob = stringArg(in.Args, "path"), stringArg(in.Args, "glob")
	case "Glob":
		target, glob = stringArg(in.Args, "path"), stringArg(in.Args, "pattern")
	}
	if target == "" && in.Tool != "Read" {
		target = "."
	}
	rel, inside := workspaceRelative(in.Workspace, target)
	if !inside {
		return deny("the orchestrator reads and searches only inside the workspace; dispatch investigate for anything outside it")
	}
	if isEnvFile(rel) || envFilterDenied(glob) {
		return deny("env files hold secrets and are not readable by the orchestrator; dispatch investigate if their shape is needed")
	}
	return pass()
}

func shellCommand(in HookInput) (string, bool) {
	switch in.Host {
	case HostClaude:
		return stringArg(in.Args, "command"), in.Tool == "Bash"
	case HostAGY:
		return stringArg(in.Args, agyShellKeys...), strings.EqualFold(in.Tool, "run_command")
	}
	return "", false
}

// workspaceRelative returns the forward-slash path of target under workspace.
// ok is false when the target is empty or resolves outside the workspace.
func workspaceRelative(workspace, target string) (string, bool) {
	if strings.TrimSpace(target) == "" {
		return "", false
	}
	abs := target
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(workspace, abs)
	}
	rel, err := filepath.Rel(workspace, filepath.Clean(abs))
	if err != nil || filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return filepath.ToSlash(rel), true
}

func stringArg(args map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := args[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

// objectArg accepts an argument object or a JSON-encoded object string, since
// agy may deliver nested tool arguments either way.
func objectArg(args map[string]any, keys ...string) map[string]any {
	for _, key := range keys {
		switch value := args[key].(type) {
		case map[string]any:
			return value
		case string:
			var decoded map[string]any
			if err := json.Unmarshal([]byte(value), &decoded); err == nil {
				return decoded
			}
		}
	}
	return nil
}
