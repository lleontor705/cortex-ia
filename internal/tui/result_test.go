package tui

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/install"
	"github.com/lleontor705/cortex-ia/internal/pipeline"
)

func driveFromHomeToReview(t *testing.T, m model) model {
	m = press(m, "enter")         // Home -> Wizard Step 1
	m = press(m, "enter")         // Step 1 -> Step 2
	m = pressDrive(t, m, "enter") // Step 2 (cursor=1: normal) -> Review (plan)
	if m.screen == screenWizardRoles {
		m = pressDrive(t, m, "enter")
	}
	if m.screen != screenReview {
		t.Fatalf("expected review screen, got %v", m.screen)
	}
	return m
}

// installToResult drives Home → Wizard → Review → Running → Result for a clean
// install whose selected MCPs all carried valid qualification evidence.
func installToResult(t *testing.T, fake *fakeService) model {
	t.Helper()
	fake.planFn = func(install.Options) (*pipeline.Plan, error) {
		return &pipeline.Plan{Digest: "digest0001"}, nil
	}
	fake.installFn = func(install.Options) (*install.InstallReceipt, error) {
		return &install.InstallReceipt{
			PlanDigest:     "digest0001",
			Configured:     []string{"cortex", "forgespec"},
			Qualified:      []string{"cortex", "forgespec"},
			Changed:        []string{"create .config/opencode/AGENTS.md", "mcp-add cortex"},
			BackupID:       "bkp-20260817-1",
			BackupVerified: true,
		}, nil
	}
	m := sized(newModel(fake, "/home/test", "vtest"))
	m = driveFromHomeToReview(t, m)
	m = pressDrive(t, m, "enter") // Review → Running → Result
	if m.screen != screenResult {
		t.Fatalf("expected result screen, got %v", m.screen)
	}
	return m
}

// TestResultShowsPassReceipt proves the PASS verdict, changed count, backup
// ID, and rollback command are all visible after a successful install.
func TestResultShowsPassReceipt(t *testing.T) {
	m := installToResult(t, &fakeService{})
	if !m.result.pass {
		t.Fatal("expected PASS result")
	}
	view := m.View()
	for _, want := range []string{
		"PASS",
		"Changed: 2",
		"bkp-20260817-1",
		"rollback",
		"enter/m home",
		"q quit",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("result view missing %q:\n%s", want, view)
		}
	}
}

// TestResultShowsFailReceipt proves a service error lands as FAIL with the
// error visible and no rollback offered.
func TestResultShowsFailReceipt(t *testing.T) {
	fake := &fakeService{}
	fake.planFn = func(install.Options) (*pipeline.Plan, error) {
		return &pipeline.Plan{Digest: "digest0001"}, nil
	}
	fake.installFn = func(install.Options) (*install.InstallReceipt, error) {
		return nil, errors.New("apply failed; restoration verified")
	}
	m := sized(newModel(fake, "/home/test", "vtest"))
	m = driveFromHomeToReview(t, m)
	m = pressDrive(t, m, "enter")

	if m.result.pass || m.result.canRollback {
		t.Fatalf("failed run must be FAIL without rollback, got pass=%v canRollback=%v", m.result.pass, m.result.canRollback)
	}
	view := m.View()
	if !strings.Contains(view, "FAIL") || !strings.Contains(view, "apply failed") {
		t.Fatalf("FAIL verdict/error not visible:\n%s", view)
	}

	// 'r' stays inert without a rollback reference.
	m = press(m, "r")
	if m.confirm.kind != confirmNone || m.screen != screenResult {
		t.Fatal("'r' must be inert when no rollback is offerable")
	}
}

// TestResultInstallUnqualifiedMCPIsVisibleFail proves an install that
// configured MCPs without valid qualification evidence is FAIL even when
// the service returned no error and assets changed: the missing names, the
// reason, and the remedy stay visible and rollback stays gated off.
func TestResultInstallUnqualifiedMCPIsVisibleFail(t *testing.T) {
	fake := &fakeService{}
	fake.planFn = func(install.Options) (*pipeline.Plan, error) {
		return &pipeline.Plan{Digest: "digest0001"}, nil
	}
	fake.installFn = func(install.Options) (*install.InstallReceipt, error) {
		return &install.InstallReceipt{
			PlanDigest:     "digest0001",
			Configured:     []string{"cortex", "forgespec"},
			Qualified:      []string{"cortex"},
			Changed:        []string{"create .config/opencode/AGENTS.md", "mcp-add forgespec"},
			BackupID:       "bkp-20260817-2",
			BackupVerified: true,
			Warnings:       []string{`MCP "forgespec" configured without valid qualification evidence`},
		}, nil
	}
	m := sized(newModel(fake, "/home/test", "vtest"))
	m = driveFromHomeToReview(t, m)
	m = pressDrive(t, m, "enter")

	if m.screen != screenResult {
		t.Fatalf("expected result screen, got %v", m.screen)
	}
	if m.result.pass {
		t.Fatal("configured-only MCP must not PASS")
	}
	if m.result.canRollback {
		t.Fatal("rollback must stay gated off on a FAIL verdict")
	}
	view := m.View()
	for _, want := range []string{
		"FAIL",
		"forgespec",
		"qualification evidence",
		"remedy:",
		"Changed: 2",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("result view missing %q:\n%s", want, view)
		}
	}
}

