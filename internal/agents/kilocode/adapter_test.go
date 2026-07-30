package kilocode

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

func TestAgentIdentity(t *testing.T) {
	a := NewAdapter()
	if a.Agent() != model.AgentKilocode {
		t.Errorf("Agent() = %v, want %v", a.Agent(), model.AgentKilocode)
	}
}

func TestPaths(t *testing.T) {
	a := NewAdapter()
	home := "/home/user"
	want := filepath.Join(home, ".config", "kilo")

	if got := a.GlobalConfigDir(home); got != want {
		t.Errorf("GlobalConfigDir = %q, want %q", got, want)
	}
	if got := a.SystemPromptFile(home); got != filepath.Join(want, "AGENTS.md") {
		t.Errorf("SystemPromptFile = %q", got)
	}
	if got := a.MCPConfigPath(home, "cortex"); got != filepath.Join(want, "opencode.json") {
		t.Errorf("MCPConfigPath = %q", got)
	}
	if got := a.SkillsDir(home); got != filepath.Join(want, "skills") {
		t.Errorf("SkillsDir = %q", got)
	}
	if got := a.SettingsPath(home); got != filepath.Join(want, "opencode.json") {
		t.Errorf("SettingsPath = %q", got)
	}
	if got := a.CommandsDir(home); got != filepath.Join(want, "commands") {
		t.Errorf("CommandsDir = %q", got)
	}
	if got := a.SubAgentsDir(home); got != filepath.Join(want, "agents") {
		t.Errorf("SubAgentsDir = %q", got)
	}
}

func TestDetect_DirMissing(t *testing.T) {
	a := &Adapter{
		lookPath: func(string) (string, error) { return "/usr/local/bin/kilo", nil },
		statPath: func(string) statResult { return statResult{err: os.ErrNotExist} },
	}
	installed, _, _, configFound, err := a.Detect("/home/test")
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if !installed {
		t.Error("expected installed=true when binary present")
	}
	if configFound {
		t.Error("expected configFound=false when dir missing")
	}
}

func TestAdapterPublishesEvidenceBackedCapabilityFacts(t *testing.T) {
	a := NewAdapter()
	if !a.SupportsTaskDelegation() || !a.SupportsSubAgents() {
		t.Fatal("Kilo CLI 1.0+ documented subagents must be exposed as direct-child delegation")
	}

	facts := a.CapabilityFacts()
	if len(facts) != 2 {
		t.Fatalf("CapabilityFacts() returned %d facts, want direct-child and nested facts", len(facts))
	}
	byID := make(map[capability.CapabilityID]capability.CapabilityFact, len(facts))
	for _, fact := range facts {
		byID[fact.ID] = fact
	}

	direct := byID["delegation/direct-child"]
	if direct.Mode != capability.CapabilityAvailable || direct.Cardinality != capability.CardinalityMany {
		t.Errorf("direct-child availability = %q/%q", direct.Mode, direct.Cardinality)
	}
	if direct.Target != "kilocode" || direct.RuntimeID != "kilo-cli" || direct.AdapterID != "cortex-ia/kilocode" {
		t.Errorf("direct-child identity = %q/%q/%q", direct.Target, direct.RuntimeID, direct.AdapterID)
	}
	if direct.RuntimeVersions.Minimum.String() != "7.4.16" || direct.RuntimeVersions.MaximumTested.String() != "7.4.16" {
		t.Errorf("direct-child runtime interval = %s", direct.RuntimeVersions.String())
	}
	if direct.EvidenceClass != capability.EvidenceDocumentation || direct.EvidenceRef != "https://kilo.ai/docs/customize/custom-subagents" {
		t.Errorf("direct-child evidence = %q/%q", direct.EvidenceClass, direct.EvidenceRef)
	}
	if direct.Enforcement != capability.EnforcementPrompt || direct.Confidence <= 0 || direct.ObservedAt.IsZero() || !direct.FreshUntil.After(direct.ObservedAt) {
		t.Errorf("direct-child qualification is incomplete: %+v", direct)
	}

	nested := byID["delegation/nested"]
	if nested.Mode != capability.CapabilityAbsent || nested.Cardinality != capability.CardinalityNone || nested.Enforcement != capability.EnforcementNone {
		t.Errorf("nested delegation must remain explicitly unsupported: %+v", nested)
	}

	catalog := capability.Catalog{
		SchemaVersion: capability.CatalogSchema.Current,
		Version:       capability.CatalogSchema.Current,
		Facts:         facts,
	}
	if err := catalog.Validate(direct.ObservedAt.Add(time.Hour)); err != nil {
		t.Fatalf("Kilocode capability catalog is invalid: %v", err)
	}
	if err := catalog.Validate(direct.FreshUntil.Add(time.Nanosecond)); err == nil || !strings.Contains(err.Error(), "fresh_until") {
		t.Fatalf("stale catalog validation error = %v, want visible freshness failure", err)
	}
}

