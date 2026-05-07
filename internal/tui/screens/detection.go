package screens

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/lleontor705/cortex-ia/internal/tui/styles"
)

// DetectionData holds the data needed to render the system detection screen.
type DetectionData struct {
	OS, Arch, PkgMgr, Shell string
	NodeVer, GitVer, GoVer  string
	Npx, Cortex             bool
	DetectedAgents          int
	SupportedAgents         int // total supported (12 in v0.3+); 0 hides the "/N" suffix
}

// RenderDetection renders the system detection screen as two side-by-side
// panels: Platform info on the left, runtime tooling on the right. A footer
// line summarises agent detection so users see scope at a glance.
func RenderDetection(data DetectionData) string {
	var sb strings.Builder

	sb.WriteString(styles.Title.Render("System Detection"))
	sb.WriteString("\n\n")

	platform := renderPlatformPanel(data)
	tools := renderToolsPanel(data)
	sb.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, platform, "  ", tools))
	sb.WriteString("\n\n")

	count := fmt.Sprintf("%d agent(s) detected", data.DetectedAgents)
	if data.SupportedAgents > 0 {
		count = fmt.Sprintf("%d / %d supported agents detected", data.DetectedAgents, data.SupportedAgents)
	}
	icon := styles.StatusOK.Render("●")
	if data.DetectedAgents == 0 {
		icon = styles.StatusWarn.Render("○")
	}
	fmt.Fprintf(&sb, "%s %s\n", icon, count)

	sb.WriteString("\n")
	sb.WriteString(RenderHelpBar(
		"enter continue",
		"esc back",
	))

	return sb.String()
}

func renderPlatformPanel(data DetectionData) string {
	var sb strings.Builder
	sb.WriteString(styles.Subtitle.Render("Platform"))
	sb.WriteString("\n\n")
	fmt.Fprintf(&sb, "  %-9s %s/%s\n", "OS:", data.OS, data.Arch)
	fmt.Fprintf(&sb, "  %-9s %s\n", "Pkg Mgr:", valueOrDash(data.PkgMgr))
	fmt.Fprintf(&sb, "  %-9s %s\n", "Shell:", valueOrDash(data.Shell))
	return styles.Panel.Render(sb.String())
}

func renderToolsPanel(data DetectionData) string {
	var sb strings.Builder
	sb.WriteString(styles.Subtitle.Render("Runtime"))
	sb.WriteString("\n\n")

	row := func(name, ver string) {
		if ver != "" {
			fmt.Fprintf(&sb, "  %s %-10s %s\n", styles.StatusOK.Render("●"), name, ver)
		} else {
			fmt.Fprintf(&sb, "  %s %-10s %s\n", styles.StatusFail.Render("○"), name, styles.Description.Render("not found"))
		}
	}

	row("Node.js", data.NodeVer)
	row("npx", boolToVer(data.Npx))
	row("Git", data.GitVer)
	row("Go", data.GoVer)
	row("Cortex", boolToVer(data.Cortex))
	return styles.Panel.Render(sb.String())
}

func valueOrDash(s string) string {
	if s == "" {
		return styles.Description.Render("—")
	}
	return s
}

func boolToVer(b bool) string {
	if b {
		return "available"
	}
	return ""
}
