# CLI reference

Every `cortex-ia` command group, its syntax and its purpose. Running `cortex-ia` with no arguments opens the terminal UI; any argument goes through the command dispatcher. Each group prints its own usage with `--help`.

Work-authority verbs (`claim`, `lease`, `transition`, `approve`, and similar) are meant for role controllers through the typed `cortex_ia_*` tools. The CLI refuses them when invoked from an agent shell.

Commands that emit machine-readable receipts print JSON to stdout; human diagnostic commands (`doctor`, `rollback`, `recover`, `report status`, `help`, `update`) print plain text. Status queries accept `show`/`get` aliases where noted, and each subcommand group prints its usage with `--help`.

## Task Boards (`cortex-ia board`)

| Command | Syntax | Purpose |
|---|---|---|
| **Create** | `cortex-ia board create <id> "<title>" "[desc]"` | Initialize a durable task-board boundary |
| **List** | `cortex-ia board list` | List all boards with completed/total counters |
| **Status** | `cortex-ia board status <id>` *(or `show`, `get`)* | Query board metadata and full task DAG snapshot |
| **Archive** | `cortex-ia board archive <id>` | Mark a completed board as archived |
| **Unarchive** | `cortex-ia board unarchive <id>` | Restore an archived board to active |
| **Delete** | `cortex-ia board delete <id>` | Permanently delete an archived board and its tasks |
| **Serve** | `cortex-ia board serve [--addr 127.0.0.1:7331]` | Run the embedded loopback web dashboard |

## Work Items & Leases (`cortex-ia work`)

| Command | Syntax | Purpose |
|---|---|---|
| **Create** | `cortex-ia work create <id> "<title>" [--board <board>] [--depends <id>]... [--objective <text>] [--acceptance <text>] [--verify <cmd>] [--file <path>]...` | Add a task to the DAG (`backlog`/`ready`) with its definition |
| **Revise** | `cortex-ia work revise --plan <file\|@stdin>` | Safely revise an unclaimed task definition |
| **Review Refresh** | `cortex-ia work review-refresh <id> --revision <n>` | Rebind the review to an observed revision |
| **Archive** | `cortex-ia work archive --board <id> --change <id> --workflow <sdd-lite\|sdd-full> --spec-plane <openspec\|cortex\|hybrid>` | Close independently approved SDD work |
| **List** | `cortex-ia work list [--board <board-id>]` | List work items, optionally scoped to one board |
| **Status** | `cortex-ia work status <id>` *(or `show`, `get`)* | Query task status, revision, claim, and active leases |
| **Approvals** | `cortex-ia work approvals <id>` | List historical approval records |
| **Fingerprint** | `cortex-ia work fingerprint <id>` | Compute current fingerprints and compare with the approval |
| **Claim** | `cortex-ia work claim <id> --owner <owner> [--path <file> ...] [--ttl 15m]` | Atomically acquire a task (and optional leases); returns `claim_token` |
| **Controller Renew** | `cortex-ia work controller-renew <id> --owner <owner> --authority @stdin` | Renew a live claim and its complete lease set |
| **Renew** | `cortex-ia work renew <id> --claim-token <tok> [--ttl 15m]` | Extend live claim TTL before expiry |
| **Lease** | `cortex-ia work lease <id> --claim-token <tok> --path <file> [--ttl 15m]` | Reserve one exclusive file lease; returns `lease_token` |
| **Reserve** | `cortex-ia work reserve <id> --claim-token <tok> --path <file> [--path <file> ...] [--ttl 15m]` *(or `file-reserve`)* | Reserve one or more files atomically |
| **Lease Renew** | `cortex-ia work lease-renew --path <file> --lease-token <tok> [--ttl 15m]` | Extend a file lease TTL while editing |
| **Release** | `cortex-ia work release --path <file> --lease-token <tok>` | Release one file lease |
| **Release All** | `cortex-ia work release-all <id> --claim-token <tok>` | Release every file lease held by a task |
| **Transition** | `cortex-ia work transition <id> --claim-token <tok> [--revision <n>] --to <in_review\|in_progress\|blocked>` | Shift task state with an optional submission receipt |
| **Approve** | `cortex-ia work approve <id> --reviewer <id> --verdict <PASS\|FAIL\|BLOCKED\|INCONCLUSIVE> [--evidence <ref>]` | Record a review verdict; `PASS` unlocks downstream tasks |
| **Retry** | `cortex-ia work retry <id> [--revision <n>]` | Clear residual locks and return a `blocked` task to `ready` |
| **Decompose** | `cortex-ia work decompose <id> --revision <n> --plan <file\|@stdin> [--contract-file <file>]` | Replace a blocked task with atomic tasks |
| **Recover** | `cortex-ia work recover` | Sweep expired claims/leases |
| **Reconcile** | `cortex-ia work reconcile <id> --reason <text> --session <id> --revision <n>` | Force-release a live-but-orphaned claim (orchestrator-only, fail-closed) |
| **Verify Lease** | `cortex-ia work verify-lease --path <file> [--task <id>] [--owner <owner>]` *(or `check-lease`)* | Verify an active file lease |

