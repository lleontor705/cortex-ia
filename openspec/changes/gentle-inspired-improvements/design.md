# Design: Gentle-inspired improvements

## Context and authority

This sdd-full/design contract covers the unchanged proposal and five specifications: 15 requirements / 45 scenarios. AUTO / HYBRID / current_workspace applies. User selections in cortex:213 and session summary cortex:220 resolve the earlier design-frontier statements; the conflicting handoff was not saved and is not evidence. No requirement change is needed to select the explicitly offered alternatives. No product/configuration edits, board/tasks, production keys or activation are authorized by this phase.

New interfaces, fields, filenames and test names below are **proposed**, not existing APIs or executed tests. Static source findings require isolated reproduction. Before tasks, obtain independent semantic review and bind exact reviewed artifact bytes. Any requirement weakening, new write authority, credential redesign or transaction/trust-model change returns for a decision before edits/pins.

## Selected alternatives

| Decision | Comparison: depth, locality, dependencies, blast radius and reversibility | Selection |
|---|---|---|
| Trust | TLS/unsigned checksums are simple but cannot authenticate publisher metadata. Pinned signatures put substantial verification behind a local updater boundary. Online key discovery introduces a new availability/trust service and activation scope. | User-selected pinned public-key signed manifests with explicit authorized rotation and repository/version binding. Use vetted Go Ed25519/SHA-256; never unsigned fallback. |
| Projection | Consumer-local rules have locality but duplicate authority interpretation. A shared read-only projection keeps that interpretation near SQLite. A writable coordination API increases coupling and bypass risk. | User-selected minimal immutable shared projection in `internal/delegation`, initially consumed through existing work-status CLI/bridge. No new store or mutation endpoint. |
| Install effects | Enlarging the pipeline transaction crosses OS/process seams and expands journaling/rollback. Separate effects preserve pipeline scope but require honest partial outcomes and recovery. | User-selected separately reported post-pipeline effects. Preserve transaction ownership/journal; never claim universal rollback. |

Prefer deep existing modules, not pass-through adapters. Substitute only filesystem, process, clock, HTTP and snapshot acquisition boundaries when tests need determinism; use real deterministic internal logic. Additive receipt/projection fields are reversible; authentication cannot be reverted to unsigned behavior. No prompt compiler, platform adapters, model routing or host-system replacement.

## Grounded seams

* `internal/assets/plugins/cortex-subagent-transport.ts:62-121` rejects empty implementation file scopes and parses step aliases; `:136-170` holds emergency/repetition state. `_shared/cortex-work-protocol.md:51,58-60,131-135` documents operational effects and advisory budgets. These are static mismatches, not reproduced failures.
* `cortex-task-latch.ts:44-75,88-133` has an in-memory latch cleared by disposal/session deletion; its incident report is not a SQLite task transition. `cortex-skill-discovery.ts:11-42,58-73` scans two repository roots and silently catches errors, not a complete host inventory.
* `internal/delegation/work.go:37-105,303-389` provides work/revision/claim/review data through multiple reads. `spec_contract.go:340-395,400-465` already implements stale-fingerprint rejection and review-refresh. `internal/app/work.go:187-199` returns the work item.
* `internal/updater/updater.go:64-90,151-223,240-335` currently checks GitHub, uses permissive version/asset matching, downloads/extracts and replaces. `internal/app/update.go:29-55` is the inspected caller. Authentication in this design is not yet implemented.
* `internal/install/service.go:185-341` runs `saveDelegation` after the pipeline and discards environment/TUI registration errors. `receipt.go:14-49` lacks separate effect records. `internal/delegation/config.go:209-240` compares bytes and atomically saves; it does not establish that arbitrary existing bytes are valid/owned.
* `internal/components/filemerge/writer.go:13-82` already returns Changed/Created, skips identical bytes, closes before rename and rejects unsafe parents. `internal/install/tui_plugin.go:32-45` already rejects malformed JSON/plugin shapes. Preserve these behaviors rather than claim to invent them.
* **Isolation hazard:** `internal/install/env.go:29-37` invokes PowerShell to change the Windows user environment independently of the supplied home. A temporary HOME alone does not isolate that effect.
* `internal/homelock/homelock_windows.go:33-59` uses CreateFile/LockFileEx; `internal/delegation/workspace_windows.go:16-28` opens for canonicalization. These are different boundaries, not a proven common race.

