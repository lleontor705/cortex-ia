---
description: Recover authoritative state and continue eligible work
agent: orchestrator
subagent: false
---

Resume from durable state: `cortex_ia_work_list` and `cortex_ia_work_status`, the plan file (`docs/plans/<board-id>.md`) when one exists, and the referenced Cortex evidence.

Check `cortex_ia_work_status` before calling an attempt live. `in_progress` alone is not enough. The compact status reports `authority.usable` and `authority.write_usable`; live implement authority needs `authority.usable=true`, and any further write needs `authority.write_usable=true`. Call with `detail: true` to see `owned_by_current_session` and `durable_claim_live`.

If a child's native state or bridge handle was lost, use a background-recovery tool only if the runtime provides one, then call `cortex_ia_work_recover`. Recovery never restores claim or lease handles. Read the revision after recovery, retry explicitly, and dispatch only tasks that are `ready`. Scope: $ARGUMENTS
