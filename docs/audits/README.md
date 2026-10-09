# Audits

Dated, in-repo audit reports. Each report records the scope, evidence, findings,
and verdict of one bounded audit or review so results stay discoverable without
replaying the originating conversation.

## Convention

- **Filename**: `<YYYY-MM-DD>-<topic>.md` with an ISO-8601 date prefix, e.g.
  `2026-10-09-workflow-improvements-audit.md`. The date is the day the audit
  concluded, not the day it started.
- **One audit per file**: keep reports atomic and lowercase, hyphenated topics.
  Never append an unrelated audit to an existing dated report.
- **Stable sections**: every report states scope, evidence, findings, verdict,
  and follow-ups so reports stay comparable across dates.
- **Append-only history**: an audit is a point-in-time record. Supersede it with
  a new dated file instead of rewriting an old one.

## Cross-references with Cortex observations

Every in-repo audit that has a durable Cortex counterpart must reference it, and
that Cortex observation must reference this file back. **Neither store is the
sole source.**

- Report to Cortex: name the observation by its stable `topic_key` and numeric id,
  e.g. `architecture/workflow-comparison-gentle-ai` (#625).
- Cortex to report: record this file's workspace-relative path in the observation
  body.
- On disagreement, reconcile explicitly and name the authoritative copy; never
  silently pick one.

Qualification and run-evidence artifacts referenced by an audit live under
`docs/evidence/` (see `docs/evidence/README.md`) rather than being inlined here.

## Secrets guard

The secrets guard applies to every in-repo report. Never write tokens, keys,
credentials, or private repository paths into an audit. Follow the redaction rule
from the evidence convention: replace an apparent secret with a typed placeholder
(`[REDACTED:<kind>]`) before persistence. This file is committed to Git history;
treat every report as public.

## Index

| Date | Report | Topic |
|---|---|---|
| 2026-10-09 | [2026-10-09-workflow-improvements-audit.md](2026-10-09-workflow-improvements-audit.md) | gentle-ai vs cortex-ia workflow grounding audit |
