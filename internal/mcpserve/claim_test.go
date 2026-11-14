package mcpserve

import (
	"context"
	"strings"
	"testing"
	"time"
)

func call(t *testing.T, s *session, name string, args map[string]any) (string, bool) {
	t.Helper()
	return s.callTool(context.Background(), name, args)
}

func TestClaimTransitionFlowKeepsTokensInServer(t *testing.T) {
	runner := cortexFake(t)
	s := newTestSession(t, runner)
	bindTask(t, s, testSessionKey)

	text, isError := call(t, s, "cortex_ia_work_claim", map[string]any{"task_id": testTask, "paths": []any{"a.txt"}})
	if isError {
		t.Fatalf("claim failed: %s", text)
	}
	assertNoSecrets(t, text)
	claim, ok := runner.lastWith("work", "claim")
	if !ok {
		t.Fatal("claim verb never ran")
	}
	if !hasPrefix(claim.argv, "work", "claim", testTask, "--owner", "opencode-session:"+testSessionKey) {
		t.Fatalf("claim argv = %v", claim.argv)
	}
	if !strings.Contains(text, `"reserved_files"`) || !strings.Contains(text, `"maintenance"`) {
		t.Fatalf("claim result lacks reservation or maintenance: %s", text)
	}

	text, isError = call(t, s, "cortex_ia_work_transition", map[string]any{
		"task_id": testTask, "to": "in_review", "revision": float64(3), "summary": "done",
	})
	if isError {
		t.Fatalf("transition failed: %s", text)
	}
	assertNoSecrets(t, text)
	transition, ok := runner.lastWith("work", "transition")
	if !ok {
		t.Fatal("transition verb never ran")
	}
	if transition.stdin != testClaimToken {
		t.Fatalf("token not sent on stdin")
	}
	if !contains(transition.argv, "@stdin") || !contains(transition.argv, "--revision") || !contains(transition.argv, "3") {
		t.Fatalf("transition argv = %v", transition.argv)
	}
	if strings.Contains(strings.Join(transition.argv, " "), testClaimToken) {
		t.Fatal("token leaked into argv")
	}
}

func TestClaimWithoutHookBindingFailsOnClaude(t *testing.T) {
	s := newTestSession(t, cortexFake(t))
	text, isError := call(t, s, "cortex_ia_work_claim", map[string]any{"task_id": testTask})
	if !isError || !strings.Contains(text, "CALLER_UNBOUND") || !strings.Contains(text, "hook") {
		t.Fatalf("want CALLER_UNBOUND hook error, got isError=%v %s", isError, text)
	}
}

func TestTransitionRequiresRevision(t *testing.T) {
	s := newTestSession(t, cortexFake(t))
	bindTask(t, s, testSessionKey)
	text, isError := call(t, s, "cortex_ia_work_transition", map[string]any{"task_id": testTask, "to": "in_review"})
	if !isError || !strings.Contains(text, "revision is required") {
		t.Fatalf("want revision error, got isError=%v %s", isError, text)
	}
}

func TestApprovePassRequiresEvidence(t *testing.T) {
	s := newTestSession(t, cortexFake(t))
	bindTask(t, s, "cc-reviewer")
	text, isError := call(t, s, "cortex_ia_work_approve", map[string]any{
		"task_id": testTask, "verdict": "PASS", "revision": float64(4),
	})
	if !isError || !strings.Contains(text, "EVIDENCE_REQUIRED") {
		t.Fatalf("want EVIDENCE_REQUIRED, got isError=%v %s", isError, text)
	}
}

func TestApproveRefusedForClaimingController(t *testing.T) {
	runner := cortexFake(t)
	s := newTestSession(t, runner)
	bindTask(t, s, testSessionKey)
	if _, isError := call(t, s, "cortex_ia_work_claim", map[string]any{"task_id": testTask, "paths": []any{"a.txt"}}); isError {
		t.Fatal("claim setup failed")
	}
	text, isError := call(t, s, "cortex_ia_work_approve", map[string]any{
		"task_id": testTask, "verdict": "PASS", "revision": float64(4), "evidence": "go test ok",
	})
	if !isError || !strings.Contains(text, "cannot approve its own task") {
		t.Fatalf("want self-approval refusal, got isError=%v %s", isError, text)
	}
}

func TestMaintenanceFailureIsReportedOnNextResult(t *testing.T) {
	runner := cortexFake(t)
	base := runner.respond
	runner.respond = func(argv []string, stdin string) (string, error) {
		if hasPrefix(argv, "work", "controller-renew") {
			return "", errorString("renewal refused")
		}
		return base(argv, stdin)
	}
	s := newTestSession(t, runner)
	s.interval = 5 * time.Millisecond
	bindTask(t, s, testSessionKey)
	if _, isError := call(t, s, "cortex_ia_work_claim", map[string]any{"task_id": testTask, "paths": []any{"a.txt"}}); isError {
		t.Fatal("claim setup failed")
	}
	for i := 0; i < 200; i++ {
		text, _ := call(t, s, "cortex_ia_work_status", map[string]any{"task_id": testTask})
		if strings.Contains(text, `"active": false`) {
			assertNoSecrets(t, text)
			if !strings.Contains(text, "renewal refused") {
				t.Fatalf("failure reason missing: %s", text)
			}
			return
		}
	}
	t.Fatal("heartbeat failure never reported")
}

type errorString string

func (e errorString) Error() string { return string(e) }
