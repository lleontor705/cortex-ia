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
// 2: Modelo AGY Predeterminado (DefaultModel)
// 3: implement
// 4: investigate
// 5: reviewer
// 6: planner
// 7: [ Guardar Configuración ]
// 8: [ Volver al Menú Principal ]

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
		if m.delegationCursor < 8 {
			m.delegationCursor++
		}
	case "left":
		m = m.cycleDelegationModel(false)
	case "right":
		m = m.cycleDelegationModel(true)
	case " ", "tab":
		m = m.toggleDelegationItem(m.delegationCursor)
	case "enter":
		switch m.delegationCursor {
		case 7:
			// Save
			configDir := filepath.Join(m.homeDir, ".config", "opencode")
			if err := delegation.Save(configDir, m.delegationCfg); err != nil {
				m.delegationSavedMsg = styleFail.Render("✖ Error guardando: " + err.Error())
			} else {
				m.delegationSavedMsg = stylePass.Render("✔ Configuración guardada en ~/.config/opencode/cortex-delegation.json")
			}
		case 8:
			m.screen = screenHome
			m.cursor = 1
			return m, homeTick()
		default:
			m = m.toggleDelegationItem(m.delegationCursor)
		}
	}
	return m, nil
}

func (m model) cycleDelegationModel(forward bool) model {
	m.delegationSavedMsg = ""
	models := m.availableModels
	if len(models) == 0 {
		models = delegation.KnownAGYModels
	}
	switch m.delegationCursor {
	case 2:
		oldDefault := m.delegationCfg.DefaultModel
		if oldDefault == "" {
			oldDefault = delegation.DefaultAGYModel
		}
		if forward {
			m.delegationCfg.DefaultModel = delegation.NextModel(oldDefault, models)
		} else {
			m.delegationCfg.DefaultModel = delegation.PrevModel(oldDefault, models)
		}
		for role, r := range m.delegationCfg.Roles {
			if r.Delegate && r.CLI == "agy" && (r.Model == "" || r.Model == oldDefault) {
				r.Model = m.delegationCfg.DefaultModel
				m.delegationCfg.Roles[role] = r
			}
		}
	case 3, 4, 5, 6:
		role := delegationRoles[m.delegationCursor-3]
		if m.delegationCfg.Roles == nil {
			m.delegationCfg.Roles = make(map[string]delegation.RoleConfig)
		}
		r := m.delegationCfg.Roles[role]
		if r.Delegate && r.CLI == "agy" {
			currentModel := r.Model
			if currentModel == "" {
				currentModel = m.delegationCfg.DefaultModel
			}
			if currentModel == "" {
				currentModel = delegation.DefaultAGYModel
			}
			if forward {
				r.Model = delegation.NextModel(currentModel, models)
			} else {
				r.Model = delegation.PrevModel(currentModel, models)
			}
			m.delegationCfg.Roles[role] = r
		}
	case 0, 1:
		m = m.toggleDelegationItem(m.delegationCursor)
	}
	m.opts.DelegationConfig = &m.delegationCfg
	return m
}

