# Translations

Cortex-IA publishes a bilingual front door: **English is the single source of truth** and Spanish (Español) is the first supported translation. This document defines the coverage matrix for the front-door files and the rules that keep every variant in sync.

## Coverage matrix

| Archivo EN (canonical) | Variante ES | Estado de sincronización |
| --- | --- | --- |
| README.md | README.es.md | `synced` |
| CONTRIBUTING.md | CONTRIBUTING.es.md | `synced` |
| SECURITY.md | SECURITY.es.md | `synced` |
| CODE_OF_CONDUCT.md | CODE_OF_CONDUCT.es.md | `synced` — official Contributor Covenant 2.1 Spanish text (`https://www.contributor-covenant.org/es/version/2/1/code_of_conduct/`), same enforcement contact as the EN file |
| SUPPORT.md | SUPPORT.es.md | `synced` |

Allowed states: `synced` (translation matches the current canonical file) and `outdated` (canonical file changed and the twin has not been updated yet).

## Source of truth

- The English file is canonical. Its technical content — commands, code blocks, badge URLs, and relative links — is authoritative and must be preserved byte-for-byte in every translation.
- A translation may localize prose, headings, and internal anchors, but never invent policies, badges, or links that the canonical file does not contain.
- When English and a translation disagree, English wins until the translation is refreshed.

## Synchronization policy

1. Any PR that touches a canonical EN front-door file **must update its `.es` twin in the same PR**.
2. If a same-PR update is not possible, leave the twin untouched and mark its row `outdated` in this matrix with a short reason, so drift is explicit rather than silent.
3. A reviewer must reject a PR that changes an EN front-door file while leaving a stale row marked `synced`.
4. Refresh an `outdated` row back to `synced` as soon as the twin is updated.

## Adding a new locale

1. Create the translated file using the naming convention `README.<lang>.md` (for example `README.pt.md`), mirroring the canonical file name plus the BCP 47 language subtag.
2. Add the reciprocal language selector as the **first line** of the translated file, for example `[English](README.md) | **Português**`, and add the matching forward link to the canonical EN file's selector line.
3. Add a row to the coverage matrix above with the canonical file, the new variant, and its sync state.
4. Keep the selector reciprocity bidirectional: every localized file links back to the canonical EN file, and the canonical EN file links to each available locale.
