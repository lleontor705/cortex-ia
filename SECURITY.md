# Security Policy

**English** | [Español](SECURITY.es.md)

Cortex-IA is a local bridge and control plane for the OpenCode ecosystem. We take
security reports seriously and ask that you disclose issues responsibly.

## Supported Versions

Cortex-IA follows a rolling release model. Only the **latest minor release**
receives security fixes; older releases are supported through upgrades only.

| Version              | Supported          |
| -------------------- | ------------------ |
| Latest minor release | :white_check_mark: |
| Older releases       | :x:                |

Release history is recorded in [CHANGELOG.md](CHANGELOG.md). Please confirm your
finding against the latest release before reporting.

## Reporting a Vulnerability

**Do not open a public issue or pull request for security vulnerabilities.**

Report privately through GitHub Security Advisories:

- <https://github.com/lleontor705/cortex-ia/security/advisories/new>

Please include, where possible:

- The affected version, commit, or branch.
- A clear description of the issue and its security impact.
- Reproduction steps or a minimal proof of concept.
- A suggested remediation, if you have one.

## Disclosure Policy

We follow coordinated disclosure:

1. We acknowledge new reports within **72 hours**.
2. We investigate, confirm the severity, and prepare a fix.
3. We release the fix and publish a GitHub Security Advisory describing the
   issue and the affected versions.
4. We credit the reporter in the advisory unless anonymity is requested.

Please give us a reasonable window to release a fix before any public
disclosure.

## Scope

This policy covers the `cortex-ia` CLI/TUI binary and the source code in this
repository. Vulnerabilities in third-party dependencies should be reported
upstream to their respective maintainers.
