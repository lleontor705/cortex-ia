package install

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The install test binary must never probe the host PATH or reach the network:
// existing Install/Doctor tests would otherwise depend on the local toolchain.
// Individual tests re-substitute the seams they exercise.
func TestMain(m *testing.M) {
	restoreLocator := SetCortexBinaryLocatorForTesting(func() (string, bool) { return "/test/bin/cortex", true })
	restoreRunner := SetCortexInstallRunnerForTesting(func(context.Context) error { return nil })
	restoreSink := SetCortexNoticeSinkForTesting(io.Discard)
	code := m.Run()
	restoreSink()
	restoreRunner()
	restoreLocator()
	os.Exit(code)
}

func TestCortexInstallArgvIsShellFree(t *testing.T) {
	argv := cortexInstallArgv()
	want := []string{"go", "install", CortexModulePath}
	if len(argv) != len(want) {
		t.Fatalf("argv length = %d, want %d (%v)", len(argv), len(want), argv)
	}
	for i := range want {
		if argv[i] != want[i] {
			t.Fatalf("argv[%d] = %q, want %q", i, argv[i], want[i])
		}
	}
	for _, arg := range argv {
		if strings.ContainsAny(arg, "&|;<>()$`\"'\n") {
			t.Fatalf("argv element %q carries shell metacharacters", arg)
		}
	}
}

func TestCortexBinCandidatesHonorGoEnvironment(t *testing.T) {
	gobin := t.TempDir()
	gopathA := t.TempDir()
	gopathB := t.TempDir()
	t.Setenv("GOBIN", gobin)
	t.Setenv("GOPATH", gopathA+string(filepath.ListSeparator)+gopathB)

	name := cortexExecutableName()
	candidates := cortexBinCandidates()
	for _, want := range []string{
		filepath.Join(gobin, name),
		filepath.Join(gopathA, "bin", name),
		filepath.Join(gopathB, "bin", name),
	} {
		found := false
		for _, candidate := range candidates {
			if candidate == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("candidates %v missing %q", candidates, want)
		}
	}
}

func TestEnsureCortexBinaryPresentSkipsInstall(t *testing.T) {
	defer SetCortexBinaryLocatorForTesting(func() (string, bool) { return "/test/bin/cortex", true })()
	installed := false
	defer SetCortexInstallRunnerForTesting(func(context.Context) error { installed = true; return nil })()

	result := EnsureCortexBinary(context.Background())
	if result.Outcome != CortexPresent || result.Path != "/test/bin/cortex" {
		t.Fatalf("unexpected result %+v", result)
	}
	if installed {
		t.Fatal("an already resolvable binary must not trigger go install")
	}
}

func TestEnsureCortexBinaryInstallsUnderBoundedContext(t *testing.T) {
	resolved := false
	defer SetCortexBinaryLocatorForTesting(func() (string, bool) {
		if resolved {
			return "/test/bin/cortex", true
		}
		return "", false
	})()
	bounded := false
	defer SetCortexInstallRunnerForTesting(func(ctx context.Context) error {
		_, bounded = ctx.Deadline()
		resolved = true
		return nil
	})()

	result := EnsureCortexBinary(context.Background())
	if result.Outcome != CortexInstalled || result.Path != "/test/bin/cortex" {
		t.Fatalf("unexpected result %+v", result)
	}
	if !bounded {
		t.Fatal("go install must run under a deadline-bounded context")
	}
}

func TestEnsureCortexBinaryDegradesToManualCommand(t *testing.T) {
	defer SetCortexBinaryLocatorForTesting(func() (string, bool) { return "", false })()
	defer SetCortexInstallRunnerForTesting(func(context.Context) error { return errors.New("no network") })()

	result := EnsureCortexBinary(context.Background())
	if result.Outcome != CortexFailed {
		t.Fatalf("outcome = %q, want failed", result.Outcome)
	}
	if result.Manual != CortexManualCommand || !strings.Contains(result.Detail, "no network") {
		t.Fatalf("unexpected failure detail %+v", result)
	}
}

func TestEnsureCortexBinaryReportsUnrefreshedPATH(t *testing.T) {
	defer SetCortexBinaryLocatorForTesting(func() (string, bool) { return "", false })()
	defer SetCortexInstallRunnerForTesting(func(context.Context) error { return nil })()

	result := EnsureCortexBinary(context.Background())
	if result.Outcome != CortexFailed || !strings.Contains(result.Detail, "PATH") {
		t.Fatalf("unexpected result %+v", result)
	}
	if result.Manual != CortexManualCommand {
		t.Fatalf("manual command = %q", result.Manual)
	}
}

func TestPreflightCortexBinaryScopeAndWarnings(t *testing.T) {
	svc, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer SetCortexBinaryLocatorForTesting(func() (string, bool) { return "", false })()
	calls := 0
	runnerErr := error(nil)
	defer SetCortexInstallRunnerForTesting(func(context.Context) error { calls++; return runnerErr })()

	if warnings := svc.preflightCortexBinary(Options{Cortex: true, DryRun: true}); len(warnings) != 0 {
		t.Fatalf("dry run must not preflight: %v", warnings)
	}
	if warnings := svc.preflightCortexBinary(Options{Cortex: false}); len(warnings) != 0 {
		t.Fatalf("cortex-free selection must not preflight: %v", warnings)
	}
	if calls != 0 {
		t.Fatalf("preflight ran %d time(s) outside scope", calls)
	}

	runnerErr = errors.New("toolchain unavailable")
	var sink bytes.Buffer
	defer SetCortexNoticeSinkForTesting(&sink)()
	warnings := svc.preflightCortexBinary(Options{Cortex: true})
	if calls != 1 || len(warnings) != 1 {
		t.Fatalf("calls = %d, warnings = %v", calls, warnings)
	}
	if !strings.Contains(warnings[0], CortexManualCommand) || !strings.Contains(sink.String(), CortexManualCommand) {
		t.Fatalf("manual command must be reported: warning=%v sink=%q", warnings, sink.String())
	}
}

func TestAssessCortexBinaryIsSuggestOnly(t *testing.T) {
	svc, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer SetCortexBinaryLocatorForTesting(func() (string, bool) { return "", false })()

	report := &DoctorReport{Verdict: DoctorHealthy}
	svc.assessCortexBinary(report)
	if report.Verdict != DoctorHealthy {
		t.Fatalf("missing binary must not change the verdict, got %q", report.Verdict)
	}
	if len(report.Findings) != 1 || !strings.Contains(report.Findings[0], CortexManualCommand) {
		t.Fatalf("expected one suggest-only finding, got %v", report.Findings)
	}

	defer SetCortexBinaryLocatorForTesting(func() (string, bool) { return "/test/bin/cortex", true })()
	present := &DoctorReport{Verdict: DoctorHealthy}
	svc.assessCortexBinary(present)
	if len(present.Findings) != 0 {
		t.Fatalf("resolvable binary must produce no finding, got %v", present.Findings)
	}
}
