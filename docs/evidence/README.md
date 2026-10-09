# Evidence

In-repo evidence artifacts: qualification results, run receipts, and
reproduction logs that back an audit or a bounded change. Evidence here is the
reproducible raw material; the narrative interpretation lives in `docs/audits/`.

## Convention

- **Filename**: `<YYYY-MM-DD>-<topic>.md` with an ISO-8601 date prefix, or an
  unambiguous artifact name when the file is machine-generated.
- **Reproducibility**: record the exact command, environment, observed outcome,
  and (where applicable) commit hash so a later reader can rerun it.
- **Raw over narrative**: store observed output and exit status; keep analysis
  out of the evidence file.
- **Append-only**: do not rewrite past evidence; add a new dated artifact.

## Cross-references with Cortex observations

Evidence carries the same two-way reference rule as audits. Every evidence
artifact that has a durable Cortex counterpart must name its observation, and
that observation must point back to this file. **Neither store is the sole
source.** On disagreement, reconcile explicitly and name the authoritative copy.

- Artifact to Cortex: reference the observation's `topic_key` and numeric id.
- Cortex to artifact: record this file's workspace-relative path.

## Secrets guard

The secrets guard applies here without exception. Evidence frequently captures
command output, so scrub tokens, keys, credentials, connection strings, and
private repository paths before committing. Replace an apparent secret with a
typed placeholder (`[REDACTED:<kind>]`). Never rely on the file being private: it
is versioned.
