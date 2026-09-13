# OpenCode Harness SDK & Plugin Qualification

Requirements: REQ-HARNESS-002, REQ-CONTEXT-001, REQ-MCP-001.

## 1. Locked Dependency Manifest and Integrities

The OpenCode harness SDK and plugin environment is pinned and qualified against authentic locked manifests under `internal/tuiassets/`:

| Package | Version | Manifest | Package-Lock Integrity (SHA-512) |
|---|---|---|---|
| `@opencode-ai/plugin` | `1.18.18` | `internal/tuiassets/package.json` | `sha512-vqQeqJtn9c+J+tIQDzYk88xip/NVNN1hym1ATmckxo6zINHAoXoul4Sw/jgnvL00rLsfAvhja28qax4h3g/5Jg==` |
| `@opencode-ai/sdk` | `1.18.18` | `internal/tuiassets/package-lock.json` | `sha512-zJlwXskIR47V1dkPJqeKBgq7nejG1uU8lJaGIGqbX3MWRCT8vKn0fEotbxuPCKnTdmWsDyNGNg9q1qIliDSMDA==` |
| `typescript` | `5.9.3` | `internal/tuiassets/package.json` | `sha512-jl1vZzPDinLr9eUt3J/t7V6FgNEw9QjvBPdysz9KfQDD41fQrC2Y4vKQdiaUpFT4bXlb1RHhLpp8wtm6M5TgSw==` |

Both `package-lock.json` and `pnpm-lock.yaml` agree on the `1.18.18` and `5.9.3` resolutions.

## 2. Transport Comment Discrepancy Resolution

An observational comment in `internal/assets/plugins/cortex-subagent-transport.ts:185` references `v1.18.29`. This reference is non-normative narrative prose. The authentic locked dependency installed, verified, and active in the repository runtime is `@opencode-ai/plugin@1.18.18`. Dependency upgrades are prohibited without explicit qualification.

## 3. Isolated Loader Architecture

The isolated harness plugin loader (`scripts/harness-plugin-loader.mjs`) provides:
- **Pinned Transpilation**: TypeScript source is transpiled to CommonJS via pinned TypeScript `5.9.3`.
- **Replaced Boundaries**: Unapproved modules (`child_process`, `node:net`, `http`) are rejected with `UNAUTHORIZED_MODULE_IMPORT`.
- **Environment Isolation**: The execution context receives only bounded environment variables; ambient secrets, tokens, and credentials cannot leak.
- **Contract-Faithful Execution**: Real plugin hooks (`dispose`, `event`, `experimental.chat.system.transform`) execute deterministically within the sandbox.

## 4. Verification

Execute the qualification suite:
```bash
node --test scripts/harness-plugin-loader.test.mjs && node scripts/qualify-harness-sdk.mjs --locked --isolate --require-plugin 1.18.18 --require-typescript 5.9.3
```
All tests pass exit 0.
