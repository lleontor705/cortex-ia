package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lleontor705/cortex-ia/internal/delegation"
	"github.com/lleontor705/cortex-ia/internal/tui/styles"
	"github.com/lleontor705/cortex-ia/internal/updater"
)

type upgradePhase int

const (
	upgradePhaseIdle upgradePhase = iota
	upgradePhaseChecking
	upgradePhaseUpToDate
	upgradePhaseAvailable
	upgradePhaseConfirm
	upgradePhaseApplying
	upgradePhaseSuccess
	upgradePhaseBlocked
	upgradePhaseError
)

func (p upgradePhase) String() string {
	names := [...]string{"Idle", "Checking", "UpToDate", "Available", "Confirm", "Applying", "Success", "Blocked", "Error"}
	if int(p) >= 0 && int(p) < len(names) {
		return names[p]
	}
	return "Unknown"
}

type upgradeAction int

const (
	upgradeActionNone upgradeAction = iota
	upgradeActionHome
	upgradeActionQuit
)

type upgradeState struct {
	homeDir, currentVersion, latestVersion, availableVersion, releaseNotes    string
	phase                                                                     upgradePhase
	err                                                                       error
	errText, authorityErr, authorityBlockedReason, appliedTag, appliedVersion string
	hasAuthority                                                              bool
	releaseDate                                                               time.Time
	spinner, width                                                            int
}

type (
	upgradeCheckMsg struct {
		hasUpdate, hasAuthority                       bool
		availableVersion, releaseNotes, blockedReason string
		err                                           error
		release                                       *updater.Release
	}
	upgradeApplyMsg struct {
		appliedVersion, appliedTag string
		err                        error
	}
	upgradeTickMsg time.Time
)

func defaultUserHome() string {
	if h := os.Getenv("CORTEX_IA_HOME"); h != "" {
		return h
	}
	h, _ := os.UserHomeDir()
	return h
}

func orDefault(val, fallback string) string {
	if val != "" {
		return val
	}
	return fallback
}

var (
	upgradeCheckSeam = func() (hasUpdate bool, availableVersion, notes, blockedReason string, err error) {
		home := defaultUserHome()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		client := updater.New("")
		client.StateHome = filepath.Join(home, ".cortex-ia")
		rel, hasUp, checkErr := client.CheckLatest(ctx, "")
		if checkErr != nil {
			return false, "", "", "", checkErr
		}
		tag, body := "", ""
		if rel != nil {
			tag, body = rel.TagName, rel.Body
		}
		active, authErr := probeActiveAuthority(ctx, delegation.DefaultDBPath(home))
		if authErr != nil {
			var drift *delegation.SchemaDriftError
			if errors.As(authErr, &drift) {
				return hasUp, tag, "", fmt.Sprintf("Actualización bloqueada por drift: %v", drift), nil
			}
			return false, "", "", "Error al verificar autoridad: " + authErr.Error(), authErr
		}
		if active {
			return hasUp, tag, body, "Hay tareas in_progress, leases de archivos o claims activos en delegation.db.", nil
		}
		return hasUp, tag, body, "", nil
	}

	upgradeApplySeam = func() (appliedVersion string, err error) {
		home := defaultUserHome()
		stateRoot := filepath.Join(home, ".cortex-ia")
		state, _ := updater.LoadUpdateState(stateRoot)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		applied, err := applyUpdateToCurrentBinary(ctx, "", strings.TrimSpace(state.AppliedFloor))
		if err == nil {
			_ = updater.RecordAppliedFloor(stateRoot, applied, "")
		}
		return applied, err
	}
)

func newUpgradeState(args ...string) (s upgradeState) {
	s = upgradeState{homeDir: defaultUserHome(), phase: upgradePhaseIdle, width: 80}
	if len(args) > 0 {
		s.homeDir = args[0]
	}
	if len(args) > 1 {
		s.currentVersion = args[1]
	}
	return
}

func upgradeCheckCmd(args ...string) tea.Cmd {
	return func() tea.Msg {
		hasUp, ver, notes, blocked, err := upgradeCheckSeam()
		var rel *updater.Release
		if ver != "" {
			rel = &updater.Release{TagName: ver, Body: notes}
		}
		return upgradeCheckMsg{hasUpdate: hasUp, availableVersion: ver, releaseNotes: notes, blockedReason: blocked, err: err, release: rel, hasAuthority: blocked != ""}
	}
}

func upgradeApplyCmd(args ...string) tea.Cmd {
	return func() tea.Msg {
		ver, err := upgradeApplySeam()
		return upgradeApplyMsg{appliedVersion: ver, appliedTag: ver, err: err}
	}
}

