# Proposal: Gentle-inspired improvements

## Intent and evidence

Deliver all adopted recommendations through bounded, independently verifiable changes, not another comparison. Decision cortex:213 authorizes AUTO, HYBRID, current_workspace, mandatory authenticated updates, and small persistent critical-boundary tests. This phase writes only this proposal; no board or DAG.

Evidence: cortex:209–211 are static comparisons, not runtime proofs; cortex:215 rejects prior overclaims. `.cortex-ia/discovery.md` supplies module/toolchain context. Existing ownership, journaling, malformed-config rejection, release gates, SQLite authority, fingerprints and review-refresh remain baselines, not missing features. Windows concerns and runtime mismatches require isolated reproduction.

## Complete recommendation coverage

Disposition applies to proposed work, not implementation completion. References are Cortex observations unless marked contract or discovery.

| ID | Disposition | Scope and evidence |
|---|---|---|
| REC-01 | adaptation | Reconcile empty operational allowed_files with explicit effects; preserve unrelated baseline and reject unauthorized writes. 210–211. |
| REC-02 | adaptation | Align budget.max_turns/max_steps schema/runtime; preserve canonical advisory, emergency and repetition semantics. 211; work contract. |
| REC-03 | adaptation | Explicit latch continuation and role-safe recovery; no invented unlatch API. 210–211. |
| REC-04 | adaptation | Exact skill/artifact resolution, precedence, provenance, collision rejection and visible errors; distinguish host/embedded inventories. 210; discovery. |
| REC-05 | adaptation | Canonical prompt deduplication, measurable residual thresholds, permitted contract-loading mechanisms, proportional routing and cognitive documentation. No savings claims. 210; writing contract. |
| REC-06 | adaptation; proven-existing baseline | Normalize before verification/freeze/review; invalidate changed evidence while preserving fingerprints/history. Bounded units, no code-golf, risk-based blind review without consensus veto. 210; workflow/work contracts. |
| REC-07 | adaptation | Separate notes, blockers, admissible next action and stale memory; preserve phase/task/verdict distinctions and authoritative provenance. 211; work contract. |
| REC-08 | implementation | Immutable CI SHA pins, toolchain/manifest alignment, reproducible build provenance and effective OS gates; preserve existing unit/lint/race gates. 209; discovery. |
| REC-09 | adaptation | Versioned MCP integration and explicit compatibility evidence, without speculative schemas or unbounded probes. Discovery; approved inventory. |
| REC-10 | implementation; external-blocked | Mandatory fail-closed authentication: bootstrap, repository/version binding, rotation and artifact safety. Production signing/activation blocked. 209,213. |
| REC-11 | adaptation | Assess/reduce secret-derived binary risk without secret access or assumed credential redesign; privilege remains unverified. 211. |
| REC-12 | adaptation | Justify narrow Windows handle/open hardening with isolated race oracle; no speculative rewrite/retry policy. 209. |
| REC-13 | implementation | Reconcile source test-policy/docs for bounded persistent authority, transport, recovery and update-verification tests; isolated fixtures only. 209,213. |
| REC-14 | adaptation | Capture bounded deterministic baseline outcomes before changes; no benchmark claims or telemetry expansion. 211,213. |
| REC-15 | adaptation; proven-existing baseline | Examine post-pipeline delegation-config effect boundary and writer change/error semantics; preserve journaling, ownership and malformed-config rejection. 209,211. |

## Protected scope

Old board cortex-ia-quality-20260910: source-render blocked r6, asset-sync backlog r1, sidebar parent superseded r4; telemetry done r4. Exclude `internal/tuiassets/cortex-ia-tui.tsx`, its sidebar smoke, `internal/assets/tui/cortex-ia-tui.js`, `internal/delegation/runner.go` and its telemetry smoke from future writes until authoritative reconciliation/review-refresh handling. No old task mutation.

## Open design frontier and release tranches

Trust bootstrap/rotation mechanism remains open. Projection interfaces remain open: compare consumer-local projections against a shared read-only projection seam, balancing locality against consistency/coupling. Config boundary remains open: explicit separately reported effects versus expanded transaction coverage, balancing smaller scope against rollback coverage. Design compares depth, dependencies, blast radius and reversibility; no selection here.

Candidate tranches: baseline/policy (14,13); runtime/context (01–07,09); installation/platform (12,15); distribution (08,10,11). These are not sequential DAG dependencies. Each requires bounded units, deterministic oracles and independent review; activation remains separate.

## Non-goals and next gate

No extra agents, retired adapters/personas/model routing, Engram migration, SQLite/host-instruction replacement, prompt compiler, full parity or security weakening. No live configuration, secrets/keys, external settings, releases, deployment, cleanup or commits. External signing and compatibility activation require separate authorization. Next: specification, then design choices, then validated tasks. Static inspection never substitutes for runtime acceptance.
