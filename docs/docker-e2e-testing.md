# Docker E2E Testing

cortex-ia's E2E suite runs the real binary inside clean Linux containers so we exercise the install pipeline against pristine filesystems.

## Layout

```
e2e/
├── lib.sh              # shared shell helpers (assert_*, log_*, resolve_binary, cleanup_test_env)
├── e2e_test.sh         # the test suite (sources lib.sh)
├── docker-test.sh      # orchestrator: builds + runs each Dockerfile
├── Dockerfile.ubuntu   # primary distro
├── Dockerfile.fedora   # dnf-based check
└── Dockerfile.arch     # pacman-based check (forces --platform=linux/amd64)
```

## Running the suite

```bash
# All distros
./e2e/docker-test.sh

# Single distro
./e2e/docker-test.sh ubuntu
./e2e/docker-test.sh fedora
./e2e/docker-test.sh arch
```

The orchestrator builds an image per distro, runs `e2e/e2e_test.sh` inside it, and aggregates pass/fail counts.

## What the suite covers

`e2e_test.sh` exercises:

1. `cortex-ia version`
2. `cortex-ia install --dry-run --target opencode` (no files written)
3. `cortex-ia install --target opencode` against a fake `claude` binary on `PATH`
4. State file, lockfile, skills directory, convention file presence
5. Skill count ≥ 19, absolute (not relative) convention refs
6. `cortex-ia doctor` passes
7. Idempotency: a second `install` produces the same installation state
8. `cortex-ia sync` keeps the installed home converged
9. `cortex-ia rollback` restores a captured snapshot
10. `cortex-ia recover list` reports pending journals
11. `cortex-ia update --check` reaches GitHub Releases

## Adding a check

Use the helpers from `lib.sh` rather than rolling your own:

```bash
log_test "My new check"
run_cmd "my-test" "$CORTEX_IA something | grep -q expected"
assert_file_exists "$HOME/.cortex-ia/some/file" "label"
assert_file_contains "$HOME/.cortex-ia/state.json" '"some-key"' "state has some-key"
assert_no_duplicate_section "$HOME/.claude/CLAUDE.md" "cortex-protocol"
```

The marker for `assert_no_duplicate_section` is the cortex-ia prefix: `<!-- cortex-ia:<id> -->`.

## Apple Silicon caveat

`Dockerfile.arch` forces `--platform=linux/amd64` and disables pacman's seccomp sandbox so it can run under QEMU emulation on M-series Macs. Build time is ~2× slower than ubuntu/fedora; consider running it nightly rather than on every PR.

## CI integration

`.github/workflows/ci.yml` runs the suite on every push to `main` or `develop`. PRs run unit tests on every push and the E2E matrix only on `ready_for_review`. The `pr-check.yml` workflow validates issue linkage / labels / branch name and must pass independently.

## Troubleshooting

| Symptom | Likely cause |
|---|---|
| `cortex-ia not found` | `resolve_binary` failed — confirm the Dockerfile's `go build -o /usr/local/bin/cortex-ia` step ran |
| `Pattern NOT found: cortex-protocol` | The conventions component didn't run during `install` / `sync` |
| `DUPLICATE section marker` | Idempotency bug — `filemerge.InjectMarkdownSection` got bypassed |
| pacman keyring errors on Arch | The image's `pacman-key --init && pacman-key --populate archlinux` step regressed |
