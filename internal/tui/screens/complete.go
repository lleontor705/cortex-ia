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

	sb.WriteString(styles.Help.Render("\nPress any key to exit"))
	return sb.String()
}