## Harness and context interfaces

### Operational admission and budget compatibility

Keep dispatch contract `1.0`, canonical tag and legacy resume compatibility. Native operational admission requires verified implement role, operational/database workflow, concrete authorized target/effects, valid task authority and `allowed_files: []`. Empty/ambiguous effects fail visibly. This declaration grants neither file leases nor host permission. Named mutation tools still reject repository writes; baseline comparison preserves unrelated pre-existing bytes. No shell sandbox claim.

External AGY still requires leased files (`herdr-bridge.ts` enforces this). Do not relax that policy or edit protected `runner.go` to make native operational admission work. External operational execution remains explicitly unavailable; return for a fresh orchestrator-authorized native dispatch, never automatic fallback.

New documentation uses `max_steps`; accept documented `budget.max_turns` as a deprecated alias for the same tool-step advisory count, not a second counter. Preserve args.steps/args.max_steps compatibility. Exactly one supplied form is valid: reject multiple forms even when equal, duplicate keys, malformed budget objects, invalid integers and role conflicts. Planner exemption follows verified host/resume identity, not an envelope claim. Preserve one current advisory notice, tools remaining available after it, non-planner emergency ceiling, five cleanup admissions, initialization rejection and repetition-only warnings. No automatic SQLite transition.

### Latch continuation

First reproduce failure, latch rejection, and disposal/new-instance behavior. Minimal supported continuation: orchestrator reconciles durable job/task state; if still latched, operator restarts the OpenCode plugin host; orchestrator restores the same durable initiative and dispatches a fresh authorized attempt. Restart clears plugin memory, not task authority. Do not delete a conversation as a workaround, invent an unlatch API, clear on an arbitrary success string or reuse tokens. Unknown identity/capability remains blocked. Automatic in-process continuation would require another design.

### Exact context and progressive disclosure

An authorized explicit locator precedes inventory fallback, never host policy. Expose resolved path, origin and precedence without logging content. Canonicalize path identity for the host OS; keep host-installed, repository-local and embedded-source inventories distinct. Reject unreadable/missing paths, invalid locators and same-precedence collisions; repository discovery supplies candidates, not permissions. Retain existing hooks and host instructions. `read` loads exact permitted contract paths; `skill` loads only names actually exposed by the host inventory. Asset copying remains with the existing asset map.

### Deterministic duplication metric

Before prompt edits, freeze revision, relevant dirty-path hashes and a sorted corpus manifest: `internal/assets/AGENTS.md`, the six role Markdown files, `skills/*/SKILL.md`, `_shared/*.md`, `commands/*.md`, and the transport's literal supplemental system text. Exclude generated assets and host/installed-user instructions; neither is edited or counted as savings.

New local checker `scripts/check-harness-contracts.mjs` normalizes CRLF to LF and whitespace within prose paragraphs/list items, ignores blank/code/pointer-only blocks, and counts exact repeated blocks of at least 20 whitespace-delimited words. For block b, length L and multiplicity n, excess is `(n-1)*L`; report `D=sum(excess)` and total eligible words W. This is a word metric, not tokens. New `scripts/harness-contracts.json` maps concepts to one canonical source and permitted pointers; independent review detects paraphrased conflicts missed by exact matching.

Acceptance: **0 broken pointers; 0 conflicting normative copies; 1 canonical definition per concept; 0 non-exempt repeated normative blocks**. Exempt only short summaries of five universal invariants: host precedence, role authority, live claims/leases, accepted-job reconciliation, no secrets. At most one summary per always-loaded role surface, **80 words maximum per surface**. No added exemption to force a pass. W must not increase over the frozen baseline; report exempt D separately. These defensible absolute thresholds implement single-source ownership and bounded safety context, not an invented percentage benefit. Baseline mode reports violations; final mode fails them. No added telemetry or savings claim.

## Immutable SQLite-derived projection

