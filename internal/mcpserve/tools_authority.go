package mcpserve

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/hostbridge"
)

const (
	gotchaEvidencePrefix = "gotchas/"
	authorityActionRecon = "RECONCILE_WORK_THEN_RETRY_WITH_FRESH_AUTHORITY"
)

var approvalVerdicts = []string{"PASS", "FAIL", "BLOCKED", "INCONCLUSIVE", "pass", "fail", "blocked", "inconclusive"}

func authorityTools() []tool {
	return []tool{
		{
			Name:        "cortex_ia_work_claim",
			Description: "Claim a ready task and atomically reserve file lease(s) (supports path or paths). Tokens stay in the server with automatic heartbeat renewal; the result lists the claim, reserved files and maintenance status without any token.",
			Schema: schemaObject(props(
				"task_id", schemaString("Ready task identifier"),
				"path", schemaString("Optional single workspace-relative file to reserve upon claiming"),
				"paths", schemaStrings("Optional list of workspace-relative files to reserve upon claiming"),
				"ttl", schemaString("Duration (e.g. 15m)"),
			), "task_id"),
			Call: workClaim,
		},
		{
			Name:        "cortex_ia_work_renew",
			Description: "Renew the live claim held by this server and report the renewed claim without its token.",
			Schema: schemaObject(props(
				"task_id", schemaString("Claimed task identifier"),
				"ttl", schemaString("Optional duration (e.g. 15m)"),
			), "task_id"),
			Call: workRenew,
		},
		{
			Name:        "cortex_ia_file_reserve",
			Description: "Reserve one or more file leases for a claimed task. Each path must be reserved before it is edited.",
			Schema: schemaObject(props(
				"task_id", schemaString("Claimed task identifier"),
				"path", schemaString("Single workspace-relative file"),
				"paths", schemaStrings("Workspace-relative files"),
				"ttl", schemaString("Optional duration (e.g. 15m)"),
			), "task_id"),
			Call: fileReserve,
		},
		{
			Name:        "cortex_ia_work_lease_renew",
			Description: "Renew a live file lease retained by this server.",
			Schema: schemaObject(props(
				"task_id", schemaString("Claimed task identifier"),
				"path", schemaString("Leased workspace-relative file"),
				"ttl", schemaString("Optional duration (e.g. 15m)"),
			), "task_id", "path"),
			Call: leaseRenew,
		},
		{
			Name:        "cortex_ia_work_release_all",
			Description: "Release all task file leases in one transaction and stop the heartbeat for the task.",
			Schema:      schemaObject(props("task_id", schemaString("Claimed task identifier")), "task_id"),
			Call:        releaseAll,
		},
		{
			Name:        "cortex_ia_file_release",
			Description: "Release one or more file reservations using the lease tokens retained by this server.",
			Schema: schemaObject(props(
				"task_id", schemaString("Claimed task identifier"),
				"path", schemaString("Single workspace-relative file path to release"),
				"paths", schemaStrings("Workspace-relative file paths to release"),
			), "task_id"),
			Call: fileRelease,
		},
		{
			Name:        "cortex_ia_work_transition",
			Description: "Transition claimed task state (in_review, in_progress, blocked) with evidence. The revision is required and is compared durably.",
			Schema: schemaObject(props(
				"task_id", schemaString("Claimed task identifier"),
				"to", schemaEnum("Target state", claimStatus...),
				"status", schemaEnum("Alias for 'to'", claimStatus...),
				"revision", schemaNumber("Current task revision, from the claim receipt or cortex_ia_work_status"),
				"summary", schemaString("Human-readable execution summary"),
				"verdict", schemaEnum("Verification verdict", approvalVerdicts...),
				"evidence_refs", schemaStrings("Pointers to verified test files, commands or observations"),
				"changed_files", schemaStrings("List of modified workspace files"),
				"blocked_reason", schemaEnum("Structured reason class for blocked transitions", "authority_expired", "upstream", "needs_user", "env", "scope_drift"),
			), "task_id", "revision"),
			Call: workTransition,
		},
		{
			Name:        "cortex_ia_work_approve",
			Description: "Record independent task review verdict. PASS transitions task to done. A PASS needs evidence or summary; the claiming controller of the same task cannot approve it.",
			Schema: schemaObject(props(
				"task_id", schemaString("Task identifier"),
				"reviewer", schemaString("Accepted for compatibility; the reviewer identity is the calling controller"),
				"verdict", schemaEnum("Review verdict", approvalVerdicts...),
				"evidence", schemaString("Evidence pointer (test output, observation id, commit)"),
				"revision", schemaNumber("Current task revision, from the claim receipt or cortex_ia_work_status"),
				"summary", schemaString("Independent review summary and lens verdict"),
				"findings", schemaStrings("List of review findings or blockers"),
			), "task_id", "verdict", "revision"),
			Call: workApprove,
		},
		{
			Name:        "cortex_ia_work_recover_own",
			Description: "Recover the caller's own expired work claim and file leases for exactly one task (fail-closed ownership check).",
			Schema:      schemaObject(props("task_id", schemaString("Task identifier")), "task_id"),
			Call:        recoverOwn,
		},
	}
}

