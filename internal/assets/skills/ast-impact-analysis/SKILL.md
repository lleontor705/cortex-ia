---
name: ast-impact-analysis
description: Map code dependencies, AST call-graphs, and impacted test suites to execute targeted oracles during Fast-TDD and direct changes.
license: MIT
metadata:
  author: OpenCode Engine
  version: "1.0.0"
---

# AST Impact Analysis & Test Selection Engine

Use this skill to identify the exact downstream blast radius, affected call chains, and minimal impacted test suites before running costly test suites or making interface modifications.

## 1. Native Cortex AST Intelligence (Primary)

Always prefer Cortex Zero-CGO AST tools over manual regex grep loops:

1. **Calculate Blast Radius**:
   - MCP: `cortex_get_blast_radius(symbol_id | file_path)`
   - CLI: `cortex code blast-radius <symbol_or_path> --project=<project>`
   - Computes all direct and indirect downstream callers affected by a symbol modification.
2. **Inspect Call Graphs & Hierarchies**:
   - MCP: `cortex_get_code_graph(project)` / `cortex_get_code_symbols(project, kind, file_path)`
   - CLI: `cortex code graph --project=<project>`
3. **Detect Dependency Cycles**:
   - MCP: `cortex_detect_cycles(project)`
   - CLI: `cortex code cycles --project=<project>`

## 2. Discovery Principles & Targeted Execution
- **Baseline Blast Radius**: Before editing an interface or function, query `cortex_get_blast_radius` to capture the baseline caller count.
- **Targeted Execution:** Never run global test suites (`npm test`, `pytest`, `cargo test`, `go test ./...`) during iterative code-fix loops when the impacted blast radius can be isolated.
- **Coupling Spike Warning**: If an edit increases downstream dependents beyond expected scope, evaluate whether an abstraction is leaking.

## 3. Language-Specific Impact & Test Execution Strategies

### Go
- Query callers: `cortex_get_blast_radius("FunctionName")`
- Targeted Test: `go test -v -run ^TestFunctionName$ ./internal/pkg/...`
- Package Sweep: `go test -v ./internal/pkg/...`

### TypeScript / JavaScript (Node, Jest, Vitest, Mocha)
- Targeted Vitest: `npx vitest run [path_to_test] -t "[target_describe_or_it]"`
- Targeted Jest: `npx jest [path_to_test] -t "[target_describe_or_it]"`
- Targeted Mocha: `npx mocha [path_to_test] --grep "[pattern]"`

### Python (pytest, unittest)
- Targeted Pytest: `pytest [path_to_test.py] -k "[function_name]"`
- Fast fail flag: `pytest -x --tb=short [path_to_test.py]`

### Rust / C#
- Rust: `cargo test test_function_name`
- .NET: `dotnet test --filter FullyQualifiedName~TestClassName`

## 4. Protocol for Fast-TDD Minion
1. **Calculate Blast Radius**: Determine which symbol and test file govern the task.
2. **Execute RED:** Run only the target test oracle. Verify expected failure exit code ($\neq 0$).
3. **Execute GREEN:** Modify implementation file within bounded scope. Run only the target test. Verify exit code $0$.
4. **Impacted Sweep:** Run only tests in the immediate blast radius package.
5. **Full Suite Gate (Reviewer Only):** Leave the global regression suite to the `reviewer` role during the delta sync gate.
