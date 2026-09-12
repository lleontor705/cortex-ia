#!/usr/bin/env bash
# =============================================================================
# test-cortex-report-hub-snapshot.sh
#
# Fixture-based test suite for cortex-report-hub-snapshot.sh
# =============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
SNAPSHOT_TOOL="$REPO_ROOT/tools/cortex-report-hub-snapshot.sh"
FIXTURES_DIR="$REPO_ROOT/testdata/report_hub_fixtures"

PASSED=0
FAILED=0

RED='\033[0;31m'
GREEN='\033[0;32m'
CYAN='\033[0;36m'
NC='\033[0m'

log_test() {
    printf "${CYAN}[TEST]${NC} %s\n" "$1"
}

log_pass() {
    printf "  ${GREEN}✓ PASS:${NC} %s\n" "$1"
    PASSED=$((PASSED + 1))
}

log_fail() {
    printf "  ${RED}✗ FAIL:${NC} %s\n" "$1"
    FAILED=$((FAILED + 1))
}

assert_exit_code() {
    local expected="$1"
    local actual="$2"
    local desc="$3"
    if [ "$expected" -eq "$actual" ]; then
        log_pass "$desc (exit $actual)"
    else
        log_fail "$desc (expected exit $expected, got $actual)"
    fi
}

assert_contains() {
    local haystack="$1"
    local needle="$2"
    local desc="$3"
    if echo "$haystack" | grep -q "$needle"; then
        log_pass "$desc"
    else
        log_fail "$desc (expected to contain '$needle')"
    fi
}

assert_not_contains() {
    local haystack="$1"
    local needle="$2"
    local desc="$3"
    if ! echo "$haystack" | grep -q "$needle"; then
        log_pass "$desc"
    else
        log_fail "$desc (should NOT contain '$needle')"
    fi
}

echo "=== Running cortex-report-hub-snapshot Fixture Tests ==="
echo "Tool: $SNAPSHOT_TOOL"
echo "Fixtures: $FIXTURES_DIR"
echo ""

# -----------------------------------------------------------------------------
# Test 1: Mixed logs parsing, ignoring non-reports, mapping codes
# -----------------------------------------------------------------------------
log_test "1. Parse mixed logs fixture and verify code mappings"
set +e
OUT=$(REPORT_HUB_LOG_SOURCE="$FIXTURES_DIR/mixed_logs.jsonl" "$SNAPSHOT_TOOL" 2>&1)
CODE=$?
set -e
assert_exit_code 0 "$CODE" "Runs successfully on mixed_logs.jsonl"

# Check code mappings via Python assertions
python3 -c '
import json, sys
data = json.loads(sys.argv[1])
assert isinstance(data, list), "Expected list of groups"
assert len(data) == 6, f"Expected 6 groups, got {len(data)}"

by_code_task = {(g["code"], g["task"]): g for g in data}

# 1. ERR_INVARIANT_VIOLATION -> critical / actionable
inv = by_code_task[("ERR_INVARIANT_VIOLATION", "task-alpha")]
assert inv["severity"] == "critical"
assert inv["action"] == "actionable"
assert inv["source"] == "worker"
assert inv["count"] == 2
assert inv["ids"] == ["rep-inv-1", "rep-inv-2"]

# 2. ERR_DELEGATION_FAILURE -> high / needs-investigation
delg = by_code_task[("ERR_DELEGATION_FAILURE", "task-beta")]
assert delg["severity"] == "high"
assert delg["action"] == "needs-investigation"
assert delg["count"] == 1
assert delg["ids"] == ["rep-del-1"]

# 3. ERR_VERIFICATION_FAIL -> medium / needs-test-evidence
ver = by_code_task[("ERR_VERIFICATION_FAIL", "task-gamma")]
assert ver["severity"] == "medium"
assert ver["action"] == "needs-test-evidence"
assert ver["count"] == 1
assert ver["ids"] == ["rep-ver-1"]

# 4. ERR_TASK_BLOCKED -> low / operational
blk1 = by_code_task[("ERR_TASK_BLOCKED", "task-delta")]
assert blk1["severity"] == "low"
assert blk1["action"] == "operational"