func upgradeTick() tea.Cmd {
	return tea.Tick(80*time.Millisecond, func(t time.Time) tea.Msg { return upgradeTickMsg(t) })
}

func (s upgradeState) startChecking() (upgradeState, tea.Cmd) {
	s.phase, s.spinner, s.err, s.errText, s.authorityErr, s.authorityBlockedReason = upgradePhaseChecking, 0, nil, "", "", ""
	return s, tea.Batch(upgradeCheckCmd(), upgradeTick())
}

func (s upgradeState) onCheck(m upgradeCheckMsg) upgradeState {
	s.spinner = 0
	if m.err != nil {
		s.phase, s.err, s.errText = upgradePhaseError, m.err, m.err.Error()
		return s
	}
	if m.availableVersion != "" {
		s.availableVersion, s.latestVersion = m.availableVersion, m.availableVersion
	}
	s.releaseNotes = m.releaseNotes
	if m.hasAuthority || m.blockedReason != "" {
		r := orDefault(m.blockedReason, "Hay tareas in_progress, leases de archivos o claims activos en delegation.db.")
		s.phase, s.hasAuthority, s.authorityBlockedReason, s.authorityErr = upgradePhaseBlocked, true, r, r
		return s
	}
	if !m.hasUpdate {
		s.phase = upgradePhaseUpToDate
	} else {
		s.phase = upgradePhaseAvailable
	}
	return s
}

func (s upgradeState) onApply(m upgradeApplyMsg) upgradeState {
	s.spinner = 0
	if m.err != nil {
		s.phase, s.err, s.errText = upgradePhaseError, m.err, m.err.Error()
		return s
	}
	app := orDefault(m.appliedVersion, s.availableVersion)
	s.phase, s.appliedVersion, s.appliedTag = upgradePhaseSuccess, app, app
	return s
}

func (s upgradeState) updateUpgradeScreen(msg tea.Msg) (upgradeState, tea.Cmd) {
	switch m := msg.(type) {
	case upgradeTickMsg, tickMsg:
		if s.phase == upgradePhaseChecking || s.phase == upgradePhaseApplying {
			s.spinner = (s.spinner + 1) % len(spinnerFrames)
			return s, upgradeTick()
		}
	case upgradeCheckMsg:
		return s.onCheck(m), nil
	case upgradeApplyMsg:
		return s.onApply(m), nil
	case tea.KeyMsg:
		if m.Type == tea.KeyCtrlC {
			return s, tea.Quit
		}
		return s.handleKey(m.String())
	}
	return s, nil
}

func (s upgradeState) handleKey(k string) (upgradeState, tea.Cmd) {
	switch {
	case (s.phase == upgradePhaseIdle || s.phase == upgradePhaseUpToDate || s.phase == upgradePhaseBlocked) && (k == "c" || k == "C" || k == "enter"):
		return s.startChecking()
	case s.phase == upgradePhaseAvailable && (k == "u" || k == "U"):
		s.phase = upgradePhaseConfirm
	case s.phase == upgradePhaseAvailable && (k == "c" || k == "C"):
		return s.startChecking()
	case s.phase == upgradePhaseConfirm && (k == "y" || k == "Y"):
		s.phase, s.spinner, s.err, s.errText = upgradePhaseApplying, 0, nil, ""
		return s, tea.Batch(upgradeApplyCmd(), upgradeTick())
	case s.phase == upgradePhaseConfirm && (k == "n" || k == "N" || k == "esc"):
		s.phase = upgradePhaseAvailable
	case s.phase == upgradePhaseSuccess && (k == "q" || k == "Q"):
		return s, tea.Quit
	case s.phase == upgradePhaseError && (k == "c" || k == "C" || k == "r" || k == "R" || k == "enter"):
		return s.startChecking()
	}
	return s, nil
}

func (s upgradeState) update(msg tea.KeyMsg) (upgradeState, upgradeAction, tea.Cmd) {
	if msg.Type == tea.KeyCtrlC || (s.phase == upgradePhaseSuccess && (msg.String() == "q" || msg.String() == "Q")) {
		return s, upgradeActionQuit, tea.Quit
	}
	if s.phase != upgradePhaseConfirm && (msg.String() == "esc" || msg.String() == "b" || msg.String() == "B") {
		return s, upgradeActionHome, nil
	}
	next, cmd := s.updateUpgradeScreen(msg)
	return next, upgradeActionNone, cmd
}

func (s upgradeState) view(width int) string {
	s.width = width
	return s.viewUpgradeScreen()
}

