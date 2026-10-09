# gentle-ai Durable-Authority Adoption Backlog (SECONDARY)

**Status**: SECONDARY — advisory backlog, not a committed plan.
**Target**: the sibling `gentle-ai` AI-development workflow.
**Cortex counterpart**: `architecture/workflow-comparison-gentle-ai` (#625).
**Grounding audit**: `docs/audits/2026-10-09-workflow-improvements-audit.md` (this
README-style audit and observation #625 reference each other; neither is the sole
source).

## Purpose and scope

`gentle-ai`'s ODD/RDD workflow is deliberately lightweight: a single recoverable
feature doc, a deterministic `Working/Checking/Ready/NeedsDecision` state machine
whose transitions are binary-owned, and human-owned delivery. The 2026-10-09 audit
found its lightness is evidence-backed, not accidental. It also found that
`gentle-ai` has **no durable claim/lease/approval authority plane** — ODD assumes a
single-owner, human-gated flow.

This backlog records the authority concepts `gentle-ai` *could* borrow from
`cortex-ia` if it ever grows concurrent owners or needs crash-resumable locks. Every
entry is:

- **SECONDARY**: a candidate, never a mandate.
- **gentle-ai-targeted**: it proposes changes to an external repository.
- **free of cortex-ia product-code implications**: no entry requires a change to
  `cortex-ia`; if one ever did, it would be excluded here and tracked separately.
- **evidence-linked** to observation `architecture/workflow-comparison-gentle-ai`
  (#625) so staleness is detectable: if that observation is superseded or the
  referenced `gentle-ai` files change, the entry must be re-validated before reuse.

No secrets, tokens, or credentials appear in this document, and none may be added.
The mechanisms below are described conceptually; `claim_token`/`lease_token` values
are never persisted anywhere, in `cortex-ia` or in a proposal.

## Entry structure

Each entry names, in order:

- **Gap** — the missing capability in `gentle-ai`'s current workflow.
- **Borrow** — the `cortex-ia` mechanism that addresses the gap (conceptual).
- **Adoption sketch** — how `gentle-ai` could approximate it in its own idiom.
- **Evidence** — the observation and audit anchor for staleness detection.

---

## E1 — Durable task ownership (claim)

- **Gap**: ODD assumes a single owner per feature doc. There is no record of *which*
  agent or human currently holds the work, so two resumers can race on the same
  `odd/tasks/<name>.md` with no authoritative winner and no crash-resumable owner.
- **Borrow**: `cortex-ia`'s task **claim** — an atomic, single-holder ownership
  record with a time-to-live (TTL) and renewal, backed only by a hashed token that
  never appears in logs or files.
- **Adoption sketch**: add an optional `owner` line to the feature-doc header
  (`owner: <agent>@<started>`, plus a short expiry). On resume, the parent claims the
  doc before editing; a live unexpired owner makes a second resumer read-only until
  it expires or is explicitly released. Keep the token/identifier out of the doc
  body — store only the owner handle and expiry.
- **Evidence**: observation `architecture/workflow-comparison-gentle-ai` (#625),
  finding "no durable claim/lease/approval authority plane"; audit
  `docs/audits/2026-10-09-workflow-improvements-audit.md`, Verdict section.

## E2 — Per-file leases for concurrent writers

- **Gap**: The audit's non-atomic dual write between the feature doc and the Engram
  mirror is read back, but there is no *writer exclusion*: nothing prevents two
  agents from editing the doc and the mirror simultaneously and producing a torn
  pair.
- **Borrow**: `cortex-ia`'s **workspace-relative file lease** — each writable path
  is reserved individually before editing, acquired in deterministic sorted order,
  and released when the change lands.
- **Adoption sketch**: before a write wave, the parent reserves the concrete paths
  it will touch (`odd/tasks/<name>.md`, the `<feature>/tasks` mirror) by taking a
  lightweight sidecar lock (e.g. `<path>.lock` holding owner + expiry). Acquire both
  in sorted order, release both after read-back, and abort cleanly on the first
  conflict instead of half-writing.
- **Evidence**: observation `architecture/workflow-comparison-gentle-ai` (#625),
  finding on mirror reconciliation; audit finding "non-atomic dual writes must be
  read back".

## E3 — TTL expiry with heartbeat renewal

- **Gap**: The `Working/Checking/Ready/NeedsDecision` machine is deterministic and
  binary-owned, but nothing expires a stalled `Working` state, so an abandoned run
  can indefinitely block resumption by others.
- **Borrow**: `cortex-ia`'s **TTL authority with periodic renewal** — claims and
  leases carry an expiry; a live worker renews on a heartbeat, and expiry is the
  single, unambiguous recovery trigger.
- **Adoption sketch**: stamp each held doc/lock with an ISO expiry. A long-running
  run heartbeats the expiry forward; on resume, an expired stamp means the prior
  owner is assumed gone and the work may be reclaimed without a human vote. Keep the
  expiry conservative and the renewal explicit.
- **Evidence**: observation `architecture/workflow-comparison-gentle-ai` (#625),
  "Deterministic 4-state machine" finding; `cortex-ia` baseline
  `internal/assets/skills/_shared/cortex-work-protocol.md` (§4 LOC/TTL budget
  context).

## E4 — Revision-CAS state transitions

- **Gap**: Binary-owned transitions protect against model voting, but the doc itself
  has no optimistic-concurrency check, so two processes starting from the same read
  can both write a transition and silently clobber one another (a lost update).
- **Borrow**: `cortex-ia`'s **revision compare-and-swap (CAS)** — every transition
  carries the revision it observed and is rejected if the durable revision has moved
  on.
- **Adoption sketch**: add a monotonic `revision:` counter to the feature-doc header.
  A transition reads revision `N`, writes `N+1`, and is refused if the on-disk header
  is no longer `N`; the loser re-reads and re-applies. This turns the existing
  deterministic state machine into a concurrency-safe one without adding model votes.
- **Evidence**: observation `architecture/workflow-comparison-gentle-ai` (#625),
  "Deterministic 4-state machine" finding; `cortex-ia` baseline `internal/delegation`
  (revision-CAS transitions).

## E5 — Fingerprinted approvals bound to a frozen revision

- **Gap**: Delivery is human-owned and the candidate is frozen to lineage before
  review, but the approval is not cryptographically tied to the exact content that
  was reviewed, so an approval can outlive the artifact it blessed.
- **Borrow**: `cortex-ia`'s **fingerprinted, evidence-bound approval** — approval is
  recorded against the frozen revision and its evidence, so amending the artifact
  after approval invalidates it.
- **Adoption sketch**: when a candidate is frozen for review, record a content
  fingerprint (e.g. a hash of the doc plus the diff) in the review record. The
  approval references that fingerprint; any later edit changes the fingerprint and
  forces re-review. Store the fingerprint only — never secrets or tokens.
- **Evidence**: observation `architecture/workflow-comparison-gentle-ai` (#625),
  "RDD review" finding (candidate frozen to lineage; escalate-only tier); audit
  finding "candidate frozen to lineage before review".

## E6 — Completion gated on approved evidence

- **Gap**: A `Ready` state marks a unit as deliverable, but nothing structurally
  prevents "done" from being declared without an independent approval bound to
  executable evidence — narrative claims can stand in for proof.
- **Borrow**: `cortex-ia`'s rule that **only an approved verdict with evidence
  produces `done`**; tests alone, chat claims, or a viewer's position never complete
  a task.
- **Adoption sketch**: make the transition into a terminal delivered state require a
  recorded approval entry whose evidence references the RED/GREEN runs and their
  commit hashes. If the evidence is missing or the run disagrees with the worker's
  verdict, the executable result governs and the unit returns to `Checking`.
- **Evidence**: observation `architecture/workflow-comparison-gentle-ai` (#625),
  "Evidence culture" finding and the "executable result wins conflicts" adoption;
  audit Adoptions #4–5.

## E7 — Append-only operational ledger

- **Gap**: Activity lives in the feature doc's `## Log` and the Engram mirror; the
  Log is append-only by convention, but there is no durable, monotonic event stream
  that survives a lost or partially written doc.
- **Borrow**: `cortex-ia`'s **append-only operational event ledger** — transitions,
  claims, leases, and approvals are recorded as immutable events, independent of any
  single working file.
- **Adoption sketch**: mirror each Log entry and state transition into a simple
  append-only events file (one JSON line per event: timestamp, actor handle, state
  from→to, revision, evidence ref). Treat the working doc as a rendered view of that
  stream, and reconcile the doc from the ledger on resume rather than the reverse.
- **Evidence**: observation `architecture/workflow-comparison-gentle-ai` (#625),
  "Evidence culture" and ODD "`## Log`" findings; audit finding on mirror read-back.

---

## Out of scope (explicitly excluded)

- Any change to `cortex-ia` product code, skill contracts, or authority tooling.
  Every entry above is a proposal for `gentle-ai` only.
- Importing ODD wholesale into `cortex-ia` — the audit's Verdict section keeps
  `cortex-ia`'s authority plane as its structural advantage.
- Secrets, tokens, or credentials of any kind. Only conceptual mechanism
  descriptions and non-sensitive identifiers appear here.

## Staleness detection

`gentle-ai` evolves independently. Every entry anchors to observation
`architecture/workflow-comparison-gentle-ai` (#625) and the 2026-10-09 audit. If
that observation is superseded, or the referenced `gentle-ai` files
(`AGENTS.md`, `docs/usage.md`, `docs/trigger-rules.md`, `docs/engram.md`,
`odd/tasks/task-size-canon.md`) change materially, re-run the audit and re-validate
each entry before reuse. No entry here is authoritative on its own.
