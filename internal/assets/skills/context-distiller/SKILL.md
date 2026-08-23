---
name: context-distiller
description: Condense verbose shell output, compiler diagnostics, and test traces into compact, structured evidence for Cortex and receipts.
license: MIT
metadata:
  author: OpenCode Engine
  version: "1.0.0"
---

# Context Distiller & Evidence Condensation

Use this skill to prevent context pollution and attention degradation (*Lost-in-the-Middle*). It transforms raw console logs into concise, machine-readable evidence summaries.

## 1. Golden Rules of Evidence Distillation
1. **Never Dump Full STDOUT:** Never paste 100+ lines of raw build/test logs into Cortex observations or typed receipts.
2. **Retain Exact Failure Locality:** Preserve file path, line number, column, exception class, and exact error message.
3. **Filter Third-Party Noise:** Strip runtime internals from external dependencies (e.g. `node_modules/`, `site-packages/`, standard library internals) unless directly related to the defect.
4. **Capture Deterministic Metrics:** Record command line, exit code, execution duration, test count passed/failed, and git diff hash.

## 2. Extraction Templates

### Compiler / Linter Errors
Transform:
```text
[Raw 80 lines of TypeScript errors...]
```
Into:
```json
{
  "kind": "linter_error",
  "file": "src/services/auth.ts",
  "line": 42,
  "code": "TS2345",
  "message": "Argument of type 'string | null' is not assignable to parameter of type 'string'."
}
```

### Test Suite Execution Output
Transform:
```text
[Raw 200 lines of Jest / Pytest runner output...]
```
Into:
```json
{
  "kind": "test_evidence",
  "runner": "jest",
  "command": "npx jest test/auth.test.ts",
  "exit_code": 0,
  "passed": 8,
  "failed": 0,
  "skipped": 0,
  "duration_ms": 340,
  "target_oracle": "should refresh expired JWT token"
}
```

### Stack Trace Distillation
Extract only the top 2 project-internal stack frames:
```text
Error: TokenExpiredError: jwt expired
  at verifyToken (src/auth/jwt.ts:28:11)
  at handleRequest (src/server.ts:89:5)
```

## 3. Cortex Observation Format
When calling `cortex_save`, provide concise observations under strict taxonomies:
- **Review & Evidence**: `topic_key: "review/{task_id}"` or `topic_key: "evidence/{task_id}"`
- **Closed-Loop Failure / Gotcha**: `type: "bugfix"`, `topic_key: "gotchas/{task_id}"`
  ```json
  {
    "failure_locality": "internal/auth/token.go:45",
    "root_cause": "Signature verification skipped on expired token",
    "remediation": "Enforce strict expiry check before returning claim map"
  }
  ```
- **Architectural Decision**: `type: "decision"`, `topic_key: "architecture/{module}"`
- **Summary**: Single-line description of the finding or outcome.
