package mcpserve

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/lleontor705/cortex-ia/v2/internal/hostbridge"
)

var (
	workflowKinds = []string{"direct-change", "fast-tdd", "hotfix", "ops-task"}
	claimStatus   = []string{"in_review", "in_progress", "blocked"}
	// workloadPolicies mirrors the CLI's --workload-policy values.
	workloadPolicies = []string{"strict", "flexible", "unbounded"}
)

func workReadTools() []tool {
	return []tool{
		{
			Name:        "cortex_ia_work_create",
			Description: "Create one work item in a same-board DAG (planner or orchestrator). board_id defaults to \"default\" for ungrouped work. Every dependency must already exist on the same board; check with board_status first. allowed_files must match the files the implementer will change: the transition refuses changes outside them.",
			Schema: schemaObject(props(
				"workflow", schemaEnum("Work workflow", workflowKinds...),
				"board_id", schemaString("Board identifier; defaults to \"default\""),
				"task_id", schemaString("Stable task identifier"),
				"title", schemaString("Task title"),
				"objective", schemaString("Task objective"),
				"acceptance_criteria", schemaString("Acceptance criteria"),
				"verification", schemaString("Raw executable verification command"),
				"allowed_files", schemaStrings("Leased repository paths"),
				"dependencies", schemaStrings("Task IDs on the same board"),
				"workload_policy", schemaEnum("Churn budget profile; defaults to flexible", workloadPolicies...),
			), "workflow", "task_id", "title"),
			Call: workCreate,
		},
		{
			Name:        "cortex_ia_work_review_refresh",
			Description: "Reopen a done task for fresh review after its approved files change (orchestrator only). Requires the current revision; a stale revision fails, so read work_status and retry.",
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
			Description: "List work items as rows, optionally filtered by board_id (all boards when omitted). Read-only.",
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
			Description: "Read a task's compact state for this caller: status, revision, dependencies with status, allowed_files, claim and lease liveness, last verdict and authority action. Read-only; no claim needed. Pass detail: true for the full record.",
			Schema: schemaObject(props(
				"task_id", schemaString("Task identifier"),
				"detail", schemaBool("Return the full durable record, projection and bridge_authority"),
			), "task_id"),
			Call: workStatus,
		},
		readOnlyCLI("cortex_ia_work_approvals", "List approval records for a task, oldest first. Read-only.", "work", "approvals"),
		readOnlyCLI("cortex_ia_work_fingerprint", "Compute file fingerprints for a task and compare them with its approved binding. Read-only.", "work", "fingerprint"),
		{
			Name:        "cortex_ia_work_recover",
			Description: "Recover every expired claim and lease, unscoped (orchestrator only). Use after a controller died and its claim lapsed. Own expired claim: work_recover_own. Live claim: work_reconcile.",
			Schema:      schemaObject(props()),
			Call: func(ctx context.Context, s *session, _ map[string]any) (string, error) {
				return s.runClean(ctx, []string{"work", "recover"})
			},
		},
		{
			Name:        "cortex_ia_work_retry",
			Description: "Move a reconciled blocked task back to ready (orchestrator only). Requires the current revision; a stale revision fails, so read work_status and retry. Use after work_reconcile or recovery.",
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
			Description: "Replace a blocked task with 2-8 smaller same-board tasks in one atomic step (planner only). Requires blocked status and the current revision. Each task needs objective, acceptance_criteria and a raw verification command.",
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
			Description: "Force-release a live claim whose controller is gone so the task can be retried (orchestrator only). Requires a reason and the current revision; default target blocked, to: ready releases directly. For expired claims use work_recover instead.",
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
	board := optString(args, "board_id")
	if board == "" {
		board = "default"
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
	for _, option := range [][2]string{{"objective", "--objective"}, {"acceptance_criteria", "--acceptance"}, {"verification", "--verify"}, {"workload_policy", "--workload-policy"}} {
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
	key := s.statusSessionKey(task)
	projection := s.acceptProjection(task, durable)
	bridge := s.bridgeAuthority(task, durable, key)
	if detail, _ := args["detail"].(bool); !detail {
		return encodeJSON(s.compactStatus(ctx, durable, bridge))
	}
	out := map[string]any{}
	for k, v := range durable {
		out[k] = v
	}
	out["found"] = true
	out["projection"] = projection
	out["bridge_authority"] = bridge
	return encodeJSON(stripTokens(out))
}

// compactStatus keeps the fields a controller acts on and drops tokens and the
// full record. Dependency statuses come from one board listing.
func (s *session) compactStatus(ctx context.Context, durable, bridge map[string]any) map[string]any {
	task, _ := durable["task_id"].(string)
	board, _ := durable["board_id"].(string)
	statuses := s.boardStatuses(ctx, board)
	dependencies := []any{}
	if deps, ok := durable["dependencies"].([]any); ok {
		for _, raw := range deps {
			id := fmt.Sprint(raw)
			status, known := statuses[id]
			if !known {
				status = "unknown"
			}
			dependencies = append(dependencies, map[string]any{"task_id": id, "status": status})
		}
	}
	var claim any
	if c, ok := durable["claim"].(map[string]any); ok {
		owner, live := claimLive(durable, s.now())
		claim = map[string]any{"owner": owner, "expires_at": c["expires_at"], "live": live}
	}
	leases := []any{}
	if list, ok := durable["leases"].([]any); ok {
		for _, raw := range list {
			if lease, ok := raw.(map[string]any); ok {
				leases = append(leases, map[string]any{"path": lease["path"], "expires_at": lease["expires_at"]})
			}
		}
	}
	var lastVerdict any
	if approval, ok := durable["latest_approval"].(map[string]any); ok {
		lastVerdict = approval["verdict"]
	}
	allowed := durable["allowed_files"]
	if allowed == nil {
		allowed = []any{}
	}
	out := map[string]any{
		"task_id":       task,
		"board_id":      board,
		"title":         durable["title"],
		"status":        durable["status"],
		"revision":      durable["revision"],
		"allowed_files": allowed,
		"dependencies":  dependencies,
		"claim":         claim,
		"leases":        leases,
		"last_verdict":  lastVerdict,
		"found":         true,
		"authority": map[string]any{
			"usable":       bridge["usable"],
			"write_usable": bridge["write_usable"],
			"action":       bridge["action"],
		},
	}
	if reason, ok := durable["blocked_reason"]; ok {
		out["blocked_reason"] = reason
	}
	out["maintenance"] = compactMaintenance(bridge)
	return out
}

// compactMaintenance tells the controller whether its claim is still being
// renewed. It carries no tokens and no policy fields.
func compactMaintenance(bridge map[string]any) map[string]any {
	m, ok := bridge["maintenance"].(map[string]any)
	if !ok {
		return map[string]any{"active": false, "reason": "no_local_handle"}
	}
	reason, _ := m["reason"].(string)
	if active, _ := m["active"].(bool); active {
		reason = "renewing"
	}
	return map[string]any{"active": m["active"], "reason": reason}
}

// boardStatuses maps task IDs to status for one board. A failed listing yields
// an empty map, so dependencies report "unknown" instead of failing the read.
func (s *session) boardStatuses(ctx context.Context, board string) map[string]string {
	statuses := map[string]string{}
	if board == "" {
		return statuses
	}
	stdout, err := s.runner.Run(ctx, []string{"work", "list", "--board", board}, "")
	if err != nil {
		return statuses
	}
	decoded, err := decodeJSON(strings.TrimSpace(stdout))
	items, ok := decoded.([]any)
	if err != nil || !ok {
		return statuses
	}
	for _, raw := range items {
		if item, ok := raw.(map[string]any); ok {
			id, _ := item["task_id"].(string)
			status, _ := item["status"].(string)
			statuses[id] = status
		}
	}
	return statuses
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
		"--revision", strconv.FormatInt(revision, 10), "--owner-session-inactive", strconv.FormatBool(s.vault.ownerIdle(task, s.now()))}
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
