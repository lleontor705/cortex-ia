package modelmgr

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func cleanTemplate() ([]byte, error) {
	return []byte("{\n  \"$schema\": \"https://opencode.ai/config.json\",\n  \"default_agent\": \"orchestrator\"\n}\n"), nil
}

func findFinding(report DoctorReport, check CheckID) (Finding, bool) {
	for _, finding := range report.Findings {
		if finding.Check == check {
			return finding, true
		}
	}
	return Finding{}, false
}

func assertFinding(t *testing.T, report DoctorReport, check CheckID, severity Severity) {
	t.Helper()
	finding, ok := findFinding(report, check)
	if !ok {
		t.Fatalf("check %s emitted no finding: %+v", check, report.Findings)
	}
	if finding.Severity != severity {
		t.Fatalf("check %s severity = %s, want %s (%s)", check, finding.Severity, severity, finding.Message)
	}
}

func TestDoctorHealthyHome(t *testing.T) {
	home := t.TempDir()
	path := configPath(home)
	managed, _ := ParseDesired("plan", highRef, "")
	digest, _ := managed.Digest()
	writeFile(t, path, `{"default_agent": "plan", "agents": {"plan": {"model": "anthropic/claude-sonnet-4-5#high"}}}`)
	before := readFile(t, path)

	report := New(home).Doctor(DoctorOptions{
		Evidence: []OwnershipRecord{{Agent: "plan", Digest: digest, ConfigPath: path}},
		Template: cleanTemplate,
		RunCommand: func(string, ...string) ([]byte, error) {
			return []byte("anthropic/claude-sonnet-4-5\nopenai/gpt-5.2\n"), nil
		},
	})
	if report.HasErrors() {
		t.Fatalf("healthy home reported errors: %+v", report.Findings)
	}
	if verdict := report.Verdict(); verdict == SeverityError || verdict == SeverityWarning {
		t.Fatalf("verdict = %s: %+v", verdict, report.Findings)
	}
	if got := readFile(t, path); got != before {
		t.Fatalf("doctor mutated the config: %q", got)
	}
	assertFinding(t, report, CheckConfigDecode, SeverityOK)
	assertFinding(t, report, CheckModelShape, SeverityOK)
	assertFinding(t, report, CheckManagedDrift, SeverityOK)
	assertFinding(t, report, CheckDefaultAgent, SeverityOK)
	assertFinding(t, report, CheckTemplateGuard, SeverityOK)
	assertFinding(t, report, CheckOpencode2Models, SeverityInfo)
}

func TestDoctorMalformedConfigKeepsOtherChecks(t *testing.T) {
	home := t.TempDir()
	writeFile(t, configPath(home),
		`{"agents": {"plan": {"model": "a/b"}}, "agents": {"build": {"model": "c/d"}}}`)
	writeFile(t, filepath.Join(home, ".config", "opencode", "agents", "reviewer.md"),
		"---\nmodel: openai/gpt-5.2\n---\nbody\n")

	report := New(home).Doctor(DoctorOptions{Template: cleanTemplate})
	decode, ok := findFinding(report, CheckConfigDecode)
	if !ok || decode.Severity != SeverityError || !strings.Contains(decode.Message, "agents") {
		t.Fatalf("duplicate member not named: %+v", report.Findings)
	}
	assertFinding(t, report, CheckModelShape, SeveritySkipped)
	assertFinding(t, report, CheckManagedDrift, SeveritySkipped)
	assertFinding(t, report, CheckMarkdownPin, SeverityWarning)
	assertFinding(t, report, CheckTemplateGuard, SeverityOK)
	if report.Verdict() != SeverityError {
		t.Fatalf("verdict = %s, want error", report.Verdict())
	}
}

func TestDoctorDriftNamesExpectedAndObserved(t *testing.T) {
	home := t.TempDir()
	path := configPath(home)
	recorded, _ := ParseDesired("plan", "openai/gpt-5.2", "")
	recordedDigest, _ := recorded.Digest()
	observed, _ := ParseDesired("plan", highRef, "")
	observedDigest, _ := observed.Digest()
	writeFile(t, path, `{"agents": {"plan": {"model": "anthropic/claude-sonnet-4-5#high"}}}`)

	report := New(home).Doctor(DoctorOptions{
		Evidence: []OwnershipRecord{{Agent: "plan", Digest: recordedDigest, ConfigPath: path}},
		Template: cleanTemplate,
	})
	drift, ok := findFinding(report, CheckManagedDrift)
	if !ok || drift.Severity != SeverityWarning {
		t.Fatalf("drift not reported: %+v", report.Findings)
	}
	if drift.Agent != "plan" || drift.Observed == "" ||
		drift.Expected != recordedDigest || drift.Observed != observedDigest {
		t.Fatalf("drift finding lacks named digests: %+v", drift)
	}
}

func TestDoctorShapeProblemIsAnError(t *testing.T) {
	home := t.TempDir()
	writeFile(t, configPath(home), `{"agents": {"plan": {"model": 42}, "build": {"model": "openai/gpt-5.2"}}}`)

	report := New(home).Doctor(DoctorOptions{Template: cleanTemplate})
	shape, ok := findFinding(report, CheckModelShape)
	if !ok || shape.Severity != SeverityError || shape.Agent != "plan" {
		t.Fatalf("malformed value not flagged per agent: %+v", report.Findings)
	}
	assertFinding(t, report, CheckDefaultAgent, SeverityOK)
	assertFinding(t, report, CheckTemplateGuard, SeverityOK)
}

