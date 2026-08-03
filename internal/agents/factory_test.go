package agents

import (
	"reflect"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/model"
)

func TestNewDefaultRegistry_ExactIDsAndOrder(t *testing.T) {
	want := []model.AgentID{
		model.AgentClaudeCode,
		model.AgentOpenCode,
		model.AgentVSCodeCopilot,
		model.AgentCodex,
	}
	if got := NewDefaultRegistry().IDs(); !reflect.DeepEqual(got, want) {
		t.Fatalf("IDs() = %v, want %v", got, want)
	}
}

func TestNewDefaultRegistry_RejectsRetiredIDs(t *testing.T) {
	r := NewDefaultRegistry()
	for _, id := range []model.AgentID{
		"gga", "gemini-cli", "cursor", "windsurf", "antigravity",
		"kilocode", "kimi", "kiro-ide", "qwen-code",
	} {
		if _, err := r.Get(id); err != ErrAgentNotFound {
			t.Errorf("Get(%q) error = %v, want ErrAgentNotFound", id, err)
		}
	}
}
