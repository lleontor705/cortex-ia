package kimi

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
	if a.Agent() != model.AgentKimi {
		t.Errorf("Agent() = %v, want %v", a.Agent(), model.AgentKimi)
	}
}

func TestPaths(t *testing.T) {
	a := NewAdapter()
	home := "/home/user"

	if got := a.GlobalConfigDir(home); got != filepath.Join(home, ".kimi") {
		t.Errorf("GlobalConfigDir = %q", got)
	}
	if got := a.SystemPromptFile(home); got != filepath.Join(home, ".kimi", "KIMI.md") {
		t.Errorf("SystemPromptFile = %q", got)
	}
	if got := a.SettingsPath(home); got != filepath.Join(home, ".kimi", "config.toml") {
		t.Errorf("SettingsPath = %q", got)
	}
	if got := a.SkillsDir(home); got != filepath.Join(home, ".kimi", "skills") {
		t.Errorf("SkillsDir = %q", got)
	}
	if got := a.MCPConfigPath(home, "cortex"); got != filepath.Join(home, ".kimi", "mcp.json") {
		t.Errorf("MCPConfigPath = %q", got)
	}
	if got := a.SubAgentsDir(home); got != filepath.Join(home, ".kimi", "agents") {
		t.Errorf("SubAgentsDir = %q", got)
	}
}

func TestInstallCommands(t *testing.T) {
	a := NewAdapter()
	cmds := a.InstallCommands(system.PlatformProfile{})
	if len(cmds) != 1 || cmds[0][0] != "uv" {
		t.Errorf("expected uv tool install, got %v", cmds)
	}
}

