package screens

import (
	"errors"
	"strings"
	"testing"
)

func TestRenderComplete_Success(t *testing.T) {
	data := CompleteData{
		Err:            nil,
		ComponentsDone: 5,
		FilesChanged:   12,
		BackupID:       "backup-001",
	}
	output := RenderComplete(data)
	if !strings.Contains(output, "Installation Complete") {
		t.Error("expected 'Installation Complete' in success output")
	}
	if strings.Contains(output, "Installation Failed") {
		t.Error("should not contain 'Installation Failed' on success")
	}
}

func TestRenderComplete_Failure(t *testing.T) {
	data := CompleteData{
		Err: errors.New("disk full"),
	}
	output := RenderComplete(data)
	if !strings.Contains(output, "Installation Failed") {
		t.Error("expected 'Installation Failed' in error output")
	}
	if !strings.Contains(output, "disk full") {
		t.Error("expected error message in output")
	}
}

func TestRenderComplete_Warnings(t *testing.T) {
	data := CompleteData{
		Err:    nil,
		Errors: []string{"missing optional dep", "config not found"},
	}
	output := RenderComplete(data)
	if !strings.Contains(output, "Warnings") {
		t.Error("expected 'Warnings' section in output")
	}
	if !strings.Contains(output, "missing optional dep") {
		t.Error("expected first warning in output")
	}
	if !strings.Contains(output, "config not found") {
		t.Error("expected second warning in output")
	}
}
