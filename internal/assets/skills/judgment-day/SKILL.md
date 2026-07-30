---
name: judgment-day
description: "Run an adversarial dual review of a proposed change before merge."
license: MIT
metadata:
  author: cortex-ia
  version: "1.0.0"
---
<role>Non-phase utility authority for independent adversarial review.</role>
<success_criteria>Independent judges inspect the same evidence, disagreements are resolved transparently, and critical or high findings are actionable.</success_criteria>
<context>Use before merge when independent scrutiny is valuable. Review the diff and evidence, not author confidence.</context>
<rules><critical>Keep judge perspectives blind until both reports exist. Distinguish defects, warnings, and suggestions.</critical><guidance>Require file and test evidence for findings and avoid inventing requirements.</guidance></rules>
<steps>1. Define review criteria. 2. Inspect the change independently twice. 3. Record findings with evidence. 4. Compare and adjudicate conflicts. 5. Return disposition and residual risk.</steps>
<output>Return judge reports, consensus findings, severity, evidence links, and merge recommendation.</output>
<references>Use the change specification, acceptance criteria, and repository review policy.</references>
