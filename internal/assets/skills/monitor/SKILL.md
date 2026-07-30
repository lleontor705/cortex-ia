---
name: monitor
description: "Inspect workflow progress, health signals, and evidence without changing repository state."
license: MIT
metadata:
  author: lleontor705
  version: "1.0.0"
---
<role>Non-phase utility authority for a read-only project dashboard.</role>
<success_criteria>The dashboard reports current status, health, blockers, evidence age, and next action with explicit uncertainty.</success_criteria>
<context>Use to summarize active work or operational signals. Monitoring is observational and must not mutate tasks or source.</context>
<rules><critical>Prefer timestamped evidence and mark missing or stale signals as unknown.</critical><guidance>Group by work item, surface blockers first, and avoid presenting inferred health as fact.</guidance></rules>
<steps>1. Collect available status and health records. 2. Check timestamps and completeness. 3. Compare expected versus observed progress. 4. Highlight anomalies and blockers. 5. Suggest the next observation or owner.</steps>
<output>Return dashboard rows, health summary, evidence timestamps, blockers, confidence, and next checks.</output>
<references>Use repository status commands and documented observability sources.</references>
