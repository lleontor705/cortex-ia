package agents

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/model"
)

func TestNewDefaultRegistry_ExactIDsAndOrder(t *testing.T) {
	if err := requireCanonicalClientInventory(NewDefaultRegistry().IDs()); err != nil {
		t.Fatal(err)
	}
}

func TestCanonicalClientInventoryRejectsDrift(t *testing.T) {
	canonical := []model.AgentID{
		model.AgentClaudeCode,
		model.AgentOpenCode,
		model.AgentVSCodeCopilot,
		model.AgentCodex,
	}

	for _, tc := range []struct {
		name string
		ids  []model.AgentID
	}{
		{name: "Gemini", ids: append(append([]model.AgentID{}, canonical[:3]...), "gemini")},
		{name: "fifth client", ids: append(append([]model.AgentID{}, canonical...), "fifth")},
		{name: "missing client", ids: canonical[:3]},
		{name: "duplicate client", ids: []model.AgentID{model.AgentClaudeCode, model.AgentOpenCode, model.AgentVSCodeCopilot, model.AgentCodex, model.AgentCodex}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := requireCanonicalClientInventory(tc.ids); err == nil {
				t.Fatalf("requireCanonicalClientInventory(%v) succeeded", tc.ids)
			}
		})
	}
}

func requireCanonicalClientInventory(ids []model.AgentID) error {
	want := []model.AgentID{
		model.AgentClaudeCode,
		model.AgentOpenCode,
		model.AgentVSCodeCopilot,
		model.AgentCodex,
	}
	if !reflect.DeepEqual(ids, want) {
		return fmt.Errorf("client inventory = %v, want exactly %v", ids, want)
	}
	return nil
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
