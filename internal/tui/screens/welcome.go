package screens

import (
	"fmt"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/tui/styles"
)

// WelcomeData holds the data needed to render the welcome screen.
//
// Two modes are supported, in priority order:
//
//   - Groups (preferred): rendered with RenderMenu, supports hotkeys + hints.
//   - Options (legacy):   plain string list rendered with RenderOptions.
//
// New callers should populate Groups; Options stays for backwards compatibility
// with anything that hasn't migrated yet (e.g. older tests).
type WelcomeData struct {
	Version string
	Groups  []MenuGroup
	Options []string
	Cursor  int
}

// RenderWelcome renders the welcome screen as a grouped menu hub.
func RenderWelcome(data WelcomeData) string {
	var sb strings.Builder

	sb.WriteString(styles.Title.Render(styles.Logo))
	sb.WriteString("\n\n")
	sb.WriteString(styles.Subtitle.Render(fmt.Sprintf("v%s — AI Agent Ecosystem Configurator", data.Version)))
	sb.WriteString("\n\n")

	sb.WriteString("Configure your AI coding agents with:\n")
	sb.WriteString(styles.StatusOK.Render("  ● Cortex") + "       — Persistent memory + knowledge graph\n")
	sb.WriteString(styles.StatusOK.Render("  ● ForgeSpec") + "    — SDD contracts + task board\n")
	sb.WriteString(styles.StatusOK.Render("  ● Mailbox") + "      — Inter-agent messaging\n")
	sb.WriteString(styles.StatusOK.Render("  ● Context7") + "     — Live documentation\n")
	sb.WriteString(styles.StatusOK.Render("  ● SDD") + "          — 9-phase development workflow\n")
	sb.WriteString(styles.StatusOK.Render("  ● GGA") + "          — AI-powered pre-commit code review\n")
	sb.WriteString("\n")

	sb.WriteString(styles.Subtitle.Render("What would you like to do?"))
	sb.WriteString("\n\n")

	if len(data.Groups) > 0 {
		sb.WriteString(RenderMenu(data.Groups, data.Cursor))
	} else {
		sb.WriteString(RenderOptions(data.Options, data.Cursor))
	}

	sb.WriteString("\n")
	sb.WriteString(RenderHelpBar(
		"↑↓ navigate",
		"enter select",
		"1-9 jump",
		"q quit",
	))

	return sb.String()
}