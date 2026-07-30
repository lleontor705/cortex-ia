package antigravity

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/capability"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
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
	if a.Agent() != model.AgentAntigravity {
		t.Errorf("expected %s, got %s", model.AgentAntigravity, a.Agent())
	}
}

func TestSystemPromptFile(t *testing.T) {
	a := NewAdapter()
	got := a.SystemPromptFile("/home/test")
	if got == "" {
		t.Error("expected non-empty SystemPromptFile")
	}
}

func TestCapabilityFactsExposeOnlyExperimentalConfigurationEvidence(t *testing.T) {
	observedAt := time.Date(2026, time.July, 26, 0, 0, 0, 0, time.UTC)
	a := NewAdapter()

	first := a.CapabilityFacts()
	second := a.CapabilityFacts()
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("CapabilityFacts() is not deterministic:\nfirst:  %#v\nsecond: %#v", first, second)
	}
	if len(first) != 2 {
		t.Fatalf("CapabilityFacts() returned %d facts, want direct-child and nested delegation", len(first))
	}
	if a.SupportsTaskDelegation() || a.SupportsSubAgents() {
		t.Fatal("configuration adapter must not claim runtime delegation authority")
	}

	for _, id := range []capability.CapabilityID{"delegation/direct-child", "delegation/nested"} {
		fact := antigravityFact(t, first, id)
		if fact.Mode != capability.CapabilityAvailable || fact.Cardinality != capability.CardinalityMany {
			t.Errorf("%s availability = %q/%q, want available/many", id, fact.Mode, fact.Cardinality)
		}
		if fact.Target != "antigravity" || fact.RuntimeID != "antigravity-2" || fact.AdapterID != "cortex-ia/antigravity" {
			t.Errorf("%s identity = %q/%q/%q", id, fact.Target, fact.RuntimeID, fact.AdapterID)
		}
		if fact.EvidenceClass != capability.EvidenceDocumentation || !strings.HasPrefix(fact.EvidenceRef, "https://antigravity.google/docs/") {
			t.Errorf("%s evidence = %q/%q, want official documentation", id, fact.EvidenceClass, fact.EvidenceRef)
		}
		if fact.Enforcement != capability.EnforcementPrompt || !fact.Experimental {
			t.Errorf("%s authority = enforcement %q experimental %v, want prompt/true", id, fact.Enforcement, fact.Experimental)
		}
		if !fact.ObservedAt.Equal(observedAt) || !fact.FreshUntil.After(observedAt) || fact.Confidence <= 0 {
			t.Errorf("%s provenance is incomplete: %+v", id, fact)
		}
	}

	catalog := capability.Catalog{
		SchemaVersion: capability.CatalogSchema.Current,
		Version:       capability.CatalogSchema.Current,
		Facts:         first,
	}
	if err := catalog.Validate(observedAt.Add(time.Hour)); err != nil {
		t.Fatalf("Antigravity capability catalog is invalid: %v", err)
	}
}

func TestCapabilityProbeRequiresOptInAndCannotClaimRuntimeAuthority(t *testing.T) {
	observed := time.Date(2026, time.July, 26, 12, 34, 56, 0, time.UTC)
	a := &Adapter{now: func() time.Time { return observed }}
	base := antigravityFact(t, a.CapabilityFacts(), "delegation/direct-child")
	request := capability.ProbeRequest{
		Base: base,
		Authority: capability.ProbeAuthority{
			CapabilityID:    base.ID,
			RuntimeVersions: base.RuntimeVersions,
			Modes:           []capability.CapabilityValue{base.Mode},
			Cardinalities:   []capability.Cardinality{base.Cardinality},
			Enforcement:     []capability.EnforcementClass{capability.EnforcementPrompt},
		},
	}

	if _, err := a.CapabilityProber().Probe(context.Background(), request); err == nil || !strings.Contains(err.Error(), "explicit opt-in") {
		t.Fatalf("Probe() without opt-in error = %v, want explicit opt-in rejection", err)
	}

	request.Authority.ExperimentalOptIn = true
	result, err := a.CapabilityProber().Probe(context.Background(), request)
	if err != nil {
		t.Fatalf("Probe() with opt-in error = %v", err)
	}
	if result.Record.Method != capability.ProbeProtocol || result.Record.Protocol != "antigravity-agent-configuration/v1" {
		t.Errorf("probe method/protocol = %q/%q", result.Record.Method, result.Record.Protocol)
	}
	if result.Record.Command != "" || result.Record.Timestamp != observed || !strings.HasPrefix(result.Record.EvidenceDigest, "sha256:") {
		t.Errorf("probe must be redacted and configuration-only: %+v", result.Record)
	}
	if result.Refined.Enforcement != capability.EnforcementPrompt || !result.Refined.Experimental {
		t.Errorf("probe widened runtime authority: %+v", result.Refined)
	}
	if err := capability.ValidateProbeRefinement(request, result); err != nil {
		t.Fatalf("ValidateProbeRefinement() error = %v", err)
	}

	request.Authority.Enforcement = []capability.EnforcementClass{capability.EnforcementRuntime}
	if _, err := a.CapabilityProber().Probe(context.Background(), request); err == nil || !strings.Contains(err.Error(), "configuration-only") {
		t.Fatalf("Probe() runtime-authority error = %v, want configuration-only rejection", err)
	}
}

func TestCapabilityProbeRejectsForeignAndUnknownFacts(t *testing.T) {
	a := NewAdapter()
	direct := antigravityFact(t, a.CapabilityFacts(), "delegation/direct-child")
	tests := []struct {
		name   string
		mutate func(*capability.CapabilityFact)
	}{
		{name: "foreign adapter", mutate: func(f *capability.CapabilityFact) { f.AdapterID = "cortex-ia/foreign" }},
		{name: "unknown capability", mutate: func(f *capability.CapabilityFact) { f.ID = "runtime/launch" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base := direct
			tt.mutate(&base)
			_, err := a.CapabilityProber().Probe(context.Background(), capability.ProbeRequest{
				Base: base,
				Authority: capability.ProbeAuthority{
					CapabilityID:      base.ID,
					RuntimeVersions:   ir.VersionRange{Minimum: ir.MustParseVersion("2.0.0"), MaximumTested: ir.MustParseVersion("2.4.2")},
					Modes:             []capability.CapabilityValue{capability.CapabilityAvailable},
					Cardinalities:     []capability.Cardinality{capability.CardinalityMany},
					Enforcement:       []capability.EnforcementClass{capability.EnforcementPrompt},
					ExperimentalOptIn: true,
				},
			})
			if err == nil || !strings.Contains(err.Error(), "unsupported Antigravity capability") {
				t.Fatalf("Probe() error = %v, want unsupported capability rejection", err)
			}
		})
	}
}

func antigravityFact(t *testing.T, facts []capability.CapabilityFact, id capability.CapabilityID) capability.CapabilityFact {
	t.Helper()
	for _, fact := range facts {
		if fact.ID == id {
			return fact
		}
	}
	t.Fatalf("CapabilityFacts() missing %q", id)
	return capability.CapabilityFact{}
}
