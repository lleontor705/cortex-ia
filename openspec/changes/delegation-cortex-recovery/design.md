# Recovery design

## Evidence and boundaries
Cortex14–21 and current file reads agree: main ref is 47bec1607b06447b622553ef77491f762956cf64; contracts ref is 76baed68ad2880d72c7d4f4c14e165e2615ac7e0; hardening ref is 47bec1607b06447b622553ef77491f762956cf64. Registered worktrees and Herdr's changed probe were read. Clean contracts status and one-line hardening diff are historical #19/#15 evidence, not fresh Git status. Planner cannot run Git commands with its present tools. Recheck before execution.

`decompose.go:50,83` inherits root workspace. `runner.go:645–688` requires clean external worktree, common Git directory and identical starting HEAD. No runtime Go change is proposed. Existing AST Store coupling is outside scope; plugin callers/imports remain unchanged.

## Selected interfaces
`cortex-convention.md` owns specification representation and pin validation; callers link to it. Pin = transport/project identity + real observation_id + SHA-256(exact UTF-8 content), retaining exact content. Full fetch precedes validation; previews are insufficient. Optional history is queried only when exposed/useful and may return []. New contract content means new snapshot/pin and fresh acceptance, never an invented revision or a mutable latest pointer. SQLite retains historical approvals; stale acceptance cannot cover a new delivery.

Planner producer -> pinned contract -> implement consumer -> independent reviewer -> SQLite approval. OpenSpec/hybrid keep current gates. Reviewer has separate Spec/Standards axes. `cortex-work-protocol.md` alone defines bounded authorized bootstrap; callers point there. Separate create from planner-only decompose permission; do not widen role frontmatter.

## Workspace decision
A: Move main to checkpoint or weaken same-HEAD: rejected; broad authority blast radius and hides the precondition.
B: Rebind inherited task workspace by ad hoc DB/CWD changes: rejected; authority seam bypass, poor reversibility.
C (selected): Keep task authority rooted at the original path, create the authorized final branch there at unchanged original HEAD, then stage each task through a fresh clean same-HEAD external worktree and migrate only validated allowlisted output back under that task's root reservations. Small interface, local Git/process seam, no dependency reversal or runtime change. The main ref is never moved; switching the root checkout to a new feature branch is not merging into main or changing main to satisfy the gate.

### Per-task execution protocol
1. Re-read task/status/workspace, current branch/HEAD, `git status --porcelain=v1`, `git worktree list --porcelain`, original refs and all pre-existing diffs. First child may create `fix/delegation-cortex-recovery` at 47bec1607b06447b622553ef77491f762956cf64 in the root checkout; never edit product while root is on main. If branch exists, verify lineage/content instead of resetting it. Preserve discovery and these planning files.
2. Acquire fresh claim and sorted root-relative reservations for every writable file. Revalidate authority immediately before every migration/edit/commit. No inherited checkpoint claim is usable.
3. For an external leaf, provision a new clean related worktree at the CURRENT root HEAD (not necessarily original47bec16 after reviewed commits). Invoke the role gate once, with isolated_worktree explicitly. Do not import checkpoint changes before preacceptance. Supply original partial files and predecessor contracts as read-only evidence; the accepted leaf imports/completes only this task's allowlist after acceptance. Do not cherry-pick all six partial files.
4. Native preacceptance with no accepted job permits native work only in the authoritative root checkout on the feature branch with valid leases. Do not manufacture a failed gate to select native. Native is not itself a stop condition. Accepted external failure requires reconciliation and fresh authority, never same-attempt native fallback.
5. After accepted success, independently verify the allowlisted diff and unchanged external HEAD. Migrate that exact diff to the root feature branch only if its recorded baseline and reservations still match. This is controlled integration of accepted output, not parallel reimplementation. Re-run checks on the root result. A mismatch stops migration. Commit only scoped task files after verification; retain originals/checkpoint and do not reset/rebase/stash/delete them. Subsequent child worktrees start at the new current root HEAD.
6. Independent review approves each child before its dependent runs. Intermediate contract expansion is not installable: canonical producer, then reviewer consumption, then routing coherence. Root branch and source checkpoints remain local.

## Task boundaries
1. `critical-review-cortex-pin-planning`: 110 changed lines, shared pin plus planner production path; includes first feature-branch preparation.
2. `critical-review-cortex-independent-review`: 65 lines, reviewer consumption and drift rejection; depends on 1.
3. `critical-review-cortex-routing-bootstrap`: 125 lines, entrypoint/authority routing coherence; depends on 2. Sequential decomposition is mandated by the store and each phase consumes the preceding contract. No competing designs.
4. `critical-review-recovery-integrate`: at most 300 lines NEW in this node (already committed child deltas not reapplied), depends on 3 and existing Herdr approval. Migrate only Herdr's reviewed two-line diff, preserve reviewed child hashes, commit the six unchanged validated planning artifacts already in root, and verify one feature branch. No redesign of contract assets.
5. `critical-review-recovery-build-readiness`: zero product-source lines; build `bin/cortex-ia`, run existing gates and enumerate local installation/E2E effects. No live installation in this task.

Herdr review may run now, in parallel with child1; it writes no product files and does not depend on contract approval.

## Verification and rollout
Every task's durable definition supplies exact commands and full writable files. Textual contract probes are necessary checks, not semantic proof: independent review walks all spec scenarios and routing phases. Digest smoke uses a full real retrieval export, matching digest exit0 and changed disposable copy nonzero. Keep smokes ephemeral, no new out-of-policy tests. Existing Go tests may run unchanged.

After integration, conforming Go1.26.1, vet/lint/tests and build must pass. Readiness must emit an exact per-file installer plan, ownership/drift result, verified-backup procedure, binary destination, reload mechanism, bounded real E2E invocation and token-free reconciliation oracle. Actual generated installation paths and live reload readiness are not yet accredited; do not invent them or grant wildcard home writes. Planner then materializes two same-board operational nodes: local-install (depends build/readiness), real-E2E (depends independently approved install). This is an evidence prerequisite, not a new user preference question. Local install uses the supported service transaction and no force-overwrite; real E2E reports actual mode and does not count probe/syntax as transport success. No remote report/push/PR. Rollback uses the verified installation backup under scoped authority, never Git reset/clean.