New `internal/delegation/work_projection.go` defines v1 projection logic; `work_projection_read.go` acquires a coherent token-free snapshot. Input: current work/dependency/claim-expiry/review facts, presentation role and separately labeled notes. Output: task ID, revision/as-of, current task status, notes, blockers, candidate actions with responsible role/conditions, and explicit authority availability. Phase status and verification verdict remain separate optional evidence values; never derive them from task status, pane state or memory.

Use one read transaction/query boundary for all needed facts, not repeated calls to the current multi-read GetWork. Share narrow query helpers if necessary without changing mutation transactions. No schema migration, projection database or persisted cache. Pure projection allocates its own data, never mutates inputs, returns copies from slice/map getters and emits no tokens. Serialized TS values are deeply frozen/copied and readonly. Tests mutate nested returned values and prove input and subsequent output unchanged.

Conservative role filtering: ready can suggest request_claim to implement; in_review can suggest independent review to reviewer; blocked/expired suggests orchestrator reconciliation; done/superseded suggests no implementation. Exclude self-approval. Missing owner/dependency/expiry evidence or unknown role suppresses execution candidates. Role is a presentation hint, never authentication. Every action states that current SQLite authority must be revalidated and required claims/leases acquired before acting. A snapshot has no freshness guarantee after return. Consumers discard older revisions and task/session-mismatched responses; retrieval failure replaces prior readiness with unknown. Stale memory remains a note and cannot create/clear an authoritative blocker.

Expose additive `projection` on existing work status CLI/bridge output; preserve existing fields/aliases. CLI may take a presentation role hint; bridge supplies its host role, not a model-selected privileged role. Unknown defaults to no actions. No HTTP write/recovery/approval endpoints, web/sidebar migration or new authority semantics.

## Authenticated updater

### Bootstrap and rotation

**Package an empty production trust set by default.** No reviewed production public key means `ErrUpdateTrustUnavailable`, rendered as “Authenticated update unavailable: no trusted release key is packaged”, including `update --check`. Do not print “already up to date”, fetch/install an archive, ship a test key or fake success. No environment/URL option supplies a root. This initiative proves the unavailable state and test-only authentication, not production activation.

Use Go `crypto/ed25519` and `crypto/sha256`, checking key/signature lengths before verification. Official Go source retrieved through Context7 confirms GenerateKey/Sign/Verify over exact message bytes. Generate temporary test keys with GenerateKey; never commit/embed them. Gentle's `internal/update/upgrade/download.go:49-88` offers bounded pinned-key precedent but uses go-minisign, absent from Cortex dependencies. Reuse the Go standard library instead of importing its policy or implementing cryptography.

New trust.go holds at most two compiled public-key records with key ID, repository and inclusive stable-version interval. Future explicitly authorized packaging adds a reviewed successor with bounded overlap, then retires the predecessor. Downloaded keys never authorize rotation. Unknown ID, malformed key, wrong repository or out-of-range version fails. Fixtures cover predecessor-only, authorized overlap/successor, unknown successor and retired keys. No production key custody/rotation is performed here.

### Versioned manifest and anti-replay policy

New `release-manifest.json` v1: schema_version, exact repository `lleontor705/cortex-ia`, exact stable tag, unique artifacts `{name, os, arch, size, sha256}`. `release-manifest.sig` v1 carries key ID and base64 Ed25519 signature over the exact manifest bytes. Reject duplicate JSON keys, trailing content, unknown schema, duplicate artifact identities, malformed hashes/encoding. Producer fixture serializes once; verifier never normalizes signed bytes. Existing checksums.txt remains an output, never an authentication source.

Automatic application accepts only canonical `vMAJOR.MINOR.PATCH`, without leading zeroes, prerelease/build suffixes or overflow. Do not use the permissive existing comparator for security. Apply requires known running version; candidate must exceed it and any successfully applied version in that client and satisfy key interval/floor. Equal/older apply is rejected; authenticated equal-version check can truthfully say no update. Unknown/dev running version cannot establish a downgrade floor and cannot auto-apply. This detects replay relative to trusted installed version, not suppression of a newer release, local administrator rollback of the entire binary or signing-key compromise; do not claim those stronger guarantees.

### Dataflow and safety

`runUpdate -> packaged trust -> bounded GitHub hint -> signed manifest verification -> repository/tag/OS/arch/exact-name/version binding -> bounded download -> actual stored byte count/SHA-256 -> full archive validation -> staged binary -> replacement -> final digest -> success`.

