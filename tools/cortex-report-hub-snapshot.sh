#!/usr/bin/env bash
# =============================================================================
# cortex-report-hub-snapshot.sh
#
# Safe local Railway report snapshot utility for Cortex-IA.
#
# Usage:
#   tools/cortex-report-hub-snapshot.sh [LOG_SOURCE_FILE]
#
# Environment Variables:
#   REPORT_HUB_LOG_SOURCE - Optional path to a JSONL log file (or '-' for stdin).
#                           When set, logs are read from this file instead of
#                           querying Railway directly.
#
# Default Execution:
#   When REPORT_HUB_LOG_SOURCE is unset and no argument is given, executes:
#     railway logs --project 8d1c83da-fcde-4fa9-b4ca-7dd4401bc89f \
#                  --environment dfcbefa5-4782-4265-90b2-926c7c2b513c \
#                  --service 5f4cd2a5-791d-4c60-8c61-fd94da2978ef \
#                  --since 15m --json
#
# Behavior:
#   - Parses only "Report Received" error records with ID, Code, Source, Task.
#   - Ignores non-report log lines (startup, healthchecks, etc.) without emitting raw output.
#   - Rejects malformed input (invalid JSON, incomplete records) non-zero with terse stderr.
#   - Redacts sensitive tokens (Bearer tokens, API keys, password/secret params).
#   - Caps value lengths (ID: 64, Code: 64, Source: 64, Task: 128) and IDs (max 20 IDs/group).
#   - Maps error codes to severity and action:
#       ERR_INVARIANT_VIOLATION -> critical / actionable
#       ERR_DELEGATION_FAILURE  -> high     / needs-investigation
#       ERR_VERIFICATION_FAIL   -> medium   / needs-test-evidence
#       ERR_TASK_BLOCKED        -> low      / operational
#       unknown                 -> low      / unclassified
#   - Emits stable, compact JSON grouped by (code, source, task).
#   - Omits timestamps.
#   - Pure read/aggregation: performs NO state writes to disk, DB, or Railway.
# =============================================================================

set -euo pipefail

LOG_SOURCE="${REPORT_HUB_LOG_SOURCE:-${1:-}}"

read -r -d '' PYTHON_PARSER << 'PYEOF' || true
import sys
import re
import json

CODE_MAP = {
    "ERR_INVARIANT_VIOLATION": ("critical", "actionable"),
    "ERR_DELEGATION_FAILURE": ("high", "needs-investigation"),
    "ERR_VERIFICATION_FAIL": ("medium", "needs-test-evidence"),
    "ERR_TASK_BLOCKED": ("low", "operational"),
}

def redact(val: str) -> str:
    if not isinstance(val, str):
        val = str(val)
    # Strip ANSI escape sequences
    val = re.sub(r'\x1b\[[0-9;]*[a-zA-Z]', '', val)
    # Redact Bearer tokens
    val = re.sub(r'(?i)\b(bearer[\s:=_-]+)[A-Za-z0-9_\-\.]+', r'[REDACTED]', val)
    # Redact common API key patterns (GitHub, OpenAI, GitLab, etc.)
    val = re.sub(r'\b(?:sk|ghp|gho|ghu|ghs|glpat)[-_][A-Za-z0-9_\-]{8,}\b', '[REDACTED]', val)
    # Redact JWT tokens
    val = re.sub(r'\bey[A-Za-z0-9_\-]{10,}\.[A-Za-z0-9_\-]{10,}\.[A-Za-z0-9_\-]+\b', '[REDACTED]', val)
    # Redact sensitive key=value patterns
    val = re.sub(r'(?i)\b(secret|token|password|passwd|apiKey|api_key)=([^\s&]+)', r'\1=[REDACTED]', val)
    # Strip control characters
    val = "".join(ch for ch in val if ch >= ' ' or ch == ' ')
    return val

def cap(val: str, max_len: int) -> str:
    if len(val) > max_len:
        return val[:max_len]
    return val

