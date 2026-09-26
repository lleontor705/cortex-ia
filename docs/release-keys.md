# Release Signing Keys

How cortex-ia decides that a downloaded release is authentic, and the ceremony
that decides which keys it trusts.

This document is a **plan**. No key material exists in this repository and no
step below is executed by the build or by CI. The first signing key is created
offline, by a human, when the first authenticated release is cut.

---

## 1. Why the Updater Fails Closed

`internal/updater` verifies every release manifest with Ed25519 before it
downloads an archive, so a build with no packaged key cannot install anything.
The active trust store is `updater.ProductionTrustedKeys`, and it is populated
exclusively at process start from a build-time string. A build that was never
given a bundle therefore reports:

```text
Authenticated update unavailable: no trusted release key is packaged
```

That message is a contract: it is the exact value of `updater.ErrNoTrustedKey`
and no "unsigned fallback" path exists. Local and development builds stay in
this state and keep working; only official releases are built with a bundle.

## 2. Material Inventory

| Item | Bytes | Where it lives | Where it must never live |
|---|---|---|---|
| Ed25519 private key | 64 (seed ‖ public key) | GitHub Actions secret `CORTEX_IA_RELEASE_SIGNING_KEY` | the repository, logs, ldflags, `~/.cortex-ia/`, release artifacts |
| Ed25519 public key | 32 | `productionTrustBundle` (ldflags), published in this document's examples | — |
| Trust bundle string | variable | Build input (`-X` flag or the workflow env that produces it) | committed config files |

The trust bundle is public by construction: it is embedded in every released
binary and can be read out of it. Only the private key is secret.

## 3. Bundle Format

`internal/updater/trust.go` declares one injection target:

```go
var productionTrustBundle string
```

The value is **standard (padded) base64 of the UTF-8 JSON encoding of a
`[]updater.TrustedKey` array**:

```json
[
  {
    "ID": "release-2026-01",
    "PublicKey": "gTpJ3Y0kDqk2yGJc8z6uW1f4b0n8cQ7xVk1sZ9pT2uE=",
    "Repository": "lleontor705/cortex-ia",
    "MinVersion": "v0.6.0",
    "MaxVersion": ""
  }
]
```

Field rules, all enforced by the existing `TrustedKey` structure and
`ValidateTrustStore`:

- `ID` — stable, non-empty identifier. It is echoed in the signature envelope
  (`release-manifest.sig` → `key_id`) and selects the record during
  verification.
- `PublicKey` — base64 of the raw 32-byte Ed25519 public key (Go marshals
  `[]byte` this way; the decoded length must be exactly `ed25519.PublicKeySize`).
- `Repository` — `owner/repo` binding. A manifest signed by this key is only
  accepted for that repository (`ErrRepositoryMismatch` otherwise).
- `MinVersion` / `MaxVersion` — inclusive canonical `vX.Y.Z` interval. An empty
  `MaxVersion` means "no ceiling", which is only acceptable for the newest key.
- At most **two** records (`updater.MaxTrustedKeys`). Three or more fail
  `ValidateTrustStore` with `ErrTooManyKeys`.

The bundle is decoded by `decodeTrustBundle` and published by `applyTrustBundle`
during package initialization. Decoding is transactional: an unset bundle yields
an empty store, and a malformed bundle (bad base64, non-array JSON, a key that is
not 32 bytes, an empty `ID`, or more than two records) is rejected with
`updater.ErrTrustBundleInvalid` and leaves the active store untouched. There is
no partial activation, ever.

## 4. Generating the First Pair (Offline)

Generate the pair on an offline machine with the standard library only; no new
dependency and no network access is required:

```go
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

func main() {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		panic(err)
	}
	fmt.Println("private (secret):", base64.StdEncoding.EncodeToString(priv))
	fmt.Println("public  (bundle):", base64.StdEncoding.EncodeToString(pub))
}
```

1. Store the printed private key as the GitHub Actions secret
   `CORTEX_IA_RELEASE_SIGNING_KEY`. It is never committed, printed in CI logs,
   or written to a file that leaves the machine.
2. Keep one offline copy in the project's password manager as the recovery
   escrow, and record the key `ID` and creation date there.
3. Discard the generator program. Only the two base64 strings remain.

## 5. Packaging the Bundle into a Build

The bundle is an ordinary `-X` string assignment. `BUNDLE` is the base64 string
produced in the next section:

```bash
go build -o bin/cortex-ia ./cmd/cortex-ia \
  -ldflags "-X main.version=v0.6.0 \
            -X github.com/lleontor705/cortex-ia/internal/updater.productionTrustBundle=$BUNDLE"
```

The GoReleaser workflow (task `aup-02`) performs the same assignment through its
own `ldflags` entry, taking `BUNDLE` from the repository variable
`CORTEX_IA_TRUST_BUNDLE`. Both names — `CORTEX_IA_RELEASE_SIGNING_KEY` for the
private seed and `CORTEX_IA_TRUST_BUNDLE` for the public bundle — are the
canonical contract; the workflow must fail closed when either is absent rather
than silently producing an unsigned release.

## 6. Building the Bundle String from a Public Key

Given the base64 public key from step 4 and the intended validity interval:

