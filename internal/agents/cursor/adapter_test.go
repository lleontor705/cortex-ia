package cursor

import (
	"context"
	"errors"
	"reflect"
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
	if a.Agent() != model.AgentCursor {
		t.Errorf("expected %s, got %s", model.AgentCursor, a.Agent())
	}
}

func TestSystemPromptFile(t *testing.T) {
	a := NewAdapter()
	got := a.SystemPromptFile("/home/test")
	if got == "" {
		t.Error("expected non-empty SystemPromptFile")
	}
}

func TestCapabilityFactsAreCatalogValidAndDeterministic(t *testing.T) {
	now := time.Date(2026, time.July, 27, 0, 0, 0, 0, time.UTC)
	a := NewAdapter()

	first := a.CapabilityFacts()
	second := a.CapabilityFacts()
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("CapabilityFacts() is not deterministic:\nfirst:  %#v\nsecond: %#v", first, second)
	}
	if len(first) != 2 {
		t.Fatalf("CapabilityFacts() length = %d, want 2", len(first))
	}
	if first[0].ID != "delegation/direct-child" || first[0].Experimental {
		t.Errorf("direct-child fact = %#v", first[0])
	}
	if first[1].ID != "delegation/parallel" || !first[1].Experimental {
		t.Errorf("parallel fact = %#v", first[1])
	}

	catalog := capability.Catalog{
		SchemaVersion: ir.MustParseVersion("1.0.0"),
		Version:       ir.MustParseVersion("1.0.0"),
		Facts:         first,
	}
	if err := catalog.Validate(now); err != nil {
		t.Fatalf("Cursor capability catalog is invalid: %v", err)
	}
}

func TestCapabilityProbeProducesDeterministicRefinementsWithExperimentalOptIn(t *testing.T) {
	now := time.Date(2026, time.July, 27, 0, 0, 0, 0, time.UTC)
	a := &Adapter{
		now: func() time.Time { return now },
		runProbe: func(context.Context, string, ...string) ([]byte, error) {
			return []byte("Cursor 3.5.0\n"), nil
		},
	}
	facts := a.CapabilityFacts()

	qualified := make([]capability.CapabilityFact, 0, len(facts))
	for _, fact := range facts {
		request := capability.ProbeRequest{
			Base: fact,
			Authority: capability.ProbeAuthority{
				CapabilityID:      fact.ID,
				RuntimeVersions:   fact.RuntimeVersions,
				Modes:             []capability.CapabilityValue{capability.CapabilityAvailable},
				Cardinalities:     []capability.Cardinality{capability.CardinalityMany},
				Enforcement:       []capability.EnforcementClass{capability.EnforcementRuntime},
				ExperimentalOptIn: fact.Experimental,
			},
		}
		result, err := a.CapabilityProber().Probe(context.Background(), request)
		if err != nil {
			t.Fatalf("Probe(%q): %v", fact.ID, err)
		}
		refined, err := capability.ApplyProbeResult(request, result)
		if err != nil {
			t.Fatalf("ApplyProbeResult(%q): %v", fact.ID, err)
		}
		qualified = append(qualified, refined)
	}
	if len(qualified) != 2 || qualified[0].EvidenceClass != capability.EvidenceExecutableProbe || !qualified[1].Experimental {
		t.Fatalf("qualified facts = %#v", qualified)
	}
}

func TestCapabilityProbeRejectsUnsupportedCapabilityWithoutLaunchingAgent(t *testing.T) {
	runs := 0
	a := &Adapter{
		now: time.Now,
		runProbe: func(context.Context, string, ...string) ([]byte, error) {
			runs++
			return []byte("Cursor 3.5.0"), nil
		},
	}
	request := capability.ProbeRequest{
		Base: capability.CapabilityFact{ID: "runtime/launch"},
		Authority: capability.ProbeAuthority{
			CapabilityID: "runtime/launch",
		},
	}

	_, err := a.CapabilityProber().Probe(context.Background(), request)
	if err == nil || !errors.Is(err, errUnsupportedCapability) {
		t.Fatalf("Probe() error = %v, want errUnsupportedCapability", err)
	}
	if runs != 0 {
		t.Fatalf("unsupported probe executed Cursor %d times", runs)
	}
}