Bounds: manifest 1 MiB, signature envelope 16 KiB, archive and extracted binary each 128 MiB, 1,024 archive members. Reject limit+1, misleading lengths and truncation. Derive locations from fixed repository/tag; require HTTPS and an explicit GitHub-owned redirect-host allowlist, never arbitrary supplied origin. Loopback fixtures substitute the HTTP boundary; no production bypass flag. Preserve request contexts/timeouts.

Authentication precedes archive download; byte verification precedes extraction. Exact signed artifact matching replaces case-fold/substring fallback in the apply path. Validate **all** archive members, including those after the target: reject absolute/drive/UNC/backslash/NUL/traversal paths, links/devices, duplicate or case-colliding normalized paths and multiple binaries. Allow bounded regular documentation/scripts/directories produced by current GoReleaser; execute none and extract only the root cortex-ia/cortex-ia.exe.

Separate trust.go, manifest.go and archive.go hide complexity behind one updater apply boundary. ApplyUpdate requires currentVersion; update all actual callers. Mutable Release objects are untrusted; recheck identity at apply. No public constructor bypasses verification.

Verification errors leave the working binary unchanged. Stage in the destination directory; propagate write/close/rename/rollback failures. Preserve a working backup when restoration cannot be verified; never overwrite unrelated `.old` files. Windows replacement safety is tested separately from speculative REC-12 races. No reboot cleanup or guessed retry. Final digest is required before success. Fault tests prove original bytes retained or explicit recovery failure with surviving backup, not universal atomic reversibility.

Producer signing is a test-only fixture. Production signed release publication, root packaging and key custody remain externally blocked. Existing release gates remain mandatory.

## Separate post-pipeline effects

Add bounded v1 `post_pipeline_effects` records to InstallReceipt: kind, `not_requested|unchanged|changed|failed|not_attempted`, transaction_covered=false, safe error/recovery code. Preserve old fields; transaction ID/backup/restoration remain pipeline-only. Changed lists actual successful changes, not attempts. Proposed partial_success is true when verified earlier effects survive a later failure; a required failed effect returns an error and suppresses full-success rendering.

After confirmed pipeline success, hold the existing home lock and execute requested separate effects in current order. Account for environment and TUI registration already called by saveDelegation; propagate discarded errors and stop later effects on failure. This is effect accounting, not sidebar UI or cleanup expansion. Reuse WriteResult, not before-write guesses. Keep compatibility wrappers for existing writer callers where necessary.

Delegation save-with-result validates requested and existing config, rejects malformed content and rechecks preimage under lock. Preserve pipeline accreditation; never infer ownership from name/template equality. Separate configuration intent is not permission to overwrite unmanaged drift: absent established authorization/preimage, return conflict and require operator reconciliation. Any explicit separate confirmation binds an expected preimage digest and does not pretend the pipeline PlanDigest covers it. Do not introduce a new ownership database. Test creation, unchanged content, authorized changes, malformed bytes and stale/unmanaged preimages.

Recovery explicitly reinspects/replans/retries the failed effect with fresh confirmation; successful pipeline effects remain. Assert unchanged pipeline digest/journal/backup through recovery, final config bytes and no false second change. Pipeline rollback does not promise to undo post-effects. Dry-run invokes no writer/process/environment change and reports no applied/qualified effects.

**Before Windows install/full-suite tests**, introduce a narrow environment process substitute: temp home alone does not protect the real user registry from ConfigureEnvironment. Replace that process entirely in fixtures and assert no real invocation. TUI registration and existing cleanup are confined to isolated fixtures; no developer configuration or files. Do not redesign environment management or broaden cleanup.

## Reproducible CI / MCP / OS qualification

