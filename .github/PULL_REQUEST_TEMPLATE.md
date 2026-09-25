<!--
Cortex-IA enforces the rules below in CI. See .github/workflows/pr-check.yml.
-->

## What & Why

<!-- Describe the change and the problem it solves. Reference the approved issue. -->

Closes #

## How Tested

<!-- List the exact commands you ran and their results. -->

## Checklist

- [ ] Branch matches `<type>/<kebab-case>`, where `<type>` is one of:
      `feat`, `fix`, `chore`, `docs`, `style`, `refactor`, `perf`, `test`,
      `build`, `ci`, `revert`.
- [ ] PR body contains `Closes #N`, `Fixes #N`, or `Resolves #N`.
- [ ] The linked issue carries the `status:approved` label.
- [ ] Exactly one `type:*` label is applied: `type:bug`, `type:feature`,
      `type:docs`, `type:refactor`, `type:chore`, `type:breaking`, or
      `type:test`.
- [ ] Commit subject follows Conventional Commits and is 10–72 characters.
- [ ] Documentation and tests were updated alongside the change.
