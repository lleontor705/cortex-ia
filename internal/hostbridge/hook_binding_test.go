package hostbridge

import (
	"testing"
	"time"
)

// A read-only call on a claimed task must not overwrite the binding a pending
// restricted call depends on.
func TestReadOnlyBridgeCallDoesNotBind(t *testing.T) {
	stateHome := t.TempDir()
	deps := Deps{StateHome: stateHome, Now: func() time.Time { return hookNow }}
	claim := "mcp__cortex_ia__cortex_ia_work_claim"
	status := "mcp__cortex_ia__cortex_ia_work_status"

	if d := mustDecide(t, claudeInput("implement", claim, map[string]any{"task_id": "T1"}), deps); d.Verdict != PassThrough {
		t.Fatalf("claim denied: %s", d.Reason)
	}
	orchestrator := claudeInput("orchestrator", status, map[string]any{"task_id": "T1"})
	orchestrator.SessionKey = "cc-orch"
	if d := mustDecide(t, orchestrator, deps); d.Verdict != PassThrough {
		t.Fatalf("status denied: %s", d.Reason)
	}

	caller, err := ConsumeCaller(stateHome, "T1", "cortex_ia_work_claim", hookNow)
	if err != nil {
		t.Fatalf("claim binding lost after status call: %v", err)
	}
	if caller.SessionKey != "cc-s1-a1" || caller.Role != "implement" {
		t.Fatalf("claim binding overwritten: %+v", caller)
	}
	if _, err := ConsumeCaller(stateHome, "T1", "cortex_ia_work_status", hookNow); err == nil {
		t.Fatal("read-only status call must not leave a binding")
	}
}

// A binding authorizes exactly one server call.
func TestCallerBindingIsConsumedOnce(t *testing.T) {
	stateHome := t.TempDir()
	bound := Caller{Host: HostClaude, SessionKey: "cc-s1", Role: "implement", TaskID: "T1", Tool: "cortex_ia_work_claim", BoundAt: hookNow}
	if err := BindCaller(stateHome, bound); err != nil {
		t.Fatalf("bind: %v", err)
	}
	if _, err := ConsumeCaller(stateHome, "T1", "cortex_ia_work_claim", hookNow); err != nil {
		t.Fatalf("first consume: %v", err)
	}
	if _, err := ConsumeCaller(stateHome, "T1", "cortex_ia_work_claim", hookNow); err == nil {
		t.Fatal("second consume must fail")
	}
}
