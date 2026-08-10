package pipeline

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lleontor705/cortex-ia/internal/agents"
	"github.com/lleontor705/cortex-ia/internal/agents/opencode"
	"github.com/lleontor705/cortex-ia/internal/components/sdd"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/capability"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
	"github.com/lleontor705/cortex-ia/internal/model"
)

func TestInstallDependencies_DefaultsAreComplete(t *testing.T) {
	deps := defaultInstallDependencies()
	if deps.qualifyCapabilities == nil || deps.now == nil || deps.prepareWorkflow == nil || deps.applyWorkflow == nil || deps.invokeComponent == nil || deps.invokePersona == nil ||
		deps.saveInstallStatus == nil || deps.clearInstallStatus == nil || deps.saveState == nil || deps.saveLock == nil ||
		deps.beginJournal == nil || deps.attachWorkflowReceipt == nil || deps.recordJournalOutcome == nil ||
		deps.commitJournal == nil || deps.restoreAndVerify == nil {
		t.Fatal("default install dependencies must supply every coordinator operation")
	}
}

func TestQualifyCapabilitiesAcceptsInstalledOpenCode11815WithinClosedAuthority(t *testing.T) {
	now := time.Date(2026, time.August, 9, 12, 0, 0, 0, time.UTC)
	base := opencode.NewAdapter()
	provider := base.CapabilityFacts()[0]
	prober := capabilityProberFunc(func(_ context.Context, request capability.ProbeRequest) (capability.ProbeResult, error) {
		if request.Authority.CapabilityID != provider.ID || request.Authority.RuntimeVersions != provider.RuntimeVersions {
			t.Fatalf("probe authority = %+v, want exact fact identity and versions", request.Authority)
		}
		if len(request.Authority.Modes) != 1 || request.Authority.Modes[0] != provider.Mode ||
			len(request.Authority.Cardinalities) != 1 || request.Authority.Cardinalities[0] != provider.Cardinality ||
			len(request.Authority.Enforcement) != 1 || request.Authority.Enforcement[0] != provider.Enforcement ||
			len(request.Authority.Permissions) != 0 || len(request.Authority.TrustClasses) != 0 {
			t.Fatalf("probe authority widened fact: %+v", request.Authority)
		}
		refined := request.Base
		version := ir.MustParseVersion("1.18.15")
		refined.RuntimeVersions = ir.VersionRange{Minimum: version, MaximumTested: version}
		return capability.ProbeResult{
			Record:  capability.ProbeRecord{ID: "probe/test/opencode-version", Method: capability.ProbeCommand, Command: "stub --version", Result: "qualified-version:1.18.15", Timestamp: now, EvidenceDigest: "sha256:stubbed"},
			Refined: refined,
		}, nil
	})
	adapter := capabilityTestAdapter{Adapter: base, facts: []capability.CapabilityFact{provider}, prober: prober}

	qualified := qualifyCapabilities(context.Background(), []agents.Adapter{adapter}, now, nil)
	facts := qualified[model.AgentOpenCode]
	if len(facts) != 1 {
		t.Fatalf("qualified facts = %+v, want one runtime-qualified fact", facts)
	}
	if gotMin, gotMax := facts[0].RuntimeVersions.Minimum.String(), facts[0].RuntimeVersions.MaximumTested.String(); gotMin != "1.18.15" || gotMax != "1.18.15" {
		t.Fatalf("qualified runtime versions = %s..%s", gotMin, gotMax)
	}
	if facts[0].FreshUntil != provider.FreshUntil {
		t.Fatalf("freshness widened from %s to %s", provider.FreshUntil, facts[0].FreshUntil)
	}
}

func TestQualifyCapabilitiesOutOfRangeDegradesWorkflowWithoutError(t *testing.T) {
	now := time.Date(2026, time.August, 9, 12, 0, 0, 0, time.UTC)
	base := opencode.NewAdapter()
	fact := base.CapabilityFacts()[0]
	prober := capabilityProberFunc(func(_ context.Context, request capability.ProbeRequest) (capability.ProbeResult, error) {
		refined := request.Base
		version := ir.MustParseVersion("1.19.0")
		refined.RuntimeVersions = ir.VersionRange{Minimum: version, MaximumTested: version}
		return capability.ProbeResult{
			Record:  capability.ProbeRecord{ID: "probe/test/opencode-version", Method: capability.ProbeCommand, Command: "stub --version", Result: "qualified-version:1.19.0", Timestamp: now, EvidenceDigest: "sha256:stubbed"},
			Refined: refined,
		}, nil
	})
	adapter := capabilityTestAdapter{Adapter: base, facts: []capability.CapabilityFact{fact}, prober: prober}
	qualified := qualifyCapabilities(context.Background(), []agents.Adapter{adapter}, now, nil)
	if len(qualified[model.AgentOpenCode]) != 0 {
		t.Fatalf("out-of-range fact was retained: %+v", qualified)
	}

	prepared, err := PrepareWorkflow(context.Background(), WorkflowRequest{
		HomeDir: t.TempDir(), Adapters: []agents.Adapter{adapter}, EvaluationTime: now,
		RequestedProfile: sdd.ProfileNativeAdvanced, QualifiedCapabilityFacts: qualified,
	})
	if err != nil {
		t.Fatalf("PrepareWorkflow() aborted after probe rejection: %v", err)
	}
	if prepared.Plan.Profile != string(sdd.ProfilePortableSequential) {
		t.Fatalf("profile = %q, want portable-sequential degradation", prepared.Plan.Profile)
	}
}

