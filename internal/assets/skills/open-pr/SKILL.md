---
name: open-pr
description: "Prepare a pull request with issue linkage, focused scope, conventional history, and a complete test plan."
license: MIT
metadata:
  author: lleontor705
  version: "1.0.0"
---
<role>Non-phase utility authority for pull request preparation.</role>
<success_criteria>The PR links its approved issue, has the correct branch and label metadata, explains changes and tests, and reports checks honestly.</success_criteria>
<context>Use only when the branch is ready for review. This utility prepares review material and follows repository contribution policy.</context>
<rules><critical>Inspect status and diff before publication. Never claim checks passed without evidence.</critical><guidance>Keep scope focused, preserve conventional history, include rollback notes, and omit secrets.</guidance></rules>
<steps>1. Verify issue and branch. 2. Review diff and history. 3. Draft summary, changes, risks, and test plan. 4. Run required checks. 5. Confirm labels and linkage before returning the PR details.</steps>
<output>Return issue linkage, branch, title, body, labels, checks, changed scope, and PR URL when created.</output>
<references>Use `.github/PULL_REQUEST_TEMPLATE.md` and the repository contribution guide.</references>
