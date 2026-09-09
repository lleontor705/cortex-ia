package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lleontor705/cortex-ia/internal/delegation"
)

// Standalone delegation configuration screen.
// Items:
// 0: Multiplexor Herdr (AutoSplit & UseHerdr)
// 1: Delegación Externa (DelegationEnabled)
// 2: implement
// 3: investigate
// 4: reviewer
// 5: planner
// 6: [ Guardar Configuración ]
// 7: [ Volver al Menú Principal ]

func (m model) updateDelegation(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key := msg.String(); key {
	case "ctrl+c", "q":
		m.quitting = true
		return m, tea.Quit
	case "esc", "b", "B":
		m.screen = screenHome
		m.cursor = 1
		return m, homeTick()
	case "up", "k":
		if m.delegationCursor > 0 {
			m.delegationCursor--
		}
	case "down", "j":
		if m.delegationCursor < 7 {
			m.delegationCursor++
		}
	case " ", "tab", "right", "left":
		m = m.toggleDelegationItem(m.delegationCursor)
	case "enter":
		if m.delegationCursor == 6 {
			// Save
			configDir := filepath.Join(m.homeDir, ".config", "opencode")
			if err := delegation.Save(configDir, m.delegationCfg); err != nil {
				m.delegationSavedMsg = styleFail.Render("✖ Error guardando: " + err.Error())
			} else {
				m.delegationSavedMsg = stylePass.Render("✔ Configuración guardada en ~/.config/opencode/cortex-delegation.json")
			}
		} else if m.delegationCursor == 7 {
			m.screen = screenHome
			m.cursor = 1
			return m, homeTick()
		} else {
			m = m.toggleDelegationItem(m.delegationCursor)
		}
	}
	return m, nil
}

func (m model) toggleDelegationItem(cursor int) model {
	m.delegationSavedMsg = ""
	switch cursor {
	case 0:
		m.delegationCfg.UseHerdr = !m.delegationCfg.UseHerdr
		m.delegationCfg.HerdrSettings.AutoSplit = m.delegationCfg.UseHerdr
	case 1:
		m.delegationCfg.DelegationEnabled = !m.delegationCfg.DelegationEnabled
	case 2, 3, 4, 5:
		role := delegationRoles[cursor-2]
		if m.delegationCfg.Roles == nil {
			m.delegationCfg.Roles = make(map[string]delegation.RoleConfig)
		}
		r := m.delegationCfg.Roles[role]
		if !r.Delegate {
			r.Delegate = true
			r.CLI = "agy"
			r.SkipPermissions = true
			if role == "implement" {
				r.Mode = "accept-edits"
			} else {
				r.Mode = "plan"
			}
		} else {
			r.Delegate = false
			r.CLI = "native"
			r.Mode = ""
			r.SkipPermissions = false
		}
		m.delegationCfg.Roles[role] = r
	}
	m.opts.DelegationConfig = &m.delegationCfg
	return m
}

func (m model) viewDelegation() string {
	width := m.contentWidth()
	top := []string{
		truncate(m.header("Configuración de Delegación y Motores Externos"), width),
		"",
		styleSubtitle.Render("Ajuste de supervisión de hojas AGY y multiplexación Herdr:"),
		"",
		styleDim.Render("Define el comportamiento persistente en ~/.config/opencode/cortex-delegation.json"),
		"",
	}

	var content []string

	// 0: Herdr
	herdrStatus := styleDim.Render("[ Inactivo ]")
	if m.delegationCfg.UseHerdr {
		herdrStatus = stylePass.Render("[ Activo ]")
	}
	line0 := fmt.Sprintf("  • %-26s ➔ %s", "Multiplexor Herdr", herdrStatus)
	if m.delegationCursor == 0 {
		line0 = styleSelected.Render(fmt.Sprintf("> • %-26s ➔ %s", "Multiplexor Herdr", herdrStatus))
	}
	content = append(content, truncate(line0, width))

	// 1: Delegation Enabled
	delStatus := styleDim.Render("[ Deshabilitada ]")
	if m.delegationCfg.DelegationEnabled {
		delStatus = stylePass.Render("[ Habilitada ]")
	}
	line1 := fmt.Sprintf("  • %-26s ➔ %s", "Delegación Externa", delStatus)
	if m.delegationCursor == 1 {
		line1 = styleSelected.Render(fmt.Sprintf("> • %-26s ➔ %s", "Delegación Externa", delStatus))
	}
	content = append(content, truncate(line1, width))

	content = append(content, "", styleDim.Render("Motores por rol / subagente:"))

	// 2..5: Roles
	for i, role := range delegationRoles {
		r := m.delegationCfg.Roles[role]
		status := styleDim.Render("[ Nativo OpenCode ]")
		if r.Delegate && r.CLI == "agy" {
			modeText := ""
			if r.Mode != "" {
				modeText = " · " + r.Mode
			}
			status = stylePass.Render(fmt.Sprintf("[ Antigravity CLI (agy)%s ]", modeText))
		}
		line := fmt.Sprintf("  • %-26s ➔ %s", role, status)
		if m.delegationCursor == i+2 {
			line = styleSelected.Render(fmt.Sprintf("> • %-26s ➔ %s", role, status))
		}
		content = append(content, truncate(line, width))
	}

	content = append(content, "")

	if m.delegationSavedMsg != "" {
		content = append(content, "  "+m.delegationSavedMsg, "")
	}

	btnSave := "  [ Guardar Configuración ]"
	if m.delegationCursor == 6 {
		btnSave = styleSelected.Render("> [ Guardar Configuración ]")
	}
	content = append(content, truncate(btnSave, width))

	btnBack := "  [ Volver al Menú Principal ]"
	if m.delegationCursor == 7 {
		btnBack = styleSelected.Render("> [ Volver al Menú Principal ]")
	}
	content = append(content, truncate(btnBack, width))

	var bottom []string
	bottom = append(bottom, m.footer("space/tab alternar · enter guardar/seleccionar · b/esc volver"))
	return strings.Join(clampScreen(top, content, bottom, m.bodyHeight(), 0, "up/down"), "\n")
}
