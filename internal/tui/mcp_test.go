package tui

import (
	"errors"
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/install"
	"github.com/lleontor705/cortex-ia/internal/mcpmanager"
)

// openMCP drives Home → Manage MCPs and drains the list command.
func openMCP(t *testing.T, svc ServiceAPI) model {
	t.Helper()
	m := sized(newModel(svc, "/home/test", "vtest"))
	m = press(m, "down")
	m = pressDrive(t, m, "enter")
	if m.screen != screenMCP {
		t.Fatalf("expected MCP screen, got %v", m.screen)
	}
	if m.mcpReport == nil {
		t.Fatal("expected MCP list loaded")
	}
	return m
}

func sampleMCPReport() *install.MCPListReport {
	return &install.MCPListReport{
		Installed: true,
		Entries: []mcpmanager.EntryReport{
			{Name: "context7", Status: mcpmanager.StatusAbsent},
			{Name: "cortex", Status: mcpmanager.StatusManaged},
			{Name: "forgespec", Status: mcpmanager.StatusConflict},
		},
		Unknown: []string{"custom-server"},
	}
}

func sampleListFn() func() (*install.MCPListReport, error) {
	return func() (*install.MCPListReport, error) { return sampleMCPReport(), nil }
}

// TestMCPScreenListsAndNavigates proves the entries, statuses, and unknown
// (never touched) names render, and the cursor stays inside the entry list.
func TestMCPScreenListsAndNavigates(t *testing.T) {
	fake := &fakeService{mcpListFn: sampleListFn()}
	m := openMCP(t, fake)

	view := m.View()
	for _, want := range []string{"cortex", "forgespec", "context7", "managed", "conflict", "custom-server"} {
		if !strings.Contains(view, want) {
			t.Fatalf("MCP view missing %q:\n%s", want, view)
		}
	}

	m = press(m, "up")
	if m.cursor != 0 {
		t.Fatalf("cursor above list: %d", m.cursor)
	}
	for i := 0; i < 6; i++ {
		m = press(m, "down")
	}
	if m.cursor != 2 {
		t.Fatalf("cursor below list: %d", m.cursor)
	}
}

// TestMCPAddAbsentRunsDirectly proves adding an absent managed preset is the
// non-destructive direction: it runs without a confirmation dialog.
func TestMCPAddAbsentRunsDirectly(t *testing.T) {
	fake := &fakeService{mcpListFn: sampleListFn()}
	m := openMCP(t, fake)

	m = press(m, "enter") // cursor 0: context7 (absent)
	if m.confirm.kind != confirmNone {
		t.Fatalf("absent add must not require confirmation, got %v", m.confirm.kind)
	}
	if m.screen != screenRunning {
		t.Fatalf("expected running screen for add, got %v", m.screen)
	}
	m = drive(t, m, mcpMutateCmd(m.svc, "context7", true))

	if len(fake.mcpAddNames) != 1 || fake.mcpAddNames[0] != "context7" {
		t.Fatalf("expected MCPAdd(context7), got %v", fake.mcpAddNames)
	}
	if m.screen != screenResult || !m.result.pass {
		t.Fatalf("expected PASS result for add, got screen=%v pass=%v", m.screen, m.result.pass)
	}
}

// TestMCPAddConfiguredOnlyIsVisibleFail proves an add that configures the
// entry without valid qualification evidence (installed=false) renders FAIL
// with the reason and remedy visible, even without a service error.
func TestMCPAddConfiguredOnlyIsVisibleFail(t *testing.T) {
	fake := &fakeService{mcpListFn: sampleListFn()}
	fake.mcpAddFn = func(name string, _ install.MCPOptions) (*install.MCPReceipt, error) {
		return &install.MCPReceipt{
			Name:       name,
			Action:     "added",
			Configured: true,
			Warnings:   []string{`MCP "context7" configured without valid qualification evidence`},
		}, nil
	}
	m := openMCP(t, fake)

	m = press(m, "enter") // cursor 0: context7 (absent → add)
	m = drive(t, m, mcpMutateCmd(m.svc, "context7", true))

	if m.screen != screenResult {
		t.Fatalf("expected result screen, got %v", m.screen)
	}
	if m.result.pass {
		t.Fatal("configured-only MCP add must be FAIL, not PASS")
	}
	if m.result.canRollback {
		t.Fatal("rollback must stay gated off on a FAIL verdict")
	}
	view := m.View()
	for _, want := range []string{
		"FAIL",
		"installed=false",
		"qualification evidence",
		"remedy:",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("MCP add result view missing %q:\n%s", want, view)
		}
	}
}

