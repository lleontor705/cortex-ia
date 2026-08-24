package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/lleontor705/cortex-ia/internal/tui/styles"
)

// spinnerFrames animates the Running screen with smooth dot spinners.
var spinnerFrames = styles.SpinnerFrames

var (
	styleTitle    = lipgloss.NewStyle().Bold(true).Foreground(styles.Primary)
	styleSubtitle = lipgloss.NewStyle().Bold(true).Foreground(styles.Secondary)
	styleDim      = lipgloss.NewStyle().Foreground(styles.Muted)
	styleSelected = lipgloss.NewStyle().Bold(true).Foreground(styles.Secondary)
	stylePass     = lipgloss.NewStyle().Bold(true).Foreground(styles.Success)
	styleFail     = lipgloss.NewStyle().Bold(true).Foreground(styles.Error)
	styleWarn     = lipgloss.NewStyle().Bold(true).Foreground(styles.Warning)
	styleConflict = lipgloss.NewStyle().Bold(true).Foreground(styles.Error)
	styleFrame    = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(styles.Primary).Padding(0, 1)
)

var homeDescriptions = []string{
	"Deploy or reconcile skills, agents, commands & MCPs",
	"Inspect and configure managed OpenCode MCP server presets",
	"Assess installation health, digests & recovery journals",
	"Remove accredited cortex-ia installation with backup",
	"Exit cortex-ia",
}

var mcpDescriptions = map[string]string{
	"cortex":    "Persistent Memory & Adaptive Context",
	"forgespec": "SDD Task Board & Spec Coordination",
	"context7":  "Upstash Documentation & Library Context",
}

// contentWidth clamps the layout to the terminal for basic responsiveness.
func (m model) contentWidth() int {
	width := m.width
	if width <= 0 {
		width = 80
	}
	if width > 100 {
		width = 100
	}
	return width - 4 // frame borders and padding
}

func truncate(s string, width int) string {
	if width <= 0 {
		return s
	}
	if lipgloss.Width(s) <= width {
		return s
	}
	runes := []rune(s)
	for len(runes) > 0 && lipgloss.Width(string(runes)) > width-1 {
		runes = runes[:len(runes)-1]
	}
	return string(runes) + "…"
}

// bodyHeight is the row budget for screen content.
func (m model) bodyHeight() int {
	if m.height <= 0 {
		return 0
	}
	h := m.height - 2
	if m.confirm.kind != confirmNone {
		h -= 5
	}
	if h < 1 {
		h = 1
	}
	return h
}

// clampScreen joins top, visible content and bottom within budget rows.
func clampScreen(top, content, bottom []string, budget, offset int, scrollKeys string) []string {
	if budget <= 0 {
		return append(append(append([]string{}, top...), content...), bottom...)
	}
	for len(top)+len(bottom) >= budget && len(top) > 1 {
		top = top[:len(top)-1]
	}
	contentBudget := budget - len(top) - len(bottom)
	if contentBudget < 0 {
		contentBudget = 0
	}
	if offset < 0 {
		offset = 0
	}
	if offset >= len(content) {
		offset = len(content)
		if offset > 0 {
			offset--
		}
	}
	visible := content[offset:]
	needsIndicator := offset > 0 || len(visible) > contentBudget
	if needsIndicator && contentBudget > 0 {
		keep := contentBudget - 1
		if keep < 0 {
			keep = 0
		}
		if len(visible) > keep {
			visible = visible[:keep]
		}
	} else if len(visible) > contentBudget {
		visible = visible[:contentBudget]
	}
	hidden := len(content) - offset - len(visible)
	lines := append(append([]string{}, top...), visible...)
	switch {
	case hidden > 0 && offset > 0:
		lines = append(lines, styleDim.Render(fmt.Sprintf("↑ %d above · +%d more (%s scroll)", offset, hidden, scrollKeys)))
	case hidden > 0:
		lines = append(lines, styleDim.Render(fmt.Sprintf("+%d more (%s scroll)", hidden, scrollKeys)))
	case offset > 0:
		lines = append(lines, styleDim.Render(fmt.Sprintf("↑ %d above (%s scroll)", offset, scrollKeys)))
	}
	return append(lines, bottom...)
}

func (m model) View() string {
	var body string
	switch m.screen {
	case screenHome:
		body = m.viewHome()
	case screenWizardHerdr:
		body = m.viewWizardHerdr()
	case screenWizardDelegation:
		body = m.viewWizardDelegation()
	case screenWizardRoles:
		body = m.viewWizardRoles()
	case screenReview:
		body = m.viewReview()
	case screenRunning:
		body = m.viewRunning()
	case screenResult:
		body = m.viewResult()
	case screenMCP:
		body = m.viewMCP()
	}
	if m.confirm.kind != confirmNone {
		body = body + "\n" + m.viewConfirm()
	}
	return styleFrame.Render(body)
}

func (m model) header(name string) string {
	return styleTitle.Render("cortex-ia "+m.version) + styleDim.Render(" · "+name+" · "+m.homeDir)
}

