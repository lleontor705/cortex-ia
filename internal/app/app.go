package app

import (
	"fmt"
	"os"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/herdr"
	"github.com/lleontor705/cortex-ia/internal/tui"
)

// Run is the main entry point for the cortex-ia CLI. Without arguments it
// launches the interactive TUI; any argument dispatches the OpenCode-only
// command surface.
func Run() error {
	args := os.Args[1:]

	if len(args) == 0 {
		return tui.Run(Version)
	}

	return runCLI(args)
}

// runCLI dispatches the OpenCode installer and local delegation bridge. Install
// mutations stay in internal/install.Service; delegation lifecycle mutations
// stay in internal/delegation. The dispatcher owns neither policy.
func runCLI(args []string) error {
	if err := preflightCLI(args); err != nil {
		return err
	}

	command := strings.ToLower(args[0])
	rest := args[1:]

	switch command {
	case "install":
		return runInstall(rest)

	case "sync":
		return runSync(rest)

	case "herdr":
		return runHerdr(rest)

	case "delegate":
		return runDelegate(rest)

	case "work":
		return runWork(rest)

	case "board":
		return runBoard(rest)

	case "openspec":
		return runOpenSpec(rest)

	case "web":
		return runWeb(rest)

	case "mcp":
		return runMCP(rest)

	case "doctor":
		return runDoctor()

	case "rollback":
		return runRollback(rest)

	case "recover":
		return runRecover(rest)

	case "uninstall":
		return runUninstall(rest)

	case "version", "--version", "-v":
		fmt.Printf("cortex-ia %s\n", Version)
		return nil

	case "help", "--help", "-h":
		printHelp()
		return nil

	default:
		if retiredCommands[command] {
			return RetiredSurfaceError{Surface: args[0]}
		}
		return fmt.Errorf("unknown command: %s (use 'cortex-ia help' for usage)", args[0])
	}
}

// retiredCommands are removed legacy surfaces. They fail clearly instead of
// silently changing behavior.
var retiredCommands = map[string]bool{
	"detect":         true,
	"verify":         true,
	"repair":         true,
	"update":         true,
	"upgrade":        true,
	"config":         true,
	"list":           true,
	"init":           true,
	"skill":          true,
	"skill-registry": true,
	"memory":         true,
	"agent-builder":  true,
	"auto-install":   true,
	"profiles":       true,
	"profile":        true,
}

// retiredFlagPrefixes are removed legacy flags. Any argument starting with
// one of them is rejected before dispatch so no command can act on a retired
// intent. Arguments after a bare "--" separator are verbatim data (the local
// MCP command vector) and are never scanned, so a server flag that happens
// to match a retired prefix stays representable.
var retiredFlagPrefixes = []string{
	"--agent",
	"--persona",
	"--profile",
	"--model",
	"--sdd",
}

// RetiredSurfaceError identifies a command or flag that was removed from the
// OpenCode-only CLI. It is returned before dispatch performs any setup.
type RetiredSurfaceError struct {
	Surface string
}

func (e RetiredSurfaceError) Error() string {
	return fmt.Sprintf(
		"%q was removed from the OpenCode CLI; available commands: install, sync, mcp add|list|remove, herdr, delegate, work, board, doctor, rollback, recover, uninstall, version, help",
		e.Surface,
	)
}

// preflightCLI scans the invocation before dispatch so a retired flag
// anywhere in the command line fails closed before any user state is read.
// Scanning stops at the "--" separator: everything after it is verbatim
// command data, not CLI flags.
func preflightCLI(args []string) error {
	for _, arg := range args {
		if arg == "--" {
			return nil
		}
		lower := strings.ToLower(arg)
		for _, prefix := range retiredFlagPrefixes {
			if strings.HasPrefix(lower, prefix) {
				return RetiredSurfaceError{Surface: arg}
			}
		}
	}
	return nil
}

