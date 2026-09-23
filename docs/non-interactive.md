# Non-Interactive (CLI-Only) Mode

cortex-ia is TUI-first but every operation also has a CLI flag set, so you can drive the whole pipeline from scripts, CI, and Dockerfiles without the Bubbletea UI.

## Recipes

### Install with an explicit target

```bash
cortex-ia install --target opencode,claude
```

`--target` accepts a comma-separated list of `opencode`, `claude`, or `all`. It defaults to `opencode`.

### Dry run (preview without touching disk)

```bash
cortex-ia install --dry-run --target opencode
```

Returns exit code 0 and prints the plan. No files are written and no backup is created.

### Sync (re-run injectors without changing the selection)

```bash
cortex-ia sync
```

Useful in CI: pull main → `cortex-ia sync` to pick up new SDD skills or convention updates.

### Health check

```bash
cortex-ia doctor
```

`doctor` is a read-only assessment of the installed home (artifact presence and drift, detected coding CLIs, managed MCP entries) and exits non-zero when the verdict is degraded or blocked. Re-run `cortex-ia sync` to reconcile the installed home with the current asset set.

### Rollback

```bash
cortex-ia rollback list             # list available backups
cortex-ia rollback                  # most recent backup
cortex-ia rollback 20260425-093015  # specific snapshot
```

A real rollback requires an interactive terminal and explicit confirmation; piped or closed input fails closed without writing.

### Read-only inspection for scripting

```bash
cortex-ia rollback list    # backups with id and label (plain text)
cortex-ia recover list     # pending recovery journals (plain text)
```

Commands that emit machine-readable receipts print JSON; use `--json` where a subcommand documents it (for example `cortex-ia mcp list --json`).

### Update check

```bash
cortex-ia update --check    # checks GitHub Releases; prints current vs latest
cortex-ia update            # downloads and applies the latest release
```

## Exit codes

| Code | Meaning |
|---|---|
| `0` | Success |
| `1` | Generic failure |
| `2` | Doctor check failed (also triggers from `--dry-run` if the plan would fail) |
| `3` | Validation error (bad flag, missing prerequisite) |

## Environment

cortex-ia reads no required env vars. Optional:

- `CORTEX_IA_HOME` — override the `~/.cortex-ia/` state root with an absolute path (rarely needed; tests use this)
- `CORTEX_IA_DEBUG` — enable debug tracing when set to `1`, `true`, or `yes` (equivalent to the per-invocation `--debug` flag)

The former `CORTEX_IA_AGY_AUTH` / `GEMINI_API_KEY` variables authenticated the retired external AGY leaf and are dead: execution is native-only.

## CI examples

GitHub Actions one-liner:

```yaml
- name: Install cortex-ia ecosystem
  run: |
    go install github.com/lleontor705/cortex-ia/cmd/cortex-ia@latest
    cortex-ia install --target opencode
    cortex-ia doctor
```

Docker (see `e2e/Dockerfile.ubuntu`):

```dockerfile
RUN cortex-ia install --target opencode && cortex-ia doctor
```