func (m model) footer(keys string) string {
	return styleDim.Render(keys)
}

func (m model) viewHome() string {
	width := m.contentWidth()
	var lines []string

	if m.height <= 0 || m.height >= 16 {
		logoStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.Secondary)
		lines = append(lines, logoStyle.Render(styles.Logo))
		lines = append(lines, styleDim.Render("  OpenCode Edition · "+m.version+" · "+m.homeDir), "")
	} else {
		lines = append(lines, truncate(m.header("Home"), width), "")
	}

	for i, entry := range homeEntries {
		prefix := fmt.Sprintf("  [%d] ", i+1)
		text := entry
		desc := " · " + styleDim.Render(homeDescriptions[i])
		if i == m.cursor {
			prefix = fmt.Sprintf("> [%d] ", i+1)
			text = styleSelected.Render(entry)
		}
		lines = append(lines, truncate(prefix+text+desc, width))
	}
	lines = append(lines, "", m.footer("↑/↓ move · 1-5/enter select · q quit"))
	return strings.Join(lines, "\n")
}

func (m model) viewReview() string {
	width := m.contentWidth()
	top := []string{
		truncate(m.header("Review — "+m.installModeTitle()), width),
		"",
	}
	if m.replanning {
		top = append(top, styleSubtitle.Render("⚡ planning…"))
	} else if m.planErr != nil {
		top = append(top, styleFail.Render("✖ plan error: "+m.planErr.Error()))
	}
	top = append(top, "MCP selection (space toggles, replans, d delegation):")
	states := []bool{m.opts.Cortex, m.opts.ForgeSpec, m.opts.Context7}
	for i, name := range managedNames {
		mark := " "
		if states[i] {
			mark = "x"
		}
		prefix := "  "
		text := fmt.Sprintf("[%s] %s", mark, name)
		desc := ""
		if d, ok := mcpDescriptions[name]; ok {
			desc = " · " + styleDim.Render(d)
		}
		if i == m.mcpCursor {
			prefix = "> "
			text = styleSelected.Render(text)
		}
		top = append(top, truncate(prefix+text+desc, width))
	}
	top = append(top, "")

	var content []string
	if m.plan != nil {
		content = m.planSummary(width)
	}
	var bottom []string
	if m.plan != nil && len(m.plan.Conflicts) > 0 {
		hint := "[o] authorize overwrite (destructive, needs confirmed backup)"
		if m.overwrite {
			hint = styleWarn.Render("overwrite authorized — enter asks for explicit confirmation")
		} else {
			hint = styleWarn.Render(hint)
		}
		bottom = append(bottom, hint)
	} else if m.plan != nil {
		bottom = append(bottom, styleDim.Render("no blocking conflicts"))
	}
	bottom = append(bottom, m.footer("enter run · b back to wizard · o overwrite · pgup/pgdn scroll · esc home"))
	return strings.Join(clampScreen(top, content, bottom, m.bodyHeight(), m.reviewScroll, "pgup/pgdn"), "\n")
}

// planSummary renders the plan effects and conflicts read-only.
func (m model) planSummary(width int) []string {
	lines := []string{fmt.Sprintf("Effects (%d, plan %s):", len(m.plan.Effects), shortDigest(m.plan.Digest))}
	noops := 0
	listed := 0
	for _, effect := range m.plan.Effects {
		switch string(effect.Kind) {
		case "noop", "mcp-noop":
			noops++
			continue
		}
		if listed >= 12 {
			continue
		}
		listed++
		effectTag := stylePass.Render(fmt.Sprintf("  + %s", effect.Kind))
		if effect.Kind == "safe-merge" {
			effectTag = styleSubtitle.Render(fmt.Sprintf("  ⚡ %s", effect.Kind))
		}
		lines = append(lines, truncate(fmt.Sprintf("%s %s", effectTag, effect.Dest), width))
	}
	if noops > 0 {
		lines = append(lines, fmt.Sprintf("  … plus %d already converged", noops))
	}
	if len(m.plan.Conflicts) > 0 {
		lines = append(lines, fmt.Sprintf("Conflicts (%d):", len(m.plan.Conflicts)))
		for _, conflict := range m.plan.Conflicts {
			suffix := ""
			if !conflict.OverwriteAuthorized {
				suffix = styleConflict.Render("  [overwrite cannot clear this]")
			}
			lines = append(lines, truncate(fmt.Sprintf("  %s: %s (%s)", conflict.Target, conflict.Kind, conflict.Reason), width)+suffix)
		}
	}
	return lines
}

func shortDigest(digest string) string {
	if len(digest) > 12 {
		return digest[:12]
	}
	return digest
}

