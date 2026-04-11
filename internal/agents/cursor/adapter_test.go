package cursor

import (
	"testing"

	"github.com/lleontor705/cortex-ia/internal/model"
)

func TestNewAdapter(t *testing.T) {
	a := NewAdapter()
	if a == nil {
		t.Fatal("expected non-nil adapter")
	}
}

func TestAgent(t *testing.T) {
	a := NewAdapter()
	if a.Agent() != model.AgentCursor {
		t.Errorf("expected %s, got %s", model.AgentCursor, a.Agent())
	}
}

func TestSystemPromptFile(t *testing.T) {
	a := NewAdapter()
	got := a.SystemPromptFile("/home/test")
	if got == "" {
		t.Error("expected non-empty SystemPromptFile")
	}
}
