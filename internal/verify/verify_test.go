package verify

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/model"
	"github.com/lleontor705/cortex-ia/internal/state"
)

func TestRun_AllPass(t *testing.T) {
	checks := []Check{
		{ID: "a", Name: "Check A", Severity: SeverityError, Fn: func(*Context) error { return nil }},
		{ID: "b", Name: "Check B", Severity: SeverityWarning, Fn: func(*Context) error { return nil }},
	}
	report := Run(&Context{}, checks)
	if report.Passed != 2 || report.Failed != 0 || report.Warned != 0 {
		t.Errorf("expected 2 passed, got passed=%d failed=%d warned=%d", report.Passed, report.Failed, report.Warned)
	}
	if report.HasErrors() {
		t.Error("expected no errors")
	}
}

func TestRun_ErrorAndWarning(t *testing.T) {
	checks := []Check{
		{ID: "pass", Name: "OK", Severity: SeverityError, Fn: func(*Context) error { return nil }},
		{ID: "fail", Name: "Fail", Severity: SeverityError, Fn: func(*Context) error { return fmt.Errorf("broken") }},
		{ID: "warn", Name: "Warn", Severity: SeverityWarning, Fn: func(*Context) error { return fmt.Errorf("degraded") }},
	}
	report := Run(&Context{}, checks)
	if report.Passed != 1 {
		t.Errorf("expected 1 passed, got %d", report.Passed)
	}
	if report.Failed != 1 {
		t.Errorf("expected 1 failed, got %d", report.Failed)
	}
	if report.Warned != 1 {
		t.Errorf("expected 1 warned, got %d", report.Warned)
	}
	if !report.HasErrors() {
		t.Error("expected HasErrors=true")
	}
}

func TestRun_Empty(t *testing.T) {
	report := Run(&Context{}, nil)
	if report.Passed != 0 || report.Failed != 0 {
		t.Error("expected empty report")
	}
}

func TestCheckFilesExist_AllPresent(t *testing.T) {
	tmpDir := t.TempDir()
	f := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(f, []byte("ok"), 0o644)

	ctx := &Context{Lock: state.Lockfile{Files: []string{f}}}
	if err := checkFilesExist(ctx); err != nil {
		t.Errorf("expected pass, got: %v", err)
	}
}

func TestCheckFilesExist_Missing(t *testing.T) {
	ctx := &Context{Lock: state.Lockfile{Files: []string{"/nonexistent/file"}}}
	if err := checkFilesExist(ctx); err == nil {
		t.Error("expected error for missing file")
	}
}

func TestCheckSkillsPresent(t *testing.T) {
	tmpDir := t.TempDir()
	skillsDir := filepath.Join(tmpDir, ".cortex-ia", "skills")
	for _, id := range []string{"bootstrap", "implement", "validate", "architect", "investigate"} {
		os.MkdirAll(filepath.Join(skillsDir, id), 0o755)
		os.WriteFile(filepath.Join(skillsDir, id, "SKILL.md"), []byte("skill"), 0o644)
	}

	ctx := &Context{HomeDir: tmpDir}
	if err := checkSkillsPresent(ctx); err != nil {
		t.Errorf("expected pass, got: %v", err)
	}
}

func TestCheckSkillsPresent_Missing(t *testing.T) {
	ctx := &Context{HomeDir: t.TempDir()}
	if err := checkSkillsPresent(ctx); err == nil {
		t.Error("expected error for missing skills")
	}
}

func TestCheckConventionPresent(t *testing.T) {
	tmpDir := t.TempDir()
	convDir := filepath.Join(tmpDir, ".cortex-ia", "skills", "_shared")
	os.MkdirAll(convDir, 0o755)
	os.WriteFile(filepath.Join(convDir, "cortex-convention.md"), []byte("conv"), 0o644)

	ctx := &Context{HomeDir: tmpDir}
	if err := checkConventionPresent(ctx); err != nil {
		t.Errorf("expected pass, got: %v", err)
	}
}

func TestCheckConventionPresent_Missing(t *testing.T) {
	ctx := &Context{HomeDir: t.TempDir()}
	if err := checkConventionPresent(ctx); err == nil {
		t.Error("expected error for missing convention")
	}
}

func TestCheckStateLockConsistent_Match(t *testing.T) {
	ctx := &Context{
		State: state.State{InstalledAgents: []model.AgentID{"codex"}},
		Lock:  state.Lockfile{InstalledAgents: []model.AgentID{"codex"}},
	}
	if err := checkStateLockConsistent(ctx); err != nil {
		t.Errorf("expected pass, got: %v", err)
	}
}

