package tui

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lleontor705/cortex-ia/internal/install"
	"github.com/lleontor705/cortex-ia/internal/targets"
)

// driveHosts executes a command and every command its messages return, so the
// chained per-host steps and the OpenCode-to-host handoff run to completion.
func driveHosts(t *testing.T, m model, cmd tea.Cmd) model {
	t.Helper()
	queue := []tea.Cmd{cmd}
	for len(queue) > 0 {
		next := queue[0]
		queue = queue[1:]
		for _, msg := range collect(t, next) {
			if _, ok := msg.(tickMsg); ok {
				continue
			}
			if _, ok := msg.(homeTickMsg); ok {
				continue
			}
			updated, follow := m.Update(msg)
			m = updated.(model)
			if follow != nil {
				queue = append(queue, follow)
			}
		}
	}
	return m
}

// pressHosts applies one keypress and runs every command it produced.
func pressHosts(t *testing.T, m model, k string) model {
	t.Helper()
	updated, cmd := m.Update(key(k))
	return driveHosts(t, updated.(model), cmd)
}

// openHostFlow presses Home → Install and returns the host picker.
func openHostFlow(t *testing.T, svc ServiceAPI, hosts TargetAPI) model {
	t.Helper()
	m := sized(newModel(svc, hosts, t.TempDir(), "vtest"))
	m = press(m, "enter")
	if m.screen != screenHostPicker {
		t.Fatalf("Install must open the host picker, got screen %v", m.screen)
	}
	return m
}

func TestHostPickerPreChecksDetectedHosts(t *testing.T) {
	m := openHostFlow(t, &fakeService{}, &fakeHosts{detected: map[targets.TargetID]bool{targets.TargetClaude: true}})
	if !m.hostPick.checked[targets.TargetClaude] || m.hostPick.checked[targets.TargetOpenCode] || m.hostPick.checked[targets.TargetAGY] {
		t.Fatalf("only the detected claude host must be pre-checked, got %v", m.hostPick.checked)
	}
	if view := m.View(); !strings.Contains(view, "[x] Claude Code") || !strings.Contains(view, "claude found") {
		t.Fatalf("picker must show the checked, detected host:\n%s", view)
	}

	m = openHostFlow(t, &fakeService{}, &fakeHosts{})
	if !m.hostPick.checked[targets.TargetOpenCode] || m.hostPick.checked[targets.TargetClaude] {
		t.Fatalf("with no host detected only OpenCode must be pre-checked, got %v", m.hostPick.checked)
	}
}

func TestHostPickerSpaceTogglesAndEmptySelectionStays(t *testing.T) {
	svc := &fakeService{}
	m := openHostFlow(t, svc, &fakeHosts{})

	m = press(m, "space")
	if m.hostPick.checked[targets.TargetOpenCode] {
		t.Fatal("space must uncheck the cursor row")
	}
	m = press(m, "down")
	m = press(m, "space")
	if !m.hostPick.checked[targets.TargetClaude] {
		t.Fatal("space must check the moved-to row")
	}

	m = press(m, "space") // uncheck claude again, leaving nothing selected
	m = press(m, "enter")
	if m.screen != screenHostPicker || m.hostPick.status == "" {
		t.Fatalf("empty selection must stay on the picker with a hint, got screen %v status %q", m.screen, m.hostPick.status)
	}
	if len(svc.planCalls) != 0 || len(svc.installCalls) != 0 {
		t.Fatal("empty selection must not reach the service")
	}
}

func TestHostPickerEscReturnsHome(t *testing.T) {
	m := openHostFlow(t, &fakeService{}, &fakeHosts{})
	m = press(m, "esc")
	if m.screen != screenHome || m.cursor != 0 {
		t.Fatalf("esc must return home on the install entry, got screen %v cursor %d", m.screen, m.cursor)
	}
}

func TestClaudeOnlyInstallNeverCallsOpenCodeService(t *testing.T) {
	svc := &fakeService{}
	hosts := &fakeHosts{detected: map[targets.TargetID]bool{targets.TargetClaude: true}}
	m := openHostFlow(t, svc, hosts)
	m = pressHosts(t, m, "enter")

	if len(hosts.installCalls) != 1 || hosts.installCalls[0] != targets.TargetClaude {
		t.Fatalf("expected Install(claude) once, got %v", hosts.installCalls)
	}
	if len(svc.planCalls) != 0 || len(svc.installCalls) != 0 {
		t.Fatalf("OpenCode service must not be touched, plan=%d install=%d", len(svc.planCalls), len(svc.installCalls))
	}
	if m.screen != screenResult || !m.result.pass {
		t.Fatalf("expected PASS result screen, got screen %v result %+v", m.screen, m.result)
	}
	view := m.View()
	if !strings.Contains(view, "Claude Code: PASS") || !strings.Contains(view, "Installed Claude Code") {
		t.Fatalf("result must show the Claude section:\n%s", view)
	}
}