func printHelp() {
	fmt.Printf(`cortex-ia %s — OpenCode ecosystem installer

Usage:
  cortex-ia                          Launch the interactive TUI
  cortex-ia install [--dry-run] [--overwrite]
                                      Install the embedded OpenCode asset set
                                      and the default managed MCP selection
  cortex-ia sync [--dry-run] [--overwrite]
                                      Reconcile an installed home with the
                                      current embedded asset set
  cortex-ia mcp add <name> --preset [--dry-run]
                                      Register a managed catalog MCP preset
  cortex-ia mcp add <name> --local [--env KEY=VALUE]... -- <command> [args...]
                                      Register a managed custom local MCP
                                      server from an exact command vector
  cortex-ia mcp add <name> --remote <url> [--header KEY=VALUE]... [--dry-run]
                                      Register a managed custom remote MCP
                                      server endpoint (http/https)
  cortex-ia mcp list [--json]        List managed MCP entries and ownership
                                      (--json prints a sanitized JSON report)
  cortex-ia mcp remove <name> [--dry-run]
                                      Deregister a managed MCP entry
  cortex-ia herdr [install|setup|status]
                                      Manage Herdr workspace multiplexer setup
  cortex-ia delegate create --request-file <path> --transport <herdr|direct>
                                      Accept a validated external leaf job
  cortex-ia delegate status|result|cancel <job-id>
                                      Inspect or cancel a delegated leaf job
  cortex-ia delegate recover         Mark workers with expired leases as lost
  cortex-ia work create|list|status  Manage the local task DAG in Cortex SQLite
  cortex-ia work claim|renew         Acquire or renew bounded task authority
  cortex-ia work lease|lease-renew|release
                                      Reserve workspace-relative file scopes
  cortex-ia work transition|approve|retry|recover
                                      Advance, review, or reconcile task state
  cortex-ia board create|list|status  Group task DAGs into local task boards
  cortex-ia board serve [--addr 127.0.0.1:7331]
                                      Serve the embedded Cortex-IA operations console
  cortex-ia web [--addr 127.0.0.1:7331] [--open]
                                      Launch local Cortex-IA web dashboard in browser
  cortex-ia doctor                   Assess installation health (read-only)
  cortex-ia rollback [backup-id]     Restore the recorded (or given) backup
  cortex-ia recover [list]           List pending recovery journals
                                      (read-only)
  cortex-ia recover <journal-id>     Restore one pending journal; typing its
                                      exact ID confirms the recovery
  cortex-ia uninstall [--dry-run]    Remove the accredited installation
  cortex-ia version                  Show version
  cortex-ia help                     Show this help

Managed MCP presets: %s

The --preset, --local, and --remote kinds are mutually exclusive: exactly
one is required per add. Catalog preset names are reserved for --preset.

Flags:
  --dry-run                          Plan and report without writing
  --overwrite                        Replace unmanaged conflicting files
                                      (explicit and confirmed; a verified
                                      backup is captured first)
  --env KEY=VALUE                    Environment assignment for --local MCP
                                      servers (repeatable; the value reaches
                                      the config file only and is never
                                      printed)
  --header KEY=VALUE                 HTTP header assignment for --remote MCP
                                      servers (repeatable; the value reaches
                                      the config file only and is never
                                      printed)

Install and sync preview the final plan — including every --overwrite
replacement — and bind the real run to that exact plan digest. If anything
drifts between preview and apply, the run aborts with a stale-plan error and
nothing is written.

Destructive commands — rollback, recover, uninstall, mcp remove, and
overwrite via --overwrite — require an interactive terminal and an explicit
confirmation. Piped or closed input always fails closed without writing
anything.

The CLI configures OpenCode, owns local task/lease control, and can supervise an optional AGY execution leaf.
Former platform adapters, persona, profile, and model-routing flags remain removed.
`, Version, presetNames())
}

func runHerdr(args []string) error {
	if len(args) == 0 {
		return herdr.Status()
	}

	sub := strings.ToLower(args[0])
	switch sub {
	case "install":
		return herdr.Install()
	case "setup":
		return herdr.Setup()
	case "status":
		return herdr.Status()
	default:
		return fmt.Errorf("unknown herdr subcommand: %s (valid: install, setup, status)", args[0])
	}
}