func TestCheckStateLockConsistent_Mismatch(t *testing.T) {
	ctx := &Context{
		State: state.State{InstalledAgents: []model.AgentID{"codex"}},
		Lock:  state.Lockfile{InstalledAgents: []model.AgentID{"claude-code"}},
	}
	if err := checkStateLockConsistent(ctx); err == nil {
		t.Error("expected error for mismatch")
	}
}

func TestCheckStateLockConsistent_BothEmpty(t *testing.T) {
	ctx := &Context{}
	if err := checkStateLockConsistent(ctx); err != nil {
		t.Errorf("expected pass for both empty, got: %v", err)
	}
}

func TestCheckInstallStatus_NoFile(t *testing.T) {
	ctx := &Context{HomeDir: t.TempDir()}
	if err := checkInstallStatus(ctx); err != nil {
		t.Errorf("expected pass when no install status file, got: %v", err)
	}
}

func TestCheckInstallStatus_InProgress(t *testing.T) {
	tmpDir := t.TempDir()
	status := state.InstallStatus{
		Status:    "in-progress",
		StartedAt: "2026-04-10T12:00:00Z",
		BackupID:  "20260410-120000",
	}
	if err := state.SaveInstallStatus(tmpDir, status); err != nil {
		t.Fatalf("SaveInstallStatus() error: %v", err)
	}

	ctx := &Context{HomeDir: tmpDir}
	err := checkInstallStatus(ctx)
	if err == nil {
		t.Fatal("expected error for in-progress install status")
	}
	msg := err.Error()
	if !strings.Contains(msg, "did not complete cleanly") {
		t.Errorf("error message %q should mention incomplete install", msg)
	}
	if !strings.Contains(msg, "20260410-120000") {
		t.Errorf("error message %q should mention backup ID", msg)
	}
}

func TestCheckInstallStatus_Complete(t *testing.T) {
	tmpDir := t.TempDir()
	status := state.InstallStatus{
		Status:    "complete",
		StartedAt: "2026-04-10T12:00:00Z",
	}
	if err := state.SaveInstallStatus(tmpDir, status); err != nil {
		t.Fatalf("SaveInstallStatus() error: %v", err)
	}

	ctx := &Context{HomeDir: tmpDir}
	if err := checkInstallStatus(ctx); err != nil {
		t.Errorf("expected pass for complete status, got: %v", err)
	}
}

