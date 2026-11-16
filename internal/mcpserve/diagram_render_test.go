package mcpserve

import (
	"strings"
	"testing"
	"time"

	"github.com/lleontor705/cortex-ia/v2/internal/hostbridge"
)

// bindImplementCall simulates the pre-tool hook for an implement controller;
// the shared bindCall helper leaves the role empty, which artifact output rejects.
func bindImplementCall(t *testing.T, s *session, tool string) {
	t.Helper()
	err := hostbridge.BindCaller(s.stateHome, hostbridge.Caller{
		Host: hostbridge.HostClaude, SessionKey: testSessionKey, Role: "implement", TaskID: testTask, Tool: tool, BoundAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("bind caller: %v", err)
	}
}

// diagram_render writes under the caller's lease only when the hook binding
// for (task, tool) names the call, mirroring the other implement tools.
func TestDiagramRenderWritesUnderLeaseWithBinding(t *testing.T) {
	runner := cortexFake(t)
	s := newTestSession(t, runner)
	bindCall(t, s, "cortex_ia_work_claim", testSessionKey)
	if text, isError := call(t, s, "cortex_ia_work_claim", map[string]any{"task_id": testTask, "paths": []any{"a.txt"}}); isError {
		t.Fatalf("claim failed: %s", text)
	}

	bindImplementCall(t, s, "cortex_ia_diagram_render")
	text, isError := call(t, s, "cortex_ia_diagram_render", map[string]any{
		"task_id": testTask, "spec_path": "spec.json", "output_path": "a.txt",
	})
	if isError {
		t.Fatalf("render under lease failed: %s", text)
	}
	assertNoSecrets(t, text)
	render, ok := runner.lastWith("diagram", "render")
	if !ok {
		t.Fatal("diagram render verb never ran")
	}
	if !strings.Contains(render.stdin, `"task_id":"`+testTask+`"`) {
		t.Fatalf("render stdin carries no task authority: %s", render.stdin)
	}
}

func TestDiagramRenderRequiresBindingAndLease(t *testing.T) {
	runner := cortexFake(t)
	s := newTestSession(t, runner)
	bindCall(t, s, "cortex_ia_work_claim", testSessionKey)
	if text, isError := call(t, s, "cortex_ia_work_claim", map[string]any{"task_id": testTask, "paths": []any{"a.txt"}}); isError {
		t.Fatalf("claim failed: %s", text)
	}

	text, isError := call(t, s, "cortex_ia_diagram_render", map[string]any{
		"task_id": testTask, "spec_path": "spec.json", "output_path": "a.txt",
	})
	if !isError || !strings.Contains(text, "CALLER_UNBOUND") {
		t.Fatalf("render without its own binding must fail with CALLER_UNBOUND, got isError=%v %s", isError, text)
	}

	bindImplementCall(t, s, "cortex_ia_diagram_render")
	text, isError = call(t, s, "cortex_ia_diagram_render", map[string]any{
		"task_id": testTask, "spec_path": "spec.json", "output_path": "unleased.html",
	})
	if !isError || !strings.Contains(text, "LEASE_REQUIRED") {
		t.Fatalf("render to an unleased path must fail with LEASE_REQUIRED, got isError=%v %s", isError, text)
	}
	if _, ran := runner.lastWith("diagram", "render"); ran {
		t.Fatal("diagram render ran for an unleased output path")
	}
}
