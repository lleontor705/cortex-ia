# Contracts and Thresholds

The generated contract is authoritative. Minimum confidence is `bootstrap/investigate 0.5`, `propose/design 0.7`, `spec/tasks 0.8`, `apply 0.6`, and `verify/archive 0.9`. Below threshold, report and do not advance.

Every handoff needs executable evidence: command, exit code, content hash, test result, and Cortex or ForgeSpec references. Narrative claims are not proof.

Phase status uses `success`, `partial`, `failed`, or `blocked`. Verify verdict is separate: `pass`, `fail`, `blocked`, or `inconclusive`. Never collapse fields or emit aliases.

Unauthorized order is blocked: proposal precedes spec/design; both precede tasks; a ready task precedes apply; terminal apply precedes verify; verify pass precedes archive. An inconclusive result never becomes pass.