func (s upgradeState) viewUpgradeScreen() string {
	w := s.width
	if w <= 0 {
		w = 80
	}
	ver := orDefault(s.currentVersion, "(desconocida)")
	avail := orDefault(s.availableVersion, orDefault(s.latestVersion, "(desconocida)"))
	applied := orDefault(s.appliedVersion, orDefault(s.appliedTag, avail))
	blocked := orDefault(s.authorityBlockedReason, orDefault(s.authorityErr, "Hay tareas in_progress, leases de archivos o claims activos en delegation.db."))
	errStr := s.errText
	if errStr == "" && s.err != nil {
		errStr = s.err.Error()
	}
	errStr = orDefault(errStr, "Error desconocido")

	lines := []string{styleSubtitle.Render("Actualización de software"), ""}
	hints := "[esc] Volver"

	switch s.phase {
	case upgradePhaseChecking, upgradePhaseApplying:
		msg, h := "Comprobando nuevas versiones y autoridad de trabajo…", "[esc] Volver"
		if s.phase == upgradePhaseApplying {
			msg, h = "Descargando, verificando y aplicando actualización…", "Por favor espera, no cierres la aplicación…"
		}
		lines = append(lines, fmt.Sprintf("  %s %s", styleSubtitle.Render(styles.SpinnerChar(s.spinner)), styleSelected.Render(msg)))
		hints = h
	case upgradePhaseUpToDate:
		lines = append(lines, fmt.Sprintf("  Versión actual instalada: %s", styleSelected.Render(ver)), "", "  "+stylePass.Render("✓ El software está actualizado a la versión más reciente."))
		hints = "[c] Comprobar de nuevo · [esc] Volver"
	case upgradePhaseAvailable:
		lines = append(lines, fmt.Sprintf("  Versión actual instalada:   %s", styleDim.Render(ver)), fmt.Sprintf("  Versión más reciente:       %s", stylePass.Render(avail)), "  Estado de verificación:     SHA-256 (checksums.txt)", "  Autoridad de trabajo:       "+stylePass.Render("Despejada (sin tareas activas)"))
		if s.releaseNotes != "" {
			lines = append(lines, "", fmt.Sprintf("  Notas de release (%s):", avail))
			lines = append(lines, formatReleaseNotes(s.releaseNotes, w-6)...)
		}
		hints = fmt.Sprintf("[u] Actualizar a %s · [c] Comprobar ahora · [esc] Volver", avail)
	case upgradePhaseConfirm:
		lines = append(lines, "  "+styleWarn.Render(fmt.Sprintf("¿Deseas descargar y aplicar la actualización a %s?", avail)), "", "  Esta operación verificará las firmas y reemplazará el ejecutable actual.", "", styleSelected.Render("  [y] Confirmar e instalar   [n/esc] Cancelar"))
		hints = "[y] Confirmar · [n/esc] Cancelar"
	case upgradePhaseSuccess:
		lines = append(lines, "  "+stylePass.Render("✓ Actualización aplicada exitosamente"), "", fmt.Sprintf("  Versión instalada: %s", styleSelected.Render(applied)), "", "  "+styleWarn.Render("Reinicia cortex-ia para comenzar a usar la nueva versión."))
		hints = "[q] Salir para reiniciar · [esc] Volver"
	case upgradePhaseBlocked:
		lines = append(lines, fmt.Sprintf("  Versión actual instalada:   %s", styleDim.Render(ver)))
		if avail != "(desconocida)" {
			lines = append(lines, fmt.Sprintf("  Versión más reciente:       %s", styleSelected.Render(avail)))
		}
		lines = append(lines, "", "  "+styleWarn.Render("⚠ ACTUALIZACIÓN BLOQUEADA POR AUTORIDAD ACTIVA"), "  "+blocked, "", "  Completa, libera o reconcilia las tareas activas antes de actualizar el binario.")
		hints = "[c] Reintentar comprobación · [esc] Volver"
	case upgradePhaseError:
		lines = append(lines, "  "+styleFail.Render("No se pudo comprobar o aplicar la actualización:"), "  "+errStr)
		hints = "[c/r] Reintentar · [esc] Volver"
	default:
		lines = append(lines, fmt.Sprintf("  Versión actual instalada: %s", styleSelected.Render(ver)), "", "  Presiona [c] o Enter para comprobar si hay actualizaciones disponibles.")
		hints = "[c] Comprobar ahora · [esc] Volver"
	}
	lines = append(lines, "", styleDim.Render(truncate(hints, w)))
	return strings.Join(lines, "\n")
}

func formatReleaseNotes(notes string, maxWidth int) []string {
	if maxWidth <= 10 {
		maxWidth = 70
	}
	var res []string
	for _, l := range strings.Split(strings.TrimSpace(notes), "\n") {
		if t := strings.TrimSpace(l); t != "" {
			if len(res) >= 5 {
				res = append(res, styleDim.Render("    …"))
				break
			}
			res = append(res, "    "+truncate(t, maxWidth))
		}
	}
	return res
}
