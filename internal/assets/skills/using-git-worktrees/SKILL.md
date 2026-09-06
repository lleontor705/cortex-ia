---
name: using-git-worktrees
description: Use when starting feature work or delegating to AGY that needs isolation from current workspace - ensures an isolated workspace exists via cortex-ia worktree or git worktree fallback
license: MIT
metadata:
  author: lleontor705
  version: "2.0.0"
---

# Using Git Worktrees in Cortex-IA

## Overview

Ensure work happens in an isolated workspace. Prefer Cortex-IA's native worktree CLI (`cortex-ia worktree`). Fall back to manual git worktrees only when no native tool is available.

**Core principle:** Detect existing isolation first. Then use native tools. Then fall back to git. Never fight the harness.

**Announce at start:** "I'm using the using-git-worktrees skill to set up an isolated workspace."

## Step 0: Detect Existing Isolation

**Before creating anything, check if you are already in an isolated workspace.**

```bash
GIT_DIR=$(cd "$(git rev-parse --git-dir)" 2>/dev/null && pwd -P)
GIT_COMMON=$(cd "$(git rev-parse --git-common-dir)" 2>/dev/null && pwd -P)
BRANCH=$(git branch --show-current)
```

**Submodule guard:** `GIT_DIR != GIT_COMMON` is also true inside git submodules. Before concluding "already in a worktree," verify you are not in a submodule:

```bash
# If this returns a path, you're in a submodule, not a worktree — treat as normal repo
git rev-parse --show-superproject-working-tree 2>/dev/null
```

**If `GIT_DIR != GIT_COMMON` (and not a submodule):** You are already in a linked worktree. Skip to Step 2 (Project Setup). Do NOT create another worktree.

Report with branch state:
- On a branch: "Already in isolated workspace at `<path>` on branch `<name>`."
- Detached HEAD: "Already in isolated workspace at `<path>` (detached HEAD, externally managed). Branch creation needed at finish time."

**If `GIT_DIR == GIT_COMMON` (or in a submodule):** You are in a normal repo checkout.

Check if the user has already indicated their worktree preference in instructions or session alignment. Before delegating implementation work, the user must explicitly select `isolated_worktree` (recommended) or `current_workspace`. If not yet declared:

> "External implementation requires selecting a workspace strategy: `isolated_worktree` (recommended) or `current_workspace`. Would you like me to set up an isolated worktree? It protects your current branch from changes."

Honor any existing declared preference without asking. If the user chooses `current_workspace`, work in place under exclusive lock rules and skip to Step 2.

## Step 1: Create Isolated Workspace

**You have two mechanisms. Try them in this order.**

### 1a. Native Cortex-IA Worktree Command (preferred)

Cortex-IA includes native worktree management commands via the CLI dispatcher:

```bash
# Create or reuse a clean isolated worktree
cortex-ia worktree create .worktrees/<feature-name>
```

The native command:
1. Automatically creates parent directories (`os.MkdirAll`).
2. Checks if the destination exists and cleans it (`CleanWorktree`: `git reset --hard HEAD` and `git clean -fd`).
3. Creates a detached worktree at current HEAD (`git worktree add --detach <path> HEAD`).
4. Returns JSON: `{"worktree": "/path/to/.worktrees/<feature-name>", "status": "ready"}`.

If the command succeeds, switch to that directory or use its absolute path for AGY delegation, and skip to Step 2.

Only proceed to Step 1b if `cortex-ia` binary is not available or fails unexpectedly.

### 1b. Git Worktree Fallback

**Only use this if Step 1a does not apply.** Create a worktree manually using git.

#### Directory Selection

Follow this priority order. Explicit user preference always beats observed filesystem state.

1. **Check instructions for a declared worktree directory preference.** If specified, use it.
2. **Check for an existing project-local worktree directory:**
   ```bash
   ls -d .worktrees 2>/dev/null     # Preferred (hidden)
   ls -d worktrees 2>/dev/null      # Alternative
   ```
   If found, use it. If both exist, `.worktrees` wins.
3. **If no guidance is available**, default to `.worktrees/` at the project root.

#### Safety Verification (project-local directories only)

**MUST verify directory is ignored before creating worktree:**

