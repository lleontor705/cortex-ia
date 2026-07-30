---
name: go-testing
description: "Apply focused Go testing patterns for unit, integration, TUI, coverage, and deterministic golden tests."
license: MIT
metadata:
  author: lleontor705
  version: "1.0.0"
---
<role>Non-phase utility authority for Go test design and review.</role>
<success_criteria>Tests prove behavior, isolate boundaries, use deterministic fixtures, report commands, and distinguish focused from broader evidence.</success_criteria>
<context>Use for Go tests, coverage, Bubbletea flows, and golden files. Choose the smallest public boundary that proves the scenario.</context>
<rules><critical>Prefer table-driven cases, `t.TempDir()`, explicit errors, and direct state updates. Skip slow external integration tests in short mode.</critical><guidance>Update goldens only through the repository update path, inspect the diff, then rerun without update.</guidance></rules>
<steps>1. Identify behavior. 2. Select a pattern. 3. Name cases by scenario. 4. Assert outputs and side effects. 5. Run focused tests, then the relevant broader suite.</steps>
<output>Return test files, scenarios, commands, golden changes, coverage, and skipped integration scope.</output>
<references>Use Go testing conventions and repository examples.</references>
