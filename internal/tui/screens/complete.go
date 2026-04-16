package screens

import (
	"fmt"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/tui/styles"
)

// CompleteData holds the data needed to render the completion screen.
type CompleteData struct {
	Err            error
	ComponentsDone int
	FilesChanged   int
	BackupID       string
	Errors         []string
}

// RenderComplete renders the installation completion screen.
func RenderComplete(data CompleteData) string {
	var sb strings.Builder

	if data.Err != nil {
		sb.WriteString(styles.StatusFail.Render("Installation Failed"))
		sb.WriteString("\n\n")
		fmt.Fprintf(&sb, "Error: %v\n", data.Err)
	} else {
		sb.WriteString(styles.StatusOK.Render("✓ Installation Complete"))
		sb.WriteString("\n\n")
		fmt.Fprintf(&sb, "Components: %d\n", data.ComponentsDone)
		fmt.Fprintf(&sb, "Files changed: %d\n", data.FilesChanged)
		if data.BackupID != "" {
			fmt.Fprintf(&sb, "Backup: %s\n", data.BackupID)
		}
	}

	if len(data.Errors) > 0 {
		sb.WriteString(styles.StatusWarn.Render("\nWarnings:"))
		sb.WriteString("\n")
		for _, e := range data.Errors {
			fmt.Fprintf(&sb, "  %s\n", e)
		}
	}

	// Next steps guidance
	sb.WriteString("\n")
	sb.WriteString(styles.Subtitle.Render("Next steps:"))
	sb.WriteString("\n")
	if data.Err != nil {
		sb.WriteString("  1. Check the errors above and fix any issues\n")
		if data.BackupID != "" {
			sb.WriteString("  2. Press " + styles.Subtitle.Render("u") + " to undo and restore previous state\n")
		}
		sb.WriteString("  3. Return to menu and re-run installation\n")
	} else {
		sb.WriteString("  1. Open a new terminal for changes to take effect\n")
		sb.WriteString("  2. Run " + styles.Subtitle.Render("cortex-ia verify") + " to validate the installation\n")
		sb.WriteString("  3. Config files are in " + styles.Description.Render("~/.cortex-ia/") + "\n")
		if data.BackupID != "" {
			sb.WriteString("  4. Press " + styles.Subtitle.Render("u") + " to undo if something went wrong\n")
		}
	}

	sb.WriteString("\n")
	sb.WriteString(styles.Description.Render("Enter menu • u undo • q quit"))

	return sb.String()
}
