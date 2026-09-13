# Cortex Report Hub Snapshot Utility

`tools/cortex-report-hub-snapshot.sh` is a safe, state-neutral local utility that queries and aggregates error telemetry logs from the Cortex Report Hub running on Railway.

## Overview & Operation

The snapshot utility fetches recent logs (or reads from a local JSONL source), isolates `[Report Received]` operational error reports, redacts sensitive credentials, caps payload lengths, maps error codes to actionable triage categories, and outputs stable, compact JSON grouped by `(code, source, task)`.

### Operational Guarantees

- **No State Writes**: Does not mutate Railway, create files, touch SQLite (`~/.cortex-ia/delegation.db`), or alter local git configuration.
- **Fail-Closed & Terse Stderr**: Malformed inputs (invalid JSON syntax or missing required record fields) trigger an immediate non-zero exit code with concise error messages on `stderr`.
- **Credential Protection**: Redacts Bearer tokens, GitHub/OpenAI/GitLab API keys, JWTs, and query parameters (`token=`, `secret=`, `password=`) from outputs.
- **Payload Capping**: Bounded lengths (ID ≤ 64 chars, Code ≤ 64 chars, Source ≤ 64 chars, Task ≤ 128 chars) and capped group sample IDs (max 20 IDs per group).
- **Omitted Timestamps**: All temporal metadata is stripped from grouped records to ensure deterministic and cacheable snapshot output.
- **Stable Sort Order**: Groups are deterministically sorted by `(code, source, task)`, and IDs within each group are sorted and deduplicated.

## Usage

### 1. Default Mode (Live Railway Logs)

When `REPORT_HUB_LOG_SOURCE` is unset, the utility queries Railway directly:

```bash
bash ./tools/cortex-report-hub-snapshot.sh
```

This executes exactly:

```bash
railway logs --project 8d1c83da-fcde-4fa9-b4ca-7dd4401bc89f \
             --environment dfcbefa5-4782-4265-90b2-926c7c2b513c \
             --service 5f4cd2a5-791d-4c60-8c61-fd94da2978ef \
             --since 15m --json
```

### 2. Local Source Mode (JSONL File)

Set `REPORT_HUB_LOG_SOURCE` to read from a local file instead of Railway:

```bash
REPORT_HUB_LOG_SOURCE=/path/to/logs.jsonl bash ./tools/cortex-report-hub-snapshot.sh
```

Alternatively, pass the file as a CLI argument:

```bash
bash ./tools/cortex-report-hub-snapshot.sh /path/to/logs.jsonl
```

### 3. Pipeline / Stdin Mode

Set `REPORT_HUB_LOG_SOURCE="-"` or pass `-` to read from standard input:

```bash
cat logs.jsonl | REPORT_HUB_LOG_SOURCE="-" bash ./tools/cortex-report-hub-snapshot.sh
```

## Error Code Mapping Matrix

Error codes extracted from reports are mapped to standardized incident severity and triage action attributes:

| Error Code | Severity | Action | Description |
|---|---|---|---|
| `ERR_INVARIANT_VIOLATION` | `critical` | `actionable` | Dirty worktree, lease collision, or expired authority token |
| `ERR_DELEGATION_FAILURE` | `high` | `needs-investigation` | Worker crashed, timed out, or non-zero exit |
| `ERR_VERIFICATION_FAIL` | `medium` | `needs-test-evidence` | Test oracle or reviewer returned FAIL |
| `ERR_TASK_BLOCKED` | `low` | `operational` | Unmet dependencies or attempt exhaustion |
| *unknown* | `low` | `unclassified` | Any other unrecognized error code |

## Output Schema

The tool emits a compact JSON array on `stdout`. Each element represents a unique group:

```json
[
  {
    "code": "ERR_INVARIANT_VIOLATION",
    "severity": "critical",
    "action": "actionable",
    "source": "worker",
    "task": "task-42",
    "count": 2,
    "ids": [
      "rep-001",
      "rep-002"
    ]
  }
]
```

If no `Report Received` records are present in the log window, it returns `[]`.

## Fixture Tests

Run the automated fixture test suite:

```bash
bash ./tools/test-cortex-report-hub-snapshot.sh
```

Test fixtures are maintained under `testdata/report_hub_fixtures/`:
- `mixed_logs.jsonl`: Interleaved startup, healthcheck, and multiple error reports.
- `capping.jsonl`: 25 identical group reports verifying ID capping at 20.
- `redaction.jsonl`: Bearer tokens, GitHub keys, and oversized strings.
- `malformed_json.jsonl`: Broken JSON syntax.
- `malformed_report_missing_fields.jsonl`: Missing required fields in report records.
- `empty_logs.jsonl`: Zero report records.