# 5. ERR_TASK_BLOCKED with empty task
blk2 = by_code_task[("ERR_TASK_BLOCKED", "")]
assert blk2["severity"] == "low"
assert blk2["action"] == "operational"
assert blk2["task"] == ""

# 6. Unknown code -> low / unclassified
unk = by_code_task[("ERR_UNKNOWN_CUSTOM", "task-epsilon")]
assert unk["severity"] == "low"
assert unk["action"] == "unclassified"

# 7. Timestamps omitted
for g in data:
    assert "timestamp" not in g
    assert "created_at" not in g

# 8. Deterministic sorting: by code, then source, then task
keys = [(g["code"], g["source"], g["task"]) for g in data]
assert keys == sorted(keys), "Groups must be stably sorted by (code, source, task)"
' "$OUT" && log_pass "Mappings, grouping, and ordering assertions verified" || log_fail "Mapping assertions failed"

# Ensure non-report lines were completely omitted (no raw log output)
assert_not_contains "$OUT" "🚀 Cortex Report Hub started" "Non-report startup log omitted from output"
assert_not_contains "$OUT" "GET /health" "Non-report health check log omitted from output"

# -----------------------------------------------------------------------------
# Test 2: ID capping (max 20 IDs/group)
# -----------------------------------------------------------------------------
log_test "2. Verify ID capping at max 20 per group"
set +e
OUT=$(REPORT_HUB_LOG_SOURCE="$FIXTURES_DIR/capping.jsonl" "$SNAPSHOT_TOOL" 2>&1)
CODE=$?
set -e
assert_exit_code 0 "$CODE" "Runs successfully on capping.jsonl"

python3 -c '
import json, sys
data = json.loads(sys.argv[1])
assert len(data) == 1
g = data[0]
cnt = g["count"]
assert cnt == 25, f"Expected count=25, got {cnt}"
ids_len = len(g["ids"])
assert ids_len == 20, f"Expected 20 IDs capped, got {ids_len}"
assert g["ids"][0] == "id-01"
assert g["ids"][-1] == "id-20"
' "$OUT" && log_pass "25 events correctly recorded count=25 and capped IDs to 20" || log_fail "Capping assertion failed"

# -----------------------------------------------------------------------------
# Test 3: Redaction and length capping
# -----------------------------------------------------------------------------
log_test "3. Verify token/secret redaction and string length capping"
set +e
OUT=$(REPORT_HUB_LOG_SOURCE="$FIXTURES_DIR/redaction.jsonl" "$SNAPSHOT_TOOL" 2>&1)
CODE=$?
set -e
assert_exit_code 0 "$CODE" "Runs successfully on redaction.jsonl"

python3 -c '
import json, sys
data = json.loads(sys.argv[1])
assert len(data) == 3

# Check redaction of Bearer and ghp_ tokens
inv = next(g for g in data if g["code"] == "ERR_INVARIANT_VIOLATION")
assert "secret123456789" not in inv["source"]
assert "[REDACTED]" in inv["source"]
assert "abcdef1234567890abcdef1234567890" not in inv["task"]
assert "[REDACTED]" in inv["task"]

# Check query param secrets
delg = next(g for g in data if g["code"] == "ERR_DELEGATION_FAILURE")
assert "supersecret123" not in delg["source"]
assert "mysecrettoken" not in delg["task"]
assert "token=[REDACTED]" in delg["source"]
assert "secret=[REDACTED]" in delg["task"]

# Check length capping
blk = next(g for g in data if g["code"] == "ERR_TASK_BLOCKED")
src_len = len(blk["source"])
tsk_len = len(blk["task"])
assert src_len <= 64, f"Source exceeded 64 chars: {src_len}"
assert tsk_len <= 128, f"Task exceeded 128 chars: {tsk_len}"
' "$OUT" && log_pass "Redaction of Bearer, API tokens, query secrets, and length caps verified" || log_fail "Redaction assertion failed"

# -----------------------------------------------------------------------------
# Test 4: Malformed input rejection
# -----------------------------------------------------------------------------
log_test "4. Verify malformed input rejection with non-zero exit and terse stderr"

