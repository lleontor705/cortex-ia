package screens

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/progress"
)

func TestRenderInstalling_InProgress(t *testing.T) {
	prog := InstallProgress{
		Percent:     40,
		CurrentStep: "Installing MCP servers",
		Items: []ProgressItem{
			{Label: "Cortex", Status: "succeeded"},
			{Label: "MCP servers", Status: "running"},
			{Label: "Skills", Status: "pending"},
		},
		Logs: []string{"cortex installed"},
		Done: false,
	}
	pb := progress.New(progress.WithDefaultGradient(), progress.WithWidth(20))
	output := RenderInstalling(prog, "⠋", pb)

	if !strings.Contains(output, "Installing") {
		t.Error("expected 'Installing' in in-progress output")
	}
	if !strings.Contains(output, "40%") {
		t.Error("expected '40%' percentage in output")
	}
}

func TestRenderInstalling_Done(t *testing.T) {
	prog := InstallProgress{
		Percent: 100,
		Items: []ProgressItem{
			{Label: "Cortex", Status: "succeeded"},
			{Label: "Skills", Status: "succeeded"},
		},
		Done:   true,
		Failed: false,
	}
	pb := progress.New(progress.WithDefaultGradient(), progress.WithWidth(20))
	output := RenderInstalling(prog, "⠋", pb)

	if !strings.Contains(output, "Installation complete") {
		t.Error("expected 'Installation complete' in done output")
	}
}

func TestRenderInstalling_Failed(t *testing.T) {
	prog := InstallProgress{
		Percent: 100,
		Items: []ProgressItem{
			{Label: "Cortex", Status: "succeeded"},
			{Label: "Skills", Status: "failed"},
		},
		Done:   true,
		Failed: true,
	}
	pb := progress.New(progress.WithDefaultGradient(), progress.WithWidth(20))
	output := RenderInstalling(prog, "⠋", pb)

	if !strings.Contains(output, "errors") {
		t.Error("expected 'errors' indicator in failed output")
	}
}
