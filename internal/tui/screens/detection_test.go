package screens

import (
	"strings"
	"testing"
)

func TestRenderDetection_ShowsTools(t *testing.T) {
	data := DetectionData{
		OS: "linux", Arch: "amd64",
		PkgMgr: "apt", Shell: "bash",
		NodeVer: "v20.0.0", GitVer: "2.40.0", GoVer: "1.22.0",
		Npx: true, Cortex: true,
		DetectedAgents: 3,
	}
	output := RenderDetection(data)

	tools := []string{"Node.js", "npx", "Git", "Go", "Cortex"}
	for _, tool := range tools {
		if !strings.Contains(output, tool) {
			t.Errorf("expected tool %q in output", tool)
		}
	}
}

func TestRenderDetection_ShowsAgentCount(t *testing.T) {
	data := DetectionData{
		OS: "darwin", Arch: "arm64",
		PkgMgr: "brew", Shell: "zsh",
		DetectedAgents: 5,
	}
	output := RenderDetection(data)
	if !strings.Contains(output, "5 agent(s) detected") {
		t.Error("expected agent count in output")
	}
}
