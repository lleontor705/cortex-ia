package kiro

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
	"time"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/capability"
	"github.com/lleontor705/cortex-ia/internal/model"
	"github.com/lleontor705/cortex-ia/internal/system"
)

func TestAgentIdentity(t *testing.T) {
	a := NewAdapter()
	if a.Agent() != model.AgentKiroIDE {
		t.Errorf("Agent() = %v, want %v", a.Agent(), model.AgentKiroIDE)
	}
}

func TestPaths(t *testing.T) {
	a := NewAdapter()
	home := "/home/user"

	if got := a.SystemPromptFile(home); got != filepath.Join(home, ".kiro", "steering", "cortex-ia.md") {
		t.Errorf("SystemPromptFile = %q", got)
	}
	if got := a.SkillsDir(home); got != filepath.Join(home, ".kiro", "skills") {
		t.Errorf("SkillsDir = %q", got)
	}
	if got := a.MCPConfigPath(home, "cortex"); got != filepath.Join(home, ".kiro", "settings", "mcp.json") {
		t.Errorf("MCPConfigPath = %q", got)
	}
	if got := a.SubAgentsDir(home); got != filepath.Join(home, ".kiro", "agents") {
		t.Errorf("SubAgentsDir = %q", got)
	}
}

func TestInstallCommandsReturnsNil(t *testing.T) {
	a := NewAdapter()
	if cmds := a.InstallCommands(system.PlatformProfile{}); cmds != nil {
		t.Errorf("InstallCommands = %v, want nil (kiro is not auto-installable)", cmds)
	}
	if a.SupportsAutoInstall() {
		t.Error("SupportsAutoInstall = true, want false")
	}
}

func TestKiroConfigDir_Platform(t *testing.T) {
	a := NewAdapter()
	got := a.GlobalConfigDir("/home/user")

	switch runtime.GOOS {
	case "darwin":
		if got != "/home/user/Library/Application Support/Kiro/User" {
			t.Errorf("darwin GlobalConfigDir = %q", got)
		}
	case "windows":
		// Windows result depends on APPDATA; just ensure non-empty.
		if got == "" {
			t.Error("windows GlobalConfigDir is empty")
		}
	default:
		// Linux respects XDG_CONFIG_HOME; default ~/.config/kiro/user.
		// Override for this assertion to keep it deterministic.
		_ = os.Unsetenv("XDG_CONFIG_HOME")
		got = a.GlobalConfigDir("/home/user")
		if got != "/home/user/.config/kiro/user" {
			t.Errorf("linux GlobalConfigDir = %q", got)
		}
	}
}

func TestDetect_BinaryNotFound(t *testing.T) {
	a := &Adapter{
		lookPath: func(string) (string, error) { return "", exec.ErrNotFound },
		statPath: os.Stat,
	}
	installed, _, _, _, err := a.Detect("/home/test")
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if installed {
		t.Error("expected installed=false when binary not found")
	}
}

func TestAgentNotInstallableError(t *testing.T) {
	err := AgentNotInstallableError{Agent: model.AgentKiroIDE}
	if err.Error() == "" {
		t.Error("expected non-empty error message")
	}
}

func TestCapabilityFactsValidateConservatively(t *testing.T) {
	now := time.Date(2026, time.July, 26, 12, 0, 0, 0, time.UTC)
	facts := NewAdapter().CapabilityFacts()
	if len(facts) == 0 {
		t.Fatal("CapabilityFacts() returned no facts")
	}

	catalog := capability.Catalog{
		SchemaVersion: capability.CatalogSchema.Current,
		Version:       capability.CatalogSchema.Current,
		Facts:         facts,
	}
	if err := catalog.Validate(now); err != nil {
		t.Fatalf("Kiro capability catalog: %v", err)
	}

	directChild := capabilityFact(t, facts, "delegation/direct-child")
	if !directChild.Experimental {
		t.Fatal("Kiro direct-child delegation must remain experimental until an executable probe qualifies it")
	}
	if directChild.EvidenceClass != capability.EvidenceInstalledSchema || directChild.Enforcement != capability.EnforcementPrompt {
		t.Fatalf("unqualified Kiro delegation must remain advisory installed-schema evidence: %+v", directChild)
	}
}

func TestCapabilityProbeQualifiesExperimentalDelegationOnlyAfterOptIn(t *testing.T) {
	now := time.Date(2026, time.July, 26, 12, 0, 0, 0, time.UTC)
	a := NewAdapter()
	a.lookPath = func(name string) (string, error) {
		if name != "kiro" {
			t.Fatalf("lookPath(%q), want kiro", name)
		}
		return "/usr/local/bin/kiro", nil
	}
	a.runCommand = func(_ context.Context, binary string, args ...string) ([]byte, error) {
		if binary != "/usr/local/bin/kiro" || !reflect.DeepEqual(args, []string{"--help"}) {
			t.Fatalf("probe command = %q %v", binary, args)
		}
		return []byte("Tools: read write subagent\n"), nil
	}
	a.now = func() time.Time { return now }

	base := capabilityFact(t, a.CapabilityFacts(), "delegation/direct-child")
	request := capability.ProbeRequest{
		Base: base,
		Authority: capability.ProbeAuthority{
			CapabilityID:      base.ID,
			RuntimeVersions:   base.RuntimeVersions,
			Modes:             []capability.CapabilityValue{capability.CapabilityAvailable},
			Cardinalities:     []capability.Cardinality{capability.CardinalityMany},
			Enforcement:       []capability.EnforcementClass{capability.EnforcementPrompt, capability.EnforcementRuntime},
			ExperimentalOptIn: true,
		},
	}
	result, err := a.CapabilityProber().Probe(context.Background(), request)
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	qualified, err := capability.ApplyProbeResult(request, result)
	if err != nil {
		t.Fatalf("ApplyProbeResult: %v", err)
	}
	if qualified.EvidenceClass != capability.EvidenceExecutableProbe || qualified.Probe == nil {
		t.Fatalf("qualified fact lacks executable probe evidence: %+v", qualified)
	}
	if err := (capability.Catalog{
		SchemaVersion: capability.CatalogSchema.Current,
		Version:       capability.CatalogSchema.Current,
		Facts:         []capability.CapabilityFact{qualified},
	}).Validate(now); err != nil {
		t.Fatalf("qualified Kiro catalog: %v", err)
	}

	if !qualified.Experimental || qualified.Enforcement != capability.EnforcementRuntime {
		t.Fatalf("probe must preserve experimental opt-in while qualifying runtime enforcement: %+v", qualified)
	}
}

func TestCapabilitySurfaceHasNoRuntimeExecutionAPI(t *testing.T) {
	typeOfAdapter := reflect.TypeOf(NewAdapter())
	for _, forbidden := range []string{"Run", "Resume", "Schedule", "LaunchWorker", "LaunchAgent"} {
		if _, found := typeOfAdapter.MethodByName(forbidden); found {
			t.Errorf("adapter exposes forbidden runtime execution method %s", forbidden)
		}
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
