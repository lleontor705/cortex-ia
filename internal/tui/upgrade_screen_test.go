package tui

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lleontor705/cortex-ia/internal/updater"
)

func stubUpgradeSeams(t *testing.T,
	checkFn func() (bool, string, string, string, error),
	applyFn func() (string, error),
) {
	t.Helper()
	prevCheck, prevApply := upgradeCheckSeam, upgradeApplySeam
	if checkFn != nil {
		upgradeCheckSeam = checkFn
	}
	if applyFn != nil {
		upgradeApplySeam = applyFn
	}
	t.Cleanup(func() {
		upgradeCheckSeam = prevCheck
		upgradeApplySeam = prevApply
	})
}

func TestUpgradeScreen_FSM_Transitions(t *testing.T) {
	stubUpgradeSeams(t,
		func() (bool, string, string, string, error) {
			return true, "v0.5.0", "• New features\n• Bug fixes", "", nil
		},
		func() (string, error) {
			return "v0.5.0", nil
		},
	)

	s := newUpgradeState(t.TempDir(), "v0.4.50")
	if s.phase != upgradePhaseIdle || s.phase.String() != "Idle" {
		t.Fatalf("expected idle phase, got %v", s.phase)
	}

	var cmd tea.Cmd
	s, cmd = s.startChecking()
	if s.phase != upgradePhaseChecking || cmd == nil {
		t.Fatalf("expected checking phase, got %v", s.phase)
	}
	if !strings.Contains(s.view(80), "Comprobando") {
		t.Fatalf("checking view missing progress: %s", s.view(80))
	}

	checkMsg := upgradeCheckCmd()().(upgradeCheckMsg)
	s, _ = s.updateUpgradeScreen(checkMsg)
	if s.phase != upgradePhaseAvailable {
		t.Fatalf("expected available phase, got %v", s.phase)
	}
	view := s.view(80)
	if !strings.Contains(view, "v0.5.0") || !strings.Contains(view, "New features") {
		t.Fatalf("available view missing details:\n%s", view)
	}

	s, _, _ = s.update(key("u"))
	if s.phase != upgradePhaseConfirm {
		t.Fatalf("expected confirm phase, got %v", s.phase)
	}
	if !strings.Contains(s.view(80), "¿Deseas descargar") {
		t.Fatalf("confirm view missing prompt: %s", s.view(80))
	}

	s, _, cmd = s.update(key("y"))
	if s.phase != upgradePhaseApplying || cmd == nil {
		t.Fatalf("expected applying phase, got %v", s.phase)
	}
	if !strings.Contains(s.view(80), "Descargando") {
		t.Fatalf("applying view missing notice: %s", s.view(80))
	}

	applyMsg := upgradeApplyCmd()().(upgradeApplyMsg)
	s, _ = s.updateUpgradeScreen(applyMsg)
	if s.phase != upgradePhaseSuccess || s.appliedVersion != "v0.5.0" {
		t.Fatalf("expected success phase with v0.5.0, got %v (ver=%s)", s.phase, s.appliedVersion)
	}
	if !strings.Contains(s.view(80), "exitosa") || !strings.Contains(s.view(80), "Reinicia cortex-ia") {
		t.Fatalf("success view missing restart instructions:\n%s", s.view(80))
	}

	sConfirm := upgradeState{phase: upgradePhaseConfirm, availableVersion: "v0.5.0"}
	sCanceled, _, _ := sConfirm.update(key("n"))
	if sCanceled.phase != upgradePhaseAvailable {
		t.Fatalf("pressing n in confirm did not revert to available: got %v", sCanceled.phase)
	}
	sEscCanceled, _, _ := sConfirm.update(key("esc"))
	if sEscCanceled.phase != upgradePhaseAvailable {
		t.Fatalf("pressing esc in confirm did not revert to available: got %v", sEscCanceled.phase)
	}
}