// TestResultSyncUnqualifiedMCPIsVisibleFail proves the sync mode shares the
// receipt-derived verdict: a converged home whose selected MCPs lack
// qualification evidence renders FAIL, not a nil-error PASS.
func TestResultSyncUnqualifiedMCPIsVisibleFail(t *testing.T) {
	fake := &fakeService{}
	fake.syncFn = func(install.Options) (*install.InstallReceipt, error) {
		return &install.InstallReceipt{
			PlanDigest: "digest0002",
			Converged:  true,
			Configured: []string{"cortex", "forgespec"},
			Qualified:  nil,
		}, nil
	}
	m := sized(newModel(fake, "/home/test", "vtest"))
	m.screen = screenRunning
	m.running = runningState{title: "Sync", phases: installPhases}
	m = drive(t, m, installRunCmd(m.svc, "sync", install.DefaultOptions()))

	if m.screen != screenResult {
		t.Fatalf("expected result screen, got %v", m.screen)
	}
	if m.result.pass {
		t.Fatal("sync with unqualified selected MCPs must be FAIL")
	}
	view := m.View()
	if !strings.Contains(view, "FAIL") || !strings.Contains(view, "cortex, forgespec") {
		t.Fatalf("sync FAIL verdict or unqualified names not visible:\n%s", view)
	}
}

// TestResultInstallEmptySelectionPassesVacuously proves an install with no
// MCPs selected has nothing to qualify and still PASSes on service success.
func TestResultInstallEmptySelectionPassesVacuously(t *testing.T) {
	fake := &fakeService{}
	fake.planFn = func(install.Options) (*pipeline.Plan, error) {
		return &pipeline.Plan{Digest: "digest0001"}, nil
	}
	fake.installFn = func(install.Options) (*install.InstallReceipt, error) {
		return &install.InstallReceipt{PlanDigest: "digest0001", Changed: []string{"create .config/opencode/AGENTS.md"}}, nil
	}
	m := sized(newModel(fake, "/home/test", "vtest"))
	m = driveFromHomeToReview(t, m)
	m = pressDrive(t, m, "enter")

	if !m.result.pass {
		t.Fatal("install without selected MCPs must PASS vacuously")
	}
}

// TestResultRollbackGatedByConfirmation proves rollback from the Result
// screen requires the explicit confirmation and then calls the service.
func TestResultRollbackGatedByConfirmation(t *testing.T) {
	fake := &fakeService{}
	m := installToResult(t, fake)

	m = press(m, "r")
	if m.confirm.kind != confirmRollback {
		t.Fatalf("expected rollback confirmation, got %v", m.confirm.kind)
	}
	m = press(m, "n")
	if m.confirm.kind != confirmNone || len(fake.rollbackCalls) != 0 {
		t.Fatal("declining rollback must cancel without calling the service")
	}

	m = press(m, "r")
	m = pressDrive(t, m, "y")
	if len(fake.rollbackCalls) != 1 || fake.rollbackCalls[0] != "" {
		t.Fatalf("expected Rollback(\"\") resolving the recorded backup, got %v", fake.rollbackCalls)
	}
	if m.result.title != "Rollback" || !m.result.pass {
		t.Fatalf("expected Rollback PASS result, got %+v", m.result)
	}
}

// TestResultPlanDriftIsVisibleFail proves a service drift rejection lands
// as FAIL with the zero-write guarantee and the remedy visible, and no
// rollback offered.
func TestResultPlanDriftIsVisibleFail(t *testing.T) {
	fake := &fakeService{}
	fake.planFn = func(install.Options) (*pipeline.Plan, error) {
		return &pipeline.Plan{Digest: "digest0001"}, nil
	}
	fake.installFn = func(opts install.Options) (*install.InstallReceipt, error) {
		if opts.ExpectedPlanDigest != "digest0001" {
			t.Fatalf("drift run must still be digest-bound, got %q", opts.ExpectedPlanDigest)
		}
		return nil, &pipeline.PlanDriftError{Expected: "digest0001", Observed: "drifted9999"}
	}
	m := sized(newModel(fake, "/home/test", "vtest"))
	m = driveFromHomeToReview(t, m)
	m = pressDrive(t, m, "enter")

	if m.screen != screenResult || m.result.pass {
		t.Fatalf("drift must render FAIL on the result screen, got screen=%v pass=%v", m.screen, m.result.pass)
	}
	if m.result.canRollback {
		t.Fatal("a drifted run wrote nothing; rollback must stay gated off")
	}
	view := m.View()
	for _, want := range []string{
		"FAIL",
		"plan drift",
		"nothing was written",
		"remedy:",
		"stale plan",
		"digest00",
		"drifted9",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("drift result missing %q:\n%s", want, view)
		}
	}
}

