package app

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/delegation"
)

func TestWorkProjectionCLI(t *testing.T) {
	home, _, item := setupWorkReviseTask(t)
	t.Setenv("CORTEX_IA_HOME", home)

	t.Run("legacy_compatibility_and_additive_projection", func(t *testing.T) {
		out, err := captureStdout(func() error {
			return runWork([]string{"status", item.ID})
		})
		if err != nil {
			t.Fatalf("work status failed: %v", err)
		}

		var raw map[string]any
		if err := json.Unmarshal([]byte(out), &raw); err != nil {
			t.Fatalf("unmarshal json: %v, output: %s", err, out)
		}

		for _, field := range []string{"task_id", "board_id", "status", "revision", "title"} {
			if _, ok := raw[field]; !ok {
				t.Fatalf("missing legacy top-level field: %s", field)
			}
		}
		if raw["task_id"] != item.ID {
			t.Fatalf("expected task_id %s, got %v", item.ID, raw["task_id"])
		}

		projRaw, ok := raw["projection"].(map[string]any)
		if !ok || projRaw == nil {
			t.Fatalf("missing or invalid projection field in output: %v", raw["projection"])
		}
		if projRaw["task_id"] != item.ID {
			t.Fatalf("expected projection.task_id %s, got %v", item.ID, projRaw["task_id"])
		}
		if projRaw["authority_available"] != true {
			t.Fatalf("expected authority_available true")
		}
	})

	t.Run("token_free_field", func(t *testing.T) {
		out, err := captureStdout(func() error {
			return runWork([]string{"status", item.ID})
		})
		if err != nil {
			t.Fatalf("work status failed: %v", err)
		}
		if strings.Contains(out, "claim_token") || strings.Contains(out, "lease_token") {
			t.Fatalf("token field found in work status output: %s", out)
		}
	})

	t.Run("separate_phase_task_verdict", func(t *testing.T) {
		out, err := captureStdout(func() error {
			return runWork([]string{"status", item.ID})
		})
		if err != nil {
			t.Fatalf("work status failed: %v", err)
		}
		var output workStatusOutput
		if err := json.Unmarshal([]byte(out), &output); err != nil {
			t.Fatalf("unmarshal workStatusOutput: %v", err)
		}

		if output.Projection.Status != delegation.WorkReady {
			t.Fatalf("expected projection status ready, got %s", output.Projection.Status)
		}
		if output.Projection.PhaseStatus != "" {
			t.Fatalf("phase_status should not be conflated with task status, got %q", output.Projection.PhaseStatus)
		}
		if output.Projection.VerificationVerdict != "" {
			t.Fatalf("verdict should not be conflated with task status, got %q", output.Projection.VerificationVerdict)
		}
	})

	t.Run("unknown_role_suppression_vs_valid_role", func(t *testing.T) {
		outUnknown, err := captureStdout(func() error {
			return runWork([]string{"status", item.ID, "--role", "unknown"})
		})
		if err != nil {
			t.Fatalf("work status --role unknown failed: %v", err)
		}
		var respUnknown workStatusOutput
		if err := json.Unmarshal([]byte(outUnknown), &respUnknown); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if len(respUnknown.Projection.CandidateActions) != 0 {
			t.Fatalf("expected 0 actions for unknown role, got %d", len(respUnknown.Projection.CandidateActions))
		}

		outImplement, err := captureStdout(func() error {
			return runWork([]string{"status", item.ID, "--role", "implement"})
		})
		if err != nil {
			t.Fatalf("work status --role implement failed: %v", err)
		}
		var respImplement workStatusOutput
		if err := json.Unmarshal([]byte(outImplement), &respImplement); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if len(respImplement.Projection.CandidateActions) != 1 || respImplement.Projection.CandidateActions[0].Action != "request_claim" {
			t.Fatalf("expected request_claim for implement role, got %+v", respImplement.Projection.CandidateActions)
		}
	})

	t.Run("failed_retrieval", func(t *testing.T) {
		_, err := captureStdout(func() error {
			return runWork([]string{"status", "nonexistent-task-id"})
		})
		if err == nil {
			t.Fatalf("expected error for nonexistent task, got nil")
		}
	})
}
