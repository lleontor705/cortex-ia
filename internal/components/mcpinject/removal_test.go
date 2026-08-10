package mcpinject_test

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/agents"
	"github.com/lleontor705/cortex-ia/internal/agents/opencode"
	"github.com/lleontor705/cortex-ia/internal/components/mcpinject"
	"github.com/lleontor705/cortex-ia/internal/model"
)

func retirementEvidence(content string) []mcpinject.RetirementEvidence {
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(content)))
	return []mcpinject.RetirementEvidence{{
		ComponentID:     model.ComponentMailbox,
		Source:          "cortex-ia.lock",
		TemplateSHA256:  "managed-mailbox-template-v1",
		ObservedSHA256:  digest,
		OwnershipSHA256: "managed-mailbox-template-v1",
	}}
}

func TestPlanRetirementRemovesOnlyExactManagedRegistration(t *testing.T) {
	tests := []struct {
		name     string
		adapter  agents.Adapter
		before   string
		wantKept []string
	}{
		{
			name:     "format preserving OpenCode JSON",
			adapter:  opencode.NewAdapter(),
			before:   "{\n  \"mcp\": {\n    \"agent-mailbox\": {\"command\":[\"npx\",\"agent-mailbox-mcp\"]},\n    \"agent-mailbox-custom\": {\"command\":[\"external\"]}\n  },\n  \"theme\": \"dark\"\n}\n",
			wantKept: []string{"agent-mailbox-custom", "\"theme\": \"dark\""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan, err := mcpinject.PlanRetirement(t.TempDir(), tt.adapter, tt.before, retirementEvidence(tt.before))
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(plan.After), "agent-mailbox-mcp") {
				t.Fatalf("managed registration remains:\n%s", plan.After)
			}
			for _, want := range tt.wantKept {
				if !strings.Contains(string(plan.After), want) {
					t.Fatalf("unmanaged bytes %q were not preserved:\n%s", want, plan.After)
				}
			}
			if plan.ReloadGuidance == "" {
				t.Fatal("missing active-runtime reload guidance")
			}
		})
	}
}

func TestPlanRetirementMetadataOnlyIsNoOp(t *testing.T) {
	before := "{\n  \"mcp\": {\"cortex\": {\"command\":[\"cortex\",\"mcp\"]}}\n}\n"
	plan, err := mcpinject.PlanRetirement(t.TempDir(), opencode.NewAdapter(), before, nil)
	if err != nil {
		t.Fatal(err)
	}
	if plan.NoOpReason == "" || string(plan.After) != before {
		t.Fatalf("metadata-only retirement should preserve bytes: %+v", plan)
	}
}

func TestPlanRetirementPreservesOpenCodeJSONC(t *testing.T) {
	home := t.TempDir()
	configDir := filepath.Join(home, ".config", "opencode")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(configDir, "opencode.jsonc")
	before := "{\n  \"mcp\": {\n    \"agent-mailbox\": { \"command\": [\"npx\", \"agent-mailbox-mcp\"] },\n    \"user\": { \"command\": [\"external\"] },\n  },\n  // Keep this setting.\n  \"share\": \"disabled\",\n}\n"
	if err := os.WriteFile(path, []byte(before), 0o644); err != nil {
		t.Fatal(err)
	}
	plan, err := mcpinject.PlanRetirement(home, opencode.NewAdapter(), before, retirementEvidence(before))
	if err != nil {
		t.Fatal(err)
	}
	if plan.Selector.Path != path || strings.Contains(string(plan.After), "agent-mailbox-mcp") || !strings.Contains(string(plan.After), "// Keep this setting.") || !strings.Contains(string(plan.After), `"user"`) {
		t.Fatalf("JSONC retirement = %+v\n%s", plan.Selector, plan.After)
	}
}

func TestPlanRetirementRejectsUnprovenOwnership(t *testing.T) {
	before := `{"mcp":{"agent-mailbox":{"command":["unmanaged"]}}}`
	_, err := mcpinject.PlanRetirement(t.TempDir(), opencode.NewAdapter(), before, nil)
	if err == nil || !strings.Contains(err.Error(), "ownership") {
		t.Fatalf("expected ownership error, got %v", err)
	}
}

func TestValidateRetirementPathProtectsExternalMailboxData(t *testing.T) {
	tests := []string{
		filepath.Join(t.TempDir(), ".agent-mailbox", "mailbox.db"),
		filepath.Join(t.TempDir(), ".agent-mailbox", "mailbox.db-wal"),
		filepath.Join(t.TempDir(), ".npm", "_cacache", "agent-mailbox-mcp"),
		filepath.Join(t.TempDir(), "archives", "agent-mailbox.zip"),
		filepath.Join(t.TempDir(), "src", "agent-mailbox-mcp", ".git", "config"),
	}
	for _, path := range tests {
		if err := mcpinject.ValidateRetirementPath(path); err == nil {
			t.Errorf("protected external path accepted: %s", path)
		}
	}
}

func FuzzPlanRetirementPreservesUnmanagedJSON(f *testing.F) {
	f.Add("external", "dark")
	f.Fuzz(func(t *testing.T, command, theme string) {
		custom, err := json.Marshal(map[string]any{
			"mcp": map[string]any{
				"agent-mailbox":        map[string]any{"command": []string{"npx", "agent-mailbox-mcp"}},
				"agent-mailbox-custom": map[string]any{"command": []string{command}},
			},
			"theme": theme,
		})
		if err != nil {
			t.Fatal(err)
		}
		plan, err := mcpinject.PlanRetirement(t.TempDir(), opencode.NewAdapter(), string(custom), retirementEvidence(string(custom)))
		if err != nil {
			t.Fatal(err)
		}
		var decoded map[string]any
		if err := json.Unmarshal(plan.After, &decoded); err != nil {
			t.Fatalf("retirement produced invalid JSON: %v", err)
		}
		mcp := decoded["mcp"].(map[string]any)
		if _, found := mcp["agent-mailbox"]; found {
			t.Fatal("managed registration remains")
		}
		if _, found := mcp["agent-mailbox-custom"]; !found || decoded["theme"] != theme {
			t.Fatalf("unmanaged data changed: %#v", decoded)
		}
	})
}
