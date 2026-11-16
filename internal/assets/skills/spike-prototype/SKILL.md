---
name: spike-prototype
description: Run a disposable time-bounded technical, logic/state, or UI experiment to answer one named uncertainty without producing production code. Use when one uncertainty must be answered by a throwaway experiment.
license: MIT
metadata:
  author: lleontor705
  version: "2.0.0"
---

# Technical Spike

A spike answers one named question with a throwaway experiment. Run it only in a scratch directory the user has granted. Keep prototypes out of production paths, so that nothing shipped changes and no prototype can be promoted by accident.

Investigate is read-only. A spike that needs no files runs under `investigate`. A spike that writes files runs as an `implement` task whose scratch paths are leased, so the writes stay tracked.

1. **Frame the question.** Write one falsifiable question, the success and failure thresholds, the time budget, the environment, and the cleanup plan. Pick one mode:
   - `technical`: a harness, benchmark, compatibility probe, or protocol experiment.
   - `logic`: a pure reducer, state machine, or function, tested with happy-path, edge, and illegal-action scenarios that show the full relevant state.
   - `ui`: two or three structurally different read-only variants on representative data. The variants must differ in layout or information hierarchy, not only in styling.
2. **Build the smallest harness.** Keep it in the scratch directory and record dependency versions and assumptions. Keep state in memory unless persistence is the question. Never point a prototype at production data.
3. **Measure.** Run reproducible probes. When a performance claim is made, state the warm-up and sampling method, and report variance and limits rather than a precise-looking number.
4. **Conclude.** Record which question was answered and what was concluded. Delete the disposable artifacts unless the user asked to keep them; a kept prototype is marked as such and cannot go to production directly. Confirm that production paths are unchanged.
5. **Save findings.** Store only durable, sanitized results in Cortex: question, method, measurements, limits, and recommendation. Leave out credentials, raw logs, and authority tokens.
6. **Choose the next route.** End with one of `stop`, `direct-change`, `fast-tdd`, or `plan`. A spike that shows nothing needs doing is a valid result.

## Report

Send the §8 completion receipt, plus the `prototype_mode`, `question`, `thresholds`, `measurements`, `limitations`, `recommendation`, and cleanup status (`scratch_removed`, `production_unchanged`). Set `next_route` per `cortex-work-protocol.md` §8.
