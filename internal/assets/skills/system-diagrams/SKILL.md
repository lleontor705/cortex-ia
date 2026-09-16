---
name: system-diagrams
description: "Author, validate, render interactive HTML/SVG, and compare system architecture, workflow, sequence, dataflow, and lifecycle diagrams using cortex_ia_diagram_* tools."
license: MIT
metadata:
  author: cortex-ia
  version: "1.0.0"
---

# System Diagrams & Architecture Maps (Inspired by Archify)

Use this skill to turn codebases, service topologies, workflows, and state machines into validated, interactive system maps.

## 1. Tool Surface

- `cortex_ia_diagram_validate`: Validate diagram JSON specification against topological and schema rules.
- `cortex_ia_diagram_render`: Render standalone interactive HTML diagram with SVG, dark/light themes, and node exploration.

## 2. Diagram Type Router

| Type | Best For | Key Elements |
|---|---|---|
| **`architecture`** | Services, storage, cloud/security boundaries, infrastructure | Components (`frontend`, `backend`, `database`, `cloud`, `security`), boundaries (`wraps`), connections |
| **`workflow`** | Process steps, CI/CD, multi-agent coordination, approvals | Steps, transitions, conditional branches |
| **`sequence`** | API call chains, async message traces, request lifecycles | Participants, ordered message exchanges |
| **`dataflow`** | Pipelines, ETL/ELT, lineage, governance, consumers | Sources, transforms, stores, data links |
| **`lifecycle`** | State transitions, error handling, retries, terminal states | States, events, error & recovery transitions |

## 3. Fast Authoring & Delivery Loop

1. **Write Candidate Specification**:
   Produce typed JSON with `meta`, `components`, `connections`, and optional `boundaries` / `cards`.
2. **Validate**:
   Run `cortex_ia_diagram_validate({ spec_path: "path/to/spec.json", quality: "showcase" })`.
   Ensure 0 errors and valid topology (all connection endpoints point to existing component IDs).
3. **Render**:
   Run `cortex_ia_diagram_render({ spec_path: "path/to/spec.json", output_path: "docs/architecture.html" })`.
   Produces a self-contained HTML artifact with embedded SVG and interactive dark/light theme.
