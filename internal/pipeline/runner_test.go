package pipeline

import (
	"errors"
	"testing"
)

type mockStep struct {
	name      string
	err       error
	ran       bool
	rolledBack bool
}

func (s *mockStep) Name() string  { return s.name }
func (s *mockStep) Run() error    { s.ran = true; return s.err }
func (s *mockStep) Rollback() error { s.rolledBack = true; return nil }

func TestRunStage_AllSuccess(t *testing.T) {
	s1 := &mockStep{name: "step1"}
	s2 := &mockStep{name: "step2"}
	s3 := &mockStep{name: "step3"}

	result := RunStage([]Step{s1, s2, s3})
	if result.Error != nil {
		t.Fatal(result.Error)
	}
	if len(result.Completed) != 3 {
		t.Errorf("expected 3 completed, got %d", len(result.Completed))
	}
}

func TestRunStage_FailureRollsBack(t *testing.T) {
	s1 := &mockStep{name: "step1"}
	s2 := &mockStep{name: "step2", err: errors.New("boom")}
	s3 := &mockStep{name: "step3"}

	result := RunStage([]Step{s1, s2, s3})
	if result.Error == nil {
		t.Error("expected error")
	}
	if result.Failed != "step2" {
		t.Errorf("expected step2 to fail, got %s", result.Failed)
	}
	if !s1.rolledBack {
		t.Error("expected step1 to be rolled back")
	}
	if s3.ran {
		t.Error("step3 should not have run")
	}
}

func TestRunStage_Empty(t *testing.T) {
	result := RunStage(nil)
	if result.Error != nil {
		t.Fatal(result.Error)
	}
}
