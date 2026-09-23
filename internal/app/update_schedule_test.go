package app

import (
	"crypto/ed25519"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/updater"
)

func requireContains(t *testing.T, out string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

// countingTransport serves a canned response in-process and counts requests, so
// check surfaces never reach the network and tests can prove nothing downloaded.
type countingTransport struct {
	calls int
	body  string
}

func (t *countingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.calls++
	return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Header: make(http.Header), Body: io.NopCloser(strings.NewReader(t.body)), Request: req}, nil
}

func stubUpdateCheck(t *testing.T, tag string) *countingTransport {
	t.Helper()
	transport := &countingTransport{body: fmt.Sprintf(`{"tag_name":%q,"published_at":"2026-01-01T00:00:00Z","assets":[]}`, tag)}
	previous := updateClientFactory
	updateClientFactory = func() *updater.Client {
		client := updater.New("test/repo")
		client.HTTPClient = &http.Client{Transport: transport}
		return client
	}
	t.Cleanup(func() { updateClientFactory = previous })
	return transport
}

type recordingSchedulerRunner struct {
	output string
	calls  []string
}

func (r *recordingSchedulerRunner) Run(name string, args ...string) ([]byte, error) {
	r.calls = append(r.calls, strings.Join(append([]string{name}, args...), " "))
	return []byte(r.output), nil
}

func stubSchedulerRunner(t *testing.T, output string) *recordingSchedulerRunner {
	t.Helper()
	runner := &recordingSchedulerRunner{output: output}
	previous := updateSchedulerRunner
	updateSchedulerRunner = func() updater.CommandRunner { return runner }
	t.Cleanup(func() { updateSchedulerRunner = previous })
	return runner
}

func injectUpdateTrust(t *testing.T) {
	t.Helper()
	t.Cleanup(updater.SetTrustedKeysForTesting([]updater.TrustedKey{
		{ID: "app-test-key", PublicKey: make([]byte, ed25519.PublicKeySize)},
	}))
}

// prepareScheduledHome isolates the state root from the developer's real home.
func prepareScheduledHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("CORTEX_IA_HOME", home)
	return home
}

func withVersion(t *testing.T, version string) {
	t.Helper()
	previous := Version
	Version = version
	t.Cleanup(func() { Version = previous })
}

func testBinaryName() string {
	if runtime.GOOS == "windows" {
		return "cortex-ia.exe"
	}
	return "cortex-ia"
}

func TestUpdateScheduleReceipts(t *testing.T) {
	prepareScheduledHome(t)
	execPath := filepath.Join(t.TempDir(), testBinaryName())
	runner := stubSchedulerRunner(t, `"`+execPath+`" `+updater.ScheduledCheckArgs+"\n")

	statusOut, err := captureStdout(func() error { return runUpdate([]string{"schedule", "status"}) })
	if err != nil {
		t.Fatalf("schedule status failed: %v", err)
	}
	requireContains(t, statusOut, "enabled", updater.ScheduledTaskName, updater.ScheduledCheckFrequency, execPath)

	enableOut, err := captureStdout(func() error { return runUpdate([]string{"schedule", "enable"}) })
	if err != nil {
		t.Fatalf("schedule enable failed: %v", err)
	}
	requireContains(t, enableOut, "enabled", updater.ScheduledTaskName, updater.ScheduledCheckArgs)

	disableOut, err := captureStdout(func() error { return runUpdate([]string{"schedule", "disable"}) })
	if err != nil {
		t.Fatalf("schedule disable failed: %v", err)
	}
	requireContains(t, disableOut, "disabled")

	if len(runner.calls) < 3 {
		t.Fatalf("expected status/enable/disable to reach the injected runner, got %d calls", len(runner.calls))
	}
}

