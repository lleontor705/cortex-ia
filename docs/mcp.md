# MCP Manager

cortex-ia manages MCP server entries inside OpenCode's `mcp` configuration
object (`~/.config/opencode/opencode.jsonc` or `opencode.json`). Every
operation goes through the same ownership model: entries it configured are
tracked with identity digests, entries it cannot prove ownership of are
never modified, and every failure is fail-closed with nothing written.

The mutations (`mcp add`, `mcp remove`) accept `--dry-run` to plan and
report without writing. `mcp list` is read-only and takes a single flag:
`--json`.

## Catalog Presets

| Preset | Kind | Launches | Description |
|--------|------|----------|-------------|
| `cortex` | local | `cortex mcp --tools=agent` | Persistent memory with knowledge graph, FTS5, and revision history |
| `context7` | local | `npx -y @upstash/context7-mcp` | Live framework and library documentation |

All presets use OpenCode's native local-server shape —
`{"type":"local","command":["<argv>..."],"enabled":true}` — with the exact
argv listed above. No preset ships a URL.

`install` and `sync` configure `cortex` by default. `context7` is opt-in via
`mcp add`; task coordination comes from the built-in `cortex-ia work` CLI.

```bash
cortex-ia mcp add context7 --preset [--dry-run]
```

Catalog preset names (`cortex`, `context7`) are reserved: a
custom `--local` or `--remote` entry cannot take a catalog name, and a
preset entry must use its exact catalog name (no aliasing).

## Custom Local Servers

```bash
cortex-ia mcp add my-server --local [--env KEY=VALUE]... -- <command> [args...]
```

The command vector after the `--` separator is captured **verbatim**: no
shell is ever involved, arguments are never re-quoted or joined, and server
flags that happen to look like cortex-ia flags are safe after the separator.
The vector must be non-empty. `--env KEY=VALUE` assignments (repeatable)
carry environment variables for the server; values reach the config file
only and are never printed, logged, or stored in identity digests.

Example:

```bash
cortex-ia mcp add my-tools --local --env API_TOKEN=secret -- npx -y my-mcp-server --verbose
```

## Custom Remote Servers

```bash
cortex-ia mcp add my-remote --remote <url> [--header KEY=VALUE]... [--dry-run]
```

The URL must be a well-formed `http` or `https` URL with a host.
`--header KEY=VALUE` assignments (repeatable) carry HTTP headers such as
authorization tokens; the same secret-handling rules as `--env` apply —
values are written to the config and never surfaced anywhere else. Note the
`KEY=VALUE` form: the header name and value are separated by the first `=`
(for example `--header "Authorization=Bearer secret"`), not by a colon.

## Kind Exclusivity

`--preset`, `--local`, and `--remote` are mutually exclusive: exactly one
kind is required per `mcp add`. Mixing them (or omitting all of them) fails
closed with a typed validation error before any configuration is read.

## Listing and Ownership

```bash
cortex-ia mcp list [--json]
```

The plain listing prints the config path and the status of every managed
preset:

| Status | Meaning |
|--------|---------|
| `managed` | Configured by cortex-ia and matching its recorded identity digest |
| `unmanaged-equivalent` | Functionally equivalent entry present that cortex-ia did not record |
| `conflict` | An entry exists at the name but does not match the recorded identity |
| `absent` | Not configured |

Entries in the config that cortex-ia never configured are listed as
*unknown* — informational only, never touched.

`--json` prints a sanitized JSON document and nothing else:

```json
{
  "installed": true,
  "config_path": "/home/you/.config/opencode/opencode.jsonc",
  "entries": [
    { "name": "cortex", "status": "managed", "digest": "sha256:…", "type": "local" }
  ],
  "unknown": ["someone-elses-server"]
}
```

The JSON projection is allow-listed to identity evidence: name, status,
digest, entry type, and env/header variable **names**. Values, commands, and
URLs are not representable in it.

## Ownership Fingerprints (mcpv2)

Ownership is accredited by two parallel pieces of evidence, both
secret-free:

- The **identity digest** shown by `mcp list` (server name, type, command
  vector, and env/header variable names only).
- The **mcpv2 full-postimage fingerprint** (`mcpv2:<64 hex>`), an
  HMAC-SHA256 keyed by a random salt stored locally under `~/.cortex-ia/`.
  It covers type, command/args, URL, env/header names **and keyed hashes of
  their values**, enabled state, and the config path — so drift on any
  mutable field is detected while state never stores a clear secret.

Records that predate mcpv2 fail closed for destructive operations
(`mcp remove`, uninstall accreditation): the remedy is manual — inspect the
entry, re-run the matching `mcp add` to accredit the full postimage, then
remove. A missing or corrupt local salt likewise blocks destructive
accreditation instead of guessing.

## Removal

```bash
cortex-ia mcp remove my-server [--dry-run]
```

Removal deregisters the managed entry. The real run is destructive: it
requires an interactive terminal and an explicit confirmation, and piped or
closed input fails closed before anything is written. Removing a name that
holds a conflicting or unknown entry fails closed through the typed conflict
error — cortex-ia only ever removes what it can prove it owns. A backup of
the config is captured before the change.

## Fail-Closed Rules

- Malformed requests (bad names, missing vectors, invalid URLs, malformed
  `KEY=VALUE`) fail before any state access.
- Ownership conflicts fail closed; the error names the server and digest
  evidence only, never configuration contents.
- Every write captures a verified backup first and is reflected in the
  installation metadata so `doctor` can audit drift.
