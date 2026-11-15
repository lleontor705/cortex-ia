package mcpserve

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	maxSnapshotContentBytes = 1 << 20
	reportCodeFallback      = "ERR_TOOL_EXECUTION_FAILED"
)

var reportCodes = map[string]bool{
	"ERR_TASK_BLOCKED": true, "ERR_DELEGATION_FAIL": true, "ERR_INVARIANT_VIOLATION": true,
	"ERR_VERIFICATION_FAIL": true, "ERR_WORKER_DIED": true, "ERR_TOOL_AUTHORITY_UNUSABLE": true,
	"ERR_TOOL_EXECUTION_FAILED": true, "ERR_TOOL_MCP_FAILED": true, "ERR_TOOL_LEASE_REQUIRED": true,
	"ERR_TOOL_INVALID_ARGS": true, "ERR_TOOL_TIMEOUT": true, "ERR_SQLITE_TIMEOUT": true,
	"ERR_SUBAGENT_READONLY_REPEATED_ABORT": true, "ERR_SUBAGENT_CIRCUIT_OPEN": true, "ERR_SUBAGENT_EMPTY_OUTPUT": true,
}

func boardTools() []tool {
	return []tool{
		{
			Name:        "cortex_ia_board_create",
			Description: "Create one durable board for an initiative (planner or orchestrator). Use one stable board_id per initiative; list boards first to avoid duplicates.",
			Schema: schemaObject(props(
				"board_id", schemaString("Stable board identifier"),
				"title", schemaString("Board title"),
				"description", schemaString("Optional description"),
			), "board_id", "title"),
			Call: boardCreate,
		},
		{
			Name:        "cortex_ia_board_list",
			Description: "List durable initiative boards with their IDs and titles. Read-only.",
			Schema:      schemaObject(props()),
			Call: func(ctx context.Context, s *session, _ map[string]any) (string, error) {
				return s.runClean(ctx, []string{"board", "list"})
			},
		},
		{
			Name:        "cortex_ia_board_status",
			Description: "Read one board and its authoritative task snapshot. Read-only; use before creating work on an existing board.",
			Schema:      schemaObject(props("board_id", schemaString("Board identifier")), "board_id"),
			Call: func(ctx context.Context, s *session, args map[string]any) (string, error) {
				board, err := reqString(args, "board_id")
				if err != nil {
					return "", err
				}
				return s.runClean(ctx, []string{"board", "status", board})
			},
		},
		{
			Name:        "cortex_ia_report_error",
			Description: "Record an operational error in the telemetry ledger. Unknown codes fall back to ERR_TOOL_EXECUTION_FAILED. Use for tool and infrastructure failures, not for expected blockers.",
			Schema: schemaObject(props(
				"code", schemaString("Standard error code (ERR_TASK_BLOCKED, ERR_DELEGATION_FAIL, ERR_INVARIANT_VIOLATION, ERR_VERIFICATION_FAIL, ERR_TOOL_EXECUTION_FAILED, ERR_TOOL_LEASE_REQUIRED, ERR_TOOL_MCP_FAILED, ERR_TOOL_INVALID_ARGS)"),
				"message", schemaString("Descriptive error message explaining the failure condition"),
				"details", schemaString("Extended stack trace, error logs, or failure details"),
				"task_id", schemaString("Associated work task ID"),
			), "code", "message"),
			Call: reportError,
		},
		{
			Name:        "cortex_ia_snapshot_read",
			Description: "Read one local Cortex observation by ID through the bounded local export. Verifies identity and optional expected_sha256. Read-only; local CLI only, never a remote MCP store.",
			Schema: schemaObject(props(
				"observation_id", schemaNumber("Positive safe integer observation ID"),
				"project", schemaString("Project identifier (1-512 UTF-8 bytes)"),
				"expected_sha256", schemaString("Optional lowercase SHA-256 digest"),
			), "observation_id", "project"),
			Call: snapshotRead,
		},
		{
			Name:        "cortex_ia_workload_audit",
			Description: "Audit uncommitted git diff against the workload budget. Read-only; run before work_transition to in_review. Returns workload_status (PASS, EXCEEDED_ADVISORY, WORKLOAD_SOURCE_BUDGET_EXCEEDED, WORKLOAD_TEST_BUDGET_EXCEEDED).",
			Schema: schemaObject(props(
				"policy", schemaEnum("Budget tier to evaluate against (default flexible)", "strict", "flexible", "unbounded"),
				"baseRef", schemaString("Git revision to diff against (default HEAD)"),
			)),
			Call: workloadAudit,
		},
	}
}

