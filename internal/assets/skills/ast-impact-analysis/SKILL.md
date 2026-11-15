---
name: ast-impact-analysis
description: Map code dependencies, AST call-graphs, and impacted test suites to execute targeted oracles during Fast-TDD and direct changes. Use before costly test runs or interface changes.
license: MIT
metadata:
  author: OpenCode Engine
  version: "1.0.0"
---

# AST impact analysis

Use this skill to find the blast radius of a change and the smallest set of tests that cover it. Run it before a costly test run or before changing an interface.

## Code-intelligence tools

Prefer the Cortex AST tools to grep loops:

- **Definitions and callers**: `cortex_get_code_symbols(project, kind, file_path)`, then bounded reads of the callers. CLI equivalent: `cortex ingest`, `cortex symbols`.
- **Call graph**: `cortex_get_code_graph(project)`. CLI: `cortex code graph --project=<project>`.
- **Cycles**: `cortex_detect_cycles(project)`. CLI: `cortex code cycles --project=<project>`.

Do not pass code symbols to `cortex_get_blast_radius`. It takes a numeric observation ID and walks observation relations, not code.

## Procedure

1. Before editing an interface or function, record its definition, direct callers, imports, and the focused test packages.
2. Run only the target test first and confirm it fails for the expected reason (the RED step in Fast-TDD).
3. Make the change in the bounded scope. Run the target test and confirm it passes.
4. Run the tests in the immediate blast-radius packages.
5. Leave the full suite to the reviewer. Do not run `go test ./...`, `npm test`, `pytest`, or `cargo test` on the whole repository inside an iteration loop.

If an edit adds many dependents beyond the expected scope, check whether an abstraction is leaking before continuing.

## Targeted test commands

- Go: `go test -v -run '^TestName$' ./internal/pkg/...`; package sweep `go test -v ./internal/pkg/...`
- Vitest: `npx vitest run <path> -t "<name>"`
- Jest: `npx jest <path> -t "<name>"`
- Mocha: `npx mocha <path> --grep "<pattern>"`
- Pytest: `pytest <path> -k "<name>"`; fail fast with `pytest -x --tb=short <path>`
- Rust: `cargo test <name>`
- .NET: `dotnet test --filter FullyQualifiedName~<ClassName>`