// TestMCPAddProbeErrorIsVisibleFail proves a probe/service error on MCP add
// is never hidden: the verdict is FAIL and the error text stays on screen.
func TestMCPAddProbeErrorIsVisibleFail(t *testing.T) {
	fake := &fakeService{mcpListFn: sampleListFn()}
	fake.mcpAddFn = func(name string, _ install.MCPOptions) (*install.MCPReceipt, error) {
		return nil, errors.New("probe failed: mcp server context7 did not return valid evidence")
	}
	m := openMCP(t, fake)

	m = press(m, "enter") // cursor 0: context7 (absent → add)
	m = drive(t, m, mcpMutateCmd(m.svc, "context7", true))

	if m.screen != screenResult {
		t.Fatalf("expected result screen, got %v", m.screen)
	}
	if m.result.pass {
		t.Fatal("probe error on MCP add must be FAIL")
	}
	view := m.View()
	if !strings.Contains(view, "FAIL") || !strings.Contains(view, "probe failed") {
		t.Fatalf("probe error not visible on result:\n%s", view)
	}
}

// TestMCPRemoveManagedRequiresConfirmation proves removing an accredited
// entry is destructive: explicit confirmation first, decline cancels without
// any service call, confirm removes exactly the selected entry.
func TestMCPRemoveManagedRequiresConfirmation(t *testing.T) {
	fake := &fakeService{mcpListFn: sampleListFn()}
	m := openMCP(t, fake)

	m = press(m, "down") // cursor 1: cortex (managed)
	m = press(m, "enter")
	if m.confirm.kind != confirmMCPRemove || m.confirm.arg != "cortex" {
		t.Fatalf("expected remove confirmation for cortex, got %v %q", m.confirm.kind, m.confirm.arg)
	}
	if !strings.Contains(m.View(), "Confirm MCP remove") {
		t.Fatalf("confirmation overlay not visible:\n%s", m.View())
	}

	m = press(m, "n")
	if m.confirm.kind != confirmNone || len(fake.mcpRemove) != 0 {
		t.Fatal("declining must cancel without calling MCPRemove")
	}

	m = press(m, "enter")
	m = pressDrive(t, m, "y")
	if len(fake.mcpRemove) != 1 || fake.mcpRemove[0] != "cortex" {
		t.Fatalf("expected MCPRemove(cortex), got %v", fake.mcpRemove)
	}
	if m.screen != screenResult || !m.result.pass {
		t.Fatalf("expected PASS result for remove, got screen=%v pass=%v", m.screen, m.result.pass)
	}
}

// TestMCPRemoveUserOwnedFailsClosedVisible proves user-owned entries demand
// an explicit confirmation and that the manager's fail-closed error reaches
// the Result screen as FAIL.
func TestMCPRemoveUserOwnedFailsClosedVisible(t *testing.T) {
	fake := &fakeService{mcpListFn: sampleListFn()}
	fake.mcpRemoveFn = func(name string, _ install.MCPOptions) (*install.MCPReceipt, error) {
		return nil, errors.New("mcpmanager: entry is user-owned and would fail closed")
	}
	m := openMCP(t, fake)

	m = press(m, "down")
	m = press(m, "down") // cursor 2: forgespec (conflict → user-owned)
	m = press(m, "enter")
	if m.confirm.kind != confirmMCPRemove || m.confirm.arg != "forgespec" {
		t.Fatalf("user-owned removal must ask for confirmation, got %v %q", m.confirm.kind, m.confirm.arg)
	}
	m = pressDrive(t, m, "y")

	if len(fake.mcpRemove) != 1 {
		t.Fatal("expected the confirmed removal attempt to reach the service")
	}
	if m.result.pass {
		t.Fatal("user-owned removal must surface as FAIL")
	}
	if !strings.Contains(m.View(), "user-owned") {
		t.Fatalf("fail-closed reason not visible on result:\n%s", m.View())
	}
}
