# AGY Execution Path Removal — spec delta

## REMOVED Requirements

### Requirement: REQ-AGY-001 — External AGY job execution engine
The system previously executed delegated role work by spawning the external Antigravity CLI (`agy`) through `RunWorker`/`runAGY` (internal/delegation/runner.go), including stream-JSON parsing, temporary-home environment sandboxing (`executionEnvironment`), workspace baseline capture and validation (`captureWorkspaceBaseline`, `validateWorkspaceChanges`), quota/rate-limit classification (`isQuotaOrRateLimit`, `ErrQuotaExceeded`), structured receipt validation (`validateStructuredReceipt`), process-tree management, and the external prompt builder (`externalPrompt`).

Reason: Final user doctrine decision (GAP-03): native OpenCode controllers are the only execution path and external AGY delegation is retired (cortex-work-protocol.md section 5). The engine is dead code — delegate and herdr are already in retiredCommands (internal/app/app.go:244-245) and RunWorker, runAGY, CreateFromRequest, and ReadRequest have zero production callers.

Migration: Remove the engine and its AGY-only tests; the durable SQLite work DAG, claims, leases, reviews, and approvals remain the sole execution authority. Herdr AGY references (ResolveAGY, antigravity-cli integration messaging, AGY status lines in internal/herdr/setup.go) are removed under this requirement; the fate of the remaining herdr surface is decision point DP-1.

### Requirement: REQ-AGY-002 — AGY install target and CLI detection
`install|sync|uninstall --target agy` installed and uninstalled the Antigravity plugin bundle (`internal/targets/agy.go`: `InstallAGY`, `UninstallAGY`, `TargetAGY`; dispatch cases at internal/app/cli.go:135,187,1008) and `internal/clidetect/detector.go` detected the `agy` CLI installation (`CLIAGY`, `DetectAGY`).

Reason: The target and detection exist solely to provision AGY supervision; doctrine removes AGY entirely, so the remaining install surface is opencode and claude only.

Migration: Remove TargetAGY, InstallAGY/UninstallAGY with their package file, the three CLI dispatch cases, and DetectAGY/CLIAGY with their tests. Existing user machines with a previously installed Antigravity plugin keep working untouched; the binary simply no longer manages that plugin.

### Requirement: REQ-AGY-003 — AGY CLI hook surface
`cortex-ia hook` processed Antigravity CLI hook payloads (`AGYHookPayload`/`AGYToolCall`/`AGYHookDecision` in `internal/app/hook.go`) to emit tool-permission decisions for the external CLI.

Reason: AGY-only transport glue with no native consumer; the native permission fence lives in internal/assets/plugins/cortex-permission-fence.ts.

Migration: Remove hook.go and the hook dispatch case (internal/app/app.go) and help references. If shared non-AGY hook handling is discovered inside the file, the implementer MUST stop and report blocked (decision point DP-3) instead of silently expanding scope.

### Requirement: REQ-AGY-004 — AGY configuration and model catalog
Delegation config (`internal/delegation/config.go`) defaulted every role to `CLI: "agy"` with `DefaultAGYModel`, `KnownAGYModels`, `NextModel`/`PrevModel`/`ModelDisplayName`, and the `CORTEX_AGY_SKIP_PERMISSIONS` gate (`skipPermissions`).

Reason: No AGY execution remains, so external model selection and external permission-skipping are meaningless surfaces.

Migration: Remove the AGY model catalog, model helpers, and role AGY defaults. Keep the Load/RoleConfig shape minimally consistent with the remaining read-only web display (delegation_config payload in internal/cortexiaweb/server.go) until decision point DP-4 resolves final field pruning.

### Requirement: REQ-AGY-005 — Delegation job write-path
Store methods `Create`, `Claim`, `MarkRunning`, `ExtendJobLease`, `CompleteWorker`, `MarkTerminationUnconfirmed`, and `Cancel` (internal/delegation/store.go, job_cancellation.go) existed solely to lifecycle AGY jobs in `delegation_jobs` (transient request documents, worker claims, receipt completion, cancellation handshakes).

Reason: Only AGY workers produced delegation jobs; engine removal orphans the write-path. Keeping dead write authority against append-only audit tables violates the fail-closed ownership doctrine.

Migration: Remove the write methods. delegation_jobs, delegation_receipts, and delegation_events remain legacy read-only audit history served to the web console (getDelegation) and dashboard views. No destructive purge, no table drop, and no row deletion without an explicit user-approved decision (DP-2), per the operational backup/rollback invariants.