| Input | Observed fact | Design |
|---|---|---|
| Go | go.mod 1.26.1; discovery host 1.26.5 Windows/amd64; CI requests 1.26 | Select 1.26.1, GOTOOLCHAIN=local, record actual compiler/version. Host 1.26.5 is not exact-toolchain qualification. |
| Actions | ci.yml quality/security reusable workflows @main; release checkout v4, setup-go v5, lint action v8, upload-artifact v4, GoReleaser action v6; PR github-script v7; stale v9 | Resolve intended existing refs to verified upstream 40-hex commits, including reusable dependencies. Record repo/ref/SHA/source before edits. No invented hashes/major upgrades. |
| Tools | release lint v2.11.4; GoReleaser ~>v2; configuration schema 2 | Preserve lint 2.11.4; resolve a concrete verified compatible GoReleaser 2.x patch before task readiness. Exact patch/SHA evidence remains pending. |
| Targets | GoReleaser linux/darwin/windows x amd64/arm64, CGO=0 artifacts; existing Linux/amd64 CGO race gate | Preserve all six builds. Runtime outcome is separate for each target; absent runner stays unverified. Required OS checks are release prerequisites, not decorative matrix entries. |
| SDK | package/locks: plugin/SDK 1.18.18, OpenTUI 0.4.5, TS 5.9.3, tsup 8.5.1; transport comment says 1.18.29 | Qualify actual 1.18.18 first. Comment is not runtime evidence. No dependency upgrade or protected sidebar build assumed. |
| MCP | cortex mcp --tools=agent; unversioned npx Context7; LocalCommandProbe only resolves PATH | Keep offline probe semantics; command presence is not protocol compatibility. No active config/server change. |
| Context7 candidate | Official npm metadata: 4.1.0, Node >=20.18.1, MCP server/node dependencies 2.0.0; root SHA-512 `ngAkFwW3LsnRGpH3XTVrjDqm3QBT4ZRpLCnShI0cIfCG+ACt07TrkRZc3n7+qjkFTcM/xIDJcHUBK5bDXUa40w==` | Candidate, not qualified installation. Use a frozen transitive lock in isolated qualification; verify registry integrity. Root pin alone does not freeze ranged dependencies. |

New docs/qualification-inputs.md records verified action/tool/MCP identities, commands, source/dependency inputs and actual target outcomes. A local checker rejects mutable refs, toolchain mismatch and missing gate dependencies. Preserve existing unit/lint/Linux `CGO_ENABLED=1 go test -race -count=1 ./...` and production environment approval. No continue-on-error or skipped-check success. Qualification workflows are not run/published by this phase; no tag push or external environment settings.

MCP qualification: local initialize and tools/list only, at most two protocol requests, 10 seconds total and 1 MiB output. Versioned fixtures cover required schema mismatch, optional absent capabilities, malformed config and unmanaged ownership. Official Context7 docs confirm resolve-library-id/query-docs and their argument shapes; no real documentation calls in tests. Cortex discovery reports 2.0.0 but no executable digest: capture exact executable version/digest and exposed agent-tool schema before compatibility claims. Do not invent tools from old prose.

Pin Context7 4.1.0 only after this qualification. Actual resolved dependency inputs must match recorded locked inputs; otherwise report unqualified. If the existing offline ProbeFunc cannot carry this evidence, add bounded caller-supplied evidence, not network probes in install. Preserve config rejection/ownership. New transitive-lock fixtures are qualification inputs, not an assertion that npx root pin alone gives reproducible installation.

REC-11 is assessment only: .goreleaser.yaml:26-27 embeds defaults and internal/telemetry/report.go uses HMAC. Synthetic builds can demonstrate embedding/usage, not production privilege or compromise. No telemetry file changes, credential redesign, real secret access or activation. Any confirmed reduction requiring those returns for authorization, never claimed implemented.

REC-12 starts with a deterministic Windows contention/open/close/rename fixture localized to homelock_windows.go, workspace_windows.go or filemerge/writer.go. No reproduction means hypothesis/verification-only and no hardening edits. Preserve non-Windows guarantees. Windows race detector qualification is conditional on supported CGO compiler evidence, additional to—not replacing—the required Linux race gate.

## All-requirement traceability and exact oracles

Commands are **future** oracles, expected exit **0** after correct implementation. Missing tests/no-tests-to-run do not pass. Each runner asserts named cases executed. JS fixtures load actual TS plugins using the pinned TypeScript transpiler, replacing host/fs/process boundaries, not merely searching text. Tests stay outside embedded plugin trees. Critical persistent tests are explicitly user-approved; unrelated exploration remains ephemeral.