func TestOpenCodeThenAGYRunsAfterOpenCodeSucceeds(t *testing.T) {
	svc := &fakeService{}
	hosts := &fakeHosts{detected: map[targets.TargetID]bool{targets.TargetOpenCode: true, targets.TargetAGY: true}}
	seenAtAGY := -1
	hosts.installFn = func(id targets.TargetID, _ targets.ApplyOptions) (*targets.TargetResult, error) {
		if id == targets.TargetAGY {
			seenAtAGY = len(svc.installCalls)
		}
		return &targets.TargetResult{Target: id, Success: true, Message: "agy ok"}, nil
	}
	m := openHostFlow(t, svc, hosts)
	m = pressHosts(t, m, "enter") // picker -> Review (plan)
	if m.screen != screenReview {
		t.Fatalf("OpenCode must reach Review first, got %v", m.screen)
	}
	m = pressHosts(t, m, "enter") // Review -> OpenCode install -> agy

	if seenAtAGY != 1 {
		t.Fatalf("agy must run after exactly one OpenCode install, saw %d", seenAtAGY)
	}
	if m.screen != screenResult || !m.result.pass {
		t.Fatalf("expected PASS result, got screen %v result %+v", m.screen, m.result)
	}
	if view := m.View(); !strings.Contains(view, "Antigravity (agy): PASS") {
		t.Fatalf("result must show the agy section:\n%s", view)
	}
}

func TestAGYSkippedWhenOpenCodeInstallFails(t *testing.T) {
	svc := &fakeService{installFn: func(install.Options) (*install.InstallReceipt, error) {
		return nil, errors.New("apply failed")
	}}
	hosts := &fakeHosts{detected: map[targets.TargetID]bool{targets.TargetOpenCode: true, targets.TargetAGY: true}}
	m := openHostFlow(t, svc, hosts)
	m = pressHosts(t, m, "enter")
	m = pressHosts(t, m, "enter")

	if len(hosts.installCalls) != 0 {
		t.Fatalf("agy must not run after a failed OpenCode install, got %v", hosts.installCalls)
	}
	if m.screen != screenResult || m.result.pass {
		t.Fatalf("expected FAIL result, got screen %v pass %v", m.screen, m.result.pass)
	}
	if view := m.View(); !strings.Contains(view, "Antigravity (agy): skipped (OpenCode step did not succeed)") {
		t.Fatalf("result must say agy was skipped:\n%s", view)
	}
}

func TestCancelledOpenCodeReviewSkipsExtraHosts(t *testing.T) {
	svc := &fakeService{}
	hosts := &fakeHosts{detected: map[targets.TargetID]bool{targets.TargetOpenCode: true, targets.TargetAGY: true}}
	m := openHostFlow(t, svc, hosts)
	m = pressHosts(t, m, "enter")
	m = press(m, "esc")

	if len(hosts.installCalls) != 0 || len(svc.installCalls) != 0 {
		t.Fatal("cancelling the review must not install anything")
	}
	if m.screen != screenResult || m.result.pass {
		t.Fatalf("cancelled review must end on a FAIL result, got screen %v", m.screen)
	}
	if view := m.View(); !strings.Contains(view, "skipped (OpenCode review cancelled)") {
		t.Fatalf("result must say agy was skipped:\n%s", view)
	}
}

func TestClaudeOverwriteConflictShowsRerunHint(t *testing.T) {
	hosts := &fakeHosts{
		detected: map[targets.TargetID]bool{targets.TargetClaude: true},
		installFn: func(targets.TargetID, targets.ApplyOptions) (*targets.TargetResult, error) {
			return nil, errors.New("Claude Code: 1 path(s) differ from what cortex-ia wrote and would be overwritten; nothing was written (re-run with Overwrite):\n  - .claude/CLAUDE.md (file differs from the copy cortex-ia wrote)")
		},
	}
	m := openHostFlow(t, &fakeService{}, hosts)
	m = pressHosts(t, m, "enter")

	view := m.View()
	if m.result.pass {
		t.Fatal("a failed claude install must be a FAIL result")
	}
	if !strings.Contains(view, "Claude Code: FAIL") || !strings.Contains(view, "cortex-ia install --target claude --overwrite") {
		t.Fatalf("conflict must render the --overwrite hint:\n%s", view)
	}
}

func TestUninstallHostOnlyRunsAfterConfirmation(t *testing.T) {
	svc := &fakeService{}
	hosts := &fakeHosts{installed: map[targets.TargetID]bool{targets.TargetClaude: true}}
	m := sized(newModel(svc, hosts, t.TempDir(), "vtest"))
	for i := 0; i < uninstallEntryIndex; i++ {
		m = press(m, "down")
	}
	m = press(m, "enter")
	if !m.hostPick.uninstall || !m.hostPick.checked[targets.TargetOpenCode] || !m.hostPick.checked[targets.TargetClaude] {
		t.Fatalf("uninstall must pre-check OpenCode and installed hosts, got %v", m.hostPick.checked)
	}
	m = press(m, "space") // drop OpenCode
	m = press(m, "enter")
	if m.confirm.kind != confirmHostUninstall || len(hosts.uninstallCalls) != 0 {
		t.Fatalf("uninstall must wait for confirmation, confirm=%v calls=%v", m.confirm.kind, hosts.uninstallCalls)
	}
	m = pressHosts(t, m, "y")

	if len(hosts.uninstallCalls) != 1 || hosts.uninstallCalls[0] != targets.TargetClaude || svc.uninstallCals != 0 {
		t.Fatalf("expected Uninstall(claude) only, host=%v service=%d", hosts.uninstallCalls, svc.uninstallCals)
	}
	if m.screen != screenResult || !strings.Contains(m.View(), "Claude Code: PASS") {
		t.Fatalf("result must show the Claude uninstall:\n%s", m.View())
	}
}
