package screens

import (
	"fmt"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/tui/styles"
)

// DetectionData holds the data needed to render the system detection screen.
type DetectionData struct {
	OS, Arch, PkgMgr, Shell    string
	NodeVer, GitVer, GoVer     string
	Npx, Cortex                bool
	DetectedAgents             int
}

// RenderDetection renders the system detection screen.
func RenderDetection(data DetectionData) string {
	var sb strings.Builder

	sb.WriteString(styles.Title.Render("System Detection"))
	sb.WriteString("\n\n")

	sb.WriteString(styles.Subtitle.Render("Platform"))
	sb.WriteString("\n")
	fmt.Fprintf(&sb, "  OS:       %s/%s\n", data.OS, data.Arch)
	fmt.Fprintf(&sb, "  Pkg Mgr:  %s\n", data.PkgMgr)
	fmt.Fprintf(&sb, "  Shell:    %s\n", data.Shell)
	sb.WriteString("\n")

	sb.WriteString(styles.Subtitle.Render("Tools"))
	sb.WriteString("\n")

	toolLine := func(name, ver string) {
		if ver != "" {
			fmt.Fprintf(&sb, "  %s %-10s %s\n", styles.StatusOK.Render("●"), name, ver)
		} else {
			fmt.Fprintf(&sb, "  %s %-10s %s\n", styles.StatusFail.Render("○"), name, styles.Description.Render("not found"))
		}
	}

	toolLine("Node.js", data.NodeVer)
	boolStr := func(b bool) string {
		if b {
			return "available"
		}
		return ""
	}
	toolLine("npx", boolStr(data.Npx))
	toolLine("Git", data.GitVer)
	toolLine("Go", data.GoVer)
	toolLine("Cortex", boolStr(data.Cortex))
	sb.WriteString("\n")

	fmt.Fprintf(&sb, "%s %d agent(s) detected\n",
		styles.StatusOK.Render("●"), data.DetectedAgents)

	return sb.String()
}
