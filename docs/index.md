<p align="center">
  <img src="assets/hero-banner.svg" alt="Cortex-IA hero banner" width="100%" />
</p>

# Cortex-IA

The deterministic **multi-agent control plane & orchestration engine** for autonomous software
development with **OpenCode** — a single portable Go binary that makes parallel agent work safe.

[![Release](https://img.shields.io/github/v/release/lleontor705/cortex-ia?color=38BDF8&label=release)](https://github.com/lleontor705/cortex-ia/releases/latest)
[![License](https://img.shields.io/github/license/lleontor705/cortex-ia?color=A855F7&label=license)](https://github.com/lleontor705/cortex-ia/blob/main/LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/lleontor705/cortex-ia)](https://goreportcard.com/report/github.com/lleontor705/cortex-ia)
[![Platforms](https://img.shields.io/badge/platforms-Windows%20%7C%20Linux%20%7C%20macOS-blue)](https://github.com/lleontor705/cortex-ia)

## Get started

| 🚀 Quickstart | ⚙️ Installation | 💻 Non-interactive CLI | 🏛️ Architecture |
|---|---|---|---|
| Your first coordinated agent workflow in three steps. | Precompiled binaries, `go install`, and the install script. | Scripting, CI, and Docker recipes with JSON receipts. | Engine layers, SQLite task authority, and safety invariants. |
| [Get started →](quickstart.md) | [Install →](installation.md) | [Automate →](non-interactive.md) | [Explore →](architecture.md) |

## What is Cortex-IA?

**Cortex-IA** is the operational control plane for multi-agent coding on OpenCode. It addresses the
failure modes of parallel agents — race conditions, conflicting file edits, hallucinated task
readiness, and unstructured coordination — by owning a deterministic task DAG in ACID SQLite and
requiring exclusive file leases before any agent writes code.

Tasks advance through a strict state machine (`backlog → ready → in_progress → in_review → done`)
with optimistic CAS locking, and no task is complete until an independent reviewer records a `PASS`.
The Cortex MCP server complements this with AST knowledge graphs, blast-radius analysis, and durable
cross-session memory — advisory evidence only, never a substitute for work authority.

## Get involved

- Source code and releases: [github.com/lleontor705/cortex-ia](https://github.com/lleontor705/cortex-ia)
- Contribution guide: [CONTRIBUTING.md](https://github.com/lleontor705/cortex-ia/blob/main/CONTRIBUTING.md)
