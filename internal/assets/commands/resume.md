---
description: Recover authoritative state and continue eligible work
agent: orchestrator
subtask: false
---

Resume from durable `cortex_work_*` state, OpenSpec artifacts, and referenced Cortex evidence. Before saying an attempt is still live, call `cortex_work_status`: `status=in_progress` is only durable lifecycle state and is insufficient; live implement authority additionally requires `bridge_authority.usable=true`, `owned_by_current_session=true`, and `durable_claim_live=true`, while any further write requires `bridge_authority.write_usable=true`. When native child state or its bridge handle was lost, use a background-recovery tool only if the effective runtime exposes it, then call `cortex_work_recover`; recovery never restores claim or lease handles. Read the post-recovery revision, retry explicitly, and dispatch only work currently ready: $ARGUMENTS