| REC / Requirement | Concrete implementation and verification seams | Exact oracle; happy / edge / failure coverage |
|---|---|---|
| 01 / REQ-HARNESS-001 | transport; lease guard preserved; new scripts/harness-operational.test.mjs | `node --test scripts/harness-operational.test.mjs`: native explicit target admitted; dirty baseline retained; missing effects/file mutation rejected; external unsupported visible. |
| 02 / REQ-HARNESS-002 | transport, shared work protocol; scripts/harness-budget.test.mjs | `node --test scripts/harness-budget.test.mjs`: aliases/advisory, planner resume/emergency/five cleanup/repetition, invalid/conflicting configuration; no state changes. |
| 03 / REQ-HARNESS-003 | latch, work protocol; scripts/harness-latch.test.mjs | `node --test scripts/harness-latch.test.mjs`: reconcile/restart/fresh attempt, missing capability, no stale tokens or automatic fallback. |
| 04 / REQ-CONTEXT-001 | skill discovery; scripts/harness-context.test.mjs | `node --test scripts/harness-context.test.mjs`: exact precedence, differing inventories, collision/unreadable rejection and host preservation. |
| 05 / REQ-CONTEXT-002 | assets AGENTS, six role/skill/command sources, shared writing contract; checker/manifest above | `node scripts/check-harness-contracts.mjs --mode check --baseline "$CORTEX_TEST_BASELINE"`: zero invalid pointers/conflicts/non-exempt duplicates, <=80-word exemptions, W nonincrease, simple route avoids SDD. |
| 06 / REQ-REVIEW-001 | delegation/spec_contract.go, implement/reviewer/workflow source; new delegation/review_freshness_test.go | `go test -count=1 -v ./internal/delegation -run '^TestReviewFreshness$'`: normalize/freeze, later file/contract change invalidates approval, refresh preserves history, no self-approval. Passing existing behavior is verification-only. |
| 07 / REQ-STATE-001 | new delegation/work_projection.go, work_projection_read.go and two modular tests; app/work.go, bridge; app/work_projection_test.go | `go test -count=1 -v ./internal/delegation ./internal/app -run '^TestWorkProjection'`: notes distinct, stale/unavailable suppression, coherent concurrent snapshot, role/self-review filtering, nested non-mutation, compatible output. |
| 08 / REQ-DISTRIBUTION-001 | .github/workflows/ci.yml, release.yml, pr-check.yml, stale.yml; .goreleaser.yaml, go.mod, qualification record/checker | `node scripts/check-qualification.mjs --mode check`; `go vet ./...`; `golangci-lint run ./...`; `go test -count=1 ./...`; Linux `CGO_ENABLED=1 go test -race -count=1 ./...`: immutable inputs, actual native outcomes, missing/mutable/failing gate rejected. |
| 09 / REQ-MCP-001 | mcpmanager/presets.go, qualification.go; new scripts/qualify-mcp.mjs and isolated lock/schema fixtures | `node scripts/qualify-mcp.mjs --offline --timeout-ms 10000 --max-output-bytes 1048576 --fixture-root "$CORTEX_TEST_FIXTURES"`: versioned compatible schema, optional absence, missing required/malformed/unmanaged rejection. |
| 10 / REQ-UPDATE-001 | updater.go, new trust.go/manifest.go/archive.go; app/update.go; new updater trust_test.go, manifest_test.go, archive_safety_test.go, authenticated_update_test.go, replacement_safety_test.go; app/update_auth_test.go | `go test -count=1 -v ./internal/updater ./internal/app -run '^Test(UpdateTrust|UpdateManifest|UpdateArchive|AuthenticatedUpdate|UpdateReplacement|UpdateCLIAuth)'`: fixture chain success, authorized/unknown rotation, replay, absent trust/signature/binding/digest/archive/replace failures; target/recovery truth preserved. |
| 11 / REQ-SECRET-001 | release/telemetry source read-only; new scripts/assess-release-inputs.mjs | `node scripts/assess-release-inputs.mjs --synthetic-only --output-root "$CORTEX_TEST_OUTPUT"`: synthetic embedding/usage, privilege unknown, real-secret check rejected; no network. |
| 12 / REQ-PLATFORM-001 | localized Windows source only after evidence; task-owned ephemeral homelock/windows_boundary_smoke_test.go or delegation/windows_boundary_smoke_test.go | Windows `go test -count=1 -v ./internal/homelock ./internal/delegation -run '^TestWindowsBoundarySmoke$'`: controlled contention, non-reproduction marked unconfirmed, unsafe interference fails without corruption. Missing fixture/runner is INCONCLUSIVE. |
| 13 / REQ-VERIFY-001 | root AGENTS.md, shared codebase-design/diagnosis-loop contracts, implement skill, docs/sdd-workflow.md; scripts/check-test-policy.mjs | `node scripts/check-test-policy.mjs`: critical exception consistent, existing coverage preserved, dedicated <=250-line tests, unsafe/oversized/weakened oracle rejected. |
| 14 / REQ-VERIFY-002 | new scripts/capture-local-baseline.mjs and local evidence format | `node scripts/capture-local-baseline.mjs --output "$CORTEX_TEST_BASELINE"`: revision/dirty scope/exact command/toolchain/OS/expected/actual/exit; missing evidence inconclusive; invented savings/collection rejected. Same discriminating command before/after. |
| 15 / REQ-INSTALL-001 | install/service.go, receipt.go, env.go, tui_plugin.go; delegation/config.go; existing filemerge writer reused; new install/post_pipeline_effects_test.go, effect_recovery_test.go, delegation/config_write_result_test.go | `go test -count=1 -v ./internal/install ./internal/delegation -run '^Test(PostPipelineEffects|EffectRecovery|ConfigWriteResult)$'`: changed/no-op/error, partial success, malformed/unmanaged/preimage rejection, pipeline preservation/retry, no real Windows environment call. |

