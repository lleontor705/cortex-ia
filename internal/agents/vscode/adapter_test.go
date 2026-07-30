package vscode

import (
	"context"
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
	if a.Agent() != model.AgentVSCodeCopilot {
		t.Errorf("expected %s, got %s", model.AgentVSCodeCopilot, a.Agent())
	}
}

func TestSystemPromptFile(t *testing.T) {
	a := NewAdapter()
	got := a.SystemPromptFile("/home/test")
	if got == "" {
		t.Error("expected non-empty SystemPromptFile")
	}
}

func TestCapabilityFactsAreVersionedAndConservative(t *testing.T) {
	observedAt := time.Date(2026, time.July, 26, 0, 0, 0, 0, time.UTC)
	facts := NewAdapter().CapabilityFacts()
	if len(facts) != 2 {
		t.Fatalf("CapabilityFacts() returned %d facts, want direct-child and nested facts", len(facts))
	}

	direct := vscodeCapabilityFact(t, facts, "delegation/direct-child")
	if direct.Mode != capability.CapabilityAvailable || direct.Cardinality != capability.CardinalityMany {
		t.Fatalf("direct-child delegation = %q/%q, want available/many", direct.Mode, direct.Cardinality)
	}
	if direct.Target != "vscode" || direct.RuntimeID != "vscode-copilot" || direct.AdapterID != "cortex-ia/vscode" {
		t.Fatalf("direct-child identity = %q/%q/%q", direct.Target, direct.RuntimeID, direct.AdapterID)
	}
	if direct.RuntimeVersions.Minimum.String() != "1.115.0" || direct.RuntimeVersions.MaximumTested.String() != "1.115.0" {
		t.Fatalf("direct-child version interval = %s, want [1.115.0,1.115.0]", direct.RuntimeVersions.String())
	}
	if direct.EvidenceClass != capability.EvidenceDocumentation || direct.EvidenceRef != "https://code.visualstudio.com/docs/agents/subagents" {
		t.Fatalf("direct-child evidence = %q/%q", direct.EvidenceClass, direct.EvidenceRef)
	}
	if !direct.Experimental || direct.Enforcement != capability.EnforcementPrompt || direct.Confidence <= 0 {
		t.Fatalf("direct-child qualification is optimistic or incomplete: %+v", direct)
	}

	nested := vscodeCapabilityFact(t, facts, "delegation/nested")
	if nested.Mode != capability.CapabilityAbsent || nested.Cardinality != capability.CardinalityNone || nested.Enforcement != capability.EnforcementNone {
		t.Fatalf("nested delegation must remain unsupported: %+v", nested)
	}

	catalog := capability.Catalog{SchemaVersion: capability.CatalogSchema.Current, Version: capability.CatalogSchema.Current, Facts: facts}
	if err := catalog.Validate(observedAt.Add(time.Hour)); err != nil {
		t.Fatalf("VS Code capability catalog is invalid: %v", err)
	}
	if err := catalog.Validate(direct.FreshUntil); err == nil || !strings.Contains(err.Error(), "fresh_until") {
		t.Fatalf("stale catalog validation error = %v, want visible freshness failure", err)
	}
}

func TestCapabilityProberRecordsRedactedVersionEvidence(t *testing.T) {
	observedAt := time.Date(2026, time.July, 27, 12, 34, 56, 0, time.UTC)
	a := NewAdapter()
	a.runProbe = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if name != "code" || strings.Join(args, " ") != "--version" {
			t.Fatalf("probe command = %q %q, want code --version", name, args)
		}
		return []byte("1.115.0\n0123456789abcdef\nx64\n"), nil
	}
	a.now = func() time.Time { return observedAt }
	base := vscodeCapabilityFact(t, a.CapabilityFacts(), "delegation/direct-child")
	request := capability.ProbeRequest{
		Base: base,
		Authority: capability.ProbeAuthority{
			CapabilityID:      base.ID,
			RuntimeVersions:   base.RuntimeVersions,
			Modes:             []capability.CapabilityValue{base.Mode},
			Cardinalities:     []capability.Cardinality{base.Cardinality},
			Enforcement:       []capability.EnforcementClass{base.Enforcement},
			ExperimentalOptIn: true,
		},
	}

	result, err := a.CapabilityProber().Probe(context.Background(), request)
	if err != nil {
		t.Fatalf("Probe() error = %v", err)
	}
	if result.Record.Method != capability.ProbeCommand || result.Record.Command != "code --version" {
		t.Fatalf("probe method/command = %q/%q", result.Record.Method, result.Record.Command)
	}
	if result.Record.Result != "qualified-version:1.115.0" || !result.Record.Timestamp.Equal(observedAt) {
		t.Fatalf("probe result/timestamp = %q/%v", result.Record.Result, result.Record.Timestamp)
	}
	if !strings.HasPrefix(result.Record.EvidenceDigest, "sha256:") || strings.Contains(result.Record.EvidenceDigest, "0123456789abcdef") {
		t.Fatalf("probe digest is not redacted: %q", result.Record.EvidenceDigest)
	}
	if _, err := capability.ApplyProbeResult(request, result); err != nil {
		t.Fatalf("ApplyProbeResult() error = %v", err)
	}
}

func TestCapabilityProberRejectsUnqualifiedInputs(t *testing.T) {
	tests := []struct {
		name       string
		output     string
		mutateBase func(*capability.CapabilityFact)
		want       string
	}{
		{name: "older version", output: "1.114.0\ncommit\nx64", want: "outside supported interval"},
		{name: "newer untested version", output: "1.116.0\ncommit\nx64", want: "outside supported interval"},
		{name: "malformed output", output: "development build", want: "semantic version"},
		{name: "foreign adapter", output: "1.115.0\ncommit\nx64", mutateBase: func(f *capability.CapabilityFact) { f.AdapterID = "cortex-ia/foreign" }, want: "unsupported capability identity"},
		{name: "nested capability", output: "1.115.0\ncommit\nx64", mutateBase: func(f *capability.CapabilityFact) {
			*f = vscodeCapabilityFact(t, NewAdapter().CapabilityFacts(), "delegation/nested")
		}, want: "explicitly unsupported"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := NewAdapter()
			a.runProbe = func(context.Context, string, ...string) ([]byte, error) { return []byte(tt.output), nil }
			base := vscodeCapabilityFact(t, a.CapabilityFacts(), "delegation/direct-child")
			if tt.mutateBase != nil {
				tt.mutateBase(&base)
			}
			_, err := a.CapabilityProber().Probe(context.Background(), capability.ProbeRequest{
				Base: base,
				Authority: capability.ProbeAuthority{
					CapabilityID:      base.ID,
					RuntimeVersions:   ir.VersionRange{Minimum: ir.MustParseVersion("1.115.0"), MaximumTested: ir.MustParseVersion("1.115.0")},
					Modes:             []capability.CapabilityValue{base.Mode},
					Cardinalities:     []capability.Cardinality{base.Cardinality},
					Enforcement:       []capability.EnforcementClass{base.Enforcement},
					ExperimentalOptIn: true,
				},
			})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Probe() error = %v, want text %q", err, tt.want)
			}
		})
	}
}

func vscodeCapabilityFact(t *testing.T, facts []capability.CapabilityFact, id capability.CapabilityID) capability.CapabilityFact {
	t.Helper()
	for _, fact := range facts {
		if fact.ID == id {
			return fact
		}
	}
	t.Fatalf("CapabilityFacts() missing %q", id)
	return capability.CapabilityFact{}
}
