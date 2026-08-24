package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lleontor705/cortex-ia/internal/delegation"
)

var delegationRoles = []string{"implement", "investigate", "reviewer", "planner"}

// --- Paso 1: Herdr Multiplexer ---

func (m model) updateWizardHerdr(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key := msg.String(); key {
	case "ctrl+c", "q":
		m.quitting = true
		return m, tea.Quit
	case "esc":
		m.screen = screenHome
		m.cursor = 0
		return m, nil
	case "up", "k", "1":
		m.wizardCursor = 0
	case "down", "j", "2":
		m.wizardCursor = 1
	case " ", "enter":
		m.delegationCfg.UseHerdr = (m.wizardCursor == 0)
		m.screen = screenWizardDelegation
		if m.delegationCfg.DelegationEnabled {
			m.wizardCursor = 0
		} else {
			m.wizardCursor = 1
		}
		return m, nil
	}
	return m, nil
}

func (m model) viewWizardHerdr() string {
	width := m.contentWidth()
	top := []string{
		truncate(m.header("Asistente de Instalación — Paso 1 de 3"), width),
		"",
		styleSubtitle.Render("¿Deseas utilizar Herdr como multiplexor de paneles y workspaces?"),
		"",
		styleDim.Render("Herdr permite dividir paneles automáticamente y supervisar la ejecución"),
		styleDim.Render("de subagentes y CLIs en tiempo real con monitores de estado."),
		"",
	}

	options := []struct {
		title string
		desc  string
	}{
		{
			title: "Sí, habilitar Herdr Workspace Multiplexer",
			desc:  "Divide paneles en vivo (split right) y supervisa estados de agentes",
		},
		{
			title: "No, trabajar en terminal estándar",
			desc:  "Ejecuta directamente sin multiplexación visual de paneles",
		},
	}

	var content []string
	for i, opt := range options {
		cursorMark := "  "
		text := fmt.Sprintf("[%s] %s", " ", opt.title)
		if (i == 0 && m.delegationCfg.UseHerdr) || (i == 1 && !m.delegationCfg.UseHerdr) {
			text = fmt.Sprintf("[%s] %s", "x", opt.title)
		}
		if m.wizardCursor == i {
			cursorMark = "> "
			text = styleSelected.Render(text)
		}
		content = append(content, truncate(cursorMark+text, width))
		content = append(content, truncate("    "+styleDim.Render(opt.desc), width))
		content = append(content, "")
	}

	var bottom []string
	bottom = append(bottom, m.footer("enter continuar · ↑/↓ seleccionar · esc cancelar / inicio"))
	return strings.Join(clampScreen(top, content, bottom, m.bodyHeight(), 0, "up/down"), "\n")
}

// --- Paso 2: Delegación a CLIs Externas ---

func (m model) updateWizardDelegation(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key := msg.String(); key {
	case "ctrl+c", "q":
		m.quitting = true
		return m, tea.Quit
	case "esc", "b", "B":
		m.screen = screenWizardHerdr
		if m.delegationCfg.UseHerdr {
			m.wizardCursor = 0
		} else {
			m.wizardCursor = 1
		}
		return m, nil
	case "up", "k", "1":
		m.wizardCursor = 0
	case "down", "j", "2":
		m.wizardCursor = 1
	case " ", "enter":
		if m.wizardCursor == 0 {
			// Delegación habilitada -> ir a configurar roles
			m.delegationCfg.DelegationEnabled = true
			m.screen = screenWizardRoles
			m.wizardCursor = 0
			return m, nil
		}
		// Delegación deshabilitada -> instalación normal directa
		m.delegationCfg.DelegationEnabled = false
		for _, role := range delegationRoles {
			if m.delegationCfg.Roles == nil {
				m.delegationCfg.Roles = make(map[string]delegation.RoleConfig)
			}
			r := m.delegationCfg.Roles[role]
			r.Delegate = false
			r.CLI = "native"
			r.Command = ""
			r.Args = nil
			m.delegationCfg.Roles[role] = r
		}
		m.screen = screenReview
		m.opts.DelegationConfig = &m.delegationCfg
		m.plan = nil
		m.replanning = true
		return m, planCmd(m.svc, m.reviewOptions())
	}
	return m, nil
}

