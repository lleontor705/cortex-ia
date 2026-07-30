package pipeline

import (
	"testing"
	"time"

	"github.com/lleontor705/cortex-ia/internal/components/sdd"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/capability"
)

func TestResolveProfileDecisionChoosesStrongestFreshEvidence(t *testing.T) {
	now := time.Date(2026, time.July, 29, 12, 0, 0, 0, time.UTC)
	decision := ResolveProfileDecision(ProfileResolutionInput{
		Now: now,
		Facts: []capability.CapabilityFact{
			profileFact("delegation/direct-child", now),
			profileFact("delegation/nested", now),
		},
	})

	if decision.Effective != sdd.ProfileNativeAdvanced {
		t.Fatalf("effective profile = %q, want strongest qualified profile", decision.Effective)
	}
	if decision.Disposition != ProfileDispositionSelected || decision.ReasonID != ProfileReasonQualified {
		t.Fatalf("decision = %+v, want selected qualified decision", decision)
	}
}

func TestResolveProfileDecisionDegradesWithStableReasonWhenEvidenceIsStale(t *testing.T) {
	now := time.Date(2026, time.July, 29, 12, 0, 0, 0, time.UTC)
	fact := profileFact("delegation/direct-child", now)
	fact.FreshUntil = now
	decision := ResolveProfileDecision(ProfileResolutionInput{Now: now, Facts: []capability.CapabilityFact{fact}})

	if decision.Effective != sdd.ProfilePortableSequential {
		t.Fatalf("effective profile = %q, want sequential degradation", decision.Effective)
	}
	if decision.Disposition != ProfileDispositionDegraded || decision.ReasonID != ProfileReasonEvidenceInsufficient {
		t.Fatalf("decision = %+v, want explicit evidence degradation", decision)
	}
}

func TestResolveProfileDecisionDoesNotUpgradeExplicitRequest(t *testing.T) {
	now := time.Date(2026, time.July, 29, 12, 0, 0, 0, time.UTC)
	decision := ResolveProfileDecision(ProfileResolutionInput{
		Now:       now,
		Requested: sdd.ProfilePortableSequential,
		Facts:     []capability.CapabilityFact{profileFact("delegation/direct-child", now), profileFact("delegation/nested", now)},
	})

	if decision.Effective != sdd.ProfilePortableSequential || decision.Disposition != ProfileDispositionSelected {
		t.Fatalf("decision = %+v, explicit sequential request must remain independent", decision)
	}
}

func TestResolveProfileDecisionExplicitNativeRequestDegradesInsteadOfSeeding(t *testing.T) {
	now := time.Date(2026, time.July, 29, 12, 0, 0, 0, time.UTC)
	decision := ResolveProfileDecision(ProfileResolutionInput{
		Now:       now,
		Requested: sdd.ProfileNativeAdvanced,
		Facts:     []capability.CapabilityFact{profileFact("delegation/direct-child", now)},
	})

	if decision.Requested != sdd.ProfileNativeAdvanced || decision.Effective != sdd.ProfilePortableFlat {
		t.Fatalf("decision = %+v, want explicit native request degraded to qualified flat", decision)
	}
	if decision.Disposition != ProfileDispositionDegraded || decision.ReasonID != ProfileReasonRequestedUnavailable {
		t.Fatalf("decision = %+v, want requested-unavailable reason", decision)
	}
}

func profileFact(id capability.CapabilityID, now time.Time) capability.CapabilityFact {
	return capability.CapabilityFact{
		ID: id, Mode: capability.CapabilityAvailable, Cardinality: capability.CardinalityOne,
		EvidenceClass: capability.EvidenceRuntimeObserved, EvidenceRef: "evidence/" + string(id),
		ObservedAt: now.Add(-time.Minute), FreshUntil: now.Add(time.Hour), Confidence: 1,
		Current: true, Enforcement: capability.EnforcementRuntime,
	}
}
