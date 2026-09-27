# Web Operations Console

The Cortex-IA web console is a small Preact operations dashboard served by the `cortex-ia` binary itself. It shows task-board sessions, work/claim/lease state, the append-only activity stream, and delegation jobs. It is a **read-mostly operational view** backed by one REST API; it is never an authority surface.

## Architecture

- **Source**: `web/src/` (Preact + Vite). The toolchain is isolated in `web/package.json`; `npm --prefix web run build` writes compiled assets to `internal/cortexiaweb/static/`.
- **Embedding**: the built assets are embedded with `go:embed` in `internal/cortexiaweb`, so the shipped binary needs no Node runtime.
- **Server**: a loopback-only HTTP server in `internal/cortexiaweb/server.go` that serves the embedded SPA and the JSON API.
- **Store**: the server reads the same SQLite authority database (`~/.cortex-ia/delegation.db`) that `cortex-ia work` writes. Read endpoints use store methods only; they never mutate work state.

## Launching

Two entry points cover the same embedded server:

```bash
# Start the server directly (default 127.0.0.1:7331)
cortex-ia board serve [--addr <loopback-host:port>]

# Convenience launcher with deep links and background mode
cortex-ia web [--addr <loopback-host:port>] [--board <board-id>] [--task <task-id>] [--open] [--daemon]
```

`cortex-ia web` flags:

| Flag | Description |
|------|-------------|
| `--addr <host:port>` | Listen or connect address. Defaults to `127.0.0.1:7331`; a bare `:port` normalizes to `127.0.0.1:port`. |
| `--board <board-id>` | Deep-link the dashboard to one board (`?board=<id>#board`). |
| `--task <task-id>` | Deep-link the dashboard to one task modal (`?task=<id>`). |
| `--open`, `-o` | Open the dashboard URL in the default browser. |
| `--daemon`, `-d` | Start `board serve` in the background if a healthy server is not already running. |

If a healthy server already answers `GET /api/overview`, `web` attaches to it instead of starting a second listener.

## Security posture

The console is hardened for single-user, localhost-only use:

- **Loopback only**: `board serve` rejects any listen address whose host is not `localhost` or a loopback IP. Requests whose `Host` header is not loopback are answered `403 Forbidden`.
- **Same-origin writes**: mutating endpoints require a same-origin request (`Sec-Fetch-Site` must not be `cross-site`, and any `Origin` must match the request host).
- **Content Security Policy**: `default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'`. No CDN, external asset, or inline-script dependency.
- **Additional headers**: `Referrer-Policy: no-referrer`, `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`.
- **Request limits**: JSON bodies are capped at 64 KiB (`http.MaxBytesReader`), unknown JSON fields are rejected, and the feed page/board query parameters are bounded.
- **Server timeouts**: `ReadHeaderTimeout` 5s, `ReadTimeout` 10s, `WriteTimeout` 15s, `IdleTimeout` 60s.

## Auto-refresh

The SPA polls the API rather than streaming: the overview and activity feed refresh roughly every 8 seconds, the open board every 5 seconds, and all polling pauses while the tab is hidden. There is no server push and no long-lived stream to keep open.

## REST API

| Method | Path | Purpose |
|--------|------|---------|
| `GET` | `/api/overview` | Aggregate dashboard counts and summaries. |
| `GET` | `/api/feed?page=<n>&board=<id>` | Paged append-only activity stream. |
| `GET` | `/api/boards` | List boards. |
| `POST` | `/api/boards` | Create a board (`board_id`, `title`, `description`). |
| `GET` | `/api/boards/{id}` | Board snapshot (tasks and state). |
| `POST` | `/api/boards/{id}/archive` | Archive a board. |
| `POST` | `/api/boards/{id}/unarchive` | Restore an archived board. |
| `DELETE` | `/api/boards/{id}` | Delete a board. |
| `POST` | `/api/tasks` | Create a task in a board. |
| `GET` | `/api/config` | Non-secret configuration projection. |
| `GET` | `/api/delegations/{id}` | One legacy delegation job plus its available receipt. |

Unknown non-API paths fall back to `index.html` so client-side routes (for example `/board` or `/overview`) resolve without a server round-trip.

## Authority boundary

The console may create boards and tasks and archive, unarchive, or delete boards. It **never** exposes claim, lease, transition, retry, recovery, or approval mutations. Those remain exclusive to the authoritative `cortex-ia work` surfaces and their minted tokens. Board placement and card position in the console are observational and never prove readiness, review, or authority.

## Status display

The console status panel is the only production consumer of the optional Herdr diagnostics helpers (`internal/herdr`). Herdr never owns task state or approval.

## See Also

- [`codebase/dashboard.md`](codebase/dashboard.md) — Bubble Tea TUI architecture
- [`codebase/reference-map.md`](codebase/reference-map.md) — CLI and package lookup
- [`agents.md`](agents.md) — role topology and authority invariants
