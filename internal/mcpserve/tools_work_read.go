package mcpserve

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/lleontor705/cortex-ia/internal/hostbridge"
)

var (
	workflowKinds = []string{"direct-change", "fast-tdd", "hotfix", "ops-task"}
	claimStatus   = []string{"in_review", "in_progress", "blocked"}
)

func workReadTools() []tool {
	return []tool{
		{
			Name:        "cortex_ia_work_create",
			Description: "Create one work item in a durable same-board DAG. Conversation ownership fields are not recorded on this host.",
			Schema: schemaObject(props(
				"workflow", schemaEnum("Work workflow", workflowKinds...),
				"board_id", schemaString("Board identifier"),
				"task_id", schemaString("Stable task identifier"),
				"title", schemaString("Task title"),
				"objective", schemaString("Task objective"),
				"acceptance_criteria", schemaString("Acceptance criteria"),
				"verification", schemaString("Raw executable verification command"),
				"allowed_files", schemaStrings("Leased repository paths"),
				"dependencies", schemaStrings("Task IDs on the same board"),
			), "workflow", "board_id", "task_id", "title"),
			Call: workCreate,
		},
		{
			Name:        "cortex_ia_work_review_refresh",
			Description: "Reopen a done task for fresh review after approved files change (Orchestrator only).",
			Schema: schemaObject(props(
				"task_id", schemaString("Task identifier"),
				"revision", schemaNumber("Observed task revision"),
			), "task_id", "revision"),
			Call: func(ctx context.Context, s *session, args map[string]any) (string, error) {
				task, err := reqString(args, "task_id")
				if err != nil {
					return "", err
				}
				revision, err := reqRevision(args)
				if err != nil {
					return "", err
				}
				return s.runClean(ctx, []string{"work", "review-refresh", task, "--revision", strconv.FormatInt(revision, 10)})
			},
		},
		{
			Name:        "cortex_ia_work_list",
			Description: "List durable work items, optionally filtered by board.",
			Schema:      schemaObject(props("board_id", schemaString("Board identifier"))),
			Call: func(ctx context.Context, s *session, args map[string]any) (string, error) {
				argv := []string{"work", "list"}
				if board := optString(args, "board_id"); board != "" {
					argv = append(argv, "--board", board)
				}
				return s.runClean(ctx, argv)
			},
		},
		{
			Name:        "cortex_ia_work_status",
			Description: "Get task details, the projection, and authority usability for the calling controller. Read-only; does not require a claim.",
			Schema: schemaObject(props(
				"task_id", schemaString("Task identifier"),
				"role", schemaString("Accepted for compatibility; presentation role is not forwarded"),
			), "task_id"),
			Call: workStatus,
		},
		readOnlyCLI("cortex_ia_work_approvals", "List historical approval records for a task.", "work", "approvals"),
		readOnlyCLI("cortex_ia_work_fingerprint", "Calculate file fingerprints for a task and verify against approved binding.", "work", "fingerprint"),
		{
			Name:        "cortex_ia_work_recover",
			Description: "Recover expired work claims and leases (Orchestrator only).",
			Schema:      schemaObject(props()),
			Call: func(ctx context.Context, s *session, _ map[string]any) (string, error) {
				return s.runClean(ctx, []string{"work", "recover"})
			},
		},
		{
			Name:        "cortex_ia_work_retry",
			Description: "Retry a reconciled blocked task using revision CAS (Orchestrator only).",
			Schema: schemaObject(props(
				"task_id", schemaString("Task identifier"),
				"revision", schemaNumber("Current task revision"),
			), "task_id", "revision"),
			Call: func(ctx context.Context, s *session, args map[string]any) (string, error) {
				task, err := reqString(args, "task_id")
				if err != nil {
					return "", err
				}
				revision, err := reqRevision(args)
				if err != nil {
					return "", err
				}
				return s.runClean(ctx, []string{"work", "retry", task, "--revision", strconv.FormatInt(revision, 10)})
			},
		},
		{
			Name:        "cortex_ia_work_decompose",
			Description: "Atomically decompose a blocked task into 2-8 smaller DAG tasks (Planner only).",
			Schema: schemaObject(props(
				"task_id", schemaString("Blocked task identifier"),
				"revision", schemaNumber("Current task revision"),
				"tasks", schemaArray("Replacement tasks", schemaObject(props(
					"task_id", schemaString("Task identifier"),
					"title", schemaString("Task title"),
					"objective", schemaString("Task objective"),
					"acceptance_criteria", schemaString("Acceptance criteria"),
					"verification", schemaString("Raw executable verification command"),
					"allowed_files", schemaStrings("Leased repository paths"),
				), "task_id", "title", "objective", "acceptance_criteria", "verification")),
			), "task_id", "revision", "tasks"),
			Call: workDecompose,
		},
		{
			Name:        "cortex_ia_work_reconcile",
			Description: "Force-release an orphaned live work claim so the task can be retried (Orchestrator only). Owner-inactivity evidence is not available on this host, so the CLI relies on claim expiry alone.",
			Schema: schemaObject(props(
				"task_id", schemaString("Task identifier"),
				"reason", schemaString("Audit explanation"),
				"revision", schemaNumber("Current task revision for compare-and-set"),
				"to", schemaEnum("Release target; defaults to blocked", "ready"),
			), "task_id", "reason", "revision"),
			Call: workReconcile,
		},
	}
}

