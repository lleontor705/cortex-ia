---
name: bootstrap
description: >
  Initialize SDD context by detecting the project stack, conventions, test
  capability, and available phase assets.
  Trigger: When the operator starts SDD in a new or unknown project.
license: MIT
metadata:
  author: lleontor705
  version: "1.0.0"
---

# Bootstrap — SDD Project Initialization

## Objective

Produce a verified project context and an inventory of phase assets. Bootstrap
does not design a change or make implementation decisions. It establishes the
facts that later phases cite.

## Activation

Activate when the operator requests initialization, when no project context is
available, or when the selected repository has changed. The input names the
project, change (when known), artifact store, and repository root.

## Method

Probe budget: at most 8 file reads and 10 tool calls before checkpoint.

1. Inspect the manifest that identifies the language and runtime (`go.mod`,
   `package.json`, `pyproject.toml`, or its equivalent). Record exact versions
   and cite file paths and lines.
2. Inspect test, formatting, lint, and CI configuration. Record the exact test
   command, test-file pattern, coverage command when present, and whether
   test-first work is expected.
3. Inspect top-level directories and one representative entry point. Name the
   observed architecture only when source evidence supports it.
4. Inventory available phase skills and project convention files. Read their
   front matter, deduplicate by canonical name, and record the selected path.
5. Emit a compact context record with evidence, limitations, and freshness.

## Decision gates

- `contract/init`: every stack claim has a source citation. If a category is
  not observable, record `partial` and stop short of guessing.
- `contract/registry`: the phase registry contains one selected entry per
  canonical skill and no path that cannot be read.
- `contract/test-capability`: the returned test command is executable or is
  explicitly marked unavailable with a reason.

Do not continue to proposal work when a required category is unknown. Return a
blocked result with the missing evidence and the smallest next inspection.

## Valid example

Given a Go repository containing `go.mod`, `*_test.go`, and a CI workflow,
bootstrap records Go and its version, `go test ./...`, the test pattern, and the
workflow path. The result is `success` only after each fact has a citation and
the phase registry resolves all expected entries.

## Invalid example

Given a directory named `service` with no readable manifest, bootstrap MUST NOT
infer a language or claim that tests exist. It returns `blocked`, identifies
the absent manifest, and leaves downstream phase activation unavailable.

## Output checks

Return a structured result containing project, stack, conventions, test
capability, architecture observation, registry entries, evidence references,
limitations, status, and confidence. Use canonical phase status values from the
generated contract. Keep aliases in presentation fields only. Verify that the
registry and context are internally consistent before returning.

## Boundary discipline

Bootstrap owns observation only. It may read files, normalize their paths, and
report evidence, but it must not edit source, choose an implementation, or
interpret an absent fact as a capability. A source citation is meaningful only
when the cited bytes were read during this activation. Keep generated contract
names stable and place human-readable labels in presentation fields. When two
files disagree, preserve both observations, identify the conflict, and return a
partial result with a clear follow-up. When the repository is too large for the
probe budget, checkpoint the inventory, state what was not inspected, and avoid
claiming completeness. A later phase may resume from the recorded evidence.
Repeatability matters: record the command, revision, timestamp, and source
fingerprints needed to reproduce the observation.
Do not replace a missing citation with confidence language.
Freshness evidence is mandatory for detected capabilities.

## References

- `_shared/sdd-phase-contract.md` — result envelope and status vocabulary.
- `contract/init`, `contract/registry`, `contract/test-capability` — executable
  gate identifiers.
- `internal/components/sdd/phasecontract` — canonical contract definitions.
- `internal/components/sdd/contractgen` — generated reference source.
