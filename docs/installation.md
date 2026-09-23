# Installation

## Prerequisites

- **OpenCode** — supported target for AI agent orchestration.
- **Herdr** (optional) — diagnostics-only integration; its only production consumer is the web console status display.
- **Node.js 18+** — for OpenCode plugins runtime.
- **`cortex` on PATH** (optional) — for Cortex Knowledge Graph MCP server.

---

## Quick Install

### Windows (PowerShell)

```powershell
irm https://raw.githubusercontent.com/lleontor705/cortex-ia/main/scripts/install.ps1 | iex
```

### Linux & macOS (Bash)

```bash
curl -sSL https://raw.githubusercontent.com/lleontor705/cortex-ia/main/scripts/install.sh | bash
```

Both installers automatically:
1. Download and install the `cortex-ia` binary into your user bin directory.
2. Add the binary directory to your persistent `PATH`.
3. Set `OPENCODE_EXPERIMENTAL_BACKGROUND_SUBAGENTS="true"` across shell profiles.
4. Execute `cortex-ia sync` to deploy all MCPs, OpenCode plugins, and agent prompts.

---

## Alternative Methods

### Go Install

```bash
go install github.com/lleontor705/cortex-ia/cmd/cortex-ia@latest
cortex-ia sync
```

### Homebrew (macOS / Linux)

```bash
brew install lleontor705/tap/cortex-ia
cortex-ia sync
```

### From Source

```bash
git clone https://github.com/lleontor705/cortex-ia.git
cd cortex-ia
go build -o bin/cortex-ia.exe ./cmd/cortex-ia
.\bin\cortex-ia.exe sync
```

---

## Verify Installation

```bash
cortex-ia version
cortex-ia doctor
```

## First Run

```bash
cortex-ia            # Interactive Bubble Tea TUI
cortex-ia web        # Web Console (http://127.0.0.1:7331)
cortex-ia sync       # Reconcile all plugins, MCPs, and agents
```

---

## Auto-Update

`cortex-ia` can check for, download, verify, and replace itself with an official
signed release. A version is only ever moved forward: replays and downgrades of
an already-applied release are rejected before the archive is downloaded.

### Requirement: a build with the packaged trust key

Applying an update requires a build that carries the release trust key. Official
releases are built with it; local, development, and `go install` builds are not,
and they fail closed with exactly:

```text
Authenticated update unavailable: no trusted release key is packaged
```

That message is a contract, not a defect: there is no unsigned fallback path, so
a build without a key cannot install anything. Local builds keep working — only
the `update` surface stays disabled. The key ceremony (offline pair generation,
bundle packaging, rotation with at most two keys, compromise procedure) is
documented in [Release Signing Keys](release-keys.md). It is a plan document and
is never executed by the build or by CI.

### Signed release assets

Every release publishes platform archives plus two verification assets:

- `release-manifest.json` — schema-1 list of the published artifacts with their
  exact size and SHA-256 digest. Archives follow
  `cortex-ia_{version}_{os}_{arch}.zip` on Windows and `.tar.gz` elsewhere.
- `release-manifest.sig` — Ed25519 signature envelope over the manifest, naming
  the key ID that signed it.

The updater verifies the signature and the trust-key validity window before it
downloads an archive, then re-checks the archive digest and size against the
signed manifest. Downloads are restricted to an HTTPS host allowlist and capped
at 128 MiB. Replacement is staged, digest-verified a second time, and rolled
back if any step fails.

### Check and apply

```bash
cortex-ia update            # check, then download and apply if newer
cortex-ia update --check    # report availability only; never downloads or applies
cortex-ia update --help
```

Check results are cached in `~/.cortex-ia/update-state.json` (schema 1, written
atomically with mode `0600`): last check time, the available version, the applied
floor, the managed binary path, and the detected install candidates.
`CORTEX_IA_HOME` overrides the state root. A missing or corrupt state file is
treated as an empty first-run state, never as a crash.

### Scheduled check (check-only)

```bash
cortex-ia update schedule enable    # register the daily check-only task
cortex-ia update schedule disable   # remove the managed task
cortex-ia update schedule status    # report registration and cadence
```

The registered task is user-level (no elevation) and runs exactly
`update --check --scheduled` once a day at 12:00. It is strictly check-only: it
refreshes the cached state and exits, and it never downloads or replaces a
binary, so the scheduler cannot open an unattended apply path. `--scheduled` is
valid only together with `--check`.

On Windows the task is registered through `schtasks` as `CortexIA Update Check`
(`/SC DAILY /ST 12:00`); on POSIX systems a marked `crontab` line is used.
`enable` refuses to overwrite a task name it does not recognize as its own, and
`disable` reports success when the task was already absent.

### TUI boot prompt

Before the interactive TUI takes over the terminal it reads the cached state
and, when a newer release is pending, asks:

```text
Actualización disponible: vX.Y.Z (actual: vA.B.C).
¿Actualizar a vX.Y.Z? [y/N]
```

- `n`, EOF, or no cached release continues into the TUI and preserves the state.
- `y` first evaluates the work-authority gate. If unexpired claims, leases, or
  `in_progress` tasks exist, the update is deferred with a message and the TUI
  still starts; the release stays cached for the next quiet boot.
- Only after an open gate does it download, verify, and replace the binary, then
  print the restart notice.

The prompt never blocks boot: a declined prompt, a closed gate, or any typed
apply failure leaves the running binary untouched. There is no silent or
unattended apply path — a release is applied only after an interactive `[y/N]`
in the same process. Without cached state the TUI stays silent unless
`CORTEX_IA_UPDATE_INLINE_CHECK=1` is set, which enables an opt-in inline check
with a short timeout that degrades to silence when the network is unreachable.

### Dual installations

The updater only replaces the binary it is running as. When more than one
`cortex-ia` binary exists among the known install locations, the check and
schedule surfaces warn:

```text
Warning: multiple cortex-ia installations detected: <path>, <path>
```

The known locations are the Windows installer target
`%LOCALAPPDATA%\Programs\cortex-ia\bin\cortex-ia.exe` and the Go bin directory
(`GOPATH\bin\cortex-ia.exe`, or `%USERPROFILE%\go\bin\cortex-ia.exe` when
`GOPATH` is unset). A `go install` copy and an installer copy can therefore
diverge silently. Keep exactly one `cortex-ia` on your `PATH` and remove the
other.