// requireAuthority returns the live handle for task when this caller owns it
// and the durable claim is still live. writing additionally requires the item
// to be in_progress.
func (s *session) requireAuthority(ctx context.Context, task, key string, writing bool) (*authority, error) {
	a := s.vault.get(task)
	if a == nil {
		return nil, errors.New("no live authority for this task in this server; claim it first")
	}
	if a.sessionKey != key {
		return nil, errors.New("task is claimed by another controller")
	}
	item, err := s.durableWork(ctx, task)
	if err != nil {
		return nil, authorityFailure("WORK_STATUS_UNAVAILABLE", task, nil, "")
	}
	owner, live := claimLive(item, s.now())
	if owner != hostbridge.Owner(key) || !live {
		return nil, authorityFailure("BRIDGE_AUTHORITY_UNUSABLE", task, item, "")
	}
	if writing && item["status"] != "in_progress" {
		return nil, authorityFailure("BRIDGE_WRITE_AUTHORITY_UNUSABLE", task, item, "")
	}
	return a, nil
}

func authorityFailure(code, task string, item map[string]any, path string) error {
	body := map[string]any{"code": code, "task_id": task, "durable_status": "unknown", "action": authorityActionRecon}
	if item != nil {
		body["durable_status"] = item["status"]
		body["revision"] = item["revision"]
		if claim, ok := item["claim"].(map[string]any); ok {
			body["claim_expires_at"] = claim["expires_at"]
		}
	}
	if path != "" {
		body["path"] = path
	}
	encoded, _ := json.Marshal(body)
	return errors.New(string(encoded))
}

func (s *session) durableWork(ctx context.Context, task string) (map[string]any, error) {
	stdout, err := s.runner.Run(ctx, []string{"work", "status", task}, "")
	if err != nil {
		return nil, err
	}
	decoded, err := decodeJSON(strings.TrimSpace(stdout))
	if err != nil {
		return nil, err
	}
	item, ok := decoded.(map[string]any)
	if !ok {
		return nil, errors.New("work status returned an unexpected shape")
	}
	return item, nil
}

// reservationPaths gathers path and paths, enforcing the CLI's 1-128 bound.
func reservationPaths(args map[string]any) ([]string, error) {
	if raw, present := args["paths"]; present && raw != nil {
		if _, ok := raw.([]any); !ok {
			return nil, errors.New("paths must be an array")
		}
	}
	var paths []string
	if value, present := args["path"]; present && value != nil {
		text, ok := value.(string)
		if !ok {
			return nil, errors.New("reservation requires 1-128 nonempty paths")
		}
		paths = append(paths, text)
	}
	extra, err := optStringList(args, "paths")
	if err != nil {
		return nil, errors.New("reservation requires 1-128 nonempty paths")
	}
	paths = append(paths, extra...)
	if len(paths) > 128 {
		return nil, errors.New("reservation requires 1-128 nonempty paths")
	}
	for _, p := range paths {
		if strings.TrimSpace(p) == "" {
			return nil, errors.New("reservation requires 1-128 nonempty paths")
		}
	}
	return paths, nil
}

// reservationReceipt checks that the CLI returned exactly one lease per
// requested path and that each carries its token.
func reservationReceipt(leases []any, requested []string) ([]map[string]any, error) {
	const incomplete = "incomplete reservation receipt; reconcile durable authority before retry"
	const invalid = "invalid reservation receipt; reconcile durable authority before retry"
	if len(leases) != len(requested) {
		return nil, errors.New(incomplete)
	}
	expected := map[string]bool{}
	for _, p := range requested {
		expected[workPath(strings.TrimSpace(p))] = true
	}
	out := make([]map[string]any, 0, len(leases))
	for _, raw := range leases {
		lease, ok := raw.(map[string]any)
		if !ok {
			return nil, errors.New(invalid)
		}
		token, _ := lease["lease_token"].(string)
		leasePath, _ := lease["path"].(string)
		if token == "" || !expected[leasePath] {
			return nil, errors.New(invalid)
		}
		delete(expected, leasePath)
		out = append(out, lease)
	}
	return out, nil
}

