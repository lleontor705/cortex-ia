package pipeline

import (
	"context"
	"slices"
	"time"

	"github.com/lleontor705/cortex-ia/internal/agents"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/capability"
	"github.com/lleontor705/cortex-ia/internal/model"
)

// qualifyCapabilities is the production runtime qualification boundary. Each
// probe receives authority closed to one declared fact. Probe or validation
// failure removes only that fact; installation can then select a portable
// degradation from the remaining evidence.
func qualifyCapabilities(ctx context.Context, adapters []agents.Adapter, now time.Time, experimentalOptIns []capability.CapabilityID) map[model.AgentID][]capability.CapabilityFact {
	qualified := make(map[model.AgentID][]capability.CapabilityFact, len(adapters))
	for _, adapter := range adapters {
		qualified[adapter.Agent()] = nil
		provider, ok := adapter.(agents.CapabilityProvider)
		if !ok {
			continue
		}
		prober := provider.CapabilityProber()
		if prober == nil {
			continue
		}
		for _, fact := range provider.CapabilityFacts() {
			request := capability.ProbeRequest{
				Base: fact,
				Authority: capability.ProbeAuthority{
					CapabilityID: fact.ID, RuntimeVersions: fact.RuntimeVersions,
					Modes: []capability.CapabilityValue{fact.Mode}, Cardinalities: []capability.Cardinality{fact.Cardinality},
					Enforcement:       []capability.EnforcementClass{fact.Enforcement},
					ExperimentalOptIn: slices.Contains(experimentalOptIns, fact.ID),
				},
			}
			result, err := prober.Probe(ctx, request)
			if err != nil {
				continue
			}
			// Experimental status is fact policy, not a probe-refinable runtime
			// property. Keep it immutable even though the shared validator uses
			// ExperimentalOptIn only as an acceptance gate.
			if result.Refined.Experimental != fact.Experimental {
				continue
			}
			refined, err := capability.ApplyProbeResult(request, result)
			if err != nil {
				continue
			}
			catalog := capability.Catalog{SchemaVersion: capability.CatalogSchema.Current, Version: capability.CatalogSchema.Current, Facts: []capability.CapabilityFact{refined}}
			if err := catalog.Validate(now); err != nil {
				continue
			}
			qualified[adapter.Agent()] = append(qualified[adapter.Agent()], refined)
		}
	}
	return qualified
}
