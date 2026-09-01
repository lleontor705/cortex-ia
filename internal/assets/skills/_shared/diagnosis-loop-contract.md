# Diagnosis Loop Contract

**Installed contract:** `~/.cortex-ia/opencode/contracts/diagnosis-loop-contract.md`

Use this contract for defects, regressions, flakes, and performance failures. General architecture assessment and ordinary code review do not require a reproduction loop.

## Red-capable oracle

Before selecting a root cause, establish one bounded command that has already been executed and can detect the user's exact symptom. Record:

- command and working directory;
- expected and observed verdict;
- why the failure is the reported defect rather than setup noise;
- duration and determinism, or reproduction rate for a flake;
- required fixture or redacted captured input.

Prefer an existing focused test, CLI invocation, HTTP probe, replay, small harness, property loop, or differential comparison. Tighten the oracle until it is as fast, deterministic, and specific as the environment permits. For intermittent failures, raise and report the reproduction rate instead of claiming determinism.

If no red-capable oracle can be produced within the granted read-only scope, stop causal claims. Return `INCONCLUSIVE` and request the missing environment, a redacted captured artifact, or an explicitly routed disposable spike. A nearby failure is not a substitute for the reported symptom.

## Reproduce, minimize, discriminate

1. Reproduce the exact symptom with the oracle.
2. Remove one input, caller, configuration value, or step at a time while keeping the oracle red. The minimized case is complete when every remaining element is load-bearing.
3. Generate three to five ranked, falsifiable hypotheses when the cause is not already proven. Each states the observation that would support or reject it.
4. Run one discriminating probe per hypothesis and change one variable at a time. Prefer debuggers or boundary-specific evidence over broad logging.
5. Tag temporary instrumentation with a unique marker and verify its removal before completion.

## Regression seam

Convert the minimized reproduction into a persistent regression test only when the repository permits that test class and the seam exercises the real failure pattern through a public interface. When persistent tests are out of policy or no faithful seam exists, retain an ephemeral smoke/oracle receipt and report the missing seam as architecture evidence for the planner.

Expected values must come from a specification, known-good literal, captured contract, or other independent source. An oracle that recomputes the implementation's result is tautological.

## Completion

A diagnosis or fix is not proven until the original unminimized oracle is rerun. A completed fix also requires the regression oracle or approved ephemeral substitute to pass, tagged instrumentation to be absent, and the confirmed causal chain to be recorded without secrets or raw logs.