func TestDoctorFindingsAreStableAndActionable(t *testing.T) {
	tests := []struct {
		name        string
		observation Observation
		code        FindingCode
		severity    Severity
		blocking    bool
	}{
		{name: "runtime beyond tested", observation: Observation{Kind: CheckRuntimeVersion, State: StateBeyondTested, Target: "opencode", Path: "runtime.version", Observed: "2.0.0", Expected: "<= 1.9.0", Evidence: "probe runtime-version: sha256:01", Remediation: "upgrade the adapter or run a qualifying probe"}, code: FindingRuntimeVersion, severity: SeverityWarning, blocking: true},
		{name: "catalog evidence stale", observation: Observation{Kind: CheckEvidenceFreshness, State: StateStale, Target: "opencode", Path: "catalog.delegate", Observed: "fresh-until 2026-07-01", Expected: "fresh at 2026-07-26", Evidence: "catalog digest sha256:02", Remediation: "refresh the capability probe"}, code: FindingEvidenceStale, severity: SeverityWarning, blocking: true},
		{name: "schema incompatible", observation: Observation{Kind: CheckSchemaInterval, State: StateMismatch, Target: "bundle", Path: "manifest.schema_version", Observed: "2.0.0", Expected: ">=1.0.0 <2.0.0", Evidence: "manifest digest sha256:03", Remediation: "regenerate with a supported schema"}, code: FindingSchemaInterval, severity: SeverityError, blocking: true},
		{name: "hash drift", observation: Observation{Kind: CheckAssetHash, State: StateCorrupt, Target: "claude", Path: ".claude/agents/implement.md", Observed: "sha256:04", Expected: "sha256:05", Evidence: "install-state asset implement", Remediation: "restore from backup or reinstall"}, code: FindingAssetHash, severity: SeverityError, blocking: true},
		{name: "ownership unknown", observation: Observation{Kind: CheckOwnership, State: StateUnknown, Target: "codex", Path: "AGENTS.md", Observed: "no marker", Expected: "owner cortex-ia", Evidence: "ownership sidecar absent", Remediation: "review dry-run and explicitly approve takeover"}, code: FindingOwnership, severity: SeverityError, blocking: true},
		{name: "permission widened", observation: Observation{Kind: CheckPermissions, State: StateMismatch, Target: "unsupported-target", Path: "permissions.network", Observed: "*", Expected: "api.example.test", Evidence: "security manifest", Remediation: "regenerate without permission widening"}, code: FindingPermissions, severity: SeverityError, blocking: true},
		{name: "secret rendered", observation: Observation{Kind: CheckSecrets, State: StatePresent, Target: "unsupported-target", Path: "mcp.env.API_KEY", Observed: "literal secret", Expected: "opaque secret reference", Evidence: "redacted digest sha256:06", Remediation: "remove the value, rotate it, and regenerate"}, code: FindingSecret, severity: SeverityError, blocking: true},
		{name: "service unsupported", observation: Observation{Kind: CheckServiceVersion, State: StateUnsupported, Target: "forgespec", Path: "services.forgespec.version", Observed: "0.8.0", Expected: ">=1.0.0 <2.0.0", Evidence: "service compatibility manifest", Remediation: "upgrade ForgeSpec"}, code: FindingServiceVersion, severity: SeverityError, blocking: true},
		{name: "binding unresolved", observation: Observation{Kind: CheckBinding, State: StateUnresolved, Target: "mailbox", Path: "bindings.message.send", Observed: "unresolved", Expected: "provider binding", Evidence: "semantic binding manifest", Remediation: "configure a compatible provider"}, code: FindingBinding, severity: SeverityError, blocking: true},
		{name: "manifest inconsistent", observation: Observation{Kind: CheckManifest, State: StateMismatch, Target: "bundle", Path: "semantic_digest", Observed: "sha256:07", Expected: "sha256:08", Evidence: "machine and human manifests", Remediation: "regenerate the bundle"}, code: FindingManifest, severity: SeverityError, blocking: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report := Diagnose("native-advanced", []Observation{tt.observation})
			if report.Qualified {
				t.Fatal("unhealthy installation must not be qualified")
			}
			if len(report.Findings) != 1 {
				t.Fatalf("expected one finding, got %d", len(report.Findings))
			}
			finding := report.Findings[0]
			if finding.Code != tt.code || finding.Severity != tt.severity || finding.Blocking != tt.blocking {
				t.Fatalf("unexpected finding policy: %+v", finding)
			}
			if finding.Target == "" || finding.Path == "" || finding.Observed == "" || finding.Expected == "" || finding.Evidence == "" || finding.Remediation == "" {
				t.Fatalf("finding is not actionable: %+v", finding)
			}
		})
	}
}

func TestDoctorHealthyInstallationHasZeroBlockers(t *testing.T) {
	observations := make([]Observation, 0, len(AllDoctorCheckKinds()))
	for _, kind := range AllDoctorCheckKinds() {
		observations = append(observations, Observation{
			Kind: kind, State: StateHealthy, Target: "opencode", Path: string(kind),
			Observed: "verified", Expected: "verified", Evidence: "fixture evidence", Remediation: "none",
		})
	}

	report := Diagnose("portable-sequential", observations)
	if !report.Qualified {
		t.Fatalf("healthy installation should be qualified: %+v", report.Findings)
	}
	if report.Profile != "portable-sequential" || len(report.Findings) != 0 || report.Blockers() != 0 {
		t.Fatalf("expected selected profile and zero findings/blockers: %+v", report)
	}
}

func TestDoctorStaleAndBeyondTestedNeverQualify(t *testing.T) {
	for _, tt := range []struct {
		name  string
		kind  CheckKind
		state CheckState
	}{
		{name: "stale evidence", kind: CheckEvidenceFreshness, state: StateStale},
		{name: "runtime beyond tested", kind: CheckRuntimeVersion, state: StateBeyondTested},
	} {
		t.Run(tt.name, func(t *testing.T) {
			report := Diagnose("native-advanced", []Observation{{
				Kind: tt.kind, State: tt.state, Target: "runtime", Path: "qualification",
				Observed: string(tt.state), Expected: "qualified", Evidence: "fixture", Remediation: "run qualification",
			}})
			if report.Qualified || report.Blockers() == 0 {
				t.Fatalf("%s must block qualification: %+v", tt.state, report)
			}
		})
	}
}
