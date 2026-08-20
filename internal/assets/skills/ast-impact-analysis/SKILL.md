---
name: ast-impact-analysis
description: Map code dependencies, AST call-graphs, and impacted test suites to execute targeted oracles during Fast-TDD and direct changes.
license: MIT
metadata:
  author: OpenCode Engine
  version: "1.0.0"
---

# AST Impact Analysis & Test Selection Engine

Use this skill to identify the minimal set of affected tests and interfaces before running costly test suites. This accelerates Fast-TDD iterations and prevents token context bloat.

## 1. Discovery Principles
- **Targeted Execution:** Never run global test suites (`npm test`, `pytest`, `cargo test`) during iterative code-fix loops if the impacted subset can be isolated.
- **Dependency Mapping:** Trace file imports, class hierarchies, and symbol references backwards to find candidate test files.

## 2. Language-Specific Impact Strategies

### TypeScript / JavaScript (Node, Jest, Vitest, Mocha)
- Find direct imports of modified module:
  - `grep -rn "from.*[module_basename]" test/ tests/ src/`
  - Run only the target test suite:
    - Jest: `npx jest [path_to_test] -t "[target_describe_or_it]"`
    - Vitest: `npx vitest run [path_to_test]`
    - Mocha: `npx mocha [path_to_test] --grep "[pattern]"`

### Python (pytest, unittest)
- Identify referencing test modules:
  - `pytest [path_to_test.py] -k "[function_name]"`
  - Fast fail flag: `pytest -x --tb=short [path_to_test.py]`

### Go / Rust / C#
- Go: `go test -run ^TestFunctionName$ ./package/...`
- Rust: `cargo test test_function_name`
- .NET: `dotnet test --filter FullyQualifiedName~TestClassName`

## 3. Protocol for Fast-TDD Minion
1. **Locate or Create Oracle:** Identify the exact test file and test case governing the task.
2. **Execute RED:** Run only that target test. Verify expected failure exit code ($\neq 0$).
3. **Execute GREEN:** Modify target implementation file. Run only that target test. Verify exit code $0$.
4. **Impacted Sweep:** Run only the immediate sibling unit tests in the same package/module.
5. **Full Suite Gate (Reviewer Only):** Leave the full global regression suite to the final verification step or the `reviewer` role.
