# cortex-ia Skill Registry

Generated: 2026-06-24T00:00:00Z

## Skills

| Name | Source | Path | Trigger |
|---|---|---|---|
| cortex-ia-issue-creation | project | skills/issue-creation/SKILL.md | Creating a GitHub issue, reporting a bug, or requesting a feature. |
| cortex-ia-branch-pr | project | skills/branch-pr/SKILL.md | Creating a pull request, opening a PR, or preparing changes for review. |
| bootstrap | embedded | internal/assets/skills/bootstrap/SKILL.md | Starting a new SDD session or initializing project context. |
| investigate | embedded | internal/assets/skills/investigate/SKILL.md | Mapping unknown code areas, diagnosing bugs, or assessing migrations. |
| draft-proposal | embedded | internal/assets/skills/draft-proposal/SKILL.md | Drafting a change proposal from exploration analysis. |
| write-specs | embedded | internal/assets/skills/write-specs/SKILL.md | Producing Given/When/Then scenarios. |
| architect | embedded | internal/assets/skills/architect/SKILL.md | Designing implementation approach for an SDD change. |
| decompose | embedded | internal/assets/skills/decompose/SKILL.md | Breaking design into dependency-ordered tasks. |
| team-lead | embedded | internal/assets/skills/team-lead/SKILL.md | Coordinating apply-phase implementation work. |
| implement | embedded | internal/assets/skills/implement/SKILL.md | Applying specs to production code. |
| validate | embedded | internal/assets/skills/validate/SKILL.md | Verifying implementation against specs and design. |
| finalize | embedded | internal/assets/skills/finalize/SKILL.md | Archiving a completed SDD change. |
| debug | embedded | internal/assets/skills/debug/SKILL.md | Systematic root-cause debugging. |
| monitor | embedded | internal/assets/skills/monitor/SKILL.md | Generating SDD status dashboards. |
| ideate | embedded | internal/assets/skills/ideate/SKILL.md | Collaborative requirements ideation. |
| execute-plan | embedded | internal/assets/skills/execute-plan/SKILL.md | Executing a written implementation plan. |
| open-pr | embedded | internal/assets/skills/open-pr/SKILL.md | Creating pull requests with issue-first enforcement. |
| file-issue | embedded | internal/assets/skills/file-issue/SKILL.md | Creating GitHub issues with required templates. |
| parallel-dispatch | embedded | internal/assets/skills/parallel-dispatch/SKILL.md | Dispatching independent tasks to parallel agents. |
| scan-registry | embedded | internal/assets/skills/scan-registry/SKILL.md | Building or refreshing the unified skill registry. |
| debate | embedded | internal/assets/skills/debate/SKILL.md | Multi-position adversarial deliberation. |
| judgment-day | embedded | internal/assets/skills/judgment-day/SKILL.md | Cross-phase adversarial review before merge. |

## Convention Files

| File | Path |
|---|---|
| AGENTS.md | AGENTS.md |
| CONTRIBUTING.md | CONTRIBUTING.md |
| docs/sdd-workflow.md | docs/sdd-workflow.md |
| docs/cortex-memory.md | docs/cortex-memory.md |

## Project Context

- Runtime: Go 1.26.1, module `github.com/lleontor705/cortex-ia`.
- UI/tooling stack: Bubbletea v1.3.10, bubbles v1.0.0, lipgloss v1.1.0, yaml.v3.
- Architecture: cmd/cortex-ia entrypoint, internal/ packages (agents, agentbuilder, app, assets, backup, catalog, components, config, model, opencode, pipeline, state, system, tui, update, verify).
- Pipeline: 2-stage (Prepare -> Apply) with rollback via lockfile + manifest snapshots.
- Agents: 12 supported adapters (claude, opencode, gemini, cursor, vscode, codex, windsurf, antigravity, kilocode, kimi, kiro, qwen).
- Components: 14 granular injectors (context7, conventions, cortex, filemerge, forgespec, gga, mailbox, mcpinject, permissions, persona, sdd, skills, theme, uninstall).
- Test pattern: Go package tests with co-located `*_test.go`; strict TDD required (AGENTS.md).
- Test command: `go test -v ./...` (or `make test`).
- Coverage: `go test -v -coverprofile=coverage/coverage.out -covermode=atomic ./...`.
- Golden files: testdata/golden/, regenerate with `go test -update ./internal/components/...`.
- Pre-existing failure: TestGoldenCortex_Claude (internal/components) — golden files stale.
- Linter: golangci-lint v2 (errcheck, govet, staticcheck, unused, ineffassign). Command: `make lint`.
- Formatter: gofmt -s.
- CI: GitHub Actions (ci.yml, pr-check.yml, release.yml, stale.yml).
- Build: goreleaser (.goreleaser.yaml).
- Commit convention: Conventional Commits (type(scope)!: description).
- Branch convention: ^(feat|fix|chore|docs|style|refactor|perf|test|build|ci|revert)/[a-z0-9._-]+$.
- Workflow: Issue-first — no PR without an approved linked issue.
- Persistence mode: Cortex MCP (mem_save / mem_search / mem_get_observation).
