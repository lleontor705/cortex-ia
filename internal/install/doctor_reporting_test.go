package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/telemetry"
)

// assessReportingProbe runs the reporting assessment on a fresh report.
func assessReportingProbe(t *testing.T, home string) *DoctorReport {
	t.Helper()
	service, err := New(home)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	report := &DoctorReport{Verdict: DoctorHealthy}
	service.assessReporting(report)
	return report
}

func TestAssessReportingFlagsMissingSecretWithoutChangingVerdict(t *testing.T) {
	t.Setenv("CORTEX_REPORT_SECRET", "")
	home := t.TempDir()

	report := assessReportingProbe(t, home)
	if report.Verdict != DoctorHealthy {
		t.Fatalf("missing secret must stay suggest-only, got %q", report.Verdict)
	}
	if len(report.Findings) != 1 {
		t.Fatalf("expected exactly one finding, got %v", report.Findings)
	}
	if !strings.Contains(report.Findings[0], "signing secret") {
		t.Errorf("finding must name the missing secret, got %q", report.Findings[0])
	}
	if !strings.Contains(report.Findings[0], "report config --secret") {
		t.Errorf("finding must carry the remediation command, got %q", report.Findings[0])
	}
}

func TestAssessReportingQuietWhenSecretResolvesFromEnvironment(t *testing.T) {
	t.Setenv("CORTEX_REPORT_SECRET", "secreto-de-prueba")
	home := t.TempDir()

	report := assessReportingProbe(t, home)
	if len(report.Findings) != 0 {
		t.Fatalf("expected no findings with an environment secret, got %v", report.Findings)
	}
	if report.Verdict != DoctorHealthy {
		t.Fatalf("unexpected verdict %q", report.Verdict)
	}
}

func TestAssessReportingQuietWhenConfiguredSecretResolves(t *testing.T) {
	t.Setenv("CORTEX_REPORT_SECRET", "")
	home := t.TempDir()
	if err := telemetry.SaveConfig(home, telemetry.Config{
		Endpoint: telemetry.CanonicalDefaultEndpoint,
		Secret:   "secreto-configurado",
		Enabled:  true,
	}); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	report := assessReportingProbe(t, home)
	if len(report.Findings) != 0 {
		t.Fatalf("expected no findings with a configured secret, got %v", report.Findings)
	}
}

func TestAssessReportingQuietWhenReportingDisabled(t *testing.T) {
	t.Setenv("CORTEX_REPORT_SECRET", "")
	home := t.TempDir()
	cfgPath := telemetry.ConfigPath(home)
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(cfgPath, []byte(`{"endpoint":"https://example.invalid/api/v1/reports","enabled":false}`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	report := assessReportingProbe(t, home)
	if len(report.Findings) != 0 {
		t.Fatalf("disabled reporting must not warn, got %v", report.Findings)
	}
}

func TestReportingWarningsBootstrapFromEnvironment(t *testing.T) {
	t.Setenv("CORTEX_REPORT_SECRET", "secreto-de-prueba")
	home := t.TempDir()
	service, err := New(home)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	warnings := service.reportingWarnings()
	if len(warnings) != 1 || !strings.Contains(warnings[0], "persisted from CORTEX_REPORT_SECRET") {
		t.Fatalf("expected the bootstrap notice, got %v", warnings)
	}
	if _, ok := telemetry.SecretOrigin(home); !ok {
		t.Fatal("bootstrap did not leave a resolvable signing secret")
	}
	// A second run must be idempotent: the config already exists.
	if again := service.reportingWarnings(); len(again) != 0 {
		t.Fatalf("expected no further warnings, got %v", again)
	}
}

func TestReportingWarningsActionableWithoutAnySecret(t *testing.T) {
	t.Setenv("CORTEX_REPORT_SECRET", "")
	home := t.TempDir()
	service, err := New(home)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	warnings := service.reportingWarnings()
	if len(warnings) != 1 || !strings.Contains(warnings[0], "report config --secret") {
		t.Fatalf("expected the remediation warning, got %v", warnings)
	}
}
