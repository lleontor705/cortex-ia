package conformance

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

type goldenIndexEntry struct {
	Target  string `json:"target"`
	Fixture string `json:"fixture"`
	Golden  string `json:"golden"`
}

func TestEvaluateReleaseAcceptsCompleteRepositoryEvidence(t *testing.T) {
	goldens := loadSupportedGoldens(t)
	checks := make([]Check, 0, len(RequiredDomains()))
	for _, domain := range RequiredDomains() {
		checks = append(checks, Check{Domain: domain, Outcome: OutcomePassed, Evidence: []string{"go-test:" + string(domain)}})
	}

	report := Evaluate(ReleaseEvidence{
		Checks: checks,
		Metrics: Metrics{
			SupportedGoldens:           len(goldens),
			PassedGoldens:              len(goldens),
			DeterminismComparisons:     len(goldens) * 2,
			EqualDeterminismResults:    len(goldens) * 2,
			RequiredBindings:           3,
			ResolvedOrBlockedBindings:  3,
			Degradations:               4,
			MachineVisibleDegradations: 4,
			HumanVisibleDegradations:   4,
			PreinstallDegradations:     4,
		},
	})

	if !report.Passed {
		t.Fatalf("Evaluate() Passed = false, findings = %#v", report.Findings)
	}
	if len(report.Findings) != 0 {
		t.Fatalf("Evaluate() findings = %#v, want none", report.Findings)
	}
	if !slices.IsSortedFunc(report.Checks, func(left, right Check) int {
		return strings.Compare(string(left.Domain), string(right.Domain))
	}) {
		t.Fatalf("Evaluate() checks are not deterministically ordered: %#v", report.Checks)
	}
}

func TestEvaluateReleaseFailsClosed(t *testing.T) {
	baseline := completeEvidence()
	tests := []struct {
		name   string
		mutate func(*ReleaseEvidence)
		code   FindingCode
	}{
		{name: "missing domain", mutate: func(e *ReleaseEvidence) { e.Checks = e.Checks[1:] }, code: FindingMissingDomain},
		{name: "duplicate domain", mutate: func(e *ReleaseEvidence) { e.Checks = append(e.Checks, e.Checks[0]) }, code: FindingDuplicateDomain},
		{name: "failed check", mutate: func(e *ReleaseEvidence) { e.Checks[0].Outcome = OutcomeFailed }, code: FindingDomainFailed},
		{name: "inconclusive is not pass", mutate: func(e *ReleaseEvidence) { e.Checks[0].Outcome = OutcomeInconclusive }, code: FindingDomainInconclusive},
		{name: "missing evidence", mutate: func(e *ReleaseEvidence) { e.Checks[0].Evidence = nil }, code: FindingMissingEvidence},
		{name: "golden failure", mutate: func(e *ReleaseEvidence) { e.Metrics.PassedGoldens-- }, code: FindingGoldenCoverage},
		{name: "determinism mismatch", mutate: func(e *ReleaseEvidence) { e.Metrics.EqualDeterminismResults-- }, code: FindingDeterminism},
		{name: "binding unresolved", mutate: func(e *ReleaseEvidence) { e.Metrics.ResolvedOrBlockedBindings-- }, code: FindingBindingCoverage},
		{name: "machine degradation hidden", mutate: func(e *ReleaseEvidence) { e.Metrics.MachineVisibleDegradations-- }, code: FindingHiddenDegradation},
		{name: "human degradation hidden", mutate: func(e *ReleaseEvidence) { e.Metrics.HumanVisibleDegradations-- }, code: FindingHiddenDegradation},
		{name: "preinstall degradation hidden", mutate: func(e *ReleaseEvidence) { e.Metrics.PreinstallDegradations-- }, code: FindingHiddenDegradation},
		{name: "secret rendered", mutate: func(e *ReleaseEvidence) { e.Metrics.RenderedSecrets = 1 }, code: FindingSecretExposure},
		{name: "permission widened", mutate: func(e *ReleaseEvidence) { e.Metrics.PermissionWidenings = 1 }, code: FindingPermissionWidening},
		{name: "required value unresolved", mutate: func(e *ReleaseEvidence) { e.Metrics.UnresolvedRequiredValues = 1 }, code: FindingUnresolvedValue},
		{name: "false enforcement", mutate: func(e *ReleaseEvidence) { e.Metrics.FalseEnforcementClaims = 1 }, code: FindingFalseEnforcement},
		{name: "approval bypass", mutate: func(e *ReleaseEvidence) { e.Metrics.CriticalApprovalBypasses = 1 }, code: FindingApprovalBypass},
		{name: "false pass", mutate: func(e *ReleaseEvidence) { e.Metrics.FalsePasses = 1 }, code: FindingFalsePass},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evidence := baseline
			evidence.Checks = slices.Clone(baseline.Checks)
			tt.mutate(&evidence)
			report := Evaluate(evidence)
			if report.Passed {
				t.Fatal("Evaluate() Passed = true, want fail-closed result")
			}
			if !hasFinding(report.Findings, tt.code) {
				t.Fatalf("Evaluate() findings = %#v, want code %q", report.Findings, tt.code)
			}
		})
	}
}

