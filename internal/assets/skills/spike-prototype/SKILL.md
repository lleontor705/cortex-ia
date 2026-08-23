---
name: spike-prototype
description: Run a disposable time-bounded experiment to reduce a named technical uncertainty without producing production code.
license: MIT
metadata:
  author: lleontor705
  version: "2.0.0"
---

# Technical spike strategy

Use only with an explicit scratch scope and permission to create and execute disposable artifacts. Never write to production paths, promote prototype code directly, or call `cortex_session_start`/`cortex_session_end` (session lifecycle is owned exclusively by the orchestrator).

1. State one falsifiable question, success/failure thresholds, budget, environment, and cleanup plan before experimentation.
2. Create the minimum isolated harness in the granted scratch directory. Record dependency versions and assumptions.
3. Run reproducible probes or benchmarks. Include warm-up/sample method when performance is claimed; report variance and limitations rather than false precision.
4. Remove disposable artifacts unless the user requested retention. Verify that production paths remain unchanged.
5. Save only durable, sanitized findings in Cortex: question, method, measurements, revision, timestamp, limitations, and recommendation. Never store credentials, raw logs, or ForgeSpec authority tokens.
6. End with an organic decision: `stop`, `direct-change`, `fast-tdd`, `sdd-lite`, or `sdd-full`. A valid spike may recommend doing nothing.

```json
{
  "workflow": "spike",
  "phase_status": "success | partial | failed | blocked",
  "question": "",
  "thresholds": [],
  "measurements": [],
  "limitations": [],
  "recommendation": "",
  "evidence_refs": [],
  "cleanup": {"scratch_removed": true, "production_unchanged": true},
  "risks": [],
  "next_route": "stop | direct-change | fast-tdd | sdd-lite | sdd-full"
}
```