// TestResultLowHeightClipsDetailsKeepsVerdict proves a 20-row terminal
// with a long detail list keeps the verdict, changed count, and help line
// visible, folds overflow behind a "+N more" summary, and lets the arrow
// keys scroll details without exceeding the budget.
func TestResultLowHeightClipsDetailsKeepsVerdict(t *testing.T) {
	fake := &fakeService{}
	fake.planFn = func(install.Options) (*pipeline.Plan, error) {
		return &pipeline.Plan{Digest: "digest0001"}, nil
	}
	fake.installFn = func(install.Options) (*install.InstallReceipt, error) {
		receipt := &install.InstallReceipt{PlanDigest: "digest0001", BackupID: "bkp-low"}
		for i := 0; i < 30; i++ {
			receipt.Warnings = append(receipt.Warnings, fmt.Sprintf("detail-%02d: diagnostic line", i))
		}
		return receipt, nil
	}
	m := sized(newModel(fake, "/home/test", "vtest"))
	m = driveFromHomeToReview(t, m)
	m = pressDrive(t, m, "enter")
	m.height = 20

	view := m.View()
	if got := viewLines(m); got > 20 {
		t.Fatalf("result must fit a 20-row terminal, got %d rows:\n%s", got, view)
	}
	for _, want := range []string{"PASS", "Backup: bkp-low", "+", "more", "q quit"} {
		if !strings.Contains(view, want) {
			t.Fatalf("low-height result missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "detail-29") {
		t.Fatalf("folded detail must not render unscrolled:\n%s", view)
	}

	for i := 0; i < 30; i++ {
		m = press(m, "down")
	}
	view = m.View()
	if !strings.Contains(view, "detail-29") {
		t.Fatalf("scrolled result must reveal the last detail:\n%s", view)
	}
	if !strings.Contains(view, "PASS") {
		t.Fatalf("verdict must survive scrolling:\n%s", view)
	}
	if got := viewLines(m); got > 20 {
		t.Fatalf("scrolled result must still fit 20 rows, got %d:\n%s", got, view)
	}
}

// TestUninstallRequiresConfirmationFromHome proves the destructive uninstall
// path: confirmation from Home, decline cancels untouched, confirm runs the
// service and renders the completion receipt.
func TestUninstallRequiresConfirmationFromHome(t *testing.T) {
	fake := &fakeService{}
	fake.uninstallFn = func(install.UninstallOptions) (*install.UninstallReceipt, error) {
		return &install.UninstallReceipt{
			Complete:     true,
			StateRemoved: true,
			Removed:      []string{".config/opencode/AGENTS.md", ".config/opencode/commands/plan.md"},
			MCPRemoved:   []string{"cortex"},
			BackupID:     "bkp-uninstall",
		}, nil
	}
	m := sized(newModel(fake, "/home/test", "vtest"))

	m = press(m, "down")
	m = press(m, "down")
	m = press(m, "down") // cursor 3: Uninstall
	m = press(m, "enter")
	if m.confirm.kind != confirmUninstall || fake.uninstallCals != 0 {
		t.Fatalf("uninstall must wait for confirmation, got confirm=%v calls=%d", m.confirm.kind, fake.uninstallCals)
	}
	if !strings.Contains(m.View(), "Confirm uninstall") {
		t.Fatalf("confirmation overlay not visible:\n%s", m.View())
	}

	m = press(m, "n")
	if m.confirm.kind != confirmNone || fake.uninstallCals != 0 {
		t.Fatal("declining uninstall must cancel without calling the service")
	}

	m = press(m, "enter")
	m = pressDrive(t, m, "y")
	if fake.uninstallCals != 1 {
		t.Fatalf("expected one confirmed uninstall call, got %d", fake.uninstallCals)
	}
	if !m.result.pass || m.result.changed != 2 {
		t.Fatalf("expected uninstall PASS with 2 changes, got %+v", m.result)
	}
	view := m.View()
	if !strings.Contains(view, "bkp-uninstall") || !strings.Contains(view, "v2 state and lock removed") {
		t.Fatalf("uninstall receipt details not visible:\n%s", view)
	}
}
