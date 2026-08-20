package tui

import (
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lleontor705/cortex-ia/internal/install"
	"github.com/lleontor705/cortex-ia/internal/pipeline"
)

// fakeService records every call and replays scripted results. It exists only
// to prove the TUI calls the service with the typed intent it rendered.
type fakeService struct {
	planCalls     []install.Options
	installCalls  []install.Options
	syncCalls     []install.Options
	doctorCalls   int
	rollbackCalls []string
	uninstallCals int
	mcpListCalls  int
	mcpAddNames   []string
	mcpRemove     []string

	planFn      func(install.Options) (*pipeline.Plan, error)
	installFn   func(install.Options) (*install.InstallReceipt, error)
	syncFn      func(install.Options) (*install.InstallReceipt, error)
	doctorFn    func() (*install.DoctorReport, error)
	rollbackFn  func(string) (*install.RollbackReceipt, error)
	uninstallFn func(install.UninstallOptions) (*install.UninstallReceipt, error)
	mcpListFn   func() (*install.MCPListReport, error)
	mcpAddFn    func(string, install.MCPOptions) (*install.MCPReceipt, error)
	mcpRemoveFn func(string, install.MCPOptions) (*install.MCPReceipt, error)
}

func (f *fakeService) Plan(opts install.Options) (*pipeline.Plan, error) {
	f.planCalls = append(f.planCalls, opts)
	if f.planFn != nil {
		return f.planFn(opts)
	}
	return &pipeline.Plan{Digest: "digest0001"}, nil
}

func (f *fakeService) Install(opts install.Options) (*install.InstallReceipt, error) {
	f.installCalls = append(f.installCalls, opts)
	if f.installFn != nil {
		return f.installFn(opts)
	}
	return &install.InstallReceipt{PlanDigest: "digest0001", BackupID: "bkp-1"}, nil
}

func (f *fakeService) Sync(opts install.Options) (*install.InstallReceipt, error) {
	f.syncCalls = append(f.syncCalls, opts)
	if f.syncFn != nil {
		return f.syncFn(opts)
	}
	return &install.InstallReceipt{PlanDigest: "digest0001", BackupID: "bkp-1"}, nil
}

func (f *fakeService) Doctor() (*install.DoctorReport, error) {
	f.doctorCalls++
	if f.doctorFn != nil {
		return f.doctorFn()
	}
	return &install.DoctorReport{Verdict: install.DoctorHealthy}, nil
}

func (f *fakeService) Rollback(backupID string) (*install.RollbackReceipt, error) {
	f.rollbackCalls = append(f.rollbackCalls, backupID)
	if f.rollbackFn != nil {
		return f.rollbackFn(backupID)
	}
	return &RollbackReceiptOK, nil
}

var RollbackReceiptOK = install.RollbackReceipt{BackupID: "bkp-1", Verified: true}

func (f *fakeService) Uninstall(opts install.UninstallOptions) (*install.UninstallReceipt, error) {
	f.uninstallCals++
	if f.uninstallFn != nil {
		return f.uninstallFn(opts)
	}
	return &install.UninstallReceipt{Complete: true, StateRemoved: true, BackupID: "bkp-2"}, nil
}

func (f *fakeService) MCPList() (*install.MCPListReport, error) {
	f.mcpListCalls++
	if f.mcpListFn != nil {
		return f.mcpListFn()
	}
	return &install.MCPListReport{Installed: true}, nil
}

func (f *fakeService) MCPAdd(name string, opts install.MCPOptions) (*install.MCPReceipt, error) {
	f.mcpAddNames = append(f.mcpAddNames, name)
	if f.mcpAddFn != nil {
		return f.mcpAddFn(name, opts)
	}
	return &install.MCPReceipt{Name: name, Action: "added", Configured: true, Qualified: true, Installed: true}, nil
}

func (f *fakeService) MCPRemove(name string, opts install.MCPOptions) (*install.MCPReceipt, error) {
	f.mcpRemove = append(f.mcpRemove, name)
	if f.mcpRemoveFn != nil {
		return f.mcpRemoveFn(name, opts)
	}
	return &install.MCPReceipt{Name: name, Action: "removed"}, nil
}

// key builds a tea.KeyMsg for the logical key names used by the TUI.
func key(k string) tea.KeyMsg {
	switch k {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEscape}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "space":
		return tea.KeyMsg{Type: tea.KeySpace}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
}

// press applies one keypress and returns the updated model.
func press(m model, k string) model {
	updated, _ := m.Update(key(k))
	return updated.(model)
}

// pressDrive presses a key and immediately executes the command the update
// returned, feeding the resulting messages back into the model.
func pressDrive(t *testing.T, m model, k string) model {
	t.Helper()
	updated, cmd := m.Update(key(k))
	return drive(t, updated.(model), cmd)
}

// collect executes a command tree (including tea.Batch results) and returns
// every produced message, so tests can drive the model headlessly.
func collect(t *testing.T, cmd tea.Cmd) []tea.Msg {
	t.Helper()
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if msg == nil {
		return nil
	}
	if rv := reflect.ValueOf(msg); rv.Kind() == reflect.Slice {
		msgs := make([]tea.Msg, 0, rv.Len())
		for i := 0; i < rv.Len(); i++ {
			inner, ok := rv.Index(i).Interface().(tea.Cmd)
			if !ok {
				continue
			}
			msgs = append(msgs, collect(t, inner)...)
		}
		return msgs
	}
	return []tea.Msg{msg}
}

// drive executes a command and feeds every resulting operation message back
// into the model, mimicking the runtime loop.
func drive(t *testing.T, m model, cmd tea.Cmd) model {
	t.Helper()
	for _, msg := range collect(t, cmd) {
		if msg == nil {
			continue
		}
		if _, isTick := msg.(tickMsg); isTick {
			continue
		}
		updated, _ := m.Update(msg)
		m = updated.(model)
	}
	return m
}

// sized gives the model a deterministic terminal size for rendering.
func sized(m model) model {
	m.width, m.height = 100, 40
	return m
}

// viewLines counts the rendered rows of the fully framed view, so height
// tests can prove the output respects the terminal budget.
func viewLines(m model) int {
	return strings.Count(m.View(), "\n") + 1
}
