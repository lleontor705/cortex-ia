package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lleontor705/cortex-ia/v2/internal/targets"
)

// hostRow is one checklist entry on the host picker. Detection and install
// state are probed once when the picker opens, never per rendered frame.
type hostRow struct {
	id        targets.TargetID
	detected  bool
	installed bool
}

// hostPicker is the Install/Uninstall host checklist.
type hostPicker struct {
	uninstall bool
	cursor    int
	rows      []hostRow
	checked   map[targets.TargetID]bool
	status    string
}

func (r hostRow) describe() string {
	bin := hostBinary(r.id)
	parts := []string{bin + " not found"}
	if r.detected {
		parts[0] = bin + " found"
	}
	if r.installed {
		parts = append(parts, "installed")
	}
	return strings.Join(parts, " · ")
}

// newHostPicker probes every host and derives the default selection: Install
// pre-checks the detected hosts (OpenCode alone when none is detected);
// Uninstall pre-checks OpenCode and every non-OpenCode host already installed.
func newHostPicker(hosts TargetAPI, uninstall bool) hostPicker {
	p := hostPicker{uninstall: uninstall, checked: map[targets.TargetID]bool{}}
	anyDetected := false
	for _, id := range targets.AllTargets() {
		row := hostRow{id: id, detected: hosts.Detected(id)}
		if id != targets.TargetOpenCode {
			row.installed = hosts.Installed(id)
		}
		anyDetected = anyDetected || row.detected
		p.rows = append(p.rows, row)
	}
	for _, row := range p.rows {
		switch {
		case uninstall:
			p.checked[row.id] = row.id == targets.TargetOpenCode || row.installed
		case anyDetected:
			p.checked[row.id] = row.detected
		default:
			p.checked[row.id] = row.id == targets.TargetOpenCode
		}
	}
	return p
}

// selected returns the checked hosts in AllTargets order.
func (p hostPicker) selected() []targets.TargetID {
	var ids []targets.TargetID
	for _, row := range p.rows {
		if p.checked[row.id] {
			ids = append(ids, row.id)
		}
	}
	return ids
}

func (m model) openHostPicker(uninstall bool) (tea.Model, tea.Cmd) {
	m.screen = screenHostPicker
	m.hostPick = newHostPicker(m.hosts, uninstall)
	m.hostRun = hostRun{}
	return m, nil
}

func (m model) updateHostPicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key := msg.String(); key {
	case "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "esc", "b", "B":
		m.screen = screenHome
		if m.hostPick.uninstall {
			m.cursor = uninstallEntryIndex
		} else {
			m.cursor = 0
		}
		m.hostRun = hostRun{}
		return m, homeTick()
	case "up", "k":
		if m.hostPick.cursor > 0 {
			m.hostPick.cursor--
		}
	case "down", "j":
		if m.hostPick.cursor < len(m.hostPick.rows)-1 {
			m.hostPick.cursor++
		}
	case " ":
		id := m.hostPick.rows[m.hostPick.cursor].id
		m.hostPick.checked[id] = !m.hostPick.checked[id]
		m.hostPick.status = ""
	case "enter":
		return m.confirmHostPicker()
	}
	return m, nil
}

// confirmHostPicker routes the chosen hosts. OpenCode keeps its existing
// Review or uninstall confirmation; the other hosts run after it or directly.
func (m model) confirmHostPicker() (tea.Model, tea.Cmd) {
	selected := m.hostPick.selected()
	if len(selected) == 0 {
		m.hostPick.status = "select at least one host (space toggles)"
		return m, nil
	}
	var extras []targets.TargetID
	openCode := false
	for _, id := range selected {
		if id == targets.TargetOpenCode {
			openCode = true
			continue
		}
		extras = append(extras, id)
	}
	m.hostRun = hostRun{
		active:           len(extras) > 0,
		uninstall:        m.hostPick.uninstall,
		openCodeSelected: openCode,
		title:            m.hostModeTitle(),
		pending:          extras,
	}
	switch {
	case m.hostPick.uninstall && openCode:
		m.confirm = confirmState{kind: confirmUninstall}
		return m, nil
	case m.hostPick.uninstall:
		m.confirm = confirmState{kind: confirmHostUninstall}
		return m, nil
	case openCode:
		return m.openInstallReview()
	}
	return m.startHostStep()
}

func (m model) hostModeTitle() string {
	if m.hostPick.uninstall {
		return "Uninstall"
	}
	return "Install"
}

func (m model) viewHostPicker() string {
	width := m.contentWidth()
	action := "Install"
	if m.hostPick.uninstall {
		action = "Uninstall"
	}
	lines := []string{
		truncate(m.header(action+" — choose hosts"), width),
		"",
		styleSection.Render("01 / Hosts (space toggles, enter confirms):"),
	}
	for i, row := range m.hostPick.rows {
		mark := " "
		if m.hostPick.checked[row.id] {
			mark = "x"
		}
		prefix := "  "
		text := fmt.Sprintf("[%s] %s", mark, targets.TargetLabel(row.id))
		if i == m.hostPick.cursor {
			prefix = "> "
			text = styleRow.Render(text)
		}
		desc := " · " + styleDim.Render(row.describe())
		lines = append(lines, truncate(prefix+text+desc, width))
	}
	lines = append(lines, "")
	if m.hostPick.status != "" {
		lines = append(lines, styleWarn.Render(m.hostPick.status))
	}
	lines = append(lines, m.footer("↑/↓ move · space toggle · enter confirm · esc home"))
	return strings.Join(lines, "\n")
}
