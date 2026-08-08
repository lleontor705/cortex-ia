# Harden Configuration Installer

## Status

Approved for specification. Implementation is intentionally split into independent security and product-scope phases.

## Problem

cortex-ia installs configuration for Claude Code, OpenCode, VS Code Copilot, and Codex. Current mutation paths can escape trusted roots, weaken permissions, leave partial installations, or lose pre-existing files during rollback. Public commands also execute AI engines, install global software, and perform model routing beyond the product boundary of installing configuration.

## Goals

- Confine every mutation to a trusted home, adapter root, or backup root.
- Make install and uninstall succeed completely or restore a verified pre-operation state.
- Preserve existing file and directory permissions.
- Keep exactly four supported adapters and keep GGA removed.
- Preserve SDD as installable configuration payload.
- Remove public execution of AI engines, global package installation, and model routing.

## Non-Goals

- Adding adapters or MCP contracts.
- Following symlinks in managed roots.
- Automatically migrating unsafe legacy manifests.
- Deleting runtime data owned by Agent Mailbox or other external services.
- Removing SDD prompts, skills, commands, or renderer payloads.

## Delivery Order

1. Filesystem confinement and destructive-command hotfixes.
2. Permission-preserving atomic writes.
3. Trusted backup store and collision-resistant IDs.
4. Transactional install.
5. Transactional uninstall.
6. Product-surface reduction and documentation.
