package kiro_test

import (
	"testing"
	"time"

	"github.com/lleontor705/cortex-ia/internal/agents/kiro"
	"github.com/lleontor705/cortex-ia/internal/components/sdd"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/capability"
)

func TestKiroProfileFallsBackUntilExperimentalProbeIsOptedIn(t *testing.T) {
	now := time.Date(2026, time.July, 26, 12, 0, 0, 0, time.UTC)
	facts := kiro.NewAdapter().CapabilityFacts()

	unprobed := sdd.SelectWorkflowProfile(sdd.ProfileSelectionInput{Now: now, Facts: facts})
	if unprobed.Profile != sdd.ProfilePortableSequential {
		t.Fatalf("unprobed Kiro profile = %q, want portable-sequential", unprobed.Profile)
	}

	qualified := findFact(t, facts, "delegation/direct-child")
	qualified.EvidenceClass = capability.EvidenceExecutableProbe
	qualified.EvidenceRef = "sha256:qualified-kiro-subagent-help"
	qualified.ObservedAt = now
	qualified.Enforcement = capability.EnforcementRuntime
	qualified.Probe = &capability.ProbeRecord{
		ID:             "probe/kiro-subagent-help",
		Method:         capability.ProbeCommand,
		Command:        "kiro --help",
		Result:         "subagent capability advertised",
		Timestamp:      now,
		EvidenceDigest: qualified.EvidenceRef,
	}

	withoutOptIn := sdd.SelectWorkflowProfile(sdd.ProfileSelectionInput{Now: now, Facts: []capability.CapabilityFact{qualified}})
	if withoutOptIn.Profile != sdd.ProfilePortableSequential {
		t.Fatalf("unapproved experimental Kiro profile = %q, want portable-sequential", withoutOptIn.Profile)
	}
	withOptIn := sdd.SelectWorkflowProfile(sdd.ProfileSelectionInput{
		Now:                now,
		Facts:              []capability.CapabilityFact{qualified},
		ExperimentalOptIns: []capability.CapabilityID{"delegation/direct-child"},
	})
	if withOptIn.Profile != sdd.ProfilePortableFlat {
		t.Fatalf("qualified opted-in Kiro profile = %q, want portable-flat", withOptIn.Profile)
	}
}

func findFact(t *testing.T, facts []capability.CapabilityFact, id capability.CapabilityID) capability.CapabilityFact {
	t.Helper()
	for _, fact := range facts {
		if fact.ID == id {
			return fact
		}
	}
	t.Fatalf("capability fact %q not found", id)
	return capability.CapabilityFact{}
}
