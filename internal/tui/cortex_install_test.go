package tui

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/install"
	"github.com/lleontor705/cortex-ia/internal/pipeline"
)

// The TUI test binary never probes the host toolchain nor runs a real install.
// Individual tests substitute the seams they exercise.
func TestMain(m *testing.M) {
	cortexBinaryMissing = func() bool { return false }
	ensureCortexBinary = func(context.Context) install.CortexInstallResult {
		return install.CortexInstallResult{Outcome: install.CortexPresent, Path: "/test/bin/cortex"}
	}
	os.Exit(m.Run())
}

// cortexSeams substitutes both preflight seams for the duration of one test.
func cortexSeams(t *testing.T, missing func() bool, runner func(context.Context) install.CortexInstallResult) {
	t.Helper()
	previousMissing, previousRunner := cortexBinaryMissing, ensureCortexBinary
	cortexBinaryMissing, ensureCortexBinary = missing, runner
	t.Cleanup(func() { cortexBinaryMissing, ensureCortexBinary = previousMissing, previousRunner })
}

func TestCortexConsentOfferedAtPlanTime(t *testing.T) {
	cortexSeams(t, func() bool { return true }, func(context.Context) install.CortexInstallResult {
		return install.CortexInstallResult{}
	})

	m := sized(newModel(&fakeService{}, t.TempDir(), "v0"))
	m = pressDrive(t, m, "1")

	if m.screen != screenReview {
		t.Fatalf("screen = %v, want review", m.screen)
	}
	if m.confirm.kind != confirmCortexInstall {
		t.Fatalf("confirm kind = %v, want confirmCortexInstall", m.confirm.kind)
	}
	if view := m.View(); !strings.Contains(view, "Confirm cortex install") {
		t.Fatalf("consent overlay must render:\n%s", view)
	}
}

func TestCortexConsentDeclineIsNonFatalAndNotRepeated(t *testing.T) {
	cortexSeams(t, func() bool { return true }, func(context.Context) install.CortexInstallResult {
		t.Fatal("declined consent must never run the installer")
		return install.CortexInstallResult{}
	})

	fake := &fakeService{}
	m := sized(newModel(fake, t.TempDir(), "v0"))
	m = pressDrive(t, m, "1")
	m = press(m, "n")

	if m.confirm.kind != confirmNone {
		t.Fatalf("decline must close the overlay, got %v", m.confirm.kind)
	}
	if !strings.Contains(m.cortexStatus, install.CortexManualCommand) {
		t.Fatalf("decline must keep the manual command, got %q", m.cortexStatus)
	}

	m = pressDrive(t, m, "down")  // select context7
	m = pressDrive(t, m, "space") // toggle -> replan
	if m.confirm.kind != confirmNone {
		t.Fatalf("replan must not repeat the declined consent, got %v", m.confirm.kind)
	}
}

func TestCortexConsentInstallReplansAndRebindsDigest(t *testing.T) {
	installing := false
	cortexSeams(t, func() bool { return !installing }, func(context.Context) install.CortexInstallResult {
		installing = true
		return install.CortexInstallResult{Outcome: install.CortexInstalled, Path: "/test/bin/cortex"}
	})

	fake := &fakeService{
		planFn: func(install.Options) (*pipeline.Plan, error) {
			return &pipeline.Plan{Digest: "digest-after-replan"}, nil
		},
	}
	m := sized(newModel(fake, t.TempDir(), "v0"))
	m = pressDrive(t, m, "1")
	plannedBefore := len(fake.planCalls)

	updated, cmd := m.Update(key("y"))
	m = updated.(model)
	if m.confirm.kind != confirmNone || !m.cortexInstalling {
		t.Fatalf("consent must start the install: confirm=%v installing=%v", m.confirm.kind, m.cortexInstalling)
	}
	if !strings.Contains(m.cortexStatus, "installing") {
		t.Fatalf("install status must be visible, got %q", m.cortexStatus)
	}

	for _, msg := range collect(t, cmd) {
		updated, next := m.Update(msg)
		m = updated.(model)
		// onCortexInstall dispatches the replan command; execute it so the
		// Review screen is settled when the assertions run.
		m = drive(t, m, next)
	}
	if m.cortexInstalling || m.replanning {
		t.Fatalf("install must settle: installing=%v replanning=%v", m.cortexInstalling, m.replanning)
	}
	if !strings.Contains(m.cortexStatus, "/test/bin/cortex") {
		t.Fatalf("install result must be reported, got %q", m.cortexStatus)
	}
	if len(fake.planCalls) != plannedBefore+1 {
		t.Fatalf("install must trigger a fresh plan: %d -> %d", plannedBefore, len(fake.planCalls))
	}
	if m.plan == nil || m.plan.Digest != "digest-after-replan" {
		t.Fatalf("replanned digest missing: %+v", m.plan)
	}

	m = pressDrive(t, m, "enter")
	if len(fake.installCalls) != 1 {
		t.Fatalf("expected one install call, got %d", len(fake.installCalls))
	}
	if got := fake.installCalls[0].ExpectedPlanDigest; got != "digest-after-replan" {
		t.Fatalf("ExpectedPlanDigest = %q, want the freshly replanned digest", got)
	}
}
