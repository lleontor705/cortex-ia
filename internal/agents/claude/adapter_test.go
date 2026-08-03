package claude

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/capability"
	"github.com/lleontor705/cortex-ia/internal/model"
)

func TestAdapterIdentity(t *testing.T) {
	a := NewAdapter()
	if a.Agent() != model.AgentClaudeCode {
		t.Errorf("expected AgentClaudeCode, got %s", a.Agent())
	}
	if a.Tier() != model.TierFull {
		t.Errorf("expected TierFull, got %s", a.Tier())
	}
}

func TestAdapterPaths(t *testing.T) {
	a := NewAdapter()
	home := "/home/test"

	if got := a.GlobalConfigDir(home); got != filepath.Join(home, ".claude") {
		t.Errorf("GlobalConfigDir = %s", got)
	}
	if got := a.SystemPromptFile(home); got != filepath.Join(home, ".claude", "CLAUDE.md") {
		t.Errorf("SystemPromptFile = %s", got)
	}
	if got := a.SkillsDir(home); got != filepath.Join(home, ".claude", "skills") {
		t.Errorf("SkillsDir = %s", got)
	}
	if got := a.MCPConfigPath(home, "cortex"); got != filepath.Join(home, ".claude", "mcp", "cortex.json") {
		t.Errorf("MCPConfigPath = %s", got)
	}
}

func TestAdapterStrategies(t *testing.T) {
	a := NewAdapter()
	if a.SystemPromptStrategy() != model.StrategyMarkdownSections {
		t.Error("expected StrategyMarkdownSections")
	}
	if a.MCPStrategy() != model.StrategySeparateMCPFiles {
		t.Error("expected StrategySeparateMCPFiles")
	}
}

func TestAdapterCapabilities(t *testing.T) {
	a := NewAdapter()
	if !a.SupportsSkills() {
		t.Error("expected SupportsSkills=true")
	}
	if !a.SupportsMCP() {
		t.Error("expected SupportsMCP=true")
	}
	if !a.SupportsTaskDelegation() {
		t.Error("expected SupportsTaskDelegation=true")
	}
	if a.SupportsSubAgents() {
		t.Error("expected SupportsSubAgents=false")
	}
	if a.SupportsSlashCommands() {
		t.Error("expected SupportsSlashCommands=false")
	}
}

func TestDetectWithConfigDir(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".claude")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}

	a := &Adapter{
		lookPath: func(name string) (string, error) {
			return "/usr/local/bin/claude", nil
		},
	}

	installed, binaryPath, cfgPath, configFound, err := a.Detect(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	if !installed {
		t.Error("expected installed=true")
	}
	if binaryPath != "/usr/local/bin/claude" {
		t.Errorf("expected /usr/local/bin/claude, got %s", binaryPath)
	}
	if cfgPath != configDir {
		t.Errorf("expected %s, got %s", configDir, cfgPath)
	}
	if !configFound {
		t.Error("expected configFound=true")
	}
}

