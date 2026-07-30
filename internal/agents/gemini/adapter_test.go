package gemini

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
	if a.Agent() != model.AgentGeminiCLI {
		t.Errorf("expected %s, got %s", model.AgentGeminiCLI, a.Agent())
	}
}

func TestSystemPromptFile(t *testing.T) {
	a := NewAdapter()
	got := a.SystemPromptFile("/home/test")
	if got == "" {
		t.Error("expected non-empty SystemPromptFile")
	}
}

func TestCapabilityFactsAreEvidenceBackedAndConservative(t *testing.T) {
	a := NewAdapter()
	first := a.CapabilityFacts()
	second := a.CapabilityFacts()
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("CapabilityFacts() is not deterministic:\nfirst:  %#v\nsecond: %#v", first, second)
	}
	if len(first) != 3 {
		t.Fatalf("CapabilityFacts() returned %d facts, want direct-child, tool isolation, and nested absence", len(first))
	}

	direct := geminiFact(t, first, "delegation/direct-child")
	isolatedTools := geminiFact(t, first, "isolation/tool-scope")
	nested := geminiFact(t, first, "delegation/nested")
	for _, fact := range []capability.CapabilityFact{direct, isolatedTools, nested} {
		if fact.RuntimeVersions.Minimum.String() != "0.52.0" || fact.RuntimeVersions.MaximumTested.String() != "0.52.0" {
			t.Errorf("%s runtime interval = %s, want exact qualified 0.52.0", fact.ID, fact.RuntimeVersions.String())
		}
		if fact.Target != "gemini" || fact.RuntimeID != "gemini-cli" || fact.AdapterID != "cortex-ia/gemini" {
			t.Errorf("%s identity = %q/%q/%q", fact.ID, fact.Target, fact.RuntimeID, fact.AdapterID)
		}
	}
	if direct.Mode != capability.CapabilityAvailable || direct.Experimental || direct.Enforcement != capability.EnforcementRuntime {
		t.Errorf("direct-child fact = %+v, want qualified non-experimental runtime capability", direct)
	}
	if isolatedTools.Mode != capability.CapabilityAvailable || !isolatedTools.Experimental || isolatedTools.Enforcement != capability.EnforcementRuntime {
		t.Errorf("tool-isolation fact = %+v, want experimental runtime capability", isolatedTools)
	}
	if nested.Mode != capability.CapabilityAbsent || nested.Cardinality != capability.CardinalityNone || nested.Enforcement != capability.EnforcementNone {
		t.Errorf("nested fact = %+v, want explicit absence", nested)
	}
	for _, fact := range []capability.CapabilityFact{direct, isolatedTools} {
		if fact.EvidenceClass != capability.EvidenceInstalledSchema || !strings.Contains(fact.EvidenceRef, "github.com/google-gemini/gemini-cli") {
			t.Errorf("%s evidence = %q/%q, want tagged official installed-schema source", fact.ID, fact.EvidenceClass, fact.EvidenceRef)
		}
		if fact.Probe == nil || fact.Probe.Method != capability.ProbeProtocol || !strings.HasPrefix(fact.Probe.EvidenceDigest, "sha256:") {
			t.Errorf("%s schema probe is incomplete: %+v", fact.ID, fact.Probe)
		}
	}

	catalog := capability.Catalog{SchemaVersion: capability.CatalogSchema.Current, Version: capability.CatalogSchema.Current, Facts: first}
	if err := catalog.Validate(time.Date(2026, time.July, 27, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("Gemini capability catalog is invalid: %v", err)
	}
}

