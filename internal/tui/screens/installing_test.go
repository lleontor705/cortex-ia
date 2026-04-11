package screens

import (
	"strings"
	"testing"
)

func TestRenderInstalling_InProgress(t *testing.T) {
	progress := InstallProgress{
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
	output := RenderInstalling(progress, 0)

	if !strings.Contains(output, "Installing") {
		t.Error("expected 'Installing' in in-progress output")
	}
	if !strings.Contains(output, "40%") {
		t.Error("expected '40%' percentage in output")
	}
	if !strings.Contains(output, "Please wait") {
		t.Error("expected 'Please wait' help text for in-progress state")
	}
}

func TestRenderInstalling_Done(t *testing.T) {
	progress := InstallProgress{
		Percent: 100,
		Items: []ProgressItem{
			{Label: "Cortex", Status: "succeeded"},
			{Label: "Skills", Status: "succeeded"},
		},
		Done:   true,
		Failed: false,
	}
	output := RenderInstalling(progress, 0)

	if !strings.Contains(output, "Installation complete") {
		t.Error("expected 'Installation complete' in done output")
	}
	if strings.Contains(output, "Please wait") {
		t.Error("should not show 'Please wait' when done")
	}
}

func TestRenderInstalling_Failed(t *testing.T) {
	progress := InstallProgress{
		Percent: 100,
		Items: []ProgressItem{
			{Label: "Cortex", Status: "succeeded"},
			{Label: "Skills", Status: "failed"},
		},
		Done:   true,
		Failed: true,
	}
	output := RenderInstalling(progress, 0)

	if !strings.Contains(output, "errors") {
		t.Error("expected 'errors' indicator in failed output")
	}
}
