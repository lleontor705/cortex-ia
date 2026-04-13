package tui

import (
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/backup"
)

func TestRenderStatusBar_ContainsVersion(t *testing.T) {
	m := New(nil, "/tmp", "1.2.3")
	m.Width = 80
	m.Screen = ScreenWelcome

	bar := renderStatusBar(m)
	if !strings.Contains(bar, "1.2.3") {
		t.Errorf("status bar should contain version, got %q", bar)
	}
}

func TestRenderStatusBar_ZeroWidthReturnsEmpty(t *testing.T) {
	m := New(nil, "/tmp", "1.0.0")
	m.Width = 0

	bar := renderStatusBar(m)
	if bar != "" {
		t.Error("status bar with zero width should be empty")
	}
}

func TestRenderStatusContext_Agents(t *testing.T) {
	m := New(nil, "/tmp", "1.0.0")
	m.Screen = ScreenAgents
	m.Agents = []AgentItem{
		{Name: "a", Selected: true},
		{Name: "b", Selected: false},
		{Name: "c", Selected: true},
	}

	ctx := renderStatusContext(m)
	if !strings.Contains(ctx, "2/3") {
		t.Errorf("agents context should show selected count, got %q", ctx)
	}
}

func TestRenderStatusContext_Backups(t *testing.T) {
	m := New(nil, "/tmp", "1.0.0")
	m.Screen = ScreenBackups
	m.Backups = make([]backup.Manifest, 5)

	ctx := renderStatusContext(m)
	if !strings.Contains(ctx, "5 backups") {
		t.Errorf("backups context should show count, got %q", ctx)
	}
}

func TestRenderStatusContext_Default(t *testing.T) {
	m := New(nil, "/tmp", "1.0.0")
	m.Screen = ScreenDetection

	ctx := renderStatusContext(m)
	if !strings.Contains(ctx, "help") {
		t.Errorf("default context should show help hint, got %q", ctx)
	}
}
