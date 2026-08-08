package opencode

import (
	"context"
	"os"
	"path/filepath"
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
	if a.Agent() != model.AgentOpenCode {
		t.Errorf("expected %s, got %s", model.AgentOpenCode, a.Agent())
	}
}

func TestSystemPromptFile(t *testing.T) {
	a := NewAdapter()
	got := a.SystemPromptFile("/home/test")
	if got == "" {
		t.Error("expected non-empty SystemPromptFile")
	}
}

func TestSettingsPathPrefersEffectiveGlobalConfig(t *testing.T) {
	adapter := NewAdapter()
	for _, tc := range []struct {
		name  string
		files []string
		want  string
	}{
		{name: "creates json by default", want: "opencode.json"},
		{name: "uses existing json", files: []string{"opencode.json"}, want: "opencode.json"},
		{name: "uses existing jsonc", files: []string{"opencode.jsonc"}, want: "opencode.jsonc"},
		{name: "jsonc wins coexistence", files: []string{"opencode.json", "opencode.jsonc"}, want: "opencode.jsonc"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			configDir := filepath.Join(home, ".config", "opencode")
			if err := os.MkdirAll(configDir, 0o755); err != nil {
				t.Fatal(err)
			}
			for _, name := range tc.files {
				if err := os.WriteFile(filepath.Join(configDir, name), []byte("{}"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			want := filepath.Join(configDir, tc.want)
			if got := adapter.SettingsPath(home); got != want {
				t.Errorf("SettingsPath() = %q, want %q", got, want)
			}
			if got := adapter.MCPConfigPath(home, "cortex"); got != want {
				t.Errorf("MCPConfigPath() = %q, want %q", got, want)
			}
		})
	}
}

func TestCapabilityFactsCarryQualificationProvenance(t *testing.T) {
	adapter := NewAdapter()
	facts := adapter.CapabilityFacts()
	if len(facts) != 2 {
		t.Fatalf("CapabilityFacts() count = %d, want 2", len(facts))
	}

	now := time.Date(2026, time.July, 26, 0, 0, 0, 0, time.UTC)
	for _, fact := range facts {
		if fact.Target != "opencode" || fact.RuntimeID != "opencode" || fact.AdapterID != "cortex-ia/opencode" {
			t.Errorf("identity for %q = target %q runtime %q adapter %q", fact.ID, fact.Target, fact.RuntimeID, fact.AdapterID)
		}
		if fact.EvidenceClass != capability.EvidenceExecutableProbe || fact.EvidenceRef == "" || fact.Probe == nil {
			t.Errorf("provenance for %q = class %q ref %q", fact.ID, fact.EvidenceClass, fact.EvidenceRef)
		}
		if fact.ObservedAt.IsZero() || !fact.FreshUntil.After(now) || fact.Confidence <= 0 {
			t.Errorf("freshness for %q = observed %v fresh until %v confidence %v", fact.ID, fact.ObservedAt, fact.FreshUntil, fact.Confidence)
		}
		if fact.Enforcement != capability.EnforcementRuntime || !fact.Current {
			t.Errorf("qualification for %q = enforcement %q current %v", fact.ID, fact.Enforcement, fact.Current)
		}
		if got := fact.RuntimeVersions.MaximumTested.String(); got != "1.18.11" {
			t.Errorf("maximum tested version for %q = %s, want 1.18.11", fact.ID, got)
		}
		if !strings.Contains(fact.EvidenceRef, "/1.18.11/") {
			t.Errorf("evidence reference for %q = %q, want 1.18.11 qualification", fact.ID, fact.EvidenceRef)
		}
	}
	catalog := capability.Catalog{
		SchemaVersion: ir.MustParseVersion("1.0.0"),
		Version:       ir.MustParseVersion("1.0.0"),
		Facts:         facts,
	}
	if err := catalog.Validate(now); err != nil {
		t.Fatalf("OpenCode capability catalog is invalid: %v", err)
	}

	directChild := factByID(t, facts, "delegation/direct-child")
	if directChild.Experimental || directChild.Cardinality != capability.CardinalityMany {
		t.Errorf("direct-child fact = experimental %v cardinality %q", directChild.Experimental, directChild.Cardinality)
	}
	nested := factByID(t, facts, "delegation/nested")
	if !nested.Experimental || nested.Cardinality != capability.CardinalityMany {
		t.Errorf("nested fact = experimental %v cardinality %q", nested.Experimental, nested.Cardinality)
	}
}

func TestCapabilityProberUsesReadOnlyVersionProbeWithinAuthority(t *testing.T) {
	adapter := NewAdapter()
	var command string
	adapter.runCommand = func(_ context.Context, name string, args ...string) ([]byte, error) {
		command = strings.Join(append([]string{name}, args...), " ")
		return []byte("opencode version: 1.18.11\nos: test\nplugins:\nnone\n"), nil
	}
	base := factByID(t, adapter.CapabilityFacts(), "delegation/direct-child")
	request := capability.ProbeRequest{
		Base: base,
		Authority: capability.ProbeAuthority{
			CapabilityID:    base.ID,
			RuntimeVersions: base.RuntimeVersions,
			Modes:           []capability.CapabilityValue{capability.CapabilityAvailable},
			Cardinalities:   []capability.Cardinality{capability.CardinalityMany},
			Enforcement:     []capability.EnforcementClass{capability.EnforcementRuntime},
		},
	}

	result, err := adapter.CapabilityProber().Probe(context.Background(), request)
	if err != nil {
		t.Fatalf("Probe() error = %v", err)
	}
	if command != "opencode debug info" {
		t.Fatalf("probe command = %q, want read-only debug info", command)
	}
	if result.Record.Command != "opencode debug info" || result.Record.Result != "available:many" || !strings.HasPrefix(result.Record.EvidenceDigest, "sha256:") {
		t.Errorf("probe record = %+v", result.Record)
	}
	if result.Record.Timestamp.IsZero() || result.Refined.RuntimeVersions.Minimum.String() != "1.18.11" || result.Refined.RuntimeVersions.MaximumTested.String() != "1.18.11" {
		t.Errorf("probe refinement = %+v", result.Refined)
	}
	if _, err := capability.ApplyProbeResult(request, result); err != nil {
		t.Fatalf("ApplyProbeResult() error = %v", err)
	}
}

func TestCapabilityProberRejectsUnqualifiedRuntimeVersion(t *testing.T) {
	adapter := NewAdapter()
	adapter.runCommand = func(context.Context, string, ...string) ([]byte, error) {
		return []byte("opencode version: 1.18.15\n"), nil
	}
	base := factByID(t, adapter.CapabilityFacts(), "delegation/direct-child")
	_, err := adapter.CapabilityProber().Probe(context.Background(), capability.ProbeRequest{
		Base: base,
		Authority: capability.ProbeAuthority{
			CapabilityID:    base.ID,
			RuntimeVersions: base.RuntimeVersions,
			Modes:           []capability.CapabilityValue{capability.CapabilityAvailable},
			Cardinalities:   []capability.Cardinality{capability.CardinalityMany},
			Enforcement:     []capability.EnforcementClass{capability.EnforcementRuntime},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "outside qualified range") {
		t.Fatalf("Probe() error = %v, want qualified-range rejection", err)
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