func TestUpgradeScreen_CheckOutcomes(t *testing.T) {
	t.Run("UpToDate", func(t *testing.T) {
		stubUpgradeSeams(t, func() (bool, string, string, string, error) {
			return false, "v0.4.50", "", "", nil
		}, nil)
		s := newUpgradeState(t.TempDir(), "v0.4.50")
		s, _ = s.updateUpgradeScreen(upgradeCheckCmd()().(upgradeCheckMsg))
		if s.phase != upgradePhaseUpToDate {
			t.Fatalf("expected up_to_date, got %v", s.phase)
		}
		if !strings.Contains(s.view(80), "actualizado") {
			t.Fatalf("up_to_date view missing text:\n%s", s.view(80))
		}
	})

	t.Run("CheckErrorAndRetry", func(t *testing.T) {
		stubUpgradeSeams(t, func() (bool, string, string, string, error) {
			return false, "", "", "", errors.New("network timeout")
		}, nil)
		s := newUpgradeState(t.TempDir(), "v0.4.50")
		s, _ = s.updateUpgradeScreen(upgradeCheckCmd()().(upgradeCheckMsg))
		if s.phase != upgradePhaseError || s.errText != "network timeout" {
			t.Fatalf("expected error phase with text, got %v: %s", s.phase, s.errText)
		}
		if !strings.Contains(s.view(80), "network timeout") {
			t.Fatalf("error view missing error message:\n%s", s.view(80))
		}
		sRetried, _, cmd := s.update(key("r"))
		if sRetried.phase != upgradePhaseChecking || cmd == nil {
			t.Fatalf("expected retry to enter checking, got %v", sRetried.phase)
		}
	})

	t.Run("AuthorityBlocked", func(t *testing.T) {
		stubUpgradeSeams(t, func() (bool, string, string, string, error) {
			return true, "v0.5.0", "", "active claim in progress", nil
		}, nil)
		s := newUpgradeState(t.TempDir(), "v0.4.50")
		s, _ = s.updateUpgradeScreen(upgradeCheckCmd()().(upgradeCheckMsg))
		if s.phase != upgradePhaseBlocked || !s.hasAuthority {
			t.Fatalf("expected blocked phase with hasAuthority, got %v auth=%v", s.phase, s.hasAuthority)
		}
		v := s.view(80)
		if !strings.Contains(v, "BLOQUEADA") || !strings.Contains(v, "active claim in progress") {
			t.Fatalf("blocked view missing warning:\n%s", v)
		}
	})

	t.Run("ApplyError", func(t *testing.T) {
		stubUpgradeSeams(t, nil, func() (string, error) {
			return "", errors.New("digest mismatch")
		})
		s := upgradeState{phase: upgradePhaseApplying, availableVersion: "v0.5.0"}
		s, _ = s.updateUpgradeScreen(upgradeApplyCmd()().(upgradeApplyMsg))
		if s.phase != upgradePhaseError || s.errText != "digest mismatch" {
			t.Fatalf("expected apply error, got %v: %s", s.phase, s.errText)
		}
	})
}

func TestUpgradeScreen_Keybindings(t *testing.T) {
	for _, initPhase := range []upgradePhase{upgradePhaseIdle, upgradePhaseUpToDate, upgradePhaseBlocked} {
		s := upgradeState{phase: initPhase}
		next, action, cmd := s.update(key("c"))
		if next.phase != upgradePhaseChecking || action != upgradeActionNone || cmd == nil {
			t.Fatalf("phase %v: 'c' did not trigger checking, got phase=%v action=%v", initPhase, next.phase, action)
		}
	}

	sAvail := upgradeState{phase: upgradePhaseAvailable, availableVersion: "v0.5.0"}
	sConfirm, act, _ := sAvail.update(key("u"))
	if sConfirm.phase != upgradePhaseConfirm || act != upgradeActionNone {
		t.Fatalf("expected confirm from 'u', got %v", sConfirm.phase)
	}

	sApp, act, cmd := sConfirm.update(key("y"))
	if sApp.phase != upgradePhaseApplying || act != upgradeActionNone || cmd == nil {
		t.Fatalf("expected applying from 'y', got %v", sApp.phase)
	}

	sSucc := upgradeState{phase: upgradePhaseSuccess}
	_, qAct, qCmd := sSucc.update(key("q"))
	if qAct != upgradeActionQuit || qCmd == nil {
		t.Fatalf("expected quit action and cmd from 'q' in success, got act=%v", qAct)
	}

	for _, k := range []string{"esc", "b"} {
		s := upgradeState{phase: upgradePhaseAvailable}
		_, actHome, _ := s.update(key(k))
		if actHome != upgradeActionHome {
			t.Fatalf("key %s should produce upgradeActionHome, got %v", k, actHome)
		}
	}

	s := upgradeState{phase: upgradePhaseChecking}
	_, ctrlAct, ctrlCmd := s.update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if ctrlAct != upgradeActionQuit || ctrlCmd == nil {
		t.Fatalf("ctrl+c did not trigger quit")
	}
}

func TestUpgradeScreen_HomeBadge(t *testing.T) {
	home := t.TempDir()
	m := sized(newModel(&fakeService{}, home, "v0.4.50"))

	if b := m.upgradeBadge(); b != "" {
		t.Fatalf("expected empty badge initially, got %q", b)
	}

	m.upgrade.phase = upgradePhaseAvailable
	m.upgrade.availableVersion = "v0.5.0"
	b := m.upgradeBadge()
	if !strings.Contains(b, "[NEW v0.5.0]") {
		t.Fatalf("expected [NEW v0.5.0] in badge, got %q", b)
	}
	homeView := m.viewHome()
	if !strings.Contains(homeView, "[NEW v0.5.0]") {
		t.Fatalf("viewHome missing badge:\n%s", homeView)
	}

	m.upgrade.phase = upgradePhaseSuccess
	if b := m.upgradeBadge(); b != "" {
		t.Fatalf("expected empty badge on success, got %q", b)
	}
	m.upgrade.phase = upgradePhaseUpToDate
	if b := m.upgradeBadge(); b != "" {
		t.Fatalf("expected empty badge on up_to_date, got %q", b)
	}

	m.upgrade.phase = upgradePhaseIdle
	m.upgrade.availableVersion = ""
	_ = updater.SaveUpdateStateAtomic(filepath.Join(home, ".cortex-ia"), updater.UpdateState{
		Available:    "v0.6.0",
		AppliedFloor: "v0.4.50",
	})
	if b := m.upgradeBadge(); !strings.Contains(b, "[NEW v0.6.0]") {
		t.Fatalf("expected disk state badge [NEW v0.6.0], got %q", b)
	}
}
