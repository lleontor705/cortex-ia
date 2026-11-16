package mcpserve

import (
	"strings"
	"testing"
)

// A status call between the hook binding and the claim must not change the
// claim owner: read-only calls never consume or replace a binding.
func TestClaimAfterInterleavedStatusIsOwnedByClaimer(t *testing.T) {
	runner := cortexFake(t)
	s := newTestSession(t, runner)
	bindCall(t, s, "cortex_ia_work_claim", testSessionKey)

	if _, isError := call(t, s, "cortex_ia_work_status", map[string]any{"task_id": testTask}); isError {
		t.Fatal("status failed")
	}
	if _, isError := call(t, s, "cortex_ia_work_claim", map[string]any{"task_id": testTask, "paths": []any{"a.txt"}}); isError {
		t.Fatal("claim failed after interleaved status")
	}
	claim, ok := runner.lastWith("work", "claim")
	if !ok {
		t.Fatal("claim verb never ran")
	}
	if !hasPrefix(claim.argv, "work", "claim", testTask, "--owner", "opencode-session:"+testSessionKey) {
		t.Fatalf("claim owner wrong: %v", claim.argv)
	}
}

// A binding for one tool must not authorize a different tool on the same task.
func TestBindingDoesNotAuthorizeOtherTool(t *testing.T) {
	s := newTestSession(t, cortexFake(t))
	bindCall(t, s, "cortex_ia_work_claim", testSessionKey)
	text, isError := call(t, s, "cortex_ia_work_transition", map[string]any{"task_id": testTask, "to": "in_review", "revision": float64(3)})
	if !isError || !strings.Contains(text, "CALLER_UNBOUND") {
		t.Fatalf("transition without its own binding must fail, got isError=%v %s", isError, text)
	}
}
