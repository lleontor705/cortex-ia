# Key Interfaces & Contracts

← [Codebase Guide](../CODEBASE-GUIDE.md)

Reference index of core Go interfaces, types, and contracts in `cortex-ia`.

---

## 1. Service Layer (`internal/install`)

The service facade coordinates front-ends (CLI and TUI) with the pipeline and MCP catalog.

```go
type ServiceAPI interface {
    Plan(opts Options) (*pipeline.Plan, error)
    Install(opts Options) (*InstallReceipt, error)
    Sync(opts Options) (*InstallReceipt, error)
    Doctor() (*DoctorReport, error)
    Rollback(backupID string) (*RollbackReceipt, error)
    Uninstall(opts UninstallOptions) (*UninstallReceipt, error)
    MCPList() (*MCPListReport, error)
    MCPAdd(name string, opts MCPOptions) (*MCPReceipt, error)
    MCPRemove(name string, opts MCPOptions) (*MCPReceipt, error)
}
```

### Options and Receipts

- **`install.Options`**: Request configuration (`HomeDir`, `Version`, `DryRun`, `Overwrite`, `Cortex`, `ForgeSpec`, `Context7`, `ExpectedPlanDigest`, `LockTimeout`).
- **`install.InstallReceipt`**: Execution result (`Status`, `PlanDigest`, `BackupID`, `BackupVerified`, `ChangedCount`, `ConfiguredMCPs`, `Effects`, `Conflicts`).
- **`install.DoctorReport`**: System health (`Verdict`: Healthy/Degraded/Blocked, `ArtifactCounts`, `ManagedMCPs`, `Findings`).

---

## 2. Pipeline Layer (`internal/pipeline`)

Pure planning, verification, and transactional apply contracts.

### Core Data Models

```go
type Request struct {
    HomeDir            string
    Version            string
    DryRun             bool
    Overwrite          bool
    Cortex             bool
    ForgeSpec          bool
    Context7           bool
    ExpectedPlanDigest string
    Now                time.Time
}

type Plan struct {
    HomeDir          string
    OpencodeRoot     string
    Effects          []Effect
    Conflicts        []Conflict
    Converged        bool
    Digest           string
    MetadataPresence state.PresenceKind
    LockPresence     state.PresenceKind
    Metadata         state.MetadataV2
}

type Effect struct {
    Kind        EffectKind // create, noop, managed-update, overwrite, safe-merge, delete, mcp-add, mcp-remove
    Dest        string
    Source      string
    SourceSHA   string
    CurrentSHA  string
    PriorExists bool
    Reason      string
}

type Conflict struct {
    Target              string
    Kind                ConflictKind // unmanaged-existing, unmanaged-drift, malformed-config, mcp-conflict, stale-drift
    Reason              string
    OverwriteAuthorized bool
}
```

---

## 3. State & Lock Layer (`internal/state`)

Persistent state representations stored in `~/.cortex-ia/`.

- **`state.MetadataV2`**: Complete record of installed assets (`Path`, `Kind`, `Digest`, `Ownership`), managed MCPs (`Name`, `Kind`, `SemanticDigest`), tool version, and installation timestamp.
- **`state.LockV2`**: Concurrency and transactional guard with active lease timestamps and agreement hashes matching `MetadataV2`.
- **`state.CheckAgreementV2(meta, lock)`**: Verifies agreement between metadata and lock files, failing closed on discrepancy.

---

## 4. OpenCode Native Layout (`internal/agents/opencode`)

Declarative structure mapping embedded runtime assets to OpenCode native directory paths:

- **`opencode.NativeLayout`**: Defines directory destinations for configs, prompts, sub-agents, commands, skills, and plugins.
- **`opencode.MapAssets(fs, root)`**: Transforms embedded assets into concrete destinations with path-safety, case-insensitivity, and collision checking.

---

← Prev: [Repository Map](repository-map.md) · Next: [Mental Model](mental-model.md) →