```bash
git check-ignore -q .worktrees 2>/dev/null || git check-ignore -q worktrees 2>/dev/null
```

**If NOT ignored:** Add to `.gitignore`, commit the change, then proceed.

**Why critical:** Prevents accidentally committing worktree contents to the repository.

#### Create the Worktree

```bash
path=".worktrees/$BRANCH_NAME"

git worktree add "$path" -b "$BRANCH_NAME"
cd "$path"
```

**Sandbox fallback:** If `git worktree add` fails with a permission error (sandbox denial), inform the user that sandbox blocked worktree creation and proceed in the current workspace. Ensure exclusive baseline validation is respected.

## Step 2: Project Setup

Auto-detect and run appropriate setup in the worktree directory:

```bash
# Go
if [ -f go.mod ]; then go mod download; fi

# Node.js
if [ -f package.json ]; then npm install; fi

# Rust
if [ -f Cargo.toml ]; then cargo build; fi

# Python
if [ -f requirements.txt ]; then pip install -r requirements.txt; fi
if [ -f pyproject.toml ]; then poetry install; fi
```

## Step 3: Verify Clean Baseline

Run tests to ensure the workspace starts completely clean before making changes or delegating:

```bash
# Use project-appropriate command
go test ./... / npm test / cargo test / pytest
```

**If tests fail:** Report failures immediately and ask whether to proceed or investigate. A dirty baseline makes every subsequent failure ambiguous.

**If tests pass:** Report ready.

### Report

```
Worktree ready at <full-path>
Tests passing (<N> tests, 0 failures)
Ready to implement <feature-name>
```

## Step 4: Cortex-IA Delegation & Teardown

### Delegating to AGY

When calling `cortex_ia_delegate_start`, supply the absolute path of the verified worktree:

```json
{
  "role": "implement",
  "task_id": "task-123",
  "workspace_strategy": "isolated_worktree",
  "worktree": "/absolute/path/to/.worktrees/<feature-name>",
  "instruction": "..."
}
```

Cortex-IA strictly validates that:
1. The worktree contains a `.git` pointer linked to the same repository (`--git-common-dir`).
2. The worktree HEAD matches the parent repository HEAD.
3. The worktree has no uncommitted changes or untracked files before execution.

### Teardown & Cleanup

Once changes are committed or merged into the target branch, clean up the ephemeral worktree:

```bash
# Using native command:
cortex-ia worktree drop .worktrees/<feature-name>

# Fallback git cleanup:
git worktree remove --force .worktrees/<feature-name>
git worktree prune
```

## Quick Reference

| Situation | Action |
|---|---|
| Already in linked worktree | Skip creation (Step 0) |
| In a submodule | Treat as normal repo (Step 0 guard) |
| Native `cortex-ia` tool available | Use `cortex-ia worktree create` (Step 1a) |
| No native tool | Git worktree fallback (Step 1b) |
| `.worktrees/` exists | Use it (verify ignored) |
| `worktrees/` exists | Use it (verify ignored) |
| Both exist | Use `.worktrees/` |
| Neither exists | Default to `.worktrees/` |
| Directory not ignored | Add to `.gitignore` + commit |
| Permission error on create | Sandbox fallback, work in current workspace |
| Tests fail during baseline | Report failures + ask user before proceeding |
| Work completed and merged | Run `cortex-ia worktree drop <path>` |

## Common Rationalizations

| Excuse | Reality |
|---|---|
| "I'm obviously not in a worktree — no need to check" | Run Step 0. Harness-created isolation and submodules both fool manual inspection; detection commands settle it. |
| "`git worktree add` is quicker than native CLI" | `cortex-ia worktree create` validates permissions, parent dirs, reset/clean, and detach in a single safe operation. |
| "The worktree directory is surely ignored already" | Run `git check-ignore`. An unignored worktree directory risks committing an entire duplicate repo tree. |
| "The workspace is fresh — baseline tests can wait" | A dirty baseline makes every later failure ambiguous. Run the tests now; proceeding past baseline failures requires human approval. |
| "Leaving worktrees around doesn't hurt" | Orphaned worktrees consume disk space and can lock branches from checkout. Always teardown via `cortex-ia worktree drop`. |