func TestUpdateScheduleScheduledCheckState(t *testing.T) {
	cases := []struct{ name, tag, wantAvailable, wantOutput string }{
		{"no update keeps available empty", "v0.4.9", "", "No update available"},
		{"newer release is cached without a download", "v0.5.0", "v0.5.0", "Update available: v0.5.0"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			home := prepareScheduledHome(t)
			injectUpdateTrust(t)
			withVersion(t, "v0.4.9")
			transport := stubUpdateCheck(t, tc.tag)

			out, err := captureStdout(func() error { return runUpdate([]string{"--check", "--scheduled"}) })
			if err != nil {
				t.Fatalf("scheduled check failed: %v", err)
			}
			requireContains(t, out, tc.wantOutput)

			state, err := updater.LoadUpdateState(home)
			if err != nil {
				t.Fatalf("load state: %v", err)
			}
			if state.LastCheckedAt.IsZero() {
				t.Error("last_checked_at was not persisted")
			}
			if state.Available != tc.wantAvailable {
				t.Errorf("available = %q, want %q", state.Available, tc.wantAvailable)
			}
			if state.AppliedFloor != "" {
				t.Errorf("check-only mode must never move the floor, got %q", state.AppliedFloor)
			}
			if transport.calls != 1 {
				t.Errorf("check-only mode must issue exactly one request, got %d", transport.calls)
			}
		})
	}
}

func TestUpdateScheduleScheduledCheckFailsClosedWithoutTrust(t *testing.T) {
	home := prepareScheduledHome(t)
	withVersion(t, "v0.4.9")
	t.Cleanup(updater.SetTrustedKeysForTesting(nil))

	if err := updater.SaveUpdateStateAtomic(home, updater.UpdateState{
		SchemaVersion: updater.UpdateStateSchemaVersion,
		AppliedFloor:  "v0.4.5",
		Available:     "v0.4.6",
	}); err != nil {
		t.Fatalf("seed state: %v", err)
	}
	before, err := os.ReadFile(updater.StatePath(home))
	if err != nil {
		t.Fatalf("read seeded state: %v", err)
	}

	if err := runUpdate([]string{"--check", "--scheduled"}); !errors.Is(err, updater.ErrNoTrustedKey) {
		t.Fatalf("expected the fail-closed trust error, got %v", err)
	}
	after, err := os.ReadFile(updater.StatePath(home))
	if err != nil {
		t.Fatalf("read state after failure: %v", err)
	}
	if string(before) != string(after) {
		t.Errorf("a failed check must not touch prior state:\nbefore: %s\nafter:  %s", before, after)
	}
}

func TestUpdateScheduleDualInstallWarning(t *testing.T) {
	home := prepareScheduledHome(t)
	injectUpdateTrust(t)
	withVersion(t, "v0.4.9")

	localRoot := t.TempDir()
	goPath := t.TempDir()
	t.Setenv("LOCALAPPDATA", localRoot)
	t.Setenv("GOPATH", goPath)
	programPath := filepath.Join(localRoot, "Programs", "cortex-ia", "bin", testBinaryName())
	gopathPath := filepath.Join(goPath, "bin", testBinaryName())
	for _, path := range []string{programPath, gopathPath} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("create fixture dir: %v", err)
		}
		if err := os.WriteFile(path, []byte("stub"), 0o644); err != nil {
			t.Fatalf("create fixture binary: %v", err)
		}
	}
	stubUpdateCheck(t, "v0.5.0")

	out, err := captureStdout(func() error { return runUpdate([]string{"--check", "--scheduled"}) })
	if err != nil {
		t.Fatalf("scheduled check failed: %v", err)
	}
	requireContains(t, out, "multiple cortex-ia installations", programPath, gopathPath)

	state, err := updater.LoadUpdateState(home)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	requireContains(t, strings.Join(state.InstallCandidates, "\n"), programPath, gopathPath)
}

func TestUpdateScheduleHelpAndFlagGuards(t *testing.T) {
	helpOut, err := captureStdout(func() error { return runUpdate([]string{"--help"}) })
	if err != nil {
		t.Fatalf("update --help failed: %v", err)
	}
	requireContains(t, helpOut, "--check", "--scheduled", "schedule")

	scheduleHelp, err := captureStdout(func() error { return runUpdate([]string{"schedule", "--help"}) })
	if err != nil {
		t.Fatalf("update schedule --help failed: %v", err)
	}
	requireContains(t, scheduleHelp, "enable", "disable", "status")

	if err := runUpdate([]string{"--scheduled"}); err == nil || !strings.Contains(err.Error(), "only valid with --check") {
		t.Errorf("--scheduled without --check must be rejected, got %v", err)
	}
	if err := runUpdate([]string{"schedule"}); err == nil || !strings.Contains(err.Error(), "requires a subcommand") {
		t.Errorf("bare schedule must require a subcommand, got %v", err)
	}
}