func TestRequiredDomainsCoverReleaseConformance(t *testing.T) {
	want := []Domain{
		DomainSchemas, DomainAdapterProfiles, DomainDeterminism, DomainEquivalence,
		DomainPrompts, DomainBindings, DomainPermissions, DomainSecrets,
		DomainDoctor, DomainInstall, DomainRollback, DomainSourceInventory,
	}
	got := RequiredDomains()
	slices.Sort(want)
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Fatalf("RequiredDomains() = %v, want %v", got, want)
	}
}

func TestEvaluateRepositoryReleaseCannotPassCallerDeclaredCleanCounts(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "release/generated.txt", "route via team-lead\n")
	baseline := completeEvidence()
	checks := slices.DeleteFunc(slices.Clone(baseline.Checks), func(check Check) bool { return check.Domain == DomainSourceInventory })
	report, collected, err := EvaluateRepositoryRelease(RepositoryReleaseRequest{
		Collector: CollectorRequest{Root: root, Now: time.Now().UTC(), Budget: 10},
		Checks:    checks, Metrics: baseline.Metrics,
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.Passed || collected.Status != EvidenceFailed || !hasFinding(report.Findings, FindingDomainFailed) {
		t.Fatalf("caller-declared clean evidence bypassed source collector: report=%+v collected=%+v", report, collected)
	}
}

func completeEvidence() ReleaseEvidence {
	checks := make([]Check, 0, len(RequiredDomains()))
	for _, domain := range RequiredDomains() {
		checks = append(checks, Check{Domain: domain, Outcome: OutcomePassed, Evidence: []string{"test:" + string(domain)}})
	}
	return ReleaseEvidence{Checks: checks, Metrics: Metrics{
		SupportedGoldens: 2, PassedGoldens: 2,
		DeterminismComparisons: 4, EqualDeterminismResults: 4,
		RequiredBindings: 2, ResolvedOrBlockedBindings: 2,
		Degradations: 1, MachineVisibleDegradations: 1, HumanVisibleDegradations: 1, PreinstallDegradations: 1,
	}}
}

func hasFinding(findings []Finding, code FindingCode) bool {
	return slices.ContainsFunc(findings, func(finding Finding) bool { return finding.Code == code })
}

func loadSupportedGoldens(t *testing.T) []goldenIndexEntry {
	t.Helper()
	root := filepath.Join("..", "renderers")
	data, err := os.ReadFile(filepath.Join(root, "testdata", "conformance", "index.golden.json"))
	if err != nil {
		t.Fatalf("read shared conformance index: %v", err)
	}
	var entries []goldenIndexEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		t.Fatalf("decode shared conformance index: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("shared conformance index contains no supported adapter/profile goldens")
	}
	seen := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		key := entry.Target + "/" + entry.Fixture
		if entry.Target == "" || entry.Fixture == "" || entry.Golden == "" {
			t.Fatalf("incomplete shared conformance entry: %#v", entry)
		}
		if _, duplicate := seen[key]; duplicate {
			t.Fatalf("duplicate supported adapter/profile golden %q", key)
		}
		seen[key] = struct{}{}
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(entry.Golden))); err != nil {
			t.Fatalf("supported golden %q is unavailable: %v", key, err)
		}
	}
	return entries
}