```bash
JSON='[{"ID":"release-2026-01","PublicKey":"<base64-32-byte-public-key>",
        "Repository":"lleontor705/cortex-ia","MinVersion":"v0.6.0","MaxVersion":""}]'
printf '%s' "$JSON" | base64 -w0
```

The printed value is the bundle. Two properties are worth checking by hand
before it is pasted into CI:

- The JSON is an array (a single object is rejected).
- The decoded public key length is 32 bytes.

> **Provisioning and verification profiles.** A correctly provisioned bundle —
> present, well-formed, and covering the release tag — selects the `strict`
> profile for every build that carries it: Ed25519 signature verification is
> restored everywhere and no consent is required. When the key pair is missing,
> mismatched, or otherwise not packaged, builds fall back to the fail-closed
> default and cannot install anything. The `checksum` profile exists precisely
> as the consent-gated bootstrap for that interim state: an operator can opt in
> per run to verify artifacts by SHA-256 checksum and size without a bundle.
> Fixing the bundle is the permanent remedy; the checksum profile is a
> deliberate, explicit bridge while keys are mismatched, not a replacement for
> signing. See [`updater-verification-profiles.md`](updater-verification-profiles.md)
> for the profiles, both consent surfaces, and the checksum threat model.

## 7. Rotation (Maximum Two Live Keys)

Rotation never replaces a key in place. The new key is added as a second record
whose `MinVersion` is the first release it signs, and the outgoing key is capped
with `MaxVersion` equal to the last release it signs. Because both bounds are
inclusive, the boundary release may be verified by either key, which is what
makes the overlap safe. The validity check is exercised in
`internal/updater/trust_rotation_test.go`.

1. Generate the replacement pair offline (step 4) with a new `ID`, for example
   `release-2027-01`.
2. Decide the boundary release `R`: the first release signed with the new key.
3. Build the bundle with exactly two records:

   ```json
   [
     {"ID": "release-2026-01", "PublicKey": "<old>",
      "Repository": "lleontor705/cortex-ia",
      "MinVersion": "v0.6.0", "MaxVersion": "v0.9.3"},
     {"ID": "release-2027-01", "PublicKey": "<new>",
      "Repository": "lleontor705/cortex-ia",
      "MinVersion": "v0.9.3", "MaxVersion": ""}
   ]
   ```

4. Sign and publish releases up to and including `R` with the old private key,
   then switch signing to the new key. Release `R` itself is signed by the new
   key so that its `key_id` matches a record whose interval covers it.
5. Once the oldest supported version is above `v0.9.3`, drop the old record.
   The bundle returns to one record and the old key is destroyed.

Cadence: rotate at least once every 12 months, and whenever a maintainer with
access to the CI secret leaves the project. Never let the bundle reach three
records — `MaxTrustedKeys` is a hard ceiling, not a guideline.

## 8. Compromise and Revocation

If a private key is suspected to be exposed:

1. **Stop releasing.** No new tags until a replacement key exists.
2. Generate the replacement pair offline and update
   `CORTEX_IA_RELEASE_SIGNING_KEY`.
3. Cap the compromised key with `MaxVersion` equal to the last release that was
   legitimately signed with it, and give the replacement key a `MinVersion`
   above that ceiling. Keep at most two records: if the compromised key and a
   healthy key already occupy both slots, drop the compromised record instead of
   capping it.
4. Ship a release signed by the **still-trusted** key. Every build carrying the
   capped bundle then refuses manifests signed by the compromised key with
   `ErrVersionOutOfRange`, including replays of a previously published tag.
5. Users already running a release that the compromised key signed must
   reinstall from a release signed by the trusted key; the updater cannot prove
   that a running binary predates the compromise.
6. Record the incident in Cortex (`bugfix/<issue>`) with the compromised `ID`,
   the ceiling applied, and the release that carried the fix.

## 9. Verifying the Injection Path

Before shipping a bundle, confirm that the build actually activates it. Without
a bundle the store must stay empty:

```bash
go test ./internal/updater -run TestTrustBundle -count=1
```

`TestTrustBundleInjectedBuildActivatesProductionStore` only has work to do when
the test binary was built with an injected bundle, so the end-to-end ldflags
probe is:

```bash
go test ./internal/updater -run TestTrustBundle -count=1 -v \
  -ldflags "-X github.com/lleontor705/cortex-ia/internal/updater.productionTrustBundle=$BUNDLE"
```

The probe must **run** (not skip) and report `PASS`; a skip means the `-X`
target did not match and the release would have shipped without a key.

Tests inject keys only through `updater.SetTrustedKeysForTesting`. No test, and
no CI job, ever receives real private key material.

## Provisioning Log

| Date | Key ID | Repository | Scope |
|---|---|---|---|
| 2026-09-23 | `release-2026-09-root` | `lleontor705/cortex-ia` | Root key for the `v0.1.0` line: `MinVersion` `v0.1.0`, empty `MaxVersion` (no ceiling). The pair was generated offline with the section 4 program; the private key is stored as the `CORTEX_IA_RELEASE_SIGNING_KEY` Actions secret and escrowed in the project password manager, while the public bundle is stored as the `CORTEX_IA_TRUST_BUNDLE` Actions variable and `CORTEX_IA_RELEASE_KEY_ID` mirrors this ID. No key material is recorded here. |
