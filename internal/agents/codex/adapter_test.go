package codex

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/capability"
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
	if a.Agent() != model.AgentCodex {
		t.Errorf("expected %s, got %s", model.AgentCodex, a.Agent())
	}
}

func TestSystemPromptFile(t *testing.T) {
	a := NewAdapter()
	got := a.SystemPromptFile("/home/test")
	if got == "" {
		t.Error("expected non-empty SystemPromptFile")
	}
}

func TestCapabilityFactsAreEvidenceBackedWithValidMajorZeroBounds(t *testing.T) {
	now := time.Date(2026, time.July, 27, 0, 0, 0, 0, time.UTC)
	adapter := NewAdapter()

	first := adapter.CapabilityFacts()
	second := adapter.CapabilityFacts()
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

	for _, fact := range first {
		if fact.Target != "codex" || fact.RuntimeID != "codex-cli" || fact.AdapterID != "cortex-ia/codex" {
			t.Errorf("fact %q identity = %q/%q/%q", fact.ID, fact.Target, fact.RuntimeID, fact.AdapterID)
		}
		if fact.RuntimeVersions.Minimum.Major != 0 || fact.RuntimeVersions.Minimum.String() != "0.145.0" || fact.RuntimeVersions.MaximumTested.String() != "0.145.0" {
			t.Errorf("fact %q runtime interval = %s", fact.ID, fact.RuntimeVersions.String())
		}
		if fact.EvidenceClass != capability.EvidenceExecutableProbe || fact.EvidenceRef == "" || fact.Probe == nil {
			t.Errorf("fact %q provenance = %q/%q/%+v", fact.ID, fact.EvidenceClass, fact.EvidenceRef, fact.Probe)
		}
		if fact.ObservedAt.IsZero() || !fact.FreshUntil.After(fact.ObservedAt) || fact.Confidence <= 0 || !fact.Current {
			t.Errorf("fact %q freshness/confidence = observed %v, fresh until %v, confidence %v, current %v", fact.ID, fact.ObservedAt, fact.FreshUntil, fact.Confidence, fact.Current)
		}
		if fact.Enforcement != capability.EnforcementRuntime || fact.Cardinality != capability.CardinalityMany {
			t.Errorf("fact %q enforcement/cardinality = %q/%q", fact.ID, fact.Enforcement, fact.Cardinality)
		}
	}

	catalog := capability.Catalog{
		SchemaVersion: capability.CatalogSchema.Current,
		Version:       capability.CatalogSchema.Current,
		Facts:         first,
	}
	if err := catalog.Validate(now); err != nil {
		t.Fatalf("Codex capability catalog is invalid: %v", err)
	}
}

func TestCapabilityFactsResolveStaleAndExperimentalEvidenceConservatively(t *testing.T) {
	facts := NewAdapter().CapabilityFacts()
	parallel := factByID(t, facts, "delegation/parallel")
	request := capability.ProbeRequest{
		Base: parallel,
		Authority: capability.ProbeAuthority{
			CapabilityID:    parallel.ID,
			RuntimeVersions: parallel.RuntimeVersions,
			Modes:           []capability.CapabilityValue{parallel.Mode},
			Cardinalities:   []capability.Cardinality{parallel.Cardinality},
			Enforcement:     []capability.EnforcementClass{parallel.Enforcement},
		},
	}
	result := capability.ProbeResult{Record: *parallel.Probe, Refined: parallel}
	if _, err := capability.ApplyProbeResult(request, result); err == nil {
		t.Fatal("experimental native capability accepted without explicit opt-in")
	}
	request.Authority.ExperimentalOptIn = true
	if _, err := capability.ApplyProbeResult(request, result); err != nil {
		t.Fatalf("experimental native capability rejected with explicit opt-in: %v", err)
	}

	stale := capability.Catalog{
		SchemaVersion: capability.CatalogSchema.Current,
		Version:       capability.CatalogSchema.Current,
		Facts:         facts,
	}
	if err := stale.Validate(facts[0].FreshUntil.Add(time.Nanosecond)); err == nil {
		t.Fatal("stale Codex evidence remained current")
	}
}

