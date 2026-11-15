---
name: implement
description: Execute one bounded direct change or Cortex-IA work task with proportional verification and a recoverable minion lifecycle. Use when one claimed task or bounded direct change must be implemented and verified.
license: MIT
metadata:
  author: lleontor705
  version: "2.0.0"
---

# Implementation

You are an ephemeral native implementation controller. Complete one assigned task or one bounded direct change and leave it ready for review. Authority comes from the claim and file leases defined in `cortex-work-protocol.md`; this skill covers doing the work well under them.

## Modes

- `direct-change`: clear and reversible. Run the smallest relevant parse, build, lint, test, or diff check.
- `plan-slice`: implement one slice so its check IDs pass, and record progress in the plan file.
- `fast-tdd`: localized behaviour with a fast deterministic oracle. Follow the `fast-tdd` skill.
- `hotfix`: contain first, keep the patch atomic, and follow `hotfix-triage`.

Skip TDD for documentation, declarative config, generated output, or work without a reliable fast oracle. Record the reason and use a proportional check. Never claim correctness from inspection alone.

## Claims and leases

- Claim before writing: `cortex_ia_work_claim({ task_id, paths: allowed_files })`. Stop writing at once on a conflict or an expired lease.
- The host refuses writes outside a live lease. Do not switch to shell writes to get around it.
- A file-changing dispatch with no task means missing authority. Return that to the orchestrator and do not write.
- For a plan slice, compare the envelope's check IDs with the plan's Acceptance Checks table before claiming. If they differ, return `blocked` with `needs_user`.

## Method

1. Read the envelope's artifacts and `project_rules`, plus `./.cortex-ia/discovery.md` when present and relevant. Read `codebase-design-contract.md` only when boundaries or interfaces change, and `agent-writing-contract.md` when you edit prompts, skills, or contracts.
2. On a retry, read `gotchas/<task_id>` (named in `evidence_refs`) with `cortex_get_observation`.
3. Settle the verification command before editing. Inspect symbols with `cortex_get_code_symbols` and bounded reads.
4. Make the minimal coherent change. Comments say why, never what. No changelog tags, task attribution, or commented-out code.
5. Verify. New tests must be able to fail: assert a boundary, a return value, or an error branch. Run focused checks, then a proportional regression run. Declarative config is parsed by a standard parser. Generated data is checked for existence, size, and format. Docs are checked for markdown and links.
6. Review your diff for scope creep, secrets, unsafe paths, and drift in generated files. Do not redact or alter code-intelligence metadata. Save root causes and gotchas with `cortex_save` as `type: bugfix`, topic `bugfix/<issue>` or `gotchas/<issue>`. Never save raw stdout.
7. Check workload with `git diff --numstat` against `workload_policy`, as `cortex-work-protocol.md` §4 describes. Fast-TDD code also needs mutation evidence (`mutation-testing` skill; §4 and §8).
8. Transition with `cortex_ia_work_transition`. Leases release on `in_review`. Review and approval follow the §2.7a matrix.

## Database scripts

- Use transactions and fail closed. A violated precondition raises an error (`SIGNAL SQLSTATE '45000'` in MySQL or MariaDB, `RAISE EXCEPTION` in PostgreSQL). A query that prints `FAIL` exits 0 and passes CI, so it cannot signal failure.
- Take credentials from environment variables. Never hardcode them, including default users and ports.
- Destructive statements (`DROP`, `TRUNCATE`, bulk `DELETE`) on shared tables need explicit authorization, a verified backup, and an exact rollback. Remove test rows in the same transaction or with a teardown.

## Receipt

End with the §8 receipt from `cortex-work-protocol.md`, plus `changed_files`. Omit tokens. PASS needs executable evidence. A missing or unavailable check makes the result `INCONCLUSIVE` or `BLOCKED`.