func readOnlyCLI(name, description string, verb ...string) tool {
	return tool{
		Name:        name,
		Description: description,
		Schema:      schemaObject(props("task_id", schemaString("Task identifier")), "task_id"),
		Call: func(ctx context.Context, s *session, args map[string]any) (string, error) {
			task, err := reqString(args, "task_id")
			if err != nil {
				return "", err
			}
			return s.runClean(ctx, append(append([]string{}, verb...), task))
		},
	}
}

func workCreate(ctx context.Context, s *session, args map[string]any) (string, error) {
	workflow, err := reqString(args, "workflow")
	if err != nil {
		return "", err
	}
	if !contains(workflowKinds, workflow) {
		return "", fmt.Errorf("workflow must be one of %s", strings.Join(workflowKinds, ", "))
	}
	board, err := reqString(args, "board_id")
	if err != nil {
		return "", err
	}
	task, err := reqString(args, "task_id")
	if err != nil {
		return "", err
	}
	title, err := reqString(args, "title")
	if err != nil {
		return "", err
	}
	argv := []string{"work", "create", "--board", board, "--id", task, "--title", title, "--workflow", workflow, "--project", s.workspace}
	for _, option := range [][2]string{{"objective", "--objective"}, {"acceptance_criteria", "--acceptance"}, {"verification", "--verify"}} {
		if value := optString(args, option[0]); value != "" {
			argv = append(argv, option[1], value)
		}
	}
	files, err := optStringList(args, "allowed_files")
	if err != nil {
		return "", err
	}
	for _, file := range files {
		argv = append(argv, "--file", file)
	}
	dependencies, err := optStringList(args, "dependencies")
	if err != nil {
		return "", err
	}
	for _, dependency := range dependencies {
		argv = append(argv, "--depends", dependency)
	}
	return s.runClean(ctx, argv)
}

func contains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

func workStatus(ctx context.Context, s *session, args map[string]any) (string, error) {
	task, err := reqString(args, "task_id")
	if err != nil {
		return "", err
	}
	stdout, runErr := s.runner.Run(ctx, []string{"work", "status", task}, "")
	if runErr != nil {
		if strings.Contains(runErr.Error(), "work item not found") {
			return encodeJSON(map[string]any{
				"task_id": task, "found": false, "status": "not_found", "error": "work item not found",
				"message":          fmt.Sprintf("Work item %q not found in Cortex-IA SQLite work database. Note: Cortex MCP observation IDs (e.g. #84) are evidence/memories, not SQLite work task items, and planning notes have no work tasks until one is created.", task),
				"projection":       unknownProjection(task),
				"bridge_authority": s.bridgeAuthority(task, nil, ""),
			})
		}
		return "", runErr
	}
	decoded, err := decodeJSON(strings.TrimSpace(stdout))
	if err != nil {
		return "", fmt.Errorf("work status returned invalid JSON: %w", err)
	}
	durable, ok := decoded.(map[string]any)
	if !ok {
		return "", errors.New("work status returned an unexpected shape")
	}
	key, _ := s.sessionKeyFor(task)
	projection := s.acceptProjection(task, durable)
	out := map[string]any{}
	for k, v := range durable {
		out[k] = v
	}
	out["found"] = true
	out["projection"] = projection
	out["bridge_authority"] = s.bridgeAuthority(task, durable, key)
	return encodeJSON(stripTokens(out))
}

