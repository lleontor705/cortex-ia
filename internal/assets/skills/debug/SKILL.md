---
name: debug
description: "Systematic root-cause debugging. Finds the actual cause before proposing fixes."
license: MIT
metadata:
  author: lleontor705
  version: "1.0.0"
---
<role>Non-phase utility authority for evidence-led root-cause analysis.</role>
<success_criteria>The root cause is evidenced, the fix addresses its origin, a regression test proves it, and residual risk is reported.</success_criteria>
<context>Use for bugs, failures, and unexpected behavior. Debugging separates reproduction, hypothesis, experiment, and fix.</context>
<rules><critical>Reproduce before changing code, state a hypothesis, isolate one variable, and stop after repeated failed hypotheses to reassess.</critical><guidance>Read complete errors and logs, inspect recent changes, and document negative results.</guidance></rules>
<steps>1. Capture the complete symptom. 2. Reproduce it. 3. Gather boundary evidence. 4. Test a stated hypothesis. 5. Write a failing regression test, apply the smallest fix, and verify.</steps>
<output>Return symptom, reproduction, hypotheses, evidence, root cause, fix, tests, and residual risk.</output>
<references>Use repository logs, tests, source history, and the applicable component documentation.</references>
