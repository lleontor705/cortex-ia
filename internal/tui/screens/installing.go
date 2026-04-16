package screens

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/lleontor705/cortex-ia/internal/tui/styles"
)

// RenderInstalling renders the installation progress screen.
func RenderInstalling(prog InstallProgress, spinnerView string, progressBar progress.Model) string {
	var sb strings.Builder

	if prog.Done {
		if prog.Failed {
			sb.WriteString(styles.StatusFail.Render("Installation completed with errors"))
		} else {
			sb.WriteString(styles.StatusOK.Render("✓ Installation complete"))
		}
	} else {
		sb.WriteString(styles.Title.Render(fmt.Sprintf("%s Installing...", spinnerView)))
	}
	sb.WriteString("\n\n")

	// Progress bar using bubbles progress component
	pct := float64(prog.Percent) / 100.0
	fmt.Fprintf(&sb, "  %s %s\n\n", progressBar.ViewAs(pct), styles.Percent.Render(fmt.Sprintf("%d%%", prog.Percent)))

	// Step list
	for _, item := range prog.Items {
		var icon string
		switch item.Status {
		case "succeeded":
			icon = styles.StatusOK.Render("✓")
		case "failed":
			icon = styles.StatusFail.Render("✗")
		case "running":
			icon = spinnerView
		default:
			icon = styles.Description.Render("○")
		}
		fmt.Fprintf(&sb, "  %s %s\n", icon, item.Label)
	}

	// Logs with overflow indicator
	if len(prog.Logs) > 0 {
		sb.WriteString("\n")
		maxLogs := 5
		total := len(prog.Logs)
		start := 0
		if total > maxLogs {
			start = total - maxLogs
			sb.WriteString(styles.Description.Render(fmt.Sprintf("  ... %d more log lines above\n", start)))
		}
		for _, log := range prog.Logs[start:] {
			sb.WriteString(styles.Description.Render("  " + log + "\n"))
		}
	}

	return sb.String()
}