## OpenSpec SDD Workspace (`cortex-ia openspec`)

| Command | Syntax | Purpose |
|---|---|---|
| **Validate** | `cortex-ia openspec validate <change> --workflow <sdd-lite\|sdd-full> --phase <phase> [--json]` | Structurally validate planning artifacts. `--workflow` and `--phase` are **required** |
| **List** | `cortex-ia openspec list` | List active change proposals in `openspec/changes/` |
| **Status** | `cortex-ia openspec status [change-name]` | Inspect task progress and status of changes |
| **Archive** | `cortex-ia openspec archive <change-name> --board <id> --workflow <sdd-lite\|sdd-full> --spec-plane <openspec\|cortex\|hybrid>` | Close independently approved SDD work |
| **New** | `cortex-ia openspec new <change-name> [domain]` | Scaffold a new OpenSpec change directory |

## Cortex Snapshots (`cortex-ia snapshot`)

| Command | Syntax | Purpose |
|---|---|---|
| **Read** | `cortex-ia snapshot read --project <project> --id <id> [--expected-sha256 <digest>]` | Read and verify one bounded local Cortex observation |

## Git Worktrees (`cortex-ia worktree`)

| Command | Syntax | Purpose |
|---|---|---|
| **List** | `cortex-ia worktree list [--repo <repo-path>]` | List authoritative Git worktrees |
| **Validate** | `cortex-ia worktree validate <worktree-path> [--repo <repo-path>] [--head <commit>]` | Validate a worktree contract against git porcelain |

`current_workspace` is the only supported execution strategy. `worktree create`, `clean`, `drop`, `delete`, `remove`, and `prune` are retired and fail closed; existing worktrees are preserved.

## Dual Ledger (`cortex-ia ledger`)

| Command | Syntax | Purpose |
|---|---|---|
| **Fact Add** | `cortex-ia ledger fact add <text> [--board <id>] [--source <src>] [--sync-cortex]` | Record a verified fact (optionally synced to Cortex memory) |
| **Fact List** | `cortex-ia ledger fact list [--board <board-id>] [--json]` | List verified facts in chronological order |
| **Progress** | `cortex-ia ledger progress record --summary <text> [--drift] [--action <act>]` | Record an orchestrator progress evaluation |
| **Status** | `cortex-ia ledger status [--board <board-id>] [--json]` | Display the full dual-ledger report (facts + progress) |

## UI Snapshot (`cortex-ia ui`)

| Command | Syntax | Purpose |
|---|---|---|
| **Snapshot** | `cortex-ia ui snapshot [--project <path>] [--session-id <id>] [--root-session-id <id>]` | Print a bounded read-only TUI snapshot |

## Documents & Diagrams (`cortex-ia doc` / `cortex-ia diagram`)

| Command | Syntax | Purpose |
|---|---|---|
| **Doc Convert** | `cortex-ia doc convert <file> [-o <out.md>] [--standalone] [--format <fmt>] [--max-lines <n>] [--ocr <hosted\|reject>] [--json]` | Convert office/PDF documents to Markdown |
| **Doc Inspect** | `cortex-ia doc inspect <file> [--json]` | Inspect document metadata |
| **Diagram Validate** | `cortex-ia diagram validate <type> <spec.json> [--quality <standard\|showcase>] [--json]` | Validate diagram topology |
| **Diagram Render** | `cortex-ia diagram render <type> <spec.json> [output.html] [--quality <standard\|showcase>] [--json]` | Render an interactive diagram HTML file |
| **Diagram Compare** | `cortex-ia diagram compare <base.json> <head.json> [output.html] [--json]` | Compare two architecture snapshots |
| **Diagram Reach** | `cortex-ia diagram reach <type> <spec.json> --from <node-id> [--direction <upstream\|downstream\|both>] [--json]` | Trace graph reachability from a node |

