# Updater Verification Profiles

`cortex-ia update` proves a downloaded release before it installs anything. The
proof is chosen by a **verification profile**, resolved per run before any
network call. Three profile names exist in the type system; only two are
selectable, and the third is reserved.

| Profile | Selectable | When it is active | Proof required |
|---|---|---|---|
| `strict` | yes (default) | A trust bundle is packaged into the build | Ed25519 signature over `release-manifest.json` |
| `checksum` | yes, only with explicit consent | No trust bundle is packaged **and** the operator consents | SHA-256 digest and size per artifact |
| `insecure` | no | never | none |

The precedence is fixed: a packaged bundle always selects `strict`, regardless
of consent. `checksum` is only reachable from a bundle-less build with consent.
`insecure` is never returned by profile resolution — no flag, environment
variable, or configuration file can select it.

## Strict profile

This is the default for every official release and the profile that must never
be weakened.

- `release-manifest.json` and its detached `release-manifest.sig` envelope are
  fetched from the pinned-HTTPS allowlist.
- The signature is verified with Ed25519 against a public key compiled into the
  binary, selected by `key_id` from the packaged trust bundle.
- The `Repository`, `MinVersion`/`MaxVersion` interval, schema version, and
  artifact table are all validated before any artifact bytes are downloaded.
- Each artifact is then re-hashed and checked against the manifest, and the
  persisted applied floor rejects any candidate at or below it.

A build with a bundle keeps these guarantees for every input; consent cannot
downgrade it. See [`release-keys.md`](release-keys.md) for how the bundle and
its key interval are provisioned and rotated.

## Checksum profile

The checksum profile is a **consent-gated bootstrap** for builds that carry no
trust bundle — local, development, and otherwise bundle-less binaries — so they
can still install a strictly newer official release instead of failing closed.
It is opt-in per run and is never the default.

What it still enforces:

- **Transport**: the manifest is fetched only through the pinned-HTTPS client,
  which accepts only allowlisted hosts and rejects any non-HTTPS or
  non-allowlisted origin (`ErrDisallowedOrigin`, `ErrInsecureScheme`) before a
  body is read.
- **Shape and binding**: the same manifest validator used by strict runs checks
  the schema version, the `Repository` field against the client repo
  (`ErrRepositoryMismatch`), the tag, the artifact table, lowercase-hex SHA-256
  digests, and size bounds.
- **Integrity**: every downloaded artifact must match the SHA-256 digest and
  size declared in the manifest (`ErrDigestMismatch`), for every profile.
- **Anti-replay**: the persisted applied floor is unchanged. A candidate at or
  below the floor is rejected (`ErrDowngradeOrReplay`) before any download.

What it does **not** do: verify a signature. The manifest is fetched over TLS
from the same origin as the release assets, and it is itself the source of the
digests, so nothing cryptographically ties the manifest to the maintainers'
offline key.

### Threat model (honest scope)

The checksum profile protects against:

- **Corruption and truncation** in transit or on the release host (digest and
  size mismatch).
- **Naive network tampering**, where an interceptor cannot serve a valid
  certificate for the allowlisted HTTPS hosts.
- **Downgrade and replay** onto an older or equal release, via the applied
  floor.

It does **not** protect against an adversary who can rewrite the release assets
**and** the checksum source — that is, anyone able to publish or replace both
the artifact and its `release-manifest.json` on the trusted origin. Because the
digests and the artifacts share one unauthenticated origin, such an adversary
defeats the checksum check. That residual risk is the entire reason `checksum`
requires explicit consent and why `strict` remains the only profile official
releases use. To restore full authentication on a bundle-less build, fix the
key provisioning described in [`release-keys.md`](release-keys.md) rather than
relying on the checksum profile.

## Insecure profile (reserved)

`insecure` names "no verification" in the type system so receipts and future
work can refer to the full set of profiles. It has **no code path**: profile
resolution never yields it, and no flag, variable, or file can select it. A
build that reaches the update gate either runs `strict`, runs `checksum` under
consent, or fails closed.

## Consent surfaces

Consent is granted explicitly, per run, through exactly two equivalent
surfaces:

| Surface | Form | Notes |
|---|---|---|
| Command-line flag | `cortex-ia update --allow-checksum-updates` | Applies to this invocation only; checked by both `update` and `update --check`. |
| Environment variable | `CORTEX_IA_ALLOW_CHECKSUM_UPDATES=1` or `=true` | Only `1` and `true` (case-insensitive) grant consent; any other value, including `0`/`false`, means "not given". |

The flag wins when both are present, because it is the explicit per-run
request. Consent is never persisted: it is process-scoped and re-resolved on
every run, so a later invocation without the flag or variable returns to the
fail-closed default. There is no configuration-file key for this consent.

## Fail-closed default

Without a packaged bundle **and** without consent, the update gate returns
`updater.ErrNoTrustedKey` verbatim:

```text
Authenticated update unavailable: no trusted release key is packaged
```

This is evaluated **before any network call** — no manifest is fetched, no
release is contacted. The message is a contract, not a hint. Local and
development builds stay in this state and keep working; only official releases
built with a bundle, or builds whose operator opted into `checksum`, may run
the update path.

## Receipts and consent warnings

Every successful apply prints the active profile in its receipt line:

```text
Successfully updated cortex-ia to v0.7.0! (verification: strict)
Successfully updated cortex-ia to v0.7.0! (verification: checksum)
```

When `checksum` is active, the run additionally prints exactly one warning line
naming where consent came from — `flag` for the command-line flag, `env` for the
environment variable:

```text
Warning: updates are verified by SHA-256 checksum only (consent: flag)
```

The warning is emitted once per process, even when more than one update surface
runs.

## Related documentation

- [`configuration.md`](configuration.md) — the `--allow-checksum-updates` flag
  and `CORTEX_IA_ALLOW_CHECKSUM_UPDATES` variable in the CLI and environment
  reference.
- [`release-keys.md`](release-keys.md) — key pair generation, bundle
  provisioning, and rotation; fixing the bundle restores `strict` everywhere.
