package pipeline

import (
	"errors"
	"testing"
)

// progressCall records a single invocation of a ProgressFunc.
type progressCall struct {
	StepID string
	Status string
	Err    error
}

func TestComponentStep_ProgressIntegration(t *testing.T) {
	var calls []progressCall

	step := &componentStep{
		homeDir:     "/tmp",
		adapter:     &mockAdapter{agentID: "int-agent"},
		componentID: "int-comp",
		injectorFn: func() ([]string, error) {
			return []string{"/a", "/b"}, nil
		},
		progress: func(stepID, status string, err error) {
			calls = append(calls, progressCall{stepID, status, err})
		},
	}

	if err := step.Run(); err != nil {
		t.Fatalf("Run() unexpected error: %v", err)
	}

	if len(calls) != 2 {
		t.Fatalf("expected exactly 2 progress calls, got %d", len(calls))
	}

	wantName := "int-agent/int-comp"

	// First call: running, no error.
	if calls[0].StepID != wantName {
		t.Errorf("call[0].StepID = %q, want %q", calls[0].StepID, wantName)
	}
	if calls[0].Status != "running" {
		t.Errorf("call[0].Status = %q, want %q", calls[0].Status, "running")
	}
	if calls[0].Err != nil {
		t.Errorf("call[0].Err = %v, want nil", calls[0].Err)
	}

	// Second call: succeeded, no error.
	if calls[1].StepID != wantName {
		t.Errorf("call[1].StepID = %q, want %q", calls[1].StepID, wantName)
	}
	if calls[1].Status != "succeeded" {
		t.Errorf("call[1].Status = %q, want %q", calls[1].Status, "succeeded")
	}
	if calls[1].Err != nil {
		t.Errorf("call[1].Err = %v, want nil", calls[1].Err)
	}
}

func TestComponentStep_ProgressOnFailure(t *testing.T) {
	var calls []progressCall
	injErr := errors.New("component exploded")

	step := &componentStep{
		homeDir:     "/tmp",
		adapter:     &mockAdapter{agentID: "fail-agent"},
		componentID: "fail-comp",
		injectorFn: func() ([]string, error) {
			return nil, injErr
		},
		progress: func(stepID, status string, err error) {
			calls = append(calls, progressCall{stepID, status, err})
		},
	}

	err := step.Run()
	if err == nil {
		t.Fatal("Run() expected error, got nil")
	}

	if len(calls) != 2 {
		t.Fatalf("expected exactly 2 progress calls, got %d", len(calls))
	}

	wantName := "fail-agent/fail-comp"

	// First call: running, no error.
	if calls[0].StepID != wantName {
		t.Errorf("call[0].StepID = %q, want %q", calls[0].StepID, wantName)
	}
	if calls[0].Status != "running" {
		t.Errorf("call[0].Status = %q, want %q", calls[0].Status, "running")
	}
	if calls[0].Err != nil {
		t.Errorf("call[0].Err = %v, want nil", calls[0].Err)
	}

	// Second call: failed, with the injector error.
	if calls[1].StepID != wantName {
		t.Errorf("call[1].StepID = %q, want %q", calls[1].StepID, wantName)
	}
	if calls[1].Status != "failed" {
		t.Errorf("call[1].Status = %q, want %q", calls[1].Status, "failed")
	}
	if !errors.Is(calls[1].Err, injErr) {
		t.Errorf("call[1].Err = %v, want %v", calls[1].Err, injErr)
	}
}
