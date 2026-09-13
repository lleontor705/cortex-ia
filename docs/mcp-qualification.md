# Model Context Protocol (MCP) Integration Qualification

Requirements: REQ-MCP-001.

## 1. Candidate Context7 4.1.0 Specification & Transitive Lock

The candidate Context7 preset is pinned and qualified against authentic npm registry manifests in `scripts/fixtures/mcp/`:

| Field | Value |
|---|---|
| Package | `@upstash/context7-mcp` |
| Qualified Version | `4.1.0` |
| Official NPM Registry Tarball | `https://registry.npmjs.org/@upstash/context7-mcp/-/context7-mcp-4.1.0.tgz` |
| Package-Lock SHA-512 Integrity | `sha512-ngAkFwW3LsnRGpH3XTVrjDqm3QBT4ZRpLCnShI0cIfCG+ACt07TrkRZc3n7+qjkFTcM/xIDJcHUBK5bDXUa40w==` |
| Node.js Engine Requirement | `>=20.18.1` |
| Executable Entrypoint | `dist/index.js` |

Transitive lock resolution has been verified via `npm install --package-lock-only` with official package integrity.

## 2. Cortex MCP Local Executable & Schema Evidence

The local Cortex MCP server is verified under isolated stdio transport without active configuration mutation:

- **Executable Version**: `cortex v2.3.9`
- **Executable SHA-256**: `69afe4f91c6552b30b81c16c06efb044684f382752c6effe3287cc2f249df98d`
- **Protocol Version**: `2024-11-05`
- **Protocol Bound**: Maximum 2 requests per server (`initialize`, `tools/list`), 10-second timeout, 1 MiB max buffer.
- **Agent Tools Schema**: 35 registered agent tools verified, including `cortex_search`, `cortex_save`, `cortex_get_rules`, `cortex_ingest_code`, `cortex_detect_cycles`.
- **Optional Capabilities**: Absent capabilities (`prompts`, `resources`) are safely reported without false failures.

## 3. Verification Commands

Run the full isolated qualification suite:
```bash
node --test scripts/qualify-mcp.test.mjs && node scripts/qualify-mcp.mjs --locked --isolate --timeout-ms 10000 --max-output-bytes 1048576 --max-requests 2 --fixture-root scripts/fixtures/mcp --require-live-schema-evidence
```
All checks exit 0.
