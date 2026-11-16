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

var (
	approvalVerdicts   = []string{"PASS", "FAIL", "BLOCKED", "INCONCLUSIVE"}
	transitionVerdicts = []string{"PASS", "FAIL", "BLOCKED", "INCONCLUSIVE", "pass", "fail", "blocked", "inconclusive"}
)

func authorityTools() []tool {
	return []tool{
		{
			Name:        "cortex_ia_work_claim",
			Description: "Claim a ready task and reserve its files in one step (implement only). Requires no other live claim by this session. Heartbeat renews the claim and leases automatically. Returns the claim without tokens. A path conflict fails the whole claim, so nothing is held; pick another task.",
			Schema: schemaObject(props(
				"task_id", schemaString("Ready task identifier"),
				"paths", schemaStrings("Optional workspace-relative files to reserve with the claim"),
			), "task_id"),
			Call: workClaim,
		},
		{
			Name:        "cortex_ia_work_renew",
			Description: "Renew the live claim and every file lease this session holds for the task in one call (implement only). Requires a live claim. Returns the new expiry without tokens. BRIDGE_AUTHORITY_UNUSABLE means the claim expired: stop writing and reconcile.",
			Schema:      schemaObject(props("task_id", schemaString("Claimed task identifier")), "task_id"),
			Call:        workRenew,
		},
		{
			Name:        "cortex_ia_file_reserve",
			Description: "Reserve leases for files you are about to edit (implement only). Requires a live claim in in_progress. Reserve each file before writing it, in sorted order. On BRIDGE_WRITE_AUTHORITY_UNUSABLE or a path conflict, release what you reserved and stop writing.",
			Schema: schemaObject(props(
				"task_id", schemaString("Claimed task identifier"),
				"paths", schemaStrings("Workspace-relative files to reserve (1-128)"),
			), "task_id", "paths"),
			Call: fileReserve,
		},
		{
			Name:        "cortex_ia_work_release_all",
			Description: "Release all file leases for the task in one transaction and stop its heartbeat (implement only). Use when abandoning the claim or before blocking it. WORK_RELEASE_UNCONFIRMED means leases are still held: read work_status before retrying.",
			Schema:      schemaObject(props("task_id", schemaString("Claimed task identifier")), "task_id"),
			Call:        releaseAll,
		},
		{
			Name:        "cortex_ia_file_release",
			Description: "Release file leases this session holds (implement only). Requires a live claim. Returns released paths and per-path failures; BRIDGE_LEASE_MISSING means the path was never reserved by this session. Release each file as soon as its edits are done.",
			Schema: schemaObject(props(
				"task_id", schemaString("Claimed task identifier"),
				"paths", schemaStrings("Workspace-relative files to release"),
			), "task_id", "paths"),
			Call: fileRelease,
		},
		{
			Name:        "cortex_ia_work_transition",
			Description: "Move the claimed task to in_review, in_progress or blocked with evidence (implement only). Requires a live claim and the current revision; a stale revision fails, so read work_status first. in_review fails GOTCHA_EVIDENCE_REQUIRED when the approval history holds a FAIL: add evidence_refs gotchas/<task_id>.",
			Schema: schemaObject(props(
				"task_id", schemaString("Claimed task identifier"),
				"to", schemaEnum("Target state", claimStatus...),
				"revision", schemaNumber("Current task revision from work_status"),
				"summary", schemaString("Human-readable execution summary"),
				"verdict", schemaEnum("Verification verdict", transitionVerdicts...),
				"evidence_refs", schemaStrings("Pointers to verified test files, commands or observations"),
				"changed_files", schemaStrings("Modified workspace files"),
				"blocked_reason", schemaEnum("Reason class for blocked transitions", "authority_expired", "upstream", "needs_user", "env", "scope_drift"),
			), "task_id", "to", "revision"),
			Call: workTransition,
		},
		{
			Name:        "cortex_ia_work_approve",
			Description: "Record an independent review verdict for a task in review (reviewer or orchestrator; never the implementing session). PASS requires evidence and moves the task to done. EVIDENCE_REQUIRED: pass evidence. Stale revision: read work_status, then re-review.",
			Schema: schemaObject(props(
				"task_id", schemaString("Task identifier"),
				"verdict", schemaEnum("Review verdict", approvalVerdicts...),
				"evidence", schemaString("Evidence pointer (test output, observation id, commit); required for PASS"),
				"revision", schemaNumber("Current task revision from work_status"),
				"summary", schemaString("Review summary and lens verdicts"),
				"findings", schemaStrings("Review findings or blockers"),
			), "task_id", "verdict", "revision"),
			Call: workApprove,
		},
		{
			Name:        "cortex_ia_work_recover_own",
			Description: "Recover this session's own expired claim and file leases for exactly one task (implement only). Use only after your claim lapsed (BRIDGE_AUTHORITY_UNUSABLE). Other controllers' claims need cortex_ia_work_recover (orchestrator).",
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

// reservationPaths reads the paths argument, enforcing the CLI's 1-128 bound.
func reservationPaths(args map[string]any) ([]string, error) {
	paths, err := optStringList(args, "paths")
	if err != nil {
		return nil, errors.New("reservation requires 1-128 nonempty paths")
	}
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
	a := &authority{sessionKey: key, claimToken: token, leases: leases, lastSeen: s.now()}
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
	receipt, err := s.renewAuthority(ctx, task, a)
	if err != nil {
		return "", err
	}
	if receipt.TaskID != task {
		return "", errors.New("controller renewal unconfirmed")
	}
	if !s.vault.isActive(a) {
		s.startHeartbeat(task, a)
	}
	return encodeJSON(receipt)
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
	return encodeJSON(map[string]any{"reserved": results, "count": len(results)})
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
	return encodeJSON(map[string]any{"released": released, "failures": failures})
}