func TestCapabilityProberRecordsRedactedVersionEvidence(t *testing.T) {
	observed := time.Date(2026, time.July, 26, 12, 34, 56, 0, time.UTC)
	a := NewAdapter()
	a.runProbe = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if name != "kilo" || strings.Join(args, " ") != "--version" {
			t.Fatalf("probe command = %q %q", name, args)
		}
		return []byte("kilo version 7.4.16\n"), nil
	}
	a.now = func() time.Time { return observed }
	base := capabilityFact(t, a.CapabilityFacts(), "delegation/direct-child")
	request := capability.ProbeRequest{
		Base: base,
		Authority: capability.ProbeAuthority{
			CapabilityID:    base.ID,
			RuntimeVersions: base.RuntimeVersions,
			Modes:           []capability.CapabilityValue{base.Mode},
			Cardinalities:   []capability.Cardinality{base.Cardinality},
			Enforcement:     []capability.EnforcementClass{base.Enforcement},
		},
	}

	result, err := a.CapabilityProber().Probe(context.Background(), request)
	if err != nil {
		t.Fatalf("Probe() error = %v", err)
	}
	if result.Record.Command != "kilo --version" || result.Record.Method != capability.ProbeCommand {
		t.Errorf("probe method/command = %q/%q", result.Record.Method, result.Record.Command)
	}
	if result.Record.Result != "qualified-version:7.4.16" || result.Record.Timestamp != observed {
		t.Errorf("probe result/timestamp = %q/%v", result.Record.Result, result.Record.Timestamp)
	}
	if !strings.HasPrefix(result.Record.EvidenceDigest, "sha256:") || strings.Contains(result.Record.EvidenceDigest, "7.4.16") {
		t.Errorf("probe digest is not redacted: %q", result.Record.EvidenceDigest)
	}
	if result.Refined.RuntimeVersions.Minimum.String() != "7.4.16" || result.Refined.RuntimeVersions.MaximumTested.String() != "7.4.16" {
		t.Errorf("refined runtime interval = %s", result.Refined.RuntimeVersions.String())
	}
	if _, err := capability.ApplyProbeResult(request, result); err != nil {
		t.Fatalf("ApplyProbeResult() error = %v", err)
	}
}

func TestCapabilityProberRejectsUnsupportedInputs(t *testing.T) {
	tests := []struct {
		name       string
		output     string
		mutateBase func(*capability.CapabilityFact)
		want       string
	}{
		{name: "version before supported interval", output: "kilo version 7.4.15", want: "outside supported interval"},
		{name: "version beyond tested interval", output: "kilo version 7.5.0", want: "outside supported interval"},
		{name: "malformed version output", output: "kilo development build", want: "semantic version"},
		{name: "foreign adapter fact", output: "kilo version 7.4.16", mutateBase: func(f *capability.CapabilityFact) { f.AdapterID = "cortex-ia/foreign" }, want: "unsupported capability identity"},
		{name: "explicitly absent capability", output: "kilo version 7.4.16", mutateBase: func(f *capability.CapabilityFact) {
			*f = capabilityFact(t, NewAdapter().CapabilityFacts(), "delegation/nested")
		}, want: "explicitly unsupported"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := NewAdapter()
			a.runProbe = func(context.Context, string, ...string) ([]byte, error) { return []byte(tt.output), nil }
			base := capabilityFact(t, a.CapabilityFacts(), "delegation/direct-child")
			if tt.mutateBase != nil {
				tt.mutateBase(&base)
			}
			_, err := a.CapabilityProber().Probe(context.Background(), capability.ProbeRequest{
				Base: base,
				Authority: capability.ProbeAuthority{
					CapabilityID:    base.ID,
					RuntimeVersions: ir.VersionRange{Minimum: ir.MustParseVersion("7.4.16"), MaximumTested: ir.MustParseVersion("7.4.16")},
					Modes:           []capability.CapabilityValue{base.Mode},
					Cardinalities:   []capability.Cardinality{base.Cardinality},
					Enforcement:     []capability.EnforcementClass{base.Enforcement},
				},
			})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Probe() error = %v, want text %q", err, tt.want)
			}
		})
	}
}

func capabilityFact(t *testing.T, facts []capability.CapabilityFact, id capability.CapabilityID) capability.CapabilityFact {
	t.Helper()
	for _, fact := range facts {
		if fact.ID == id {
			return fact
		}
	}
	t.Fatalf("capability fact %q not found", id)
	return capability.CapabilityFact{}
}
