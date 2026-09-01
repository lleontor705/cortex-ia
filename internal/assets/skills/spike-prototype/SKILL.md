---
name: spike-prototype
description: Run a disposable time-bounded technical, logic/state, or UI experiment to answer one named uncertainty without producing production code.
license: MIT
metadata:
  author: lleontor705
  version: "2.0.0"
---

# Technical spike strategy

Use only with an explicit scratch scope and permission to create and execute disposable artifacts. Never write to production paths, promote prototype code directly, or call `cortex_session_start`/`cortex_session_end` (session lifecycle is owned exclusively by the orchestrator).

1. State one falsifiable question, success/failure thresholds, budget, environment, and cleanup plan before experimentation. Select exactly one mode:
   - `technical`: a harness, benchmark, compatibility probe, or protocol experiment;
   - `logic`: a portable pure reducer/state machine/function surface plus happy, edge, and illegal-action scenarios that expose full relevant state;
   - `ui`: two or three structurally different, read-only variants using representative data; variants must differ in layout or information hierarchy, not only styling.
2. Create the minimum isolated harness in the granted scratch directory. Record dependency versions and assumptions. Keep state in memory unless persistence is the question; never point a prototype at production mutations.
3. Run reproducible probes or benchmarks. Include warm-up/sample method when performance is claimed; report variance and limitations rather than false precision.
4. Record which question the prototype answered and the chosen conclusion. Remove disposable artifacts unless the user requested retention; retained prototypes remain explicitly marked and cannot be promoted directly to production. Verify that production paths remain unchanged.
5. Save only durable, sanitized findings in Cortex: question, method, measurements, revision, timestamp, limitations, and recommendation. Never store credentials, raw logs, or work-control authority tokens.
6. End with an organic decision: `stop`, `direct-change`, `fast-tdd`, `sdd-lite`, or `sdd-full`. A valid spike may recommend doing nothing.

```json
{
  "workflow": "spike",
  "prototype_mode": "technical | logic | ui",
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
