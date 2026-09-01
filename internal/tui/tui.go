// Package tui renders the minimal OpenCode-only installation interface: a
// five-screen Bubble Tea program (Home, Review, Running, Result, MCP Manager, Web)
// over internal/install.Service.
//
// The TUI owns no installation logic of its own. Every operation — plan,
// install, sync, doctor, rollback, uninstall, and managed MCP add/list/remove
// — goes through the same Service API the CLI uses; the TUI only adapts
// keyboard input to typed requests and renders typed receipts. Destructive
// actions (overwrite, uninstall, rollback, and removal of MCP entries that
// are not ownership-accredited) always require an explicit per-action
// confirmation before the service is called.
package tui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sync"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lleontor705/cortex-ia/internal/backup"
	"github.com/lleontor705/cortex-ia/internal/cortexiaweb"
	"github.com/lleontor705/cortex-ia/internal/delegation"
	"github.com/lleontor705/cortex-ia/internal/install"
	"github.com/lleontor705/cortex-ia/internal/pipeline"
)

// ServiceAPI is the exact service surface the TUI consumes. It is satisfied
// by *install.Service and by test fakes; the TUI never widens it and never
// duplicates logic that lives behind it.
type ServiceAPI interface {
	Plan(opts install.Options) (*pipeline.Plan, error)
	Install(opts install.Options) (*install.InstallReceipt, error)
	Sync(opts install.Options) (*install.InstallReceipt, error)
	Doctor() (*install.DoctorReport, error)
	Rollback(backupID string) (*install.RollbackReceipt, error)
	ListBackups() ([]backup.Manifest, error)
	Uninstall(opts install.UninstallOptions) (*install.UninstallReceipt, error)
	MCPList() (*install.MCPListReport, error)
	MCPAdd(name string, opts install.MCPOptions) (*install.MCPReceipt, error)
	MCPRemove(name string, opts install.MCPOptions) (*install.MCPReceipt, error)
}

// Run starts the interactive TUI against the user's real home directory. The
// entry signature is fixed by internal/app: no arguments launch the TUI.
func Run(version string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}
	service, err := install.New(homeDir)
	if err != nil {
		return err
	}
	program := tea.NewProgram(newModel(service, homeDir, version), tea.WithAltScreen())
	_, err = program.Run()
	return err
}

var webServerOnce sync.Once

func startWebBackground(homeDir string) {
	webServerOnce.Do(func() {
		dbPath := delegation.DefaultDBPath(homeDir)
		store, err := delegation.OpenStore(dbPath)
		if err != nil {
			return
		}
		ready := make(chan string, 1)
		go func() {
			_ = cortexiaweb.Serve(context.Background(), store, "127.0.0.1:7331", ready)
		}()
	})
}

func openBrowser(targetURL string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", targetURL)
	case "darwin":
		cmd = exec.Command("open", targetURL)
	default:
		cmd = exec.Command("xdg-open", targetURL)
	}
	_ = cmd.Start()
}
