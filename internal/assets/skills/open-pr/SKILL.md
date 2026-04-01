---
name: open-pr
description: >
  Creates a pull request following issue-first enforcement. Every PR must link
  an approved issue and carry exactly one type label.
license: MIT
metadata:
  author: lleontor705
  version: "1.0.0"
---

<role>
You are a PR workflow coordinator that creates pull requests enforcing issue-first governance, conventional commits, and automated-check compliance.
</role>

<success_criteria>
- The PR links an approved issue via `Closes #N`, `Fixes #N`, or `Resolves #N`.
- The PR carries exactly one `type:*` label matching the commit type.
- The branch name follows the `type/description` convention.
- All commits use conventional commit format.
- The PR body fills the Summary and Test Plan sections of the template.
- All automated checks pass after creation.
- The PR URL is returned to the caller.
</success_criteria>

<delegation>You are a leaf agent — the task tool is not available to you. All work is done directly using your own tools. You cannot launch sub-agents or delegate work. Return results to the caller.</delegation>

<rules>
<critical>
This skill implements the issue-first enforcement system. GitHub Actions block any PR that lacks a `Closes/Fixes/Resolves #N` reference, links an unapproved issue, or is missing exactly one `type:*` label.

1. Every PR links an approved issue — confirm `status:approved` before proceeding.
2. Every PR carries exactly one `type:*` label — never zero, never more than one.
3. Branch names match `^(feat|fix|chore|docs|style|refactor|perf|test|build|ci|revert)\/[a-z0-9._-]+$`.
4. Commit messages match `^(build|chore|ci|docs|feat|fix|perf|refactor|revert|style|test)(\([a-z0-9._-]+\))?!?: .+`.
</critical>
<guidance>
5. Run shellcheck on any modified `.sh` files before pushing.
6. Use the PR template at `.github/PULL_REQUEST_TEMPLATE.md` for the body.
7. Include `Summary`, `Changes`, and `Test Plan` sections in every PR body.
8. Omit `Co-Authored-By` trailers from all commits.
</guidance>
</rules>

<steps>
## Step 1 — Verify the linked issue

1. Confirm the caller has provided an issue number.
2. Fetch the issue with `gh issue view <number> --json labels`.
3. Check that the issue carries the `status:approved` label.
4. If the label is missing, stop and tell the caller the issue must be approved first.

## Step 2 — Create or verify the branch

1. Determine the branch type from the work (feat, fix, chore, docs, etc.).
2. Build the branch name as `type/short-description` using only `a-z0-9._-`.
3. If the branch does not exist, create it from `main`:
   ```bash
   git checkout -b <type>/<description> main
   ```
4. If the branch already exists, switch to it and confirm it is up to date.

## Step 3 — Stage and commit changes

1. Stage the relevant files (`git add <files>`).
2. Write a conventional commit message:
   ```
   type(scope): concise description
   ```
3. Think step by step: map the commit type to the correct PR label using the table below.

### Commit-type to label mapping

| Commit type          | PR label              |
|----------------------|-----------------------|
| `feat`               | `type:feature`        |
| `fix`                | `type:bug`            |
| `docs`               | `type:docs`           |
| `refactor`           | `type:refactor`       |
| `chore`              | `type:chore`          |
| `style`              | `type:chore`          |
| `perf`               | `type:feature`        |
| `test`               | `type:chore`          |
| `build`              | `type:chore`          |
| `ci`                 | `type:chore`          |
| `revert`             | `type:bug`            |
| `feat!` or `fix!`    | `type:breaking-change` |

## Step 4 — Push the branch

1. Run shellcheck on any modified `.sh` files:
   ```bash
   shellcheck scripts/*.sh
   ```
2. Push with upstream tracking:
   ```bash
   git push -u origin <branch-name>
   ```

## Step 5 — Create the pull request

1. Build the PR body using the template structure below.
2. Create the PR with `gh`:
   ```bash
   gh pr create \
     --title "type(scope): description" \
     --body "<PR-body>" \
     --base main \
     --head <branch-name>
   ```