func TestCapabilityProberRecordsBoundedRedactedVersionEvidence(t *testing.T) {
	now := time.Date(2026, time.July, 27, 1, 2, 3, 0, time.UTC)
	adapter := NewAdapter()
	var command string
	adapter.runProbe = func(_ context.Context, name string, args ...string) ([]byte, error) {
		command = strings.Join(append([]string{name}, args...), " ")
		return []byte("codex-cli 0.145.0\n"), nil
	}
	adapter.now = func() time.Time { return now }
	fact := factByID(t, adapter.CapabilityFacts(), "delegation/direct-child")
	request := capability.ProbeRequest{
		Base: fact,
		Authority: capability.ProbeAuthority{
			CapabilityID:    fact.ID,
			RuntimeVersions: fact.RuntimeVersions,
			Modes:           []capability.CapabilityValue{fact.Mode},
			Cardinalities:   []capability.Cardinality{fact.Cardinality},
			Enforcement:     []capability.EnforcementClass{fact.Enforcement},
		},
	}

	result, err := adapter.CapabilityProber().Probe(context.Background(), request)
	if err != nil {
		t.Fatalf("Probe() error = %v", err)
	}
	if command != "codex --version" {
		t.Fatalf("probe command = %q, want codex --version", command)
	}
	if result.Record.Result != "qualified-version:0.145.0" || result.Record.Timestamp != now {
		t.Errorf("probe result = %+v", result.Record)
	}
	if !strings.HasPrefix(result.Record.EvidenceDigest, "sha256:") || strings.Contains(result.Record.EvidenceDigest, "codex-cli") {
		t.Errorf("probe evidence is not redacted: %q", result.Record.EvidenceDigest)
	}
	if result.Refined.RuntimeVersions.Minimum.String() != "0.145.0" || result.Refined.RuntimeVersions.MaximumTested.String() != "0.145.0" {
		t.Errorf("probe refinement interval = %s", result.Refined.RuntimeVersions.String())
	}
	if _, err := capability.ApplyProbeResult(request, result); err != nil {
		t.Fatalf("ApplyProbeResult() error = %v", err)
	}
}

func TestCapabilityProberRejectsUnknownOrOutOfRangeEvidenceWithoutExecution(t *testing.T) {
	tests := []struct {
		name       string
		base       capability.CapabilityFact
		output     string
		wantRuns   int
		wantErrIs  error
		wantErrSub string
	}{
		{
			name:      "unknown capability",
			base:      capability.CapabilityFact{ID: "runtime/launch"},
			wantRuns:  0,
			wantErrIs: errUnsupportedCodexCapability,
		},
		{
			name:       "newer unqualified release",
			base:       factByID(t, NewAdapter().CapabilityFacts(), "delegation/direct-child"),
			output:     "codex-cli 0.146.0\n",
			wantRuns:   1,
			wantErrSub: "outside qualified range",
		},
		{
			name:       "unrecognized version output",
			base:       factByID(t, NewAdapter().CapabilityFacts(), "delegation/direct-child"),
			output:     "codex-cli unknown\n",
			wantRuns:   1,
			wantErrSub: "semantic version",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runs := 0
			adapter := NewAdapter()
			adapter.runProbe = func(context.Context, string, ...string) ([]byte, error) {
				runs++
				return []byte(tt.output), nil
			}
			_, err := adapter.CapabilityProber().Probe(context.Background(), capability.ProbeRequest{
				Base: tt.base,
				Authority: capability.ProbeAuthority{
					CapabilityID:    tt.base.ID,
					RuntimeVersions: tt.base.RuntimeVersions,
					Modes:           []capability.CapabilityValue{tt.base.Mode},
					Cardinalities:   []capability.Cardinality{tt.base.Cardinality},
					Enforcement:     []capability.EnforcementClass{tt.base.Enforcement},
				},
			})
			if err == nil {
				t.Fatal("Probe() error = nil, want conservative rejection")
			}
			if tt.wantErrIs != nil && !errors.Is(err, tt.wantErrIs) {
				t.Fatalf("Probe() error = %v, want %v", err, tt.wantErrIs)
			}
			if tt.wantErrSub != "" && !strings.Contains(err.Error(), tt.wantErrSub) {
				t.Fatalf("Probe() error = %v, want substring %q", err, tt.wantErrSub)
			}
			if runs != tt.wantRuns {
				t.Fatalf("probe runs = %d, want %d", runs, tt.wantRuns)
			}
		})
	}
}

func factByID(t *testing.T, facts []capability.CapabilityFact, id capability.CapabilityID) capability.CapabilityFact {
	t.Helper()
	for _, fact := range facts {
		if fact.ID == id {
			return fact
		}
	}
	t.Fatalf("capability fact %q not found", id)
	return capability.CapabilityFact{}
}