## MCP Management (`cortex-ia mcp`)

| Command | Syntax | Purpose |
|---|---|---|
| **Add (preset)** | `cortex-ia mcp add <name> --preset [--dry-run]` | Register a managed catalog MCP preset |
| **Add (local)** | `cortex-ia mcp add <name> --local [--env KEY=VALUE]... -- <command> [args...]` | Register a managed custom local MCP server |
| **Add (remote)** | `cortex-ia mcp add <name> --remote <url> [--header KEY=VALUE]... [--dry-run]` | Register a managed custom remote MCP server |
| **List** | `cortex-ia mcp list [--json]` | List managed MCP entries and ownership |
| **Adopt** | `cortex-ia mcp adopt <name> [--dry-run]` | Accredit an existing user-owned MCP entry that already equals a managed preset |
| **Remove** | `cortex-ia mcp remove <name> [--dry-run]` | Deregister a managed MCP entry |

`--preset`, `--local`, and `--remote` are mutually exclusive: exactly one is required per `add`.

## Reporting (`cortex-ia report`)

| Command | Syntax | Purpose |
|---|---|---|
| **Report Error** | `cortex-ia report error --code <code> --message <msg> [--details <text\|@stdin>]` *(or `send`)* | Generate and send a signed error report |
| **Report Config** | `cortex-ia report config [--endpoint <url>] [--secret <key>] [--enable\|--disable]` | Configure the reporting endpoint |
| **Report Flush** | `cortex-ia report flush` | Retry bounded queued reports |
| **Report Status** | `cortex-ia report status` | Show the current reporting configuration |

`cortex-ia hook pre-tool --host <claude|agy>` is the host pre-tool guard, invoked by Claude Code and agy hooks rather than by users. The former `delegate` and `herdr` subcommands are retired and fail closed.

## Model Management (`cortex-ia model`)

| Command | Syntax | Purpose |
|---|---|---|
| **List** | `cortex-ia model list` | List all configured model assignments |
| **Get** | `cortex-ia model get <agent>` | Show the model assigned to an agent |
| **Set** | `cortex-ia model set <agent> <provider/model[#variant]> [--effort <level>]` | Assign a model to an agent |
| **Unset** | `cortex-ia model unset <agent>` | Remove an agent's model assignment |
| **Doctor** | `cortex-ia model doctor` | Check model configuration health |
| **Catalog** | `cortex-ia model catalog` | Read-only list of available providers, models, and variants |

## Usage Statistics (`cortex-ia stats`)

| Command | Syntax | Purpose |
|---|---|---|
| **Stats** | `cortex-ia stats [--json]` | Print bounded read-only usage statistics (`--json` for machine-readable output) |

## Maintenance & Lifecycle (`install` / `sync` / `doctor` / `rollback` / `recover` / `uninstall` / `update`)

| Command | Syntax | Purpose |
|---|---|---|
| **Install** | `cortex-ia install [--target <list>] [--dry-run] [--overwrite] [--theme]` | Install assets and plugins for `opencode` (default), `claude`, `agy`, or `all`; `--theme` applies the bundled cortex theme (opencode target only) |
| **Sync** | `cortex-ia sync [--target <list>] [--dry-run] [--overwrite]` | Reconcile the installed home with the current asset set |
| **Doctor** | `cortex-ia doctor` | Read-only installation health report |
| **Rollback** | `cortex-ia rollback [backup-id]` / `cortex-ia rollback list` | Restore a backup or list available backups |
| **Recover** | `cortex-ia recover [list]` / `cortex-ia recover <journal-id>` | List or restore pending recovery journals |
| **Uninstall** | `cortex-ia uninstall [--target <list>] [--dry-run]` | Remove the accredited installation |
| **Update** | `cortex-ia update [--check] [--scheduled] [--allow-checksum-updates]` *(or `upgrade`)* | Check for / install the latest release |
| **Update Schedule** | `cortex-ia update schedule <enable\|disable\|status>` | Manage the headless daily check-only update task |

`--theme`, `--scheduled`, and `--allow-checksum-updates` are documented with their full flag tables in [Configuration](../getting-started/configuration.md).
