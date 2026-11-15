---
name: context-distiller
description: Condense verbose shell output, compiler diagnostics, and test traces into compact, structured evidence for Cortex and receipts. Use when raw logs or traces would flood context.
license: MIT
metadata:
  author: OpenCode Engine
  version: "1.0.0"
---

# Context Distiller

Use this skill when raw command output would crowd out the reasoning around it. Keep what a reviewer needs to reproduce and judge the result, and leave the rest in the log file or re-run the command.

## What to keep

- **Failure locality**: file, line, column, error code, and the exact message.
- **Reproduction**: the minimal command, exit code, duration, and passed/failed/skipped counts.
- **Stack frames**: the top two or three project-internal frames. Drop third-party frames (`node_modules/`, `site-packages/`, standard library) unless they are the defect.

Bound any snippet to about 25 lines. Full logs do not belong in observations or receipts.

## Shapes

Compiler or linter error:

```xml
<failure_trace>
  <target_file>src/services/auth.ts</target_file>
  <location>line 42</location>
  <error_code>TS2345</error_code>
  <minimal_repro>npx tsc --noEmit</minimal_repro>
  <diagnostic_snippet>Argument of type 'string | null' is not assignable to parameter of type 'string'.</diagnostic_snippet>
</failure_trace>
```

Test run:

```json
{"kind": "test_evidence", "runner": "jest", "command": "npx jest test/auth.test.ts", "exit_code": 0, "passed": 8, "failed": 0, "skipped": 0, "duration_ms": 340, "target_oracle": "should refresh expired JWT token"}
```

## Saving to Cortex

Save distilled findings with `cortex_save`, using the types and topic-key families in the table of `cortex-convention.md`. A gotcha is a `bugfix` under `gotchas/<issue>`, with `failure_locality`, `root_cause`, and `remediation`. Add a one-line summary to every observation.

Never store secrets, authority tokens, or raw logs.