Paths abbreviated in this table are under internal unless prefixed scripts/docs/.github. Each named test runner asserts case counts. Environment paths are set to unique temporary fixture/baseline/output directories and recorded; reject empty variables. PowerShell expands the same variables as `$env:NAME`. Go fixtures use t.TempDir. Full-suite execution waits for verified environment substitution. Existing tests >300 lines are not appended. Deeper exploratory probes remain task-owned and ephemeral; cleanup never touches pre-existing files. No API compatibility inference from version comments.

## Bounded work-unit candidates (no materialized DAG)

Every behavior travels with its regression oracle. Caps: <=250 TS/JS/Python or <=350 Go source, <=600 tests total, <=250 per modular test file; also forecast <=350 total changed lines per TS unit / <=500 per Go unit. Declarative pins are separately counted. Numbers below are planning ceilings, not measured churn. Split an oversized candidate by complete behavior before claiming; no code-golf or tiny all-types/all-tests layers.

| Unit candidate | Proposed scope/files | Source + tests forecast | Genuine prerequisite |
|---|---|---:|---|
| P1 baseline/policy | root AGENTS, named policy docs, capture-local-baseline.mjs/check-test-policy.mjs | 180 JS + 120 | None; baseline before each behavior edit |
| H1 operational | transport + operational test | 100 TS + 180 | P1 |
| H2 budgets | transport/work protocol + budget test | 120 TS + 220 | H1 shared-file serialization |
| H3 latch | latch/work protocol + latch test | 80 TS + 180 | H2 shared contract |
| C1 resolution | skill discovery + context test | 150 TS + 180 | P1 |
| C2 prompt batches | checker/manifest, six role files, AGENTS, skill/command batches | <=180 JS + <=120 per batch; <=250 Markdown changed lines per batch | P1; H3 before overlapping work-protocol changes; exact batches fixed at tasks phase |
| R1 freshness | review_freshness_test.go and implement/reviewer/workflow sources | 60 source + 220 | P1; serialize overlapping C2 files |
| S1 projection/status | new projection/read files, app/work.go, modular projection tests | 240 Go + 240 | P1 |
| S2 role bridge | herdr-bridge.ts + scripts/harness-projection.test.mjs | 100 TS + 200 | S1 output contract |
| I1 writer outcome | delegation/config.go + config_write_result_test.go | 150 Go + 220 | P1 |
| I2 isolated post-effects | install service/receipt/env/TUI registration + post_pipeline_effects_test.go | 240 Go + 240 | I1; isolation and regression land together |
| I3 recovery/rendering | service + actual app/TUI receipt consumers, effect_recovery_test.go | 180 Go + 220 | I2; bounded caller lookup fixes exact consumer list before task creation |
| U1 fail-closed bootstrap | updater.go/trust.go/app/update.go + trust/CLI tests | 200 Go + 240 | P1; disable unsigned application in this first slice |
| U2 manifest verification | manifest.go/updater.go + manifest/authentication tests | 230 Go + 240 | U1; no production activation |
| U3 archive/publication safety | archive.go/updater.go + modular archive/replacement tests | 240 Go + 240 | U2; production apply remains unavailable until complete chain is reviewed |
| Q1 immutable qualification | four workflows, GoReleaser config, record/checker | 180 JS + 120; declarative pins separate | P1; resolve real SHAs/tool patch first; I2 before full suite |
| Q2 MCP fixture qualification | presets/qualification, script/locked schema fixtures | <=150 Go, <=100 JS + 180 | P1; actual package/executable evidence |
| A1 synthetic assessment | assessment script; release/telemetry read-only | 100 JS + 120 | P1 |
| W1 Windows assessment | one localized ephemeral smoke; conditional source fix | <=150 Go + 220 | P1; Windows runner and causal locality |