func boardCreate(ctx context.Context, s *session, args map[string]any) (string, error) {
	id, err := reqString(args, "board_id")
	if err != nil {
		return "", err
	}
	title, err := reqString(args, "title")
	if err != nil {
		return "", err
	}
	argv := []string{"board", "create", "--id", id, "--title", title}
	if description := optString(args, "description"); description != "" {
		argv = append(argv, "--description", description)
	}
	return s.runClean(ctx, argv)
}

func reportError(ctx context.Context, s *session, args map[string]any) (string, error) {
	code, err := reqString(args, "code")
	if err != nil {
		return "", err
	}
	message, err := reqString(args, "message")
	if err != nil {
		return "", err
	}
	if !reportCodes[code] {
		code = reportCodeFallback
	}
	task := optString(args, "task_id")
	role := s.callerRole(task)
	if role == "" {
		role = "unknown"
	}
	argv := []string{"report", "error", "--code", code, "--message", message, "--role", role, "--source", role, "--workspace", s.workspace}
	if key, keyErr := s.sessionKeyFor(task); keyErr == nil {
		argv = append(argv, "--session-id", key)
	}
	if details := optString(args, "details"); details != "" {
		argv = append(argv, "--details", details)
	}
	if task != "" {
		argv = append(argv, "--task", task)
	}
	stdout, runErr := s.runner.Run(ctx, argv, "")
	if runErr != nil {
		return encodeJSON(map[string]any{"error": runErr.Error()})
	}
	return cleanOutput(stdout), nil
}

func snapshotRead(ctx context.Context, s *session, args map[string]any) (string, error) {
	rawID, ok := asInt64(args["observation_id"])
	if !ok || rawID < 1 {
		return "", errors.New("observation_id must be a positive safe integer")
	}
	project, err := reqString(args, "project")
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(project) == "" || len(project) > 512 {
		return "", errors.New("project must contain 1-512 UTF-8 bytes")
	}
	expected, hasExpected := presentString(args, "expected_sha256")
	if hasExpected && !isLowerHexDigest(expected) {
		return "", errors.New("expected_sha256 must be a lowercase SHA-256 digest")
	}
	argv := []string{"snapshot", "read", "--project", project, "--id", strconv.FormatInt(rawID, 10)}
	if hasExpected {
		argv = append(argv, "--expected-sha256", expected)
	}
	stdout, err := s.runner.Run(ctx, argv, "")
	if err != nil {
		return "", fmt.Errorf("local Cortex snapshot read failed before producing a verified snapshot: %w", err)
	}
	return verifySnapshot(stdout, project, rawID, expected, hasExpected)
}

func isLowerHexDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, r := range value {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}

func verifySnapshot(stdout, project string, id int64, expected string, hasExpected bool) (string, error) {
	decoded, err := decodeJSON(strings.TrimSpace(stdout))
	if err != nil {
		return "", errors.New("invalid structured Cortex snapshot")
	}
	snapshot, ok := decoded.(map[string]any)
	if !ok {
		return "", errors.New("invalid structured Cortex snapshot")
	}
	observed, _ := asInt64(snapshot["observation_id"])
	content, isString := snapshot["content"].(string)
	if snapshot["transport"] != "local_cortex_cli" || snapshot["project"] != project || observed != id || !isString {
		return "", errors.New("snapshot identity mismatch")
	}
	if len(content) > maxSnapshotContentBytes || !utf8.ValidString(content) {
		return "", errors.New("invalid snapshot content or byte length")
	}
	if declared, ok := asInt64(snapshot["byte_length"]); !ok || declared != int64(len(content)) {
		return "", errors.New("invalid snapshot content or byte length")
	}
	sum := sha256.Sum256([]byte(content))
	digest := hex.EncodeToString(sum[:])
	if snapshot["sha256"] != digest || (hasExpected && expected != digest) {
		return "", errors.New("snapshot digest mismatch")
	}
	return encodeJSON(snapshot)
}
