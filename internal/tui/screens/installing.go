package screens

import (
	"fmt"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/tui/styles"
)

// RenderInstalling renders the installation progress screen.
func RenderInstalling(progress InstallProgress, spinnerFrame int) string {
	var sb strings.Builder

	spinner := styles.SpinnerChar(spinnerFrame)

	if progress.Done {
		if progress.Failed {
			sb.WriteString(styles.StatusFail.Render("Installation completed with errors"))
		} else {
			sb.WriteString(styles.StatusOK.Render("✓ Installation complete"))
		}
	} else {
		sb.WriteString(styles.Title.Render(fmt.Sprintf("%s Installing...", spinner)))
	}
	sb.WriteString("\n\n")

	// Progress bar
	percent := progress.Percent
	filled := percent / 5
	empty := 20 - filled
	bar := styles.ProgressFilled.Render(strings.Repeat("█", filled)) +
		styles.ProgressEmpty.Render(strings.Repeat("░", empty))
	sb.WriteString(fmt.Sprintf("  %s %s\n\n", bar, styles.Percent.Render(fmt.Sprintf("%d%%", percent))))

	// Step list
	for _, item := range progress.Items {
		var icon string
		switch item.Status {
		case "succeeded":
			icon = styles.StatusOK.Render("✓")
		case "failed":
			icon = styles.StatusFail.Render("✗")
		case "running":
			icon = styles.StatusWarn.Render(spinner)
		default:
			icon = styles.Description.Render("○")
		}
		fmt.Fprintf(&sb, "  %s %s\n", icon, item.Label)
	}

	// Logs (show last 5)
	if len(progress.Logs) > 0 {
		sb.WriteString("\n")
		start := 0
		if len(progress.Logs) > 5 {
			start = len(progress.Logs) - 5
		}
		for _, log := range progress.Logs[start:] {
			sb.WriteString(styles.Description.Render("  " + log + "\n"))
		}
	}

	if !progress.Done {
		sb.WriteString(styles.Help.Render("\nPlease wait..."))
	}

	return sb.String()
}
