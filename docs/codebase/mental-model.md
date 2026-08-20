# Mental Model

← [Codebase Guide](../CODEBASE-GUIDE.md)

How `cortex-ia` works end-to-end: a user executes an interactive TUI or CLI command, the transactional pipeline plans all required file and MCP mutations, captures a verified snapshot backup, applies atomic writes and 3-way JSONC merges to `~/.config/opencode/`, and commits state to `~/.cortex-ia/`.

---

## 1. End-to-End Execution Flow

```text
┌────────────────────────────────────────────────────────┐
│ Front End (TUI / CLI Dispatcher: internal/app)         │
└──────────────────────────┬─────────────────────────────┘
                           │ Request {Home, MCPSelection, Overwrite, DryRun}
                           ▼
┌────────────────────────────────────────────────────────┐
│ Installation Service (internal/install.Service)        │
│  - Acquires Home Mutation Lock (~/.cortex-ia/lock.json)│
└──────────────────────────┬─────────────────────────────┘
                           │
                           ▼
┌────────────────────────────────────────────────────────┐
│ Transactional Pipeline (internal/pipeline)             │
│                                                        │
│  Phase 1: Pure Planning (PlanInstall / PlanSync)       │
│    ├── Asset Map: layout.go ──▶ mappings               │
│    ├── Preimage Inspection: checksum disk state        │
│    ├── MCP Planning: mcpmanager qualification          │
│    └── Compute Deterministic Plan Digest (SHA-256)     │
│                                                        │
│  Phase 2: Snapshot & Verification (internal/backup)    │
│    ├── Capture pre-mutation byte copies of targets     │
│    └── Verify backup manifest integrity before writes  │
│                                                        │
│  Phase 3: Atomic Apply & Journaling                    │
│    ├── Write embedded assets via temp file + rename    │
│    ├── Merge opencode.jsonc (preserve user comments)   │
│    └── Update managed MCP presets                      │
│                                                        │
│  Phase 4: Commit & Verification                        │
│    ├── Write journal commit marker                     │
│    ├── Write Metadata V2 + Lock V2 (~/.cortex-ia/)     │
│    └── Return InstallReceipt to front end              │
└────────────────────────────────────────────────────────┘
```

---

## 2. Core Architectural Concepts

| Concept | Package | Responsibility |
| :--- | :--- | :--- |
| **Pure Planning** | `internal/pipeline` | Derives the exact set of effects (`create`, `managed-update`, `safe-merge`, `delete`, `mcp-add`, `mcp-remove`) without modifying the filesystem. |
| **Plan Digest** | `internal/pipeline` | Cryptographic SHA-256 commitment of all planned effects, preimages, and targets. Binds user confirmation to the exact execution set. |
| **Atomic File Merge** | `internal/components/filemerge` | Decodes JSONC, performs 3-way object merging with comments and formatting preserved, and writes via temporary atomic swap. |
| **MCP Management** | `internal/mcpmanager` | Manages local command vector registrations, checks identity accreditation, and isolates user-added MCP servers. |
| **Journaled Rollback** | `internal/backup` | In the event of an apply or verification error, rolls back all modified files to the exact preimage bytes captured in Phase 2. |

---

## 3. Platform Support Strategy

1. **OpenCode (Primary Active)**: Full native SDD asset set and MCP management under `~/.config/opencode/`.
2. **Google Antigravity (Planned)**: Native rules, skills, and sidecar integration under `~/.gemini/antigravity/`.
3. **Claude CLI (Planned)**: Native prompts and tool definitions under `~/.claude/`.

---

## 4. Key Invariants

1. **Read-Only Planning**: Calling `PlanInstall` or `PlanSync` never touches disk or writes state.
2. **Deterministic Digests**: Effect lists are sorted strictly by `effectOrder`, then `Kind`, then `Dest`.
3. **Fail-Closed on Unmanaged Files**: If an unmanaged file collides with a destination, planning reports a `ConflictUnmanagedExisting` blocker unless `--overwrite` is explicitly authorized.
4. **Never Block on Drift**: The service never appropriates or overwrites drifted user files without consent.

---

← Prev: (start) · Next: [Repository Map](repository-map.md) →