def main():
    target = sys.argv[1] if len(sys.argv) > 1 else "-"
    if target != "-":
        try:
            stream = open(target, "r", encoding="utf-8")
        except Exception as e:
            sys.stderr.write(f"error: failed to open log source '{target}': {e}\n")
            sys.exit(1)
    else:
        stream = sys.stdin

    groups = {}  # key: (code, source, task) -> list of IDs

    try:
        for line_num, raw_line in enumerate(stream, start=1):
            line = raw_line.strip()
            if not line:
                continue

            try:
                record = json.loads(line)
            except Exception as e:
                sys.stderr.write(f"error: malformed JSON on line {line_num}: {e}\n")
                sys.exit(1)

            if not isinstance(record, dict):
                sys.stderr.write(f"error: malformed input on line {line_num}: expected JSON object\n")
                sys.exit(1)

            # Check if this record is a "Report Received" error log
            msg = ""
            for k in ("message", "msg", "text", "log"):
                if k in record and isinstance(record[k], str):
                    msg = record[k]
                    break

            is_report = False
            rep_id = None
            code = None
            source = None
            task = None

            if "Report Received" in msg:
                is_report = True
                id_m = re.search(r'\bID=(\S+)', msg)
                code_m = re.search(r'\bCode=(\S+)', msg)
                source_m = re.search(r'\bSource=(\S+)', msg)
                task_m = re.search(r'\bTask=(.*?)(?=\s+CPU=\d+|\s*$)', msg)
                if not task_m:
                    task_m = re.search(r'\bTask=(\S*)', msg)

                if not id_m or not code_m or not source_m or task_m is None:
                    sys.stderr.write(f"error: malformed Report Received record on line {line_num}: missing required field\n")
                    sys.exit(1)

                rep_id = id_m.group(1).strip()
                code = code_m.group(1).strip()
                source = source_m.group(1).strip()
                task = task_m.group(1).strip()
            elif record.get("event") == "Report Received" or record.get("type") == "Report Received":
                is_report = True
                rep_id = str(record.get("id") or record.get("report_id") or record.get("ID") or "").strip()
                code = str(record.get("code") or record.get("error_code") or record.get("Code") or "").strip()
                source = str(record.get("source") or record.get("Source") or "").strip()
                task = str(record.get("task") or record.get("task_id") or record.get("Task") or "").strip()
                if not rep_id or not code or not source:
                    sys.stderr.write(f"error: malformed Report Received record on line {line_num}: missing required field\n")
                    sys.exit(1)
            elif "id" in record and "code" in record and "source" in record and "task" in record:
                is_report = True
                rep_id = str(record["id"]).strip()
                code = str(record["code"]).strip()
                source = str(record["source"]).strip()
                task = str(record["task"]).strip()
                if not rep_id or not code or not source:
                    sys.stderr.write(f"error: malformed Report Received record on line {line_num}: missing required field\n")
                    sys.exit(1)

            if not is_report:
                # Non-report log line (e.g. startup, healthcheck) - ignore with no raw log output
                continue

            if not rep_id or not code or not source:
                sys.stderr.write(f"error: malformed Report Received record on line {line_num}: empty required field\n")
                sys.exit(1)

            # Redact sensitive values and cap lengths
            clean_id = cap(redact(rep_id), 64)
            clean_code = cap(redact(code), 64)
            clean_source = cap(redact(source), 64)
            clean_task = cap(redact(task), 128)

            key = (clean_code, clean_source, clean_task)
            if key not in groups:
                groups[key] = []
            groups[key].append(clean_id)

    finally:
        if stream is not sys.stdin:
            stream.close()

    # Build stable, compact output
    output = []
    for (grp_code, grp_source, grp_task) in sorted(groups.keys(), key=lambda k: (k[0], k[1], k[2])):
        all_ids = groups[(grp_code, grp_source, grp_task)]
        severity, action = CODE_MAP.get(grp_code, ("low", "unclassified"))
        # Deduplicate and stably sort IDs within group
        unique_ids = sorted(list(dict.fromkeys(all_ids)))
        capped_ids = unique_ids[:20]
        output.append({
            "code": grp_code,
            "severity": severity,
            "action": action,
            "source": grp_source,
            "task": grp_task,
            "count": len(all_ids),
            "ids": capped_ids,
        })

    compact_json = json.dumps(output, separators=(',', ':'))
    sys.stdout.write(compact_json + "\n")

if __name__ == "__main__":
    main()
PYEOF

run_parser() {
    python3 -c "$PYTHON_PARSER" "$@"
}

if [ -n "$LOG_SOURCE" ]; then
    if [ "$LOG_SOURCE" != "-" ] && [ ! -r "$LOG_SOURCE" ]; then
        echo "error: log source file not found: $LOG_SOURCE" >&2
        exit 1
    fi
    run_parser "$LOG_SOURCE"
else
    railway logs --project 8d1c83da-fcde-4fa9-b4ca-7dd4401bc89f --environment dfcbefa5-4782-4265-90b2-926c7c2b513c --service 5f4cd2a5-791d-4c60-8c61-fd94da2978ef --since 15m --json | run_parser -
fi
