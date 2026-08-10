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
	for _, tc := range []struct {
		name string
		ids  []model.AgentID
	}{
		{name: "Gemini", ids: []model.AgentID{model.AgentOpenCode, "gemini"}},
		{name: "Claude", ids: []model.AgentID{model.AgentOpenCode, "claude-code"}},
		{name: "missing client", ids: []model.AgentID{}},
		{name: "duplicate client", ids: []model.AgentID{model.AgentOpenCode, model.AgentOpenCode}},
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
		model.AgentOpenCode,
	}
	if !reflect.DeepEqual(ids, want) {
		return fmt.Errorf("client inventory = %v, want exactly %v", ids, want)
	}
	return nil
}

func TestNewDefaultRegistry_RejectsRetiredIDs(t *testing.T) {
	r := NewDefaultRegistry()
	for _, id := range []model.AgentID{
		"claude-code", "codex", "vscode-copilot", "gga", "gemini-cli", "cursor", "windsurf", "antigravity",
	} {
		if _, err := r.Get(id); err != ErrAgentNotFound {
			t.Errorf("Get(%q) error = %v, want ErrAgentNotFound", id, err)
		}
	}
}