# Case A: Broken JSON
set +e
ERR_OUT=$(REPORT_HUB_LOG_SOURCE="$FIXTURES_DIR/malformed_json.jsonl" "$SNAPSHOT_TOOL" 2>&1 1>/dev/null)
CODE=$?
set -e
assert_exit_code 1 "$CODE" "Rejects invalid JSON with non-zero exit"
assert_contains "$ERR_OUT" "error: malformed JSON" "Terse stderr contains 'error: malformed JSON'"

# Case B: Incomplete Report Received record
set +e
ERR_OUT=$(REPORT_HUB_LOG_SOURCE="$FIXTURES_DIR/malformed_report_missing_fields.jsonl" "$SNAPSHOT_TOOL" 2>&1 1>/dev/null)
CODE=$?
set -e
assert_exit_code 1 "$CODE" "Rejects missing fields in Report Received record"
assert_contains "$ERR_OUT" "error: malformed Report Received record" "Terse stderr contains 'error: malformed Report Received record'"

# Case C: Missing log file
set +e
ERR_OUT=$(REPORT_HUB_LOG_SOURCE="$FIXTURES_DIR/nonexistent.jsonl" "$SNAPSHOT_TOOL" 2>&1 1>/dev/null)
CODE=$?
set -e
assert_exit_code 1 "$CODE" "Rejects non-existent log source file"
assert_contains "$ERR_OUT" "error: log source file not found" "Terse stderr contains 'error: log source file not found'"

# -----------------------------------------------------------------------------
# Test 5: Empty logs handling
# -----------------------------------------------------------------------------
log_test "5. Empty logs handling"
set +e
OUT=$(REPORT_HUB_LOG_SOURCE="$FIXTURES_DIR/empty_logs.jsonl" "$SNAPSHOT_TOOL" 2>&1)
CODE=$?
set -e
assert_exit_code 0 "$CODE" "Returns 0 on logs with no error reports"
[ "$OUT" = "[]" ] && log_pass "Returns empty array '[]'" || log_fail "Expected '[]', got '$OUT'"

# Test /dev/null
set +e
OUT=$(REPORT_HUB_LOG_SOURCE="/dev/null" "$SNAPSHOT_TOOL" 2>&1)
CODE=$?
set -e
assert_exit_code 0 "$CODE" "Returns 0 on empty /dev/null source"
[ "$OUT" = "[]" ] && log_pass "Returns empty array '[]' on /dev/null" || log_fail "Expected '[]', got '$OUT'"

# -----------------------------------------------------------------------------
# Test 6: CLI Argument and Stdin support
# -----------------------------------------------------------------------------
log_test "6. Verify CLI argument and stdin pipe support"
set +e
OUT_ARG=$("$SNAPSHOT_TOOL" "$FIXTURES_DIR/mixed_logs.jsonl" 2>&1)
CODE_ARG=$?
OUT_STDIN=$(cat "$FIXTURES_DIR/mixed_logs.jsonl" | REPORT_HUB_LOG_SOURCE="-" "$SNAPSHOT_TOOL" 2>&1)
CODE_STDIN=$?
set -e
assert_exit_code 0 "$CODE_ARG" "CLI argument syntax works"
assert_exit_code 0 "$CODE_STDIN" "Stdin pipe syntax works"
[ "$OUT_ARG" = "$OUT_STDIN" ] && log_pass "CLI arg and stdin produce identical output" || log_fail "CLI arg and stdin outputs differ"

# -----------------------------------------------------------------------------
# Test 7: State neutrality (No disk state writes)
# -----------------------------------------------------------------------------
log_test "7. Verify no state writes to disk"
GIT_STATUS_BEFORE=$(git status --porcelain)
REPORT_HUB_LOG_SOURCE="$FIXTURES_DIR/mixed_logs.jsonl" "$SNAPSHOT_TOOL" > /dev/null
GIT_STATUS_AFTER=$(git status --porcelain)
[ "$GIT_STATUS_BEFORE" = "$GIT_STATUS_AFTER" ] && log_pass "No unexpected disk or git state mutations" || log_fail "State changed during snapshot run"

# -----------------------------------------------------------------------------
# Summary
# -----------------------------------------------------------------------------
echo ""
echo "=============================================="
printf "Results: ${GREEN}%d passed${NC}, ${RED}%d failed${NC}\n" "$PASSED" "$FAILED"
echo "=============================================="

if [ "$FAILED" -gt 0 ]; then
    exit 1
fi
exit 0
