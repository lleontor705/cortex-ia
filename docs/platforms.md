# Supported Platforms

cortex-ia targets the same matrix as the agents it configures.

| OS | Status | Notes |
|---|---|---|
| **macOS** Intel | ✅ Full | Tested on every release |
| **macOS** Apple Silicon | ✅ Full | Native arm64 build; QEMU fallback for Arch E2E |
| **Linux** Ubuntu/Debian | ✅ Full | apt + npm; primary Linux target |
| **Linux** Fedora/RHEL | ✅ Full | dnf + npm; Docker E2E `Dockerfile.fedora` |
| **Linux** Arch/Manjaro | ✅ Full | pacman + npm; Docker E2E `Dockerfile.arch` (forces `--platform=linux/amd64`) |
| **Linux** Other | ⚠️ Best-effort | Should work; not in CI |
| **Windows** native | ✅ Full | PowerShell-aware; symlink fallback for filemerge tests |
| **Windows** WSL2 | ✅ Full | Treat as Linux |

## Per-OS notes

### Windows symlink privilege

Tests in `internal/components/filemerge` create symbolic links. Without `SeCreateSymbolicLinkPrivilege`, Windows returns `ERROR_PRIVILEGE_NOT_HELD` (errno 1314) and the tests skip themselves. To run them:

- **Enable Developer Mode** (Settings → System → For developers → Developer Mode), or
- **Run the test process as Administrator**, or
- **Grant the privilege explicitly** via `Local Security Policy → User Rights Assignment → Create symbolic links`.

cortex-ia's runtime install path uses an atomic copy fallback when symlinks aren't available, so Windows installs work without admin rights even without the privilege.

### Apple Silicon — Arch Linux E2E

`Dockerfile.arch` forces `--platform=linux/amd64` so Docker uses QEMU emulation on M-series Macs. The pacman seccomp sandbox is disabled in that Dockerfile (`DisableSandbox`) because it's incompatible with QEMU user-mode.

### Path conventions

| Platform | `~/.cortex-ia/` | Skills root | Backups root |
|---|---|---|---|
| Linux/macOS | `$HOME/.cortex-ia/` | `$HOME/.cortex-ia/skills/` | `$HOME/.cortex-ia/backups/` |
| Windows | `%USERPROFILE%\.cortex-ia\` | `%USERPROFILE%\.cortex-ia\skills\` | `%USERPROFILE%\.cortex-ia\backups\` |

Per-agent config paths are documented in [`agents.md`](agents.md).

## Architectures

`darwin/amd64`, `darwin/arm64`, `linux/amd64`, `linux/arm64`, `windows/amd64` are released by goreleaser. `windows/arm64` builds are published on a best-effort basis.
