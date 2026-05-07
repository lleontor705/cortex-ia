# Non-Interactive (CLI-Only) Mode

cortex-ia is TUI-first but every operation also has a CLI flag set, so you can drive the whole pipeline from scripts, CI, and Dockerfiles without the Bubbletea UI.

## Recipes

### Fresh install with explicit selection

```bash
cortex-ia install \
  --agent claude-code \
  --agent opencode \
  --preset full \
  --persona professional \
  --model-preset balanced
```

### Project-level config

Drop a `.cortex-ia.yaml` at the project root and run:

```bash
cortex-ia install --local
```

See [`configuration.md`](configuration.md) for the schema.

### Dry run (preview without touching disk)

```bash
cortex-ia install --dry-run --preset full
```

Returns exit code 0 + a plan to stdout. No files written, no backup created.

### Sync (re-run injectors without changing the selection)

```bash
cortex-ia sync
```

Useful in CI: pull main → `cortex-ia sync` to pick up new SDD skills or convention updates.

### Health check + auto-repair

```bash
cortex-ia doctor || cortex-ia repair
```

`doctor` exits non-zero if any of the 6 checks fail. `repair` re-applies the lockfile.

### Rollback

```bash
cortex-ia rollback                          # most recent backup
cortex-ia rollback 20260425-093015          # specific snapshot
```

### Lists for scripting

```bash
cortex-ia list agents       # supported + detected agents
cortex-ia list components   # available components
cortex-ia list backups      # snapshots with id, source, file count
```

All `list` commands print plain text suitable for `awk`/`cut`. JSON output is on the roadmap.

### Update check

```bash
cortex-ia update    # checks GitHub Releases; prints current vs latest
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

- `CORTEX_IA_HOME` — override `~/.cortex-ia/` (rarely needed; tests use this)
- `XDG_CONFIG_HOME` — respected for Linux config-dir resolution where applicable

## CI examples

GitHub Actions one-liner:

```yaml
- name: Install cortex-ia ecosystem
  run: |
    go install github.com/lleontor705/cortex-ia/cmd/cortex-ia@latest
    cortex-ia install --local --agent claude-code --preset full --persona professional
    cortex-ia doctor
```

Docker (see `e2e/Dockerfile.ubuntu`):

```dockerfile
RUN cortex-ia install --preset full --persona minimal && cortex-ia doctor
```
