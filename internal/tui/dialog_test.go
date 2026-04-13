package tui

import (
	"strings"
	"testing"
)

func TestRenderDialog_ContainsTitle(t *testing.T) {
	d := Dialog{
		Type:    DialogRestoreConfirm,
		Title:   "Confirm Restore",
		Message: "Restore backup b1?",
	}
	output := renderDialog(d, 80, 24)
	if !strings.Contains(output, "Confirm Restore") {
		t.Error("dialog should contain title")
	}
}

func TestRenderDialog_ContainsMessage(t *testing.T) {
	d := Dialog{
		Type:    DialogDeleteConfirm,
		Title:   "Confirm Delete",
		Message: "Delete backup b1?",
	}
	output := renderDialog(d, 80, 24)
	if !strings.Contains(output, "Delete backup b1?") {
		t.Error("dialog should contain message")
	}
}

func TestRenderDialog_ContainsWarning(t *testing.T) {
	d := Dialog{
		Type:    DialogDeleteConfirm,
		Title:   "Test",
		Message: "msg",
		Warning: "This is irreversible",
	}
	output := renderDialog(d, 80, 24)
	if !strings.Contains(output, "irreversible") {
		t.Error("dialog should contain warning")
	}
}

func TestRenderDialog_NoWarning(t *testing.T) {
	d := Dialog{
		Type:    DialogRestoreConfirm,
		Title:   "Test",
		Message: "msg",
	}
	output := renderDialog(d, 80, 24)
	if output == "" {
		t.Error("dialog should render without warning")
	}
}

func TestRenderDialog_ZeroDimensions(t *testing.T) {
	d := Dialog{
		Type:    DialogRestoreConfirm,
		Title:   "Test",
		Message: "msg",
	}
	output := renderDialog(d, 0, 0)
	if output == "" {
		t.Error("dialog should render even with zero dimensions")
	}
}
