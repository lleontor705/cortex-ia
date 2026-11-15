# Structured Failure Trace and Bounded Recovery Example

This guide illustrates how to transform massive, noisy command failure buffers into concise, actionable `<failure_trace>` blocks that preserve context windows and prevent repetitive agent hallucination loops.

---

## 1. The Antipattern: Raw Terminal Buffer Dump

When a command fails, dumping 300+ lines of raw console output into an LLM prompt overwhelms self-attention heads:

```text
=== RUN   TestAgentRunner_Execution
2026/09/18 14:42:01 INFO [runner] initializing sandbox root=/tmp/sandbox-99
2026/09/18 14:42:01 DEBUG [sqlite] connecting to :memory: cache=shared
... [280 lines of verbose debug logs, goroutine dumps, and passing tests] ...
--- FAIL: TestAgentRunner_Execution (0.12s)
    runner_test.go:88: 
        	Error Trace:	runner_test.go:88
        	Error:      	Not equal: 
        	            	expected: "in_review"
        	            	actual  : "blocked"
        	Test:       	TestAgentRunner_Execution
FAIL
FAIL	github.com/lleontor705/cortex-ia/internal/delegation	0.210s
FAIL
```

---

## 2. The Solution: Distilled `<failure_trace>`

The test harness or error filter intercepts the process exit, trims all non-essential telemetry, and injects a bounded XML trace ($\le 25$ lines):

```xml
<failure_trace>
  <command>go test -count=1 ./internal/delegation -run TestAgentRunner_Execution</command>
  <exit_code>1</exit_code>
  <error_locality>internal/delegation/runner_test.go:88</error_locality>
  <diagnostic>
    --- FAIL: TestAgentRunner_Execution (0.12s)
        runner_test.go:88: Not equal:
            expected: "in_review"
            actual  : "blocked"
  </diagnostic>
</failure_trace>
```

---

## 3. The 3-Step Agent Remediation Protocol

Upon receiving a `<failure_trace>`, the agent must execute this disciplined recovery cycle:

```markdown
1. LOCATE: Inspect lines 80-95 in internal/delegation/runner_test.go and 
   the corresponding transition function in internal/delegation/runner.go.

2. HYPOTHESIZE: 
   "Hypothesis: The runner transitioned to 'blocked' because the file lease TTL 
   expired before the task completed. Increasing the simulated TTL in the test 
   mock from 10ms to 500ms will allow the transition to 'in_review' to succeed."

3. VERIFY: Re-run only the targeted oracle:
   `go test -count=1 ./internal/delegation -run TestAgentRunner_Execution`
```

By enforcing this structure in the agent's prompt:
- Context token consumption drops by **92%**.
- Attention remains focused on the exact fault site.
- The agent avoids thrashing across unrelated files.
