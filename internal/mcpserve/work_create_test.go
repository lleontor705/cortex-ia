package mcpserve

import (
	"context"
	"slices"
	"testing"
)

func TestWorkCreatePassesWorkloadPolicyToCLI(t *testing.T) {
	runner := cortexFake(t)
	s := newTestSession(t, runner)
	args := map[string]any{
		"workflow":        "direct-change",
		"task_id":         "fix-parser",
		"title":           "Fix parser",
		"workload_policy": "strict",
	}
	if _, err := workCreate(context.Background(), s, args); err != nil {
		t.Fatalf("workCreate: %v", err)
	}
	call, ok := runner.lastWith("work", "create")
	if !ok {
		t.Fatal("work create was not invoked")
	}
	i := slices.Index(call.argv, "--workload-policy")
	if i < 0 || i+1 >= len(call.argv) || call.argv[i+1] != "strict" {
		t.Fatalf("argv lacks --workload-policy strict: %v", call.argv)
	}
}

func TestWorkCreateOmitsWorkloadPolicyWhenUnset(t *testing.T) {
	runner := cortexFake(t)
	s := newTestSession(t, runner)
	args := map[string]any{"workflow": "direct-change", "task_id": "fix-parser", "title": "Fix parser"}
	if _, err := workCreate(context.Background(), s, args); err != nil {
		t.Fatalf("workCreate: %v", err)
	}
	call, _ := runner.lastWith("work", "create")
	if slices.Contains(call.argv, "--workload-policy") {
		t.Fatalf("unset workload_policy must not reach the CLI: %v", call.argv)
	}
}
