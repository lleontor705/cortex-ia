package windsurf_test

import (
	"testing"
	"time"

	"github.com/lleontor705/cortex-ia/internal/agents"
	"github.com/lleontor705/cortex-ia/internal/agents/windsurf"
	"github.com/lleontor705/cortex-ia/internal/components/sdd"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/capability"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
)

var _ agents.CapabilityProvider = windsurf.NewAdapter()

func TestCapabilityFactsAreValidAndConservative(t *testing.T) {
	t.Parallel()

	observedAt := time.Date(2026, time.July, 26, 0, 0, 0, 0, time.UTC)
	facts := windsurf.NewAdapter().CapabilityFacts()
	catalog := capability.Catalog{
		SchemaVersion: ir.MustParseVersion("1.0.0"),
		Version:       ir.MustParseVersion("1.0.0"),
		Facts:         facts,
	}
	if err := catalog.Validate(observedAt); err != nil {
		t.Fatalf("CapabilityFacts() returned an invalid catalog: %v", err)
	}

	directChild := findFact(t, facts, "delegation/direct-child")
	if directChild.Cardinality != capability.CardinalityMany {
		t.Fatalf("direct-child cardinality = %q, want %q", directChild.Cardinality, capability.CardinalityMany)
	}
	if directChild.EvidenceClass != capability.EvidenceDocumentation || directChild.EvidenceRef == "" {
		t.Fatalf("direct-child provenance = (%q, %q), want documented non-empty evidence", directChild.EvidenceClass, directChild.EvidenceRef)
	}
	if !directChild.ObservedAt.Equal(observedAt) || !directChild.FreshUntil.After(observedAt) {
		t.Fatalf("direct-child freshness = observed %v, fresh until %v", directChild.ObservedAt, directChild.FreshUntil)
	}
	if !directChild.Experimental {
		t.Fatal("direct-child delegation must remain experimental until an installed-runtime probe qualifies it")
	}
	if err := catalog.Validate(directChild.FreshUntil); err == nil {
		t.Fatal("CapabilityFacts() remained current at its freshness deadline")
	}

	nested := findFact(t, facts, "delegation/nested")
	if nested.Mode != capability.CapabilityAbsent || nested.Cardinality != capability.CardinalityNone {
		t.Fatalf("nested delegation = mode %q cardinality %q, want absent/none", nested.Mode, nested.Cardinality)
	}
}

func TestCapabilityFactsNeverOptimisticallyUpgradeProfile(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.July, 26, 0, 0, 0, 0, time.UTC)
	facts := windsurf.NewAdapter().CapabilityFacts()

	tests := []struct {
		name   string
		optIns []capability.CapabilityID
	}{
		{name: "without experimental opt-in"},
		{name: "with experimental opt-in but without runtime proof", optIns: []capability.CapabilityID{"delegation/direct-child"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			selection := sdd.SelectWorkflowProfile(sdd.ProfileSelectionInput{
				Now:                now,
				Facts:              facts,
				NativeCapabilities: []capability.CapabilityID{"delegation/nested"},
				ExperimentalOptIns: tt.optIns,
			})
			if selection.Profile != sdd.ProfilePortableSequential {
				t.Fatalf("profile = %q, want %q; degradations: %v", selection.Profile, sdd.ProfilePortableSequential, selection.Degradations)
			}
		})
	}
}

func TestCapabilityProberIsDisabledWithoutStableInstalledEvidence(t *testing.T) {
	t.Parallel()

	if prober := windsurf.NewAdapter().CapabilityProber(); prober != nil {
		t.Fatalf("CapabilityProber() = %T, want nil until a stable installed-runtime probe is available", prober)
	}
}

func findFact(t *testing.T, facts []capability.CapabilityFact, id capability.CapabilityID) capability.CapabilityFact {
	t.Helper()
	for _, fact := range facts {
		if fact.ID == id {
			return fact
		}
	}
	t.Fatalf("CapabilityFacts() missing %q", id)
	return capability.CapabilityFact{}
}