func workDecompose(ctx context.Context, s *session, args map[string]any) (string, error) {
	task, err := reqString(args, "task_id")
	if err != nil {
		return "", err
	}
	revision, err := reqRevision(args)
	if err != nil {
		return "", err
	}
	tasks, ok := args["tasks"].([]any)
	if !ok {
		return "", errors.New("tasks must be an array")
	}
	plan := map[string]any{"tasks": tasks}
	stdin, err := encodeJSON(plan)
	if err != nil {
		return "", err
	}
	argv := []string{"work", "decompose", task, "--revision", strconv.FormatInt(revision, 10), "--plan", "@stdin"}
	stdout, err := s.runner.Run(ctx, argv, stdin)
	if err != nil {
		return "", err
	}
	return cleanOutput(stdout), nil
}

func workReconcile(ctx context.Context, s *session, args map[string]any) (string, error) {
	task, err := reqString(args, "task_id")
	if err != nil {
		return "", err
	}
	reason, err := reqString(args, "reason")
	if err != nil {
		return "", err
	}
	revision, err := reqRevision(args)
	if err != nil {
		return "", err
	}
	key, err := s.sessionKeyFor(task)
	if err != nil {
		return "", err
	}
	argv := []string{"work", "reconcile", task, "--reason", reason, "--session", key,
		"--revision", strconv.FormatInt(revision, 10), "--owner-session-inactive", "false"}
	if to := optString(args, "to"); to != "" {
		argv = append(argv, "--to", to)
	}
	return s.runClean(ctx, argv)
}

func unknownProjection(task string) map[string]any {
	return map[string]any{"task_id": task, "revision": 0, "status": "unknown", "authority_available": false,
		"candidate_actions": []any{}, "blockers": []any{}, "notes": []any{}}
}

// acceptProjection keeps the newest projection seen per task so a stale status
// read cannot roll the controller back to an older revision.
func (s *session) acceptProjection(task string, durable map[string]any) map[string]any {
	if durable["task_id"] != task {
		return unknownProjection(task)
	}
	revision, _ := asInt64(durable["revision"])
	projection, ok := durable["projection"].(map[string]any)
	if !ok {
		projection = map[string]any{"task_id": task, "revision": revision, "status": durable["status"], "authority_available": durable["found"] != false}
	}
	if cached, seen := s.projections[task]; seen && revision < cached.revision {
		return cached.projection
	}
	s.projections[task] = projectionEntry{revision: revision, projection: projection}
	return projection
}

// bridgeAuthority reports the handle and durable claim state for one caller.
func (s *session) bridgeAuthority(task string, durable map[string]any, key string) map[string]any {
	a := s.vault.get(task)
	owner, live := claimLive(durable, s.now())
	status, _ := durable["status"].(string)
	handle := a != nil
	ownedHere := handle && a.sessionKey == key
	ownerMatches := owner != "" && owner == hostbridge.Owner(key)
	usable := handle && ownedHere && ownerMatches && live
	writeUsable := usable && status == "in_progress"
	view := map[string]any{
		"handle_present":               handle,
		"owned_by_current_session":     ownedHere,
		"durable_owner_matches_bridge": ownerMatches,
		"durable_claim_live":           live,
		"usable":                       usable,
		"write_usable":                 writeUsable,
		"retained_lease_paths":         []string{},
	}
	action := "STOP_WRITING_AND_RECONCILE"
	switch {
	case writeUsable:
		action = "CONTINUE_WITH_HEARTBEAT"
	case usable:
		action = "NON_WRITING_AUTHORITY_ONLY"
	}
	view["action"] = action
	if a != nil {
		view["retained_lease_paths"] = s.vault.leasePaths(a)
		view["maintenance"] = map[string]any{"active": s.vault.isActive(a), "reason": s.vault.reason(a), "interval_ms": heartbeatInterval.Milliseconds(), "ttl": heartbeatTTL}
	}
	return view
}

// claimLive reads the claim owner and whether its expiry lies in the future.
func claimLive(durable map[string]any, now time.Time) (string, bool) {
	claim, ok := durable["claim"].(map[string]any)
	if !ok {
		return "", false
	}
	owner, _ := claim["owner"].(string)
	expires, err := time.Parse(time.RFC3339Nano, fmt.Sprint(claim["expires_at"]))
	return owner, err == nil && expires.After(now)
}
