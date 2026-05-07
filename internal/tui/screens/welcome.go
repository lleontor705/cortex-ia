package screens

import (
	"fmt"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/tui/styles"
)

// WelcomeData holds the data needed to render the welcome screen.
type WelcomeData struct {
	Version  string
	Options  []string
	Cursor   int
	FirstRun bool
}

// RenderWelcome renders the welcome screen as a menu hub.
func RenderWelcome(data WelcomeData) string {
	var sb strings.Builder

	sb.WriteString(styles.Title.Render(styles.Logo))
	sb.WriteString("\n")
	sb.WriteString(styles.Subtitle.Render(fmt.Sprintf("v%s", data.Version)))
	sb.WriteString(styles.Description.Render(" — AI Agent Ecosystem Configurator"))
	sb.WriteString("\n")

	// Compact feature list in one line
	features := []string{"Cortex", "ForgeSpec", "Mailbox", "Context7", "SDD", "GGA"}
	sb.WriteString(styles.Description.Render("Components: "))
	for i, f := range features {
		if i > 0 {
			sb.WriteString(styles.Description.Render(" · "))
		}
		sb.WriteString(styles.StatusOK.Render(f))
	}
	sb.WriteString("\n\n")

	if data.FirstRun {
		sb.WriteString(styles.Subtitle.Render("  Welcome! Select \"Install ecosystem\" to get started."))
		sb.WriteString("\n\n")
	}

	sb.WriteString(RenderOptions(data.Options, data.Cursor))

	return sb.String()
}
