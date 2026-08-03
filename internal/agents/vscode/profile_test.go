package vscode_test

import (
	"testing"
	"time"

	"github.com/lleontor705/cortex-ia/internal/agents"
	"github.com/lleontor705/cortex-ia/internal/agents/vscode"
	"github.com/lleontor705/cortex-ia/internal/components/sdd"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/capability"
)

var _ agents.CapabilityProvider = vscode.NewAdapter()

func TestCapabilityFactsSelectSequentialWithoutUnsupportedNesting(t *testing.T) {
	now := time.Date(2026, time.July, 27, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name   string
		facts  []capability.CapabilityFact
		optIns []capability.CapabilityID
	}{
		{name: "documented capability is not runtime proof", facts: vscode.NewAdapter().CapabilityFacts()},
		{name: "experimental opt-in cannot upgrade prompt evidence", facts: vscode.NewAdapter().CapabilityFacts(), optIns: []capability.CapabilityID{"delegation/direct-child"}},
		{name: "stale evidence remains sequential", facts: vscode.NewAdapter().CapabilityFacts(), optIns: []capability.CapabilityID{"delegation/direct-child", "delegation/nested"}},
	}
	tests[2].facts[0].FreshUntil = now

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			selection := sdd.SelectWorkflowProfile(sdd.ProfileSelectionInput{
				Now:                now,
				Facts:              tt.facts,
				NativeCapabilities: []capability.CapabilityID{"delegation/nested"},
				ExperimentalOptIns: tt.optIns,
			})
			if selection.Profile != sdd.ProfilePortableSequential {
				t.Fatalf("profile = %q, want %q; degradations: %v", selection.Profile, sdd.ProfilePortableSequential, selection.Degradations)
			}
		})
	}

	if vscode.NewAdapter().SupportsTaskDelegation() || vscode.NewAdapter().SupportsSubAgents() {
		t.Fatal("legacy adapter booleans must preserve the conservative no-delegation correction")
	}
}