func TestInstallDependenciesQualifiesBeforePrepareWithoutRunningProcesses(t *testing.T) {
	now := time.Date(2026, time.August, 9, 12, 0, 0, 0, time.UTC)
	deps := defaultInstallDependencies()
	order := []string{}
	deps.now = func() time.Time { return now }
	deps.qualifyCapabilities = func(_ context.Context, adapters []agents.Adapter, gotNow time.Time, _ []capability.CapabilityID) map[model.AgentID][]capability.CapabilityFact {
		order = append(order, "qualify")
		if len(adapters) != 1 || gotNow != now {
			t.Fatalf("qualification input = adapters:%d now:%s", len(adapters), gotNow)
		}
		return map[model.AgentID][]capability.CapabilityFact{model.AgentOpenCode: {{ID: "delegation/direct-child"}}}
	}
	deps.prepareWorkflow = func(_ context.Context, request WorkflowRequest) (PreparedWorkflowInstall, error) {
		order = append(order, "prepare")
		if request.CapabilityEvaluationTime != now || !request.EvaluationTime.IsZero() || len(request.QualifiedCapabilityFacts[model.AgentOpenCode]) != 1 {
			t.Fatalf("prepare request lost qualified facts or clock: %+v", request)
		}
		return PreparedWorkflowInstall{}, nil
	}

	_, err := installWithDependencies(t.TempDir(), newTestRegistry(), model.Selection{
		Agents: []model.AgentID{model.AgentOpenCode}, Components: []model.ComponentID{model.ComponentSDD},
	}, "test-v1", true, deps)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(order, ",") != "qualify,prepare" {
		t.Fatalf("dependency order = %v, want qualification before preparation", order)
	}
}

type capabilityTestAdapter struct {
	agents.Adapter
	facts  []capability.CapabilityFact
	prober capability.Prober
}

func (a capabilityTestAdapter) CapabilityFacts() []capability.CapabilityFact { return a.facts }
func (a capabilityTestAdapter) CapabilityProber() capability.Prober          { return a.prober }

type capabilityProberFunc func(context.Context, capability.ProbeRequest) (capability.ProbeResult, error)

func (f capabilityProberFunc) Probe(ctx context.Context, request capability.ProbeRequest) (capability.ProbeResult, error) {
	return f(ctx, request)
}

func TestInstallDependencies_BeginJournalHookIsScoped(t *testing.T) {
	boom := errors.New("injected begin journal failure")
	deps := defaultInstallDependencies()
	deps.beginJournal = func(string, string, []ManagedTarget) (*InstallJournal, error) {
		return nil, boom
	}

	selection := model.Selection{
		Agents:     []model.AgentID{model.AgentOpenCode},
		Components: []model.ComponentID{model.ComponentCortex},
	}
	result, err := installWithDependencies(t.TempDir(), newTestRegistry(), selection, "test-v1", false, deps)
	if !errors.Is(err, boom) || !strings.Contains(err.Error(), "capture install journal") {
		t.Fatalf("injected Install() error = %v, want begin-journal failure", err)
	}
	if result.BackupID == "" {
		t.Fatal("injected post-backup failure lost backup evidence")
	}

	if _, err := Install(t.TempDir(), newTestRegistry(), selection, "test-v1", false); err != nil {
		t.Fatalf("public Install() inherited per-call hook: %v", err)
	}
}

func TestInstallDependencies_PreparedWriterUsesScopedRecordHook(t *testing.T) {
	homeDir := t.TempDir()
	target := ManagedTarget{Path: "target.txt", Kind: TargetFile}
	if err := os.WriteFile(filepath.Join(homeDir, target.Path), []byte("after"), 0o600); err != nil {
		t.Fatal(err)
	}
	journal := &InstallJournal{TargetRoot: homeDir, Targets: []ManagedTarget{target}}
	called := false
	writer := newPreparedWriter([]ManagedTarget{target})
	writer.bindJournal(journal)
	writer.recordJournalOutcome = func(got *InstallJournal, outcome MutationOutcome) error {
		called = got == journal && outcome.Path == target.Path
		return nil
	}

	if err := writer.run(func() error { return nil }); err != nil {
		t.Fatalf("prepared writer error = %v", err)
	}
	if !called {
		t.Fatal("prepared writer did not use its scoped journal record hook")
	}
}