func workClaim(ctx context.Context, s *session, args map[string]any) (string, error) {
	task, err := reqString(args, "task_id")
	if err != nil {
		return "", err
	}
	key, err := s.sessionKeyFor(task)
	if err != nil {
		return "", err
	}
	if retained := s.vault.get(task); retained != nil {
		item, err := s.durableWork(ctx, task)
		if err != nil {
			return "", authorityFailure("WORK_STATUS_UNAVAILABLE", task, nil, "")
		}
		owner, live := claimLive(item, s.now())
		if live && owner == hostbridge.Owner(retained.sessionKey) {
			return "", fmt.Errorf("authority for %s is already held by this controller", task)
		}
		s.vault.remove(task)
	}
	if owned := s.vault.ownedBy(key); len(owned) > 0 {
		return "", errors.New("this implement session already owns one work task; dispatch a separate controller for additional work")
	}
	paths, err := reservationPaths(args)
	if err != nil {
		return "", err
	}
	argv := []string{"work", "claim", task, "--owner", hostbridge.Owner(key)}
	for _, p := range paths {
		argv = append(argv, "--path", p)
	}
	if ttl := optString(args, "ttl"); ttl != "" {
		argv = append(argv, "--ttl", ttl)
	}
	stdout, err := s.runner.Run(ctx, argv, "")
	if err != nil {
		return "", err
	}
	decoded, err := decodeJSON(strings.TrimSpace(stdout))
	claim, ok := decoded.(map[string]any)
	if err != nil || !ok {
		return "", errors.New("cortex-ia returned an invalid claim receipt")
	}
	token, _ := claim["claim_token"].(string)
	if token == "" {
		return "", errors.New("cortex-ia returned no claim token")
	}
	reservedRaw, _ := claim["reserved_files"].([]any)
	leases := map[string]string{}
	for _, raw := range reservedRaw {
		if lease, ok := raw.(map[string]any); ok {
			p, _ := lease["path"].(string)
			t, _ := lease["lease_token"].(string)
			if p != "" && t != "" {
				leases[p] = t
			}
		}
	}
	a := &authority{sessionKey: key, claimToken: token, leases: leases}
	s.vault.put(task, a)
	s.startHeartbeat(task, a)
	receipt, err := reservationReceipt(reservedRaw, paths)
	if err != nil {
		return "", fmt.Errorf("%v — the claim is held by this session; call cortex_ia_work_release_all or cortex_ia_work_recover_own before retrying", err)
	}
	delete(claim, "claim_token")
	reserved := make([]any, 0, len(receipt))
	for _, lease := range receipt {
		reserved = append(reserved, stripTokens(lease))
	}
	claim["reserved_files"] = reserved
	claim["maintenance"] = map[string]any{"active": true, "interval_ms": heartbeatInterval.Milliseconds(), "ttl": heartbeatTTL}
	return encodeJSON(stripTokens(claim))
}

func workRenew(ctx context.Context, s *session, args map[string]any) (string, error) {
	task, err := reqString(args, "task_id")
	if err != nil {
		return "", err
	}
	key, err := s.sessionKeyFor(task)
	if err != nil {
		return "", err
	}
	a, err := s.requireAuthority(ctx, task, key, false)
	if err != nil {
		return "", err
	}
	argv := []string{"work", "renew", task, "--claim-token", "@stdin"}
	if ttl := optString(args, "ttl"); ttl != "" {
		argv = append(argv, "--ttl", ttl)
	}
	stdout, err := s.runner.Run(ctx, argv, a.claimToken)
	if err != nil {
		return "", err
	}
	if !s.vault.isActive(a) {
		s.startHeartbeat(task, a)
	}
	return cleanOutput(stdout), nil
}

func fileReserve(ctx context.Context, s *session, args map[string]any) (string, error) {
	task, err := reqString(args, "task_id")
	if err != nil {
		return "", err
	}
	key, err := s.sessionKeyFor(task)
	if err != nil {
		return "", err
	}
	a, err := s.requireAuthority(ctx, task, key, true)
	if err != nil {
		return "", err
	}
	paths, err := reservationPaths(args)
	if err != nil {
		return "", err
	}
	if len(paths) == 0 {
		return "", errors.New("either 'path' or 'paths' must be provided to reserve files")
	}
	argv := []string{"work", "reserve", task, "--claim-token", "@stdin"}
	for _, p := range paths {
		argv = append(argv, "--path", p)
	}
	if ttl := optString(args, "ttl"); ttl != "" {
		argv = append(argv, "--ttl", ttl)
	}
	stdout, err := s.runner.Run(ctx, argv, a.claimToken)
	if err != nil {
		return "", err
	}
	decoded, err := decodeJSON(strings.TrimSpace(stdout))
	response, ok := decoded.(map[string]any)
	if err != nil || !ok {
		return "", errors.New("cortex-ia returned an invalid reservation receipt")
	}
	leaseList, isList := response["reserved"].([]any)
	if !isList {
		leaseList = []any{response}
	}
	receipt, err := reservationReceipt(leaseList, paths)
	if err != nil {
		return "", err
	}
	for _, lease := range receipt {
		p, _ := lease["path"].(string)
		t, _ := lease["lease_token"].(string)
		s.vault.setLease(a, p, t)
	}
	results := make([]any, 0, len(receipt))
	for _, lease := range receipt {
		results = append(results, stripTokens(lease))
	}
	if len(results) == 1 && optString(args, "path") != "" {
		return encodeJSON(results[0])
	}
	return encodeJSON(map[string]any{"reserved": results, "count": len(results)})
}