Disjoint H1/C1/R1/S1/I1/U1/Q1/Q2/A1/W1 candidates may proceed independently after P1 only when actual file lists do not overlap. Shared protocol/updater/service files serialize slices; C2 must not race H/R edits. This table is neither task IDs nor executable dependencies. Tasks phase enumerates all exact paths, baseline evidence and version pins before work creation. Do not use blanket writable globs or empty implementation allowlists.

## Protected scope and final gates

Read-only authoritative inspection of cortex-ia-quality-20260910 confirmed source-render blocked r6, asset-sync backlog r1 and telemetry done r4. Exclude internal/tuiassets/cortex-ia-tui.tsx, internal/tuiassets/sidebar-smoke.tsx, internal/assets/tui/cortex-ia-tui.js, internal/delegation/runner.go, internal/delegation/delegation_telemetry_smoke_test.go and historical sidebar smoke outputs. No necessary source overlap is identified in selected seams. Install fixtures may copy embedded sidebar bytes only in temp homes, not rebuild/change them. If external operational support or transport changes require runner edits, stop and identify overlap for orchestrator reconciliation before writes. Changes to approved files require applicable review-refresh and independent review; old tasks are never mutated here. No sidebar UI or telemetry expansion.

Normalize scoped files before verification, freeze, then independently review the same bytes. Changed files/contracts invalidate acceptance; preserve approval history. Existing hook `gofmt -s -w .` is not permission to edit unleased paths: normalize only scoped source during a task. Preserve final vet/lint/unit and Linux CGO race gates after isolation is proven. Build with `go build -o "$CORTEX_TEST_OUTPUT/cortex-ia" ./cmd/cortex-ia`. No web source/generated assets or protected sidebar build is required. Independent security/authority review resolves disagreements through evidence, not consensus veto. Only SQLite-authorized reviewer PASS completes work.

**Semantic assessment:** all REC-01..15 map to concrete requirements, source seams and happy/edge/failure oracles. Existing writer, malformed-TUI, ownership/journal and review-refresh protections are preserved or verification-only when demonstrated. No requirement amendment is presently needed; the selected choices resolve the historic frontier. Assessments, candidate versions, omitted runners and signed fixtures are not production implementation/activation.

**Structural assessment:** establish separately with cortex_ia_openspec_validate(relative_directory=openspec/changes/gentle-inspired-improvements, workflow=sdd-full, phase=design). Structural PASS is not semantic or product PASS.

**Remaining gates before affected tasks/readiness:** independent semantic/security review; verified CI SHAs and exact GoReleaser patch; locked MCP/SDK compatibility evidence; exact C2/I3 file enumeration; safe environment substitution before install/full-suite tests; real OS runners and causal harness/Windows reproductions. Production key packaging/signing/publication, external activation and credential redesign remain explicitly unauthorized/external-blocked. None is marked completed.