func TestDetect_DirMissing(t *testing.T) {
	a := &Adapter{
		lookPath:    func(string) (string, error) { return "/usr/local/bin/kimi", nil },
		statPath:    func(string) statResult { return statResult{err: os.ErrNotExist} },
		pathExists:  func(string) bool { return false },
		userHomeDir: func() (string, error) { return "/home/test", nil },
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

func TestCapabilityFactsCarryConservativeKimiEvidence(t *testing.T) {
	facts := NewAdapter().CapabilityFacts()
	if len(facts) != 3 {
		t.Fatalf("CapabilityFacts() returned %d facts, want 3", len(facts))
	}

	byID := make(map[capability.CapabilityID]capability.CapabilityFact, len(facts))
	for _, fact := range facts {
		byID[fact.ID] = fact
		if fact.Target != "kimi" || fact.RuntimeID != "kimi-cli" || fact.AdapterID != "cortex-ia/kimi" {
			t.Errorf("fact %q identity = target %q runtime %q adapter %q", fact.ID, fact.Target, fact.RuntimeID, fact.AdapterID)
		}
	}

	direct := byID["delegation/direct-child"]
	if direct.Mode != capability.CapabilityAvailable || direct.Cardinality != capability.CardinalityMany {
		t.Errorf("direct-child fact = mode %q cardinality %q", direct.Mode, direct.Cardinality)
	}
	if direct.EvidenceClass != capability.EvidenceDocumentation || direct.EvidenceRef == "" {
		t.Errorf("direct-child evidence = class %q ref %q", direct.EvidenceClass, direct.EvidenceRef)
	}
	if direct.RuntimeVersions.Minimum.String() != "1.49.0" || direct.RuntimeVersions.MaximumTested.String() != "1.49.0" {
		t.Errorf("direct-child versions = %s", direct.RuntimeVersions.String())
	}
	if direct.ObservedAt.IsZero() || !direct.FreshUntil.After(direct.ObservedAt) || direct.Enforcement != capability.EnforcementPrompt {
		t.Errorf("direct-child freshness/enforcement = observed %v fresh %v enforcement %q", direct.ObservedAt, direct.FreshUntil, direct.Enforcement)
	}

	parallel := byID["delegation/background-parallel"]
	if !parallel.Experimental || parallel.Enforcement != capability.EnforcementPrompt {
		t.Errorf("background-parallel must remain advisory and experimental until an opted-in probe: %+v", parallel)
	}

	nested := byID["delegation/nested"]
	if nested.Mode != capability.CapabilityAbsent || nested.Cardinality != capability.CardinalityNone || nested.Enforcement != capability.EnforcementNone {
		t.Errorf("nested fact must visibly declare unsupported: %+v", nested)
	}
}

func TestCapabilityFactsDegradeWhenEvidenceIsAdvisoryOrStale(t *testing.T) {
	facts := NewAdapter().CapabilityFacts()
	if facts[0].Enforcement == capability.EnforcementRuntime {
		t.Fatal("documentation-only evidence must not silently qualify runtime enforcement")
	}

	stale := facts[0].FreshUntil.Add(time.Second)
	catalog := capability.Catalog{
		SchemaVersion: ir.MustParseVersion("1.0.0"),
		Version:       ir.MustParseVersion("1.0.0"),
		Facts:         facts,
	}
	if err := catalog.Validate(stale); err == nil || !strings.Contains(err.Error(), "fresh_until") {
		t.Fatalf("Validate(stale) error = %v, want visible freshness failure", err)
	}
}

func TestCapabilityProberQualifiesSupportedVersionWithoutRuntimeControl(t *testing.T) {
	observed := time.Date(2026, time.July, 26, 12, 0, 0, 0, time.UTC)
	a := NewAdapter()
	a.runInfo = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if name != "kimi" || strings.Join(args, " ") != "info --json" {
			t.Fatalf("command = %q %q", name, args)
		}
		return []byte(`{"kimi_cli_version":"1.49.0","agent_spec_versions":["1"],"wire_protocol_version":"1.10","python_version":"3.13.5"}`), nil
	}
	a.now = func() time.Time { return observed }

	base := a.CapabilityFacts()[0]
	result, err := a.CapabilityProber().Probe(context.Background(), capability.ProbeRequest{
		Base: base,
		Authority: capability.ProbeAuthority{
			CapabilityID:    base.ID,
			RuntimeVersions: base.RuntimeVersions,
			Modes:           []capability.CapabilityValue{capability.CapabilityAvailable},
			Cardinalities:   []capability.Cardinality{capability.CardinalityMany},
			Enforcement:     []capability.EnforcementClass{capability.EnforcementPrompt},
		},
	})
	if err != nil {
		t.Fatalf("Probe() error = %v", err)
	}
	if result.Refined.Enforcement != capability.EnforcementPrompt || result.Record.Method != capability.ProbeCommand {
		t.Fatalf("probe result = %+v", result)
	}
	if result.Record.Command != "kimi info --json" || result.Record.Timestamp != observed || !strings.HasPrefix(result.Record.EvidenceDigest, "sha256:") {
		t.Errorf("probe record = %+v", result.Record)
	}
	if _, err := capability.ApplyProbeResult(capability.ProbeRequest{
		Base: base,
		Authority: capability.ProbeAuthority{
			CapabilityID:    base.ID,
			RuntimeVersions: base.RuntimeVersions,
			Modes:           []capability.CapabilityValue{capability.CapabilityAvailable},
			Cardinalities:   []capability.Cardinality{capability.CardinalityMany},
			Enforcement:     []capability.EnforcementClass{capability.EnforcementPrompt},
		},
	}, result); err != nil {
		t.Fatalf("ApplyProbeResult() error = %v", err)
	}
}

func TestCapabilityProberBlocksUnsupportedVersionAndNativeWithoutOptIn(t *testing.T) {
	tests := []struct {
		name      string
		version   string
		factIndex int
		optIn     bool
		want      string
	}{
		{name: "unsupported runtime version", version: "2.0.0", factIndex: 0, want: "outside supported interval"},
		{name: "experimental native capability", version: "1.49.0", factIndex: 1, want: "explicit opt-in"},
		{name: "experimental native capability opted in", version: "1.49.0", factIndex: 1, optIn: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := NewAdapter()
			a.runInfo = func(context.Context, string, ...string) ([]byte, error) {
				return []byte(`{"kimi_cli_version":"` + tt.version + `","agent_spec_versions":["1"],"wire_protocol_version":"1.10","python_version":"3.13.5"}`), nil
			}
			base := a.CapabilityFacts()[tt.factIndex]
			_, err := a.CapabilityProber().Probe(context.Background(), capability.ProbeRequest{
				Base: base,
				Authority: capability.ProbeAuthority{
					CapabilityID:      base.ID,
					RuntimeVersions:   base.RuntimeVersions,
					Modes:             []capability.CapabilityValue{capability.CapabilityAvailable},
					Cardinalities:     []capability.Cardinality{capability.CardinalityMany},
					Enforcement:       []capability.EnforcementClass{capability.EnforcementPrompt},
					ExperimentalOptIn: tt.optIn,
				},
			})
			if tt.want == "" && err != nil {
				t.Fatalf("Probe() error = %v", err)
			}
			if tt.want != "" && (err == nil || !strings.Contains(err.Error(), tt.want)) {
				t.Fatalf("Probe() error = %v, want text %q", err, tt.want)
			}
		})
	}
}