func (m model) viewWizardDelegation() string {
	width := m.contentWidth()
	top := []string{
		truncate(m.header("Asistente de Instalación — Paso 2 de 3"), width),
		"",
		styleSubtitle.Render("¿Deseas delegar fases del flujo de trabajo a CLIs externas?"),
		"",
		styleDim.Render("Permite enviar roles (como implement o reviewer) a Antigravity CLI (agy)"),
		styleDim.Render("o Claude Code (claude) de forma transparente o mediante Herdr."),
		"",
	}

	options := []struct {
		title string
		desc  string
	}{
		{
			title: "Sí, configurar delegación a CLIs externas",
			desc:  "Personaliza qué fases se envían a Antigravity (agy) o Claude Code",
		},
		{
			title: "No, instalación estándar (100% nativo OpenCode)",
			desc:  "Todos los roles se ejecutan exclusivamente con subagentes nativos",
		},
	}

	var content []string
	for i, opt := range options {
		cursorMark := "  "
		text := fmt.Sprintf("[%s] %s", " ", opt.title)
		if (i == 0 && m.delegationCfg.DelegationEnabled) || (i == 1 && !m.delegationCfg.DelegationEnabled) {
			text = fmt.Sprintf("[%s] %s", "x", opt.title)
		}
		if m.wizardCursor == i {
			cursorMark = "> "
			text = styleSelected.Render(text)
		}
		content = append(content, truncate(cursorMark+text, width))
		content = append(content, truncate("    "+styleDim.Render(opt.desc), width))
		content = append(content, "")
	}

	var bottom []string
	bottom = append(bottom, m.footer("enter continuar · ↑/↓ seleccionar · b / esc volver"))
	return strings.Join(clampScreen(top, content, bottom, m.bodyHeight(), 0, "up/down"), "\n")
}

// --- Paso 3: Matriz de Fases y Motores ---

func (m model) updateWizardRoles(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key := msg.String(); key {
	case "ctrl+c", "q":
		m.quitting = true
		return m, tea.Quit
	case "esc", "b", "B":
		m.screen = screenWizardDelegation
		m.wizardCursor = 0
		return m, nil
	case "up", "k":
		if m.wizardCursor > 0 {
			m.wizardCursor--
		}
	case "down", "j":
		if m.wizardCursor < len(delegationRoles) { // 0..3: roles, 4: continue button
			m.wizardCursor++
		}
	case " ", "tab", "right", "left":
		if m.wizardCursor < len(delegationRoles) {
			role := delegationRoles[m.wizardCursor]
			if m.delegationCfg.Roles == nil {
				m.delegationCfg.Roles = make(map[string]delegation.RoleConfig)
			}
			r := m.delegationCfg.Roles[role]
			// Cycle: Native -> agy -> claude -> Native
			if !r.Delegate {
				r.Delegate = true
				r.CLI = "agy"
				r.Command = "agy"
				r.Args = []string{"--prompt-file"}
			} else if r.CLI == "agy" {
				r.Delegate = true
				r.CLI = "claude"
				r.Command = "claude"
				r.Args = []string{"-p"}
			} else {
				r.Delegate = false
				r.CLI = "native"
				r.Command = ""
				r.Args = nil
			}
			m.delegationCfg.Roles[role] = r
			m.opts.DelegationConfig = &m.delegationCfg
		}
	case "enter":
		// Guardar y avanzar a Review
		m.screen = screenReview
		m.opts.DelegationConfig = &m.delegationCfg
		m.plan = nil
		m.replanning = true
		return m, planCmd(m.svc, m.reviewOptions())
	}
	return m, nil
}

func (m model) viewWizardRoles() string {
	width := m.contentWidth()
	top := []string{
		truncate(m.header("Asistente de Instalación — Paso 3 de 3"), width),
		"",
		styleSubtitle.Render("Asignación de Motores por Fase / Subagente:"),
		"",
		styleDim.Render("Presiona Espacio o Tab para ciclar entre [Nativo], [agy] y [claude]:"),
		"",
	}

	var content []string
	for i, role := range delegationRoles {
		r := m.delegationCfg.Roles[role]
		status := styleDim.Render("[ Nativo OpenCode ]")
		if r.Delegate {
			if r.CLI == "agy" {
				status = stylePass.Render("[ Antigravity CLI (agy) ]")
			} else if r.CLI == "claude" {
				status = styleSubtitle.Render("[ Claude Code (claude) ]")
			}
		}
		line := fmt.Sprintf("  • %-12s ➔ %s", role, status)
		if m.wizardCursor == i {
			line = styleSelected.Render(fmt.Sprintf("> • %-12s ➔ %s", role, status))
		}
		content = append(content, truncate(line, width))
	}

	content = append(content, "")
	btnText := "  [ Continuar a la Revisión del Plan ➔ ]"
	if m.wizardCursor == len(delegationRoles) {
		btnText = styleSelected.Render("> [ Continuar a la Revisión del Plan ➔ ]")
	}
	content = append(content, truncate(btnText, width))

	var bottom []string
	bottom = append(bottom, m.footer("space/tab cambiar motor · enter continuar a review · b volver"))
	return strings.Join(clampScreen(top, content, bottom, m.bodyHeight(), 0, "up/down"), "\n")
}
