# Recovery and Reflection

Retry ceilings are executable policy: transient failures may retry three times; semantic retries may retry twice with reflection; no-progress apply/verify cycles may occur twice. A ceiling halts dispatch.

Each semantic retry records prior evidence, failure class, root-cause reflection, next hypothesis, and its counter. Three specification violations trigger fresh design/decomposition; excessive task failure or repeated no-progress cycles require a human decision.

After restart or compaction, reconcile ForgeSpec status and revisions with Cortex evidence and apply progress. Keep terminal tasks terminal, resume from the last incomplete checkpoint, and never fabricate missing evidence.