func TestDetectWithoutBinary(t *testing.T) {
	tmpDir := t.TempDir()

	a := &Adapter{
		lookPath: func(name string) (string, error) {
			return "", &os.PathError{Op: "lookpath", Path: name, Err: os.ErrNotExist}
		},
	}

	installed, _, _, _, err := a.Detect(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	if installed {
		t.Error("expected installed=false")
	}
}

func TestAdapterProvidesVersionedCapabilityFacts(t *testing.T) {
	facts := NewAdapter().CapabilityFacts()
	if len(facts) != 2 {
		t.Fatalf("CapabilityFacts() returned %d facts, want 2", len(facts))
	}

	now := time.Date(2026, time.July, 27, 0, 0, 0, 0, time.UTC)
	for _, fact := range facts {
		if fact.Target != "claude" || fact.RuntimeID != "claude-code" || fact.AdapterID != "cortex-ia/claude" {
			t.Errorf("fact %q identity = %q/%q/%q", fact.ID, fact.Target, fact.RuntimeID, fact.AdapterID)
		}
		if fact.RuntimeVersions.Minimum.String() == "0.0.0" || fact.RuntimeVersions.MaximumTested.String() == "0.0.0" {
			t.Errorf("fact %q has unversioned runtime interval", fact.ID)
		}
		if fact.EvidenceRef == "" || fact.ObservedAt.IsZero() || !fact.FreshUntil.After(fact.ObservedAt) {
			t.Errorf("fact %q has incomplete provenance or freshness", fact.ID)
		}
		if fact.Confidence <= 0 || fact.Confidence > 1 {
			t.Errorf("fact %q confidence = %v", fact.ID, fact.Confidence)
		}
		if fact.Enforcement != capability.EnforcementRuntime || fact.Probe == nil {
			t.Errorf("fact %q enforcement/probe = %q/%+v", fact.ID, fact.Enforcement, fact.Probe)
		}
	}
	if err := (capability.Catalog{
		SchemaVersion: capability.CatalogSchema.Current,
		Version:       capability.CatalogSchema.Current,
		Facts:         facts,
	}).Validate(now); err != nil {
		t.Fatalf("Claude capability catalog is invalid: %v", err)
	}
}

func TestCapabilityFactsRequireExplicitNativeOptInAndExpireConservatively(t *testing.T) {
	facts := NewAdapter().CapabilityFacts()
	var experimental capability.CapabilityFact
	for _, fact := range facts {
		if fact.ID == claudeAgentTeamsCapability {
			experimental = fact
		}
	}
	if !experimental.Experimental {
		t.Fatal("agent teams capability is not marked experimental")
	}

	result := capability.ProbeResult{Record: *experimental.Probe, Refined: experimental}
	request := capability.ProbeRequest{
		Base: experimental,
		Authority: capability.ProbeAuthority{
			CapabilityID:    experimental.ID,
			RuntimeVersions: experimental.RuntimeVersions,
			Modes:           []capability.CapabilityValue{experimental.Mode},
			Cardinalities:   []capability.Cardinality{experimental.Cardinality},
			Enforcement:     []capability.EnforcementClass{experimental.Enforcement},
		},
	}
	if _, err := capability.ApplyProbeResult(request, result); err == nil {
		t.Fatal("experimental native capability accepted without explicit opt-in")
	}
	request.Authority.ExperimentalOptIn = true
	if _, err := capability.ApplyProbeResult(request, result); err != nil {
		t.Fatalf("experimental native capability rejected with explicit opt-in: %v", err)
	}

	staleCatalog := capability.Catalog{
		SchemaVersion: capability.CatalogSchema.Current,
		Version:       capability.CatalogSchema.Current,
		Facts:         facts,
	}
	if err := staleCatalog.Validate(facts[0].FreshUntil.Add(time.Nanosecond)); err == nil {
		t.Fatal("stale capability facts remained current instead of failing conservatively")
	}
}

func TestCapabilityProberRecordsRedactedVersionEvidence(t *testing.T) {
	a := NewAdapter()
	now := time.Date(2026, time.July, 27, 1, 2, 3, 0, time.UTC)
	a.runProbe = func(context.Context, string, ...string) ([]byte, error) {
		return []byte("2.1.199 (Claude Code)\n"), nil
	}
	a.now = func() time.Time { return now }
	fact := a.CapabilityFacts()[0]
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

	result, err := a.CapabilityProber().Probe(context.Background(), request)
	if err != nil {
		t.Fatalf("Probe() error = %v", err)
	}
	if result.Record.Method != capability.ProbeCommand || result.Record.Command != "claude --version" {
		t.Errorf("probe method/command = %q/%q", result.Record.Method, result.Record.Command)
	}
	if result.Record.Result != "qualified-version:2.1.199" {
		t.Errorf("probe result = %q", result.Record.Result)
	}
	if result.Record.Timestamp != now {
		t.Errorf("probe timestamp = %v, want %v", result.Record.Timestamp, now)
	}
	if result.Record.EvidenceDigest == "" || result.Record.EvidenceDigest == "2.1.199 (Claude Code)" {
		t.Errorf("probe evidence digest is not redacted: %q", result.Record.EvidenceDigest)
	}
	if _, err := capability.ApplyProbeResult(request, result); err != nil {
		t.Fatalf("ApplyProbeResult() error = %v", err)
	}
}

func TestCapabilityProberRejectsUnqualifiedVersions(t *testing.T) {
	a := NewAdapter()
	a.runProbe = func(context.Context, string, ...string) ([]byte, error) {
		return []byte("3.0.0 (Claude Code)\n"), nil
	}
	fact := a.CapabilityFacts()[0]

	_, err := a.CapabilityProber().Probe(context.Background(), capability.ProbeRequest{Base: fact})
	if err == nil {
		t.Fatal("Probe() error = nil, want conservative rejection")
	}
	if !errors.Is(err, errUnqualifiedClaudeVersion) {
		t.Fatalf("Probe() error = %v, want errUnqualifiedClaudeVersion", err)
	}
}
