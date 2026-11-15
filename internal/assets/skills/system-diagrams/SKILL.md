---
name: system-diagrams
description: "Author, validate, render interactive HTML/SVG, and compare system architecture, workflow, sequence, dataflow, and lifecycle diagrams using cortex_ia_diagram_* tools. Use when a codebase, workflow, or state machine needs a diagram."
license: MIT
metadata:
  author: cortex-ia
  version: "1.0.0"
---

# System Diagrams

Use this to turn a codebase, service topology, workflow, or state machine into a diagram that has been validated before it is shared.

## Choosing the type

| Type | Use for | Key elements |
|---|---|---|
| `architecture` | Services, storage, cloud and security boundaries | Components (`frontend`, `backend`, `database`, `cloud`, `security`), boundaries (`wraps`), connections |
| `workflow` | Process steps, CI/CD, approvals, multi-agent coordination | Steps, transitions, conditional branches |
| `sequence` | API call chains, async messages, request lifecycles | Participants, ordered messages |
| `dataflow` | Pipelines, ETL/ELT, lineage, consumers | Sources, transforms, stores, data links |
| `lifecycle` | State transitions, errors, retries, terminal states | States, events, error and recovery transitions |

## Steps

1. **Write the spec.** Produce typed JSON with `meta`, `components`, and `connections`, plus `boundaries` or `cards` when they add meaning.
2. **Validate.** Run `cortex_ia_diagram_validate({ spec_path: "path/to/spec.json", quality: "showcase" })`. Fix every error. Every connection must point at an existing component ID.
3. **Render.** Run `cortex_ia_diagram_render({ spec_path: "path/to/spec.json", output_path: "docs/architecture.html" })`. The result is a self-contained HTML file with inline SVG and a light and dark theme.
