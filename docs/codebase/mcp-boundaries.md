# MCP and Local Control Boundaries

← [Codebase Guide](../CODEBASE-GUIDE.md)

Cortex-IA cleanly separates the **Epistemic & Evidence Plane** (Cortex MCP) from the **Operational Control Plane** (`cortex-ia work` CLI).

| Surface | Runtime | Owns | Must NOT Own |
|---|---|---|---|
| **Cortex MCP** | `cortex mcp --tools=agent` | AST symbol graph, blast radius impact trees, durable observations, gotchas, session context | Task readiness, claims, file leases, DAG transitions, approvals |
| **Context7 MCP** | optional `npx -y @upstash/context7-mcp` | Read-only library & framework documentation | Persistence, execution authority, task state |
| **Cortex-IA Work Control** | Native Go CLI + SQLite WAL | Task DAG, CAS revisions, TTL claims, exclusive file leases, recovery, review approvals | Long-form evidence or raw agent transcripts |
| **Cortex-IA Task Board** | Embedded HTTP UI (`cortex-ia web`) | Board grouping, real-time SSE Kanban visualization, task intake | Authorization grants, remote exposure |
| **OpenSpec SDD** | Repository Markdown (`openspec/`) | Human-readable proposals, delta requirements, designs, task decompositions | Runtime locks, process supervision |

---

## 1. The Managed MCP Catalog (`internal/mcpmanager/presets.go`)

Cortex-IA manages only a closed catalog of presets inside OpenCode's native `mcp` object:

| Preset | Kind | Managed template |
|---|---|---|
| `cortex` | local | `cortex mcp --tools=agent` |
| `context7` | local | `npx -y @upstash/context7-mcp@4.1.0` |

- `Presets()` returns managed entries sorted by name as deep copies, so callers can never mutate the catalog in place.
- `DefaultSelection()` selects `cortex` and leaves `context7` opt-in. Task coordination is built into the `cortex-ia work` CLI and is never an MCP preset.
- `RetiredPresets()` are removal-only legacy templates (`forgespec`); they are never selectable or addable and exist so `sync` can safely clean up older installations.
- Catalog names are reserved: a custom local or remote entry cannot reuse them, and a preset entry must use its exact catalog name (no aliasing).

## 2. Ownership Is Accredited by Transactional Metadata (`internal/mcpmanager/manager.go`)

Ownership of one `mcp` entry is proven exclusively by an `OwnershipRecord` that the installer pipeline persisted after a successful apply, bound to the server name, the semantic digest, and the resolved config path. Name matching and template equality are never ownership evidence.

| Status | Meaning |
|---|---|
| `absent` | No entry exists under the managed name |
| `managed` | Entry equals the preset **and** a matching ownership record accredits it |
| `unmanaged-equivalent` | Entry equals a preset but no record accredits it; user-owned, never rewritten or implicitly removed |
| `conflict` | Entry exists under a managed name with different content; fail closed |

The second evidence stage is the mcpv2 full-postimage fingerprint (`mcpv2:<64 hex>`, HMAC-SHA256 keyed by a local salt). It covers type, argv/URL, env/header names **and keyed hashes of their values**, enabled state, and config path, so drift on any mutable field is detected while state never stores a clear secret.

## 3. Typed Fail-Closed Conflicts

`ConflictError` carries a typed `ConflictKind`; the manager guarantees the config file is never written when one is returned:

- `managed-name-modified` — entry differs from the managed template (user-modified or foreign).
- `unaccredited-entry` — entry equals the preset but no record accredits it; never appropriated.
- `unmanaged-name` — name is outside the managed catalog.
- `absent-entry` — an adopt target names a managed preset with no live entry.
- `malformed-config` — the `mcp` object is not a JSON object or the document cannot be parsed unambiguously.
- `postimage-drift` — an accredited custom entry drifted after accreditation; the user must inspect it before retrying.
- `legacy-ownership` — only legacy mcpv1 evidence exists (or the local salt is missing); the remedy is to re-run `mcp add`, which re-accredits the full postimage, then remove.

## 4. Qualification Boundary (`internal/mcpmanager/qualification.go`)

Success is never inferred from configuration alone. `Add` reports `Installed = Configured AND Qualified`, and supplying no probe fails closed (`Qualified=false`).

- **`LocalCommandProbe`** resolves the command binary with `exec.LookPath`. It is deterministic and offline: it proves the configured command can start, not that a remote service is reachable.
- **`RemoteURLProbe`** validates a well-formed `http(s)` URL with a host. It never tests reachability and never embeds the URL in evidence, because URLs may carry credentials.
- **`DefaultContext7QualificationEvidence`** freezes the qualified Context7 `4.1.0` contract (package name, version, executable path, lockfile integrity, and required tool schema).
- Probes must be deterministic and offline by contract; a probe error, a rejecting probe, or evidence naming another server never reports success.

## 5. Typed Desired Contract (`internal/mcpmanager/desired.go`)

`Desired` is pure, validated data describing exactly one server. Validation runs before any filesystem access:

- `preset`, `local`, and `remote` kinds are mutually exclusive: exactly one of `Preset`, `Command`, or `URL` may carry a value.
- Local servers keep the exact argv vector verbatim; nothing is ever joined into a shell command.
- Remote URLs must parse as `http` or `https` with a host.
- Env and header `KEY=VALUE` assignments apply to exactly one kind; values reach the configuration file only and are never representable in digests or ownership records.

## 6. Config Surface Selection (`internal/mcpmanager/manager.go`)

- The manager mutates `~/.config/opencode/opencode.jsonc` when present, otherwise `opencode.json`, following OpenCode's global load precedence.
- Writes target OpenCode v2's `mcp.servers.<name>` shape, falling back to legacy `mcp.<name>` only for configurations already using it.
- The merge boundary (`internal/components/filemerge`) preserves unrelated keys, unknown MCP entries, and JSONC comments.
- Listings expose a sanitized identity projection only: server name, entry type, variable names, and the secret-free digest. Values, argv vectors, and URLs are never surfaced.

---

## Invariant Rules

1. **`cortex-ia work status` is strictly authoritative**: Task readiness and active authority are read solely from SQLite. Browser card positions or chat messages never substitute for `work status`.
2. **Cortex observations are advisory**: Knowledge graph nodes or stored gotchas provide context but never grant permission to touch files or bypass review gates.
3. **Tokens stay in memory**: `claim_token` and `lease_token` reside in live memory only. SQLite stores only their SHA-256 digests.
4. **No external execution leaf**: Every role controller executes natively inside OpenCode; no external CLI receives work-control tokens, approval, session, or MCP authority.

---

## See Also

- [Mental Model](mental-model.md) — where MCP planning sits in the transactional install pipeline
- [Key Interfaces](interfaces.md) — `ServiceAPI`, plan, effect, and conflict contracts
- [`mcp.md`](../mcp.md) — operator-facing `mcp add/list/remove` reference
- [`mcp-qualification.md`](../mcp-qualification.md) — qualification evidence per integration
- [`qualification-inputs.md`](../qualification-inputs.md) — bounded qualification input schema

← Prev: [Repository Map](repository-map.md) · Next: [Key Interfaces](interfaces.md) →
