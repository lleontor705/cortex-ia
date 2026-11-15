package install

import (
	"path/filepath"
	"strings"
	"testing"
)

func findingContains(findings []string, fragment string) bool {
	for _, finding := range findings {
		if strings.Contains(finding, fragment) {
			return true
		}
	}
	return false
}

func assessWiring(t *testing.T, home string) *DoctorReport {
	t.Helper()
	service, err := New(home)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	report := &DoctorReport{Verdict: DoctorHealthy}
	service.assessTUIWiring(report)
	return report
}

func TestAssessTUIWiringV2(t *testing.T) {
	restore := SetOpenCodeVersionDetectorForTesting(func(string) (int, bool) { return opencodeMajorV2, true })
	t.Cleanup(restore)

	t.Run("Wired", func(t *testing.T) {
		home := t.TempDir()
		writeConfig(t, filepath.Join(testConfigDir(t, home), cliConfigName),
			`{"$schema":"https://opencode.ai/v2/cli.json","plugins":["./tui-plugins/cortex-ia"],"theme":{"name":"cortex","mode":"dark"}}`)
		report := assessWiring(t, home)
		if report.Verdict != DoctorHealthy {
			t.Errorf("expected healthy wired v2, got %s", report.Verdict)
		}
		if !findingContains(report.Findings, "OpenCode v2") {
			t.Errorf("expected the finding to label the detected version, got %v", report.Findings)
		}
	})

	t.Run("MissingPlugin", func(t *testing.T) {
		home := t.TempDir()
		writeConfig(t, filepath.Join(testConfigDir(t, home), cliConfigName),
			`{"plugins":["other.js"],"theme":{"name":"cortex","mode":"dark"}}`)
		report := assessWiring(t, home)
		if report.Verdict != DoctorDegraded {
			t.Errorf("expected degraded, got %s", report.Verdict)
		}
		if !findingContains(report.Findings, "missing plugin entry") {
			t.Errorf("expected a plugin finding, got %v", report.Findings)
		}
	})

	t.Run("MissingTheme", func(t *testing.T) {
		home := t.TempDir()
		writeConfig(t, filepath.Join(testConfigDir(t, home), cliConfigName),
			`{"plugins":["./tui-plugins/cortex-ia"]}`)
		report := assessWiring(t, home)
		if report.Verdict != DoctorDegraded {
			t.Errorf("expected degraded, got %s", report.Verdict)
		}
		if !findingContains(report.Findings, "missing cortex theme") {
			t.Errorf("expected a theme finding, got %v", report.Findings)
		}
	})

	t.Run("MissingFile", func(t *testing.T) {
		home := t.TempDir()
		_ = testConfigDir(t, home)
		report := assessWiring(t, home)
		if report.Verdict != DoctorDegraded {
			t.Errorf("expected degraded on missing cli.json, got %s", report.Verdict)
		}
		if !findingContains(report.Findings, "OpenCode v2") || !findingContains(report.Findings, "missing") {
			t.Errorf("expected a version-labeled missing finding, got %v", report.Findings)
		}
	})

	t.Run("MalformedFile", func(t *testing.T) {
		home := t.TempDir()
		writeConfig(t, filepath.Join(testConfigDir(t, home), cliConfigName), "{malformed")
		report := assessWiring(t, home)
		if report.Verdict != DoctorDegraded {
			t.Errorf("expected degraded on malformed cli.json, got %s", report.Verdict)
		}
	})
}

func TestAssessTUIWiringV1(t *testing.T) {
	restore := SetOpenCodeVersionDetectorForTesting(func(string) (int, bool) { return opencodeMajorV1, true })
	t.Cleanup(restore)

	t.Run("Wired", func(t *testing.T) {
		home := t.TempDir()
		writeConfig(t, filepath.Join(testConfigDir(t, home), tuiConfigName),
			`{"$schema":"https://opencode.ai/tui.json","plugin":["./tui-plugins/cortex-ia-tui.js"],"theme":{"name":"cortex","mode":"light"}}`)
		report := assessWiring(t, home)
		if report.Verdict != DoctorHealthy {
			t.Errorf("expected healthy wired v1, got %s (findings %v)", report.Verdict, report.Findings)
		}
		if !findingContains(report.Findings, "OpenCode v1") {
			t.Errorf("expected the finding to label v1, got %v", report.Findings)
		}
	})

	t.Run("MissingFile", func(t *testing.T) {
		home := t.TempDir()
		_ = testConfigDir(t, home)
		report := assessWiring(t, home)
		if report.Verdict != DoctorDegraded {
			t.Errorf("expected degraded on missing tui.jsonc, got %s", report.Verdict)
		}
		if !findingContains(report.Findings, "OpenCode v1") {
			t.Errorf("expected a version-labeled finding, got %v", report.Findings)
		}
	})
}