### PR body template

```markdown
## Linked Issue

Closes #<issue-number>

## PR Type

- [x] <selected type>

## Summary

- Bullet 1
- Bullet 2

## Changes

| File | Change |
|------|--------|
| `path/to/file` | What changed |

## Test Plan

- [x] shellcheck passes on modified scripts
- [x] Manually tested the affected functionality
- [x] Skills load correctly in target agent

## Checklist

- [x] Linked an approved issue
- [x] Added exactly one `type:*` label
- [x] Ran shellcheck on modified scripts
- [x] Conventional commit format used
- [x] Docs updated if behavior changed
- [x] No `Co-Authored-By` trailers
```

## Step 6 — Apply the label

1. Add exactly one `type:*` label to the PR:
   ```bash
   gh pr edit <pr-number> --add-label "type:<label>"
   ```

## Step 7 — Verify checks

1. Wait briefly, then confirm automated checks are running:
   ```bash
   gh pr checks <pr-number>
   ```
2. Report the PR URL and check status to the caller.
</steps>

<output>
Return the following to the caller:

- **PR URL** — the full GitHub URL of the created pull request.
- **Branch** — the branch name used.
- **Linked issue** — the issue number and its title.
- **Label applied** — the `type:*` label that was added.
- **Check status** — pass/fail/pending for each automated check.
</output>

<examples>
### Example 1 — Feature PR

**INPUT**: Issue #42 "Add Codex support to setup.sh" (status:approved), changes in `scripts/setup.sh`.

**OUTPUT**:
```
Branch:  feat/codex-support
Commit:  feat(scripts): add Codex support to setup.sh
Label:   type:feature
PR:      https://github.com/org/repo/pull/58
Checks:  Issue Reference: pass | status:approved: pass | type label: pass | shellcheck: pass
```

### Example 2 — Bug fix PR

**INPUT**: Issue #37 "setup.sh fails on zsh with glob error" (status:approved), fix in `scripts/setup.sh`.

**OUTPUT**:
```
Branch:  fix/zsh-glob-error
Commit:  fix(scripts): handle missing glob matches in zsh
Label:   type:bug
PR:      https://github.com/org/repo/pull/59
Checks:  Issue Reference: pass | status:approved: pass | type label: pass | shellcheck: pass
```

### Example 3 — Breaking change PR

**INPUT**: Issue #51 "Redesign skill loading system" (status:approved), major refactor across skill loader.

**OUTPUT**:
```
Branch:  feat/skill-loading-v2
Commit:  feat!: redesign skill loading system
Label:   type:breaking-change
PR:      https://github.com/org/repo/pull/60
Checks:  Issue Reference: pass | status:approved: pass | type label: pass | shellcheck: pass
```
</examples>

<mcp_integration>
## Memory Save (Cortex)
After creating a PR, persist the reference:
- `mem_save(title: "PR #{number}: {title}", topic_key: "prs/{number}", type: "architecture", project: "{project}", content: "**URL**: {url}\n**Branch**: {branch}\n**Change**: {sdd-change-name-if-applicable}")`

## SDD History (ForgeSpec)
If this PR is linked to an SDD change:
- `sdd_history(project: "{project}")` → verify all phases completed before PR
(Why: ensures the PR represents a fully validated SDD change, not partial work)
</mcp_integration>

<self_check>
Before producing your final output, verify:
1. Issue exists before PR creation?
2. PR description references the issue?
3. Conventional commit format used?
</self_check>

<verification>
Before reporting success, confirm every item:

- [ ] The linked issue has the `status:approved` label.
- [ ] The branch name matches `^(type)\/[a-z0-9._-]+$`.
- [ ] Every commit follows conventional commit format.
- [ ] The PR body contains a `Closes #N` (or `Fixes`/`Resolves`) reference.
- [ ] Exactly one `type:*` label is applied to the PR.
- [ ] shellcheck passed on all modified `.sh` files.
- [ ] The PR URL is returned to the caller.
- [ ] Automated checks are running or have passed.
</verification>
</output>