func TestDoctorOpencode2UnavailableIsNonFatal(t *testing.T) {
	home := t.TempDir()
	writeFile(t, configPath(home), `{"default_agent": "build"}`)

	report := New(home).Doctor(DoctorOptions{
		Template:   cleanTemplate,
		RunCommand: func(string, ...string) ([]byte, error) { return nil, errors.New("executable file not found in PATH") },
	})
	assertFinding(t, report, CheckOpencode2Models, SeveritySkipped)
	if report.HasErrors() {
		t.Fatalf("a skipped cross-check must not fail the report: %+v", report.Findings)
	}

	report = New(home).Doctor(DoctorOptions{
		Template:   cleanTemplate,
		RunCommand: func(string, ...string) ([]byte, error) { return []byte("no model information available\n"), nil },
	})
	assertFinding(t, report, CheckOpencode2Models, SeveritySkipped)
	if report.HasErrors() {
		t.Fatalf("unparseable output must not fail the report: %+v", report.Findings)
	}
}

func TestDoctorUninstalledHomeIsHonest(t *testing.T) {
	report := New(t.TempDir()).Doctor(DoctorOptions{Template: cleanTemplate})
	if report.HasErrors() {
		t.Fatalf("uninstalled home reported errors: %+v", report.Findings)
	}
	if report.ConfigPath == "" || report.HomeDir == "" {
		t.Fatalf("report must disclose its paths: %+v", report)
	}
	assertFinding(t, report, CheckConfigDecode, SeverityOK)
	assertFinding(t, report, CheckManagedDrift, SeverityOK)
	assertFinding(t, report, CheckMarkdownPin, SeverityOK)
	assertFinding(t, report, CheckDefaultAgent, SeverityOK)
	assertFinding(t, report, CheckOpencode2Models, SeveritySkipped)
}

func TestDoctorTemplateGuardRejectsPinnedKeys(t *testing.T) {
	for name, fixture := range map[string]string{
		"agents": `{"agents": {"plan": {"model": "anthropic/claude-sonnet-4-5"}}}`,
		"model":  `{"model": "anthropic/claude-sonnet-4-5"}`,
	} {
		template := func() ([]byte, error) { return []byte(fixture), nil }
		report := New(t.TempDir()).Doctor(DoctorOptions{Template: template})
		guard, ok := findFinding(report, CheckTemplateGuard)
		if !ok || guard.Severity != SeverityError || !strings.Contains(guard.Message, name) {
			t.Fatalf("%s key not caught by the template guard: %+v", name, report.Findings)
		}
	}
}

func TestDoctorInvalidRecordedDigestIsCorrupt(t *testing.T) {
	home := t.TempDir()
	writeFile(t, configPath(home), `{"agents": {"plan": {"model": "openai/gpt-5.2"}}}`)
	report := New(home).Doctor(DoctorOptions{
		Evidence: []OwnershipRecord{{Agent: "plan", Digest: "deadbeef"}},
		Template: cleanTemplate,
	})
	drift, ok := findFinding(report, CheckManagedDrift)
	if !ok || drift.Severity != SeverityError || drift.Expected != "deadbeef" {
		t.Fatalf("corrupt recorded digest not reported: %+v", report.Findings)
	}
}

// TestDoctorOpencode2CountBeyondEchoCap pins byte-identity at the echo cap and
// the deliberate count fix past it: the finding reports every parsed reference
// while the echoed list stays bounded.
func TestDoctorOpencode2CountBeyondEchoCap(t *testing.T) {
	home := t.TempDir()
	writeFile(t, configPath(home), `{"default_agent": "build"}`)
	outputFor := func(count int) CommandRunner {
		return func(string, ...string) ([]byte, error) {
			var builder strings.Builder
			for i := 0; i < count; i++ {
				fmt.Fprintf(&builder, "vendor/model-%02d\n", i)
			}
			return []byte(builder.String()), nil
		}
	}
	echo := func(count int) string {
		refs := make([]string, 0, count)
		for i := 0; i < count; i++ {
			refs = append(refs, fmt.Sprintf("vendor/model-%02d", i))
		}
		return strings.Join(refs, ", ")
	}
	for _, tc := range []struct {
		name   string
		count  int
		echoed int
	}{
		{"at-cap", opencode2ModelsRefLimit, opencode2ModelsRefLimit},
		{"beyond-cap", opencode2ModelsRefLimit + 4, opencode2ModelsRefLimit},
	} {
		t.Run(tc.name, func(t *testing.T) {
			report := New(home).Doctor(DoctorOptions{Template: cleanTemplate, RunCommand: outputFor(tc.count)})
			finding, ok := findFinding(report, CheckOpencode2Models)
			if !ok || finding.Severity != SeverityInfo {
				t.Fatalf("finding = %+v", finding)
			}
			want := fmt.Sprintf("opencode2 reported %d model reference(s): %s", tc.count, echo(tc.echoed))
			if finding.Message != want {
				t.Fatalf("message = %q, want %q", finding.Message, want)
			}
		})
	}
}