func leaseRenew(ctx context.Context, s *session, args map[string]any) (string, error) {
	task, err := reqString(args, "task_id")
	if err != nil {
		return "", err
	}
	rawPath, err := reqString(args, "path")
	if err != nil {
		return "", err
	}
	key, err := s.sessionKeyFor(task)
	if err != nil {
		return "", err
	}
	a, err := s.requireAuthority(ctx, task, key, false)
	if err != nil {
		return "", err
	}
	leasePath := workPath(rawPath)
	token := s.vault.leaseToken(a, leasePath)
	if token == "" {
		return "", authorityFailure("BRIDGE_LEASE_MISSING", task, nil, leasePath)
	}
	argv := []string{"work", "lease-renew", "--path", leasePath, "--lease-token", "@stdin"}
	if ttl := optString(args, "ttl"); ttl != "" {
		argv = append(argv, "--ttl", ttl)
	}
	stdout, err := s.runner.Run(ctx, argv, token)
	if err != nil {
		return "", err
	}
	if decoded, decodeErr := decodeJSON(strings.TrimSpace(stdout)); decodeErr == nil {
		if lease, ok := decoded.(map[string]any); ok {
			p, _ := lease["path"].(string)
			t, _ := lease["lease_token"].(string)
			if p != "" && t != "" {
				s.vault.dropLease(a, leasePath)
				s.vault.setLease(a, p, t)
			}
		}
	}
	return cleanOutput(stdout), nil
}

func releaseAll(ctx context.Context, s *session, args map[string]any) (string, error) {
	task, err := reqString(args, "task_id")
	if err != nil {
		return "", err
	}
	key, err := s.sessionKeyFor(task)
	if err != nil {
		return "", err
	}
	a, err := s.requireAuthority(ctx, task, key, false)
	if err != nil {
		return "", err
	}
	released := s.vault.leasePaths(a)
	stdout, err := s.runner.Run(ctx, []string{"work", "release-all", task, "--claim-token", "@stdin"}, a.claimToken)
	if err != nil {
		return "", err
	}
	decoded, err := decodeJSON(strings.TrimSpace(stdout))
	receipt, ok := decoded.(map[string]any)
	if err != nil || !ok || receipt["task_id"] != task || receipt["released_all"] != true {
		return "", errors.New("WORK_RELEASE_UNCONFIRMED: reservations retained locally; query task status before retrying")
	}
	s.vault.stop(task, "release_all")
	s.vault.clearLeases(a)
	result := map[string]any{"released": released, "failures": []any{}}
	for k, v := range receipt {
		result[k] = v
	}
	return encodeJSON(stripTokens(result))
}

func fileRelease(ctx context.Context, s *session, args map[string]any) (string, error) {
	task, err := reqString(args, "task_id")
	if err != nil {
		return "", err
	}
	key, err := s.sessionKeyFor(task)
	if err != nil {
		return "", err
	}
	a, err := s.requireAuthority(ctx, task, key, false)
	if err != nil {
		return "", err
	}
	var targets []string
	seen := map[string]bool{}
	add := func(p string) {
		if p != "" && !seen[p] {
			seen[p] = true
			targets = append(targets, p)
		}
	}
	single := optString(args, "path")
	add(single)
	extra, err := optStringList(args, "paths")
	if err != nil {
		return "", err
	}
	for _, p := range extra {
		add(p)
	}
	if len(targets) == 0 {
		return "", errors.New("either 'path' or 'paths' must be provided to release files")
	}
	released := []string{}
	failures := []map[string]string{}
	for _, raw := range targets {
		leasePath := workPath(raw)
		token := s.vault.leaseToken(a, leasePath)
		if token == "" {
			failures = append(failures, map[string]string{"path": leasePath, "error": "BRIDGE_LEASE_MISSING"})
			continue
		}
		if _, runErr := s.runner.Run(ctx, []string{"work", "release", "--path", leasePath, "--lease-token", "@stdin"}, token); runErr != nil {
			failures = append(failures, map[string]string{"path": leasePath, "error": runErr.Error()})
			continue
		}
		s.vault.dropLease(a, leasePath)
		released = append(released, leasePath)
	}
	if len(targets) == 1 && single != "" && len(failures) == 0 {
		return encodeJSON(map[string]any{"released": released[0], "status": "released"})
	}
	return encodeJSON(map[string]any{"released": released, "failures": failures})
}