func (m model) toggleDelegationItem(cursor int) model {
	m.delegationSavedMsg = ""
	models := m.availableModels
	if len(models) == 0 {
		models = delegation.KnownAGYModels
	}
	switch cursor {
	case 0:
		m.delegationCfg.UseHerdr = !m.delegationCfg.UseHerdr
		m.delegationCfg.HerdrSettings.AutoSplit = m.delegationCfg.UseHerdr
	case 1:
		m.delegationCfg.DelegationEnabled = !m.delegationCfg.DelegationEnabled
	case 2:
		oldDefault := m.delegationCfg.DefaultModel
		if oldDefault == "" {
			oldDefault = delegation.DefaultAGYModel
		}
		m.delegationCfg.DefaultModel = delegation.NextModel(oldDefault, models)
		for role, r := range m.delegationCfg.Roles {
			if r.Delegate && r.CLI == "agy" && (r.Model == "" || r.Model == oldDefault) {
				r.Model = m.delegationCfg.DefaultModel
				m.delegationCfg.Roles[role] = r
			}
		}
	case 3, 4, 5, 6:
		role := delegationRoles[cursor-3]
		if m.delegationCfg.Roles == nil {
			m.delegationCfg.Roles = make(map[string]delegation.RoleConfig)
		}
		r := m.delegationCfg.Roles[role]
		if !r.Delegate {
			r.Delegate = true
			r.CLI = "agy"
			r.SkipPermissions = true
			if r.Model == "" {
				r.Model = m.delegationCfg.DefaultModel
				if r.Model == "" {
					r.Model = delegation.DefaultAGYModel
				}
			}
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

	cursorLine := 0
	var content []string

	// 0: Herdr
	herdrStatus := styleDim.Render("[ Inactivo ]")
	if m.delegationCfg.UseHerdr {
		herdrStatus = stylePass.Render("[ Activo ]")
	}
	line0 := fmt.Sprintf("  • %-26s ➔ %s", "Multiplexor Herdr", herdrStatus)
	if m.delegationCursor == 0 {
		cursorLine = len(content)
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
		cursorLine = len(content)
		line1 = styleSelected.Render(fmt.Sprintf("> • %-26s ➔ %s", "Delegación Externa", delStatus))
	}
	content = append(content, truncate(line1, width))

	// 2: Default AGY Model
	defModel := m.delegationCfg.DefaultModel
	if defModel == "" {
		defModel = delegation.DefaultAGYModel
	}
	dispName := delegation.ModelDisplayName(defModel, m.availableModels)
	modelStatus := stylePass.Render(fmt.Sprintf("[ %s ]", defModel))
	if dispName != defModel {
		modelStatus = stylePass.Render(fmt.Sprintf("[ %s (%s) ]", defModel, dispName))
	}
	line2 := fmt.Sprintf("  • %-26s ➔ %s", "Modelo AGY Predeterminado", modelStatus)
	if m.delegationCursor == 2 {
		cursorLine = len(content)
		line2 = styleSelected.Render(fmt.Sprintf("> • %-26s ➔ %s", "Modelo AGY Predeterminado", modelStatus))
	}
	content = append(content, truncate(line2, width))

	content = append(content, "", styleDim.Render("Motores por rol / subagente:"))

	// 3..6: Roles
	for i, role := range delegationRoles {
		r := m.delegationCfg.Roles[role]
		status := styleDim.Render("[ Nativo OpenCode ]")
		if r.Delegate && r.CLI == "agy" {
			modeText := ""
			if r.Mode != "" {
				modeText = " · " + r.Mode
			}
			activeModel := r.Model
			if activeModel == "" {
				activeModel = m.delegationCfg.DefaultModel
			}
			if activeModel == "" {
				activeModel = delegation.DefaultAGYModel
			}
			status = stylePass.Render(fmt.Sprintf("[ Antigravity CLI (agy)%s ] · Modelo: %s", modeText, activeModel))
		}
		line := fmt.Sprintf("  • %-26s ➔ %s", role, status)
		if m.delegationCursor == i+3 {
			cursorLine = len(content)
			line = styleSelected.Render(fmt.Sprintf("> • %-26s ➔ %s", role, status))
		}
		content = append(content, truncate(line, width))
	}

	content = append(content, "")

	if m.delegationSavedMsg != "" {
		content = append(content, "  "+m.delegationSavedMsg, "")
	}

	btnSave := "  [ Guardar Configuración ]"
	if m.delegationCursor == 7 {
		cursorLine = len(content)
		btnSave = styleSelected.Render("> [ Guardar Configuración ]")
	}
	content = append(content, truncate(btnSave, width))

	btnBack := "  [ Volver al Menú Principal ]"
	if m.delegationCursor == 8 {
		cursorLine = len(content)
		btnBack = styleSelected.Render("> [ Volver al Menú Principal ]")
	}
	content = append(content, truncate(btnBack, width))

	var bottom []string
	bottom = append(bottom, m.footer("space/tab alternar · ←/→ cambiar modelo · enter guardar/seleccionar · b/esc volver"))
	offset := cursorOffset(cursorLine, len(content), m.bodyHeight(), len(top), len(bottom))
	return strings.Join(clampScreen(top, content, bottom, m.bodyHeight(), offset, "up/down"), "\n")
}