func (m model) viewRunning() string {
	width := m.contentWidth()
	lines := []string{
		truncate(m.header("Running — "+m.running.title), width),
		"",
	}
	for i, phase := range m.running.phases {
		switch {
		case i < m.running.current:
			lines = append(lines, stylePass.Render(fmt.Sprintf("  ✓ %s", phase)))
		case i == m.running.current:
			spinner := styles.SpinnerChar(m.running.spinner)
			lines = append(lines, fmt.Sprintf("  %s %s", styleSubtitle.Render(spinner), styleSelected.Render(phase)))
		default:
			lines = append(lines, styleDim.Render(fmt.Sprintf("  · %s", phase)))
		}
	}
	lines = append(lines, "", m.footer("running… ctrl+c aborts"))
	return strings.Join(lines, "\n")
}

func (m model) viewResult() string {
	width := m.contentWidth()
	verdict := styleFail.Render("FAIL")
	if m.result.pass {
		verdict = stylePass.Render("PASS")
	}
	top := []string{
		truncate(m.header("Result — "+m.result.title), width),
		"",
		verdict,
		fmt.Sprintf("Changed: %d", m.result.changed),
	}
	if m.result.backupID != "" {
		top = append(top, fmt.Sprintf("Backup: %s", m.result.backupID))
	}
	if m.result.canRollback {
		top = append(top, styleWarn.Render("Rollback: "+m.result.rollbackCmd))
	}
	content := make([]string, 0, len(m.result.detail))
	for _, detail := range m.result.detail {
		content = append(content, truncate("  "+detail, width))
	}
	keys := "enter/m home · ↑/↓ details · q quit"
	if m.result.canRollback {
		keys = "enter/m home · ↑/↓ details · r rollback (confirm) · q quit"
	}
	bottom := []string{m.footer(keys)}
	return strings.Join(clampScreen(top, content, bottom, m.bodyHeight(), m.resultScroll, "↑/↓"), "\n")
}

func (m model) viewMCP() string {
	width := m.contentWidth()
	lines := []string{
		truncate(m.header("MCP Manager"), width),
		"",
	}
	if m.mcpErr != nil {
		lines = append(lines, styleFail.Render("list error: "+m.mcpErr.Error()), "")
	}
	if m.mcpReport == nil {
		lines = append(lines, styleDim.Render("loading…"))
	} else {
		if m.mcpReport.ConfigPath != "" {
			lines = append(lines, styleDim.Render(truncate("config: "+m.mcpReport.ConfigPath, width)), "")
		}
		if len(m.mcpReport.Entries) == 0 {
			lines = append(lines, styleDim.Render("no managed entries"))
		}
		for i, entry := range m.mcpReport.Entries {
			prefix := "  "
			text := fmt.Sprintf("%-12s %s", entry.Name, entry.Status)
			switch string(entry.Status) {
			case "managed":
				text = fmt.Sprintf("%-12s %s", entry.Name, stylePass.Render(string(entry.Status)))
			case "conflict", "unmanaged-equivalent":
				text = fmt.Sprintf("%-12s %s", entry.Name, styleConflict.Render(string(entry.Status)))
			}
			if d, ok := mcpDescriptions[entry.Name]; ok {
				text = text + "  " + styleDim.Render("("+d+")")
			}
			if i == m.cursor {
				prefix = "> "
				text = styleSelected.Render(text)
			}
			lines = append(lines, truncate(prefix+text, width))
		}
		if len(m.mcpReport.Unknown) > 0 {
			lines = append(lines, "", styleDim.Render("unknown (never touched): "+strings.Join(m.mcpReport.Unknown, ", ")))
		}
		if !m.mcpReport.Installed {
			lines = append(lines, styleDim.Render("no accredited v2 installation: managed adds fail closed until install runs"))
		}
	}
	lines = append(lines, "", m.footer("↑/↓ move · space/enter add|remove · r refresh · esc home"))
	return strings.Join(lines, "\n")
}

// viewConfirm renders the explicit confirmation overlay for destructive actions.
func (m model) viewConfirm() string {
	width := m.contentWidth()
	var title, prompt string
	switch m.confirm.kind {
	case confirmOverwrite:
		title = "Confirm overwrite"
		count := 0
		digest := ""
		if m.plan != nil {
			count = len(m.plan.Conflicts)
			digest = shortDigest(m.plan.Digest)
		}
		prompt = fmt.Sprintf("Overwrite replaces %d conflicting file(s) after a verified backup (plan %s).", count, digest)
	case confirmUninstall:
		title = "Confirm uninstall"
		prompt = "Uninstall removes cortex-ia-managed files and MCP entries from this home. A verified backup is captured first."
	case confirmMCPRemove:
		title = "Confirm MCP remove"
		prompt = fmt.Sprintf("Remove MCP entry %q. User-owned entries fail closed and nothing is mutated.", m.confirm.arg)
	case confirmRollback:
		title = "Confirm rollback"
		prompt = "Rollback restores the recorded backup, undoing managed changes made after it."
	default:
		return ""
	}
	lines := []string{
		styleWarn.Render("⚠ " + title),
		truncate(prompt, width),
		styleDim.Render("[y] yes, proceed   [n]/esc no, cancel"),
	}
	return styleFrame.BorderForeground(styles.Warning).Render(strings.Join(lines, "\n"))
}