// currentSurfaceRule is an explicit per-file real-source guard rule.
//
// Per ADR-18 the guard reads the ACTUAL repository files responsible for the
// C1/C2/H1/C3 blockers and applies explicit forbidden/required rules. It is an
// incremental guard, not a claim of complete repository conformance, and it
// uses no broad/directory-wide allowlist. Forbidden tokens are assembled from
// fragments at the call site so this test file does not itself carry literal
// forbidden current-surface vocabulary.
type currentSurfaceRule struct {
	Path      string
	Forbidden []string
	Required  []string
}

func TestReleaseRealSourceGuardEnforcesLocalizedMailboxAndTeamLeadAbsence(t *testing.T) {
	root := repositoryRoot(t)

	// Assemble forbidden tokens from fragments so the test source stays clean.
	mailboxID := strings.Join([]string{"agent-", "mailbox"}, "")
	mailboxWord := strings.Join([]string{"Mail", "box"}, "")
	msgSend := strings.Join([]string{"msg_", "send"}, "")
	a2aSubmit := strings.Join([]string{"a2a_", "submit_task"}, "")
	teamLeadPolicy := strings.Join([]string{"Team", "LeadPolicy"}, "")
	resolveTeamLead := strings.Join([]string{"resolve", "TeamLead"}, "")
	mbAutoPull := strings.Join([]string{"mail", "box + conventions auto-pulled"}, "")

	rules := []currentSurfaceRule{
		// C1: CLI help must advertise seven current components and no live Mailbox.
		{
			Path:      "internal/app/app.go",
			Forbidden: []string{mailboxID, "All 8 components"},
			Required:  []string{"All 7 components"},
		},
		// C2: Agent Builder must not advertise Mailbox or messaging/A2A tools.
		{
			Path:      "internal/agentbuilder/prompt.go",
			Forbidden: []string{mailboxWord, msgSend, a2aSubmit},
		},
		// H1: role asset generation must not expose team-lead configurability.
		{
			Path:      "internal/assets/roles/generate.go",
			Forbidden: []string{teamLeadPolicy, resolveTeamLead},
		},
		// C3 stale reference (REMOVE-classified): generated config example.
		{
			Path:      "internal/config/config.go",
			Forbidden: []string{mailboxID},
		},
		// C3 stale reference (REMOVE-classified): minimal-preset auto-pull claim.
		// The governed historical compatibility row is retained (required markers).
		{
			Path:      "docs/codebase/mental-model.md",
			Forbidden: []string{mbAutoPull},
			Required:  []string{"retired identifier only", "Legacy decode, exact migration, rollback"},
		},
	}

	for _, rule := range rules {
		name := strings.ReplaceAll(rule.Path, "/", "_")
		t.Run(name, func(t *testing.T) {
			assertCurrentSurface(t, root, rule)
		})
	}
}

// repositoryRoot walks upward from the test working directory until it finds
// go.mod, returning the repository root. This proves the guard reads real
// repository sources rather than temporary fixtures.
func repositoryRoot(t *testing.T) string {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	dir := cwd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not locate repository root (go.mod) walking up from %s", cwd)
		}
		dir = parent
	}
}

func assertCurrentSurface(t *testing.T, root string, rule currentSurfaceRule) {
	t.Helper()
	target := filepath.Join(root, filepath.FromSlash(rule.Path))
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("real-source guard could not read %s: %v", rule.Path, err)
	}
	content := string(data)
	for _, forbidden := range rule.Forbidden {
		if strings.Contains(content, forbidden) {
			t.Errorf("real-source guard: %s must not contain forbidden token %q (current-surface regression)",
				rule.Path, forbidden)
		}
	}
	for _, required := range rule.Required {
		if !strings.Contains(content, required) {
			t.Errorf("real-source guard: %s must retain required marker %q", rule.Path, required)
		}
	}
}