func TestCapabilityFactsRequireExplicitExperimentalNativeOptIn(t *testing.T) {
	facts := NewAdapter().CapabilityFacts()
	experimental := geminiFact(t, facts, "isolation/tool-scope")
	result := capability.ProbeResult{Record: *experimental.Probe, Refined: experimental}
	request := geminiProbeRequest(experimental, false)
	if _, err := capability.ApplyProbeResult(request, result); err == nil || !strings.Contains(err.Error(), "explicit opt-in") {
		t.Fatalf("experimental capability without opt-in error = %v, want explicit opt-in rejection", err)
	}
	request.Authority.ExperimentalOptIn = true
	if _, err := capability.ApplyProbeResult(request, result); err != nil {
		t.Fatalf("experimental capability with opt-in error = %v", err)
	}

	catalog := capability.Catalog{SchemaVersion: capability.CatalogSchema.Current, Version: capability.CatalogSchema.Current, Facts: facts}
	if err := catalog.Validate(experimental.FreshUntil.Add(time.Nanosecond)); err == nil {
		t.Fatal("stale Gemini evidence remained current instead of forcing conservative resolution")
	}
}

func TestCapabilityProberRecordsRedactedQualifiedZeroMajorVersion(t *testing.T) {
	a := NewAdapter()
	now := time.Date(2026, time.July, 27, 1, 2, 3, 0, time.UTC)
	a.runProbe = func(context.Context, string, ...string) ([]byte, error) {
		return []byte("0.52.0\n"), nil
	}
	a.now = func() time.Time { return now }
	base := geminiFact(t, a.CapabilityFacts(), "delegation/direct-child")
	request := geminiProbeRequest(base, false)

	result, err := a.CapabilityProber().Probe(context.Background(), request)
	if err != nil {
		t.Fatalf("Probe() error = %v", err)
	}
	if result.Record.Command != "gemini --version" || result.Record.Result != "qualified-version:0.52.0" || result.Record.Timestamp != now {
		t.Errorf("probe record = %+v", result.Record)
	}
	if !strings.HasPrefix(result.Record.EvidenceDigest, "sha256:") || strings.Contains(result.Record.EvidenceDigest, "0.52.0") {
		t.Errorf("probe evidence is not redacted: %q", result.Record.EvidenceDigest)
	}
	if _, err := capability.ApplyProbeResult(request, result); err != nil {
		t.Fatalf("ApplyProbeResult() error = %v", err)
	}
}

func TestCapabilityProberRejectsUnqualifiedAndForeignFacts(t *testing.T) {
	tests := []struct {
		name   string
		output string
		mutate func(*capability.CapabilityFact)
	}{
		{name: "newer unqualified version", output: "Gemini CLI 0.53.0"},
		{name: "invalid version output", output: "Gemini CLI development build"},
		{name: "foreign adapter", output: "0.52.0", mutate: func(f *capability.CapabilityFact) { f.AdapterID = "cortex-ia/foreign" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := NewAdapter()
			a.runProbe = func(context.Context, string, ...string) ([]byte, error) { return []byte(tt.output), nil }
			base := geminiFact(t, a.CapabilityFacts(), "delegation/direct-child")
			if tt.mutate != nil {
				tt.mutate(&base)
			}
			_, err := a.CapabilityProber().Probe(context.Background(), geminiProbeRequest(base, false))
			if err == nil || !errors.Is(err, errUnqualifiedGeminiVersion) {
				t.Fatalf("Probe() error = %v, want errUnqualifiedGeminiVersion", err)
			}
		})
	}
}

func geminiFact(t *testing.T, facts []capability.CapabilityFact, id capability.CapabilityID) capability.CapabilityFact {
	t.Helper()
	for _, fact := range facts {
		if fact.ID == id {
			return fact
		}
	}
	t.Fatalf("CapabilityFacts() missing %q", id)
	return capability.CapabilityFact{}
}

func geminiProbeRequest(base capability.CapabilityFact, optIn bool) capability.ProbeRequest {
	return capability.ProbeRequest{
		Base: base,
		Authority: capability.ProbeAuthority{
			CapabilityID:      base.ID,
			RuntimeVersions:   base.RuntimeVersions,
			Modes:             []capability.CapabilityValue{base.Mode},
			Cardinalities:     []capability.Cardinality{base.Cardinality},
			Enforcement:       []capability.EnforcementClass{base.Enforcement},
			ExperimentalOptIn: optIn,
		},
	}
}
