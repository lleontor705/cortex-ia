package qwen

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
	"github.com/lleontor705/cortex-ia/internal/system"
)

func TestAgentIdentity(t *testing.T) {
	a := NewAdapter()
	if a.Agent() != model.AgentQwenCode {
		t.Errorf("Agent() = %v, want %v", a.Agent(), model.AgentQwenCode)
	}
}

func TestPaths(t *testing.T) {
	a := NewAdapter()
	home := "/home/user"

	if got := a.GlobalConfigDir(home); got != filepath.Join(home, ".qwen") {
		t.Errorf("GlobalConfigDir = %q", got)
	}
	if got := a.SystemPromptFile(home); got != filepath.Join(home, ".qwen", "QWEN.md") {
		t.Errorf("SystemPromptFile = %q", got)
	}
	if got := a.SettingsPath(home); got != filepath.Join(home, ".qwen", "settings.json") {
		t.Errorf("SettingsPath = %q", got)
	}
}

func TestInstallCommands(t *testing.T) {
	a := NewAdapter()
	cmds := a.InstallCommands(system.PlatformProfile{OS: "linux"})
	if len(cmds) != 1 || cmds[0][0] != "npm" {
		t.Errorf("expected npm install, got %v", cmds)
	}
}

func TestDetect_DirMissing(t *testing.T) {
	a := &Adapter{
		lookPath: func(string) (string, error) { return "/usr/local/bin/qwen", nil },
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

func TestCapabilityFactsAreEvidenceBackedAndConservative(t *testing.T) {
	a := NewAdapter()
	if a.SupportsTaskDelegation() || a.SupportsSubAgents() {
		t.Fatal("experimental Qwen delegation must remain unavailable through legacy unconditional capability flags")
	}
	if got := a.SubAgentsDir("/home/user"); got != "" {
		t.Fatalf("SubAgentsDir() = %q, want no unconditional runtime-owned directory", got)
	}

	first := a.CapabilityFacts()
	second := a.CapabilityFacts()
	if len(first) != 2 || len(second) != 2 {
		t.Fatalf("CapabilityFacts() lengths = %d/%d, want direct-child and nested facts", len(first), len(second))
	}
	for index := range first {
		if first[index] != second[index] {
			t.Fatalf("CapabilityFacts() is not deterministic at index %d", index)
		}
	}

	direct := qwenCapabilityFact(t, first, "delegation/direct-child")
	if direct.Mode != capability.CapabilityAvailable || direct.Cardinality != capability.CardinalityMany {
		t.Errorf("direct-child availability = %q/%q", direct.Mode, direct.Cardinality)
	}
	if direct.Target != "qwen" || direct.RuntimeID != "qwen-code-cli" || direct.AdapterID != "cortex-ia/qwen" {
		t.Errorf("direct-child identity = %q/%q/%q", direct.Target, direct.RuntimeID, direct.AdapterID)
	}
	if direct.RuntimeVersions.Minimum.String() != "0.14.1" || direct.RuntimeVersions.MaximumTested.String() != "0.14.1" {
		t.Errorf("direct-child runtime interval = %s, want truthful qualified 0.x interval", direct.RuntimeVersions.String())
	}
	if direct.EvidenceClass != capability.EvidenceDocumentation || direct.EvidenceRef != "https://github.com/QwenLM/qwen-code/blob/v0.14.1/docs/users/features/sub-agents.md" {
		t.Errorf("direct-child evidence = %q/%q", direct.EvidenceClass, direct.EvidenceRef)
	}
	if direct.Enforcement != capability.EnforcementPrompt || !direct.Experimental || direct.Confidence <= 0 || direct.ObservedAt.IsZero() || !direct.FreshUntil.After(direct.ObservedAt) {
		t.Errorf("direct-child qualification is not conservatively bounded: %+v", direct)
	}

	nested := qwenCapabilityFact(t, first, "delegation/nested")
	if nested.Mode != capability.CapabilityAbsent || nested.Cardinality != capability.CardinalityNone || nested.Enforcement != capability.EnforcementNone {
		t.Errorf("nested delegation must remain explicitly unsupported: %+v", nested)
	}

	catalog := capability.Catalog{SchemaVersion: capability.CatalogSchema.Current, Version: capability.CatalogSchema.Current, Facts: first}
	if err := catalog.Validate(direct.ObservedAt.Add(time.Hour)); err != nil {
		t.Fatalf("Qwen capability catalog is invalid: %v", err)
	}
	if err := catalog.Validate(direct.FreshUntil); err == nil || !strings.Contains(err.Error(), "fresh_until") {
		t.Fatalf("stale catalog validation error = %v, want visible freshness failure", err)
	}
}

func TestCapabilityProberRecordsBoundedVersionEvidenceWithoutRuntimeAuthority(t *testing.T) {
	observed := time.Date(2026, time.July, 27, 1, 2, 3, 0, time.UTC)
	a := NewAdapter()
	runs := 0
	a.runProbe = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		runs++
		if name != "qwen" || strings.Join(args, " ") != "--version" {
			t.Fatalf("probe command = %q %q, want qwen --version", name, args)
		}
		return []byte("0.14.1\n"), nil
	}
	a.now = func() time.Time { return observed }
	base := qwenCapabilityFact(t, a.CapabilityFacts(), "delegation/direct-child")
	request := qwenProbeRequest(base, false)

	if _, err := a.CapabilityProber().Probe(context.Background(), request); err == nil || !strings.Contains(err.Error(), "explicit opt-in") {
		t.Fatalf("Probe() without opt-in error = %v, want explicit opt-in rejection", err)
	}
	if runs != 0 {
		t.Fatalf("probe ran %d commands before experimental opt-in", runs)
	}

	request.Authority.ExperimentalOptIn = true
	result, err := a.CapabilityProber().Probe(context.Background(), request)
	if err != nil {
		t.Fatalf("Probe() error = %v", err)
	}
	if result.Record.Method != capability.ProbeCommand || result.Record.Command != "qwen --version" || result.Record.Result != "qualified-version:0.14.1" || result.Record.Timestamp != observed {
		t.Errorf("probe record = %+v", result.Record)
	}
	if !strings.HasPrefix(result.Record.EvidenceDigest, "sha256:") || strings.Contains(result.Record.EvidenceDigest, "0.14.1") {
		t.Errorf("probe digest is not redacted: %q", result.Record.EvidenceDigest)
	}
	if result.Refined.Enforcement != capability.EnforcementPrompt || !result.Refined.Experimental {
		t.Errorf("probe claimed runtime authority: %+v", result.Refined)
	}
	if _, err := capability.ApplyProbeResult(request, result); err != nil {
		t.Fatalf("ApplyProbeResult() error = %v", err)
	}
}

func TestCapabilityProberRejectsUnqualifiedInputsConservatively(t *testing.T) {
	tests := []struct {
		name          string
		output        string
		mutateBase    func(*capability.CapabilityFact)
		mutateRequest func(*capability.ProbeRequest)
		wantRuns      int
		want          string
	}{
		{name: "older unqualified version", output: "0.14.0", wantRuns: 1, want: "outside qualified interval"},
		{name: "newer untested version", output: "qwen-code 0.15.0", wantRuns: 1, want: "outside qualified interval"},
		{name: "caller cannot widen tagged interval", output: "qwen-code 0.15.0", mutateRequest: func(request *capability.ProbeRequest) {
			request.Authority.RuntimeVersions = ir.VersionRange{Minimum: ir.MustParseVersion("0.14.1"), MaximumTested: ir.MustParseVersion("0.15.0")}
		}, wantRuns: 1, want: "outside qualified interval"},
		{name: "malformed version output", output: "qwen development build", wantRuns: 1, want: "semantic version"},
		{name: "foreign adapter fact", output: "0.14.1", mutateBase: func(f *capability.CapabilityFact) { f.AdapterID = "cortex-ia/foreign" }, wantRuns: 0, want: "unsupported capability identity"},
		{name: "explicitly absent capability", output: "0.14.1", mutateBase: func(f *capability.CapabilityFact) {
			*f = qwenCapabilityFact(t, NewAdapter().CapabilityFacts(), "delegation/nested")
		}, wantRuns: 0, want: "explicitly unsupported"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runs := 0
			a := NewAdapter()
			a.runProbe = func(context.Context, string, ...string) ([]byte, error) {
				runs++
				return []byte(tt.output), nil
			}
			base := qwenCapabilityFact(t, a.CapabilityFacts(), "delegation/direct-child")
			if tt.mutateBase != nil {
				tt.mutateBase(&base)
			}
			request := qwenProbeRequest(base, true)
			if tt.mutateRequest != nil {
				tt.mutateRequest(&request)
			}
			_, err := a.CapabilityProber().Probe(context.Background(), request)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Probe() error = %v, want text %q", err, tt.want)
			}
			if runs != tt.wantRuns {
				t.Fatalf("probe runs = %d, want %d", runs, tt.wantRuns)
			}
		})
	}
}

func qwenCapabilityFact(t *testing.T, facts []capability.CapabilityFact, id capability.CapabilityID) capability.CapabilityFact {
	t.Helper()
	for _, fact := range facts {
		if fact.ID == id {
			return fact
		}
	}
	t.Fatalf("CapabilityFacts() missing %q", id)
	return capability.CapabilityFact{}
}

func qwenProbeRequest(base capability.CapabilityFact, optIn bool) capability.ProbeRequest {
	return capability.ProbeRequest{
		Base: base,
		Authority: capability.ProbeAuthority{
			CapabilityID:      base.ID,
			RuntimeVersions:   ir.VersionRange{Minimum: ir.MustParseVersion("0.14.1"), MaximumTested: ir.MustParseVersion("0.14.1")},
			Modes:             []capability.CapabilityValue{base.Mode},
			Cardinalities:     []capability.Cardinality{base.Cardinality},
			Enforcement:       []capability.EnforcementClass{base.Enforcement},
			ExperimentalOptIn: optIn,
		},
	}
}
