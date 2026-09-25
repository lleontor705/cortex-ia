package app

import (
	"fmt"
	"os"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/install"
	"github.com/lleontor705/cortex-ia/internal/logging"
	"github.com/lleontor705/cortex-ia/internal/targets"
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
	args, debugRequested := stripDebugFlag(args)
	if debugRequested || debugEnvEnabled() {
		enableDebugLogging(args)
	}
	if len(args) == 0 {
		return tui.Run(Version)
	}

	if err := preflightCLI(args); err != nil {
		return err
	}

	command := strings.ToLower(args[0])
	rest := args[1:]

	switch command {
	case "install":
		return runInstallWithTheme(rest)

	case "sync":
		return runSync(rest)

	case "snapshot":
		return runCortexSnapshot(rest)

	case "work":
		return runWork(rest)

	case "worktree":
		return runWorktree(rest)

	case "board":
		return runBoard(rest)

	case "ledger":
		return runLedger(rest)

	case "ui":
		return runUI(rest)

	case "openspec":
		return runOpenSpec(rest)

	case "web":
		return runWeb(rest)

	case "doc":
		return runDoc(rest)

	case "diagram":
		return runDiagram(rest)

	case "mcp":
		return runMCP(rest)

	case "model":
		return runModel(rest)

	case "stats":
		return runStats(rest)

	case "report":
		return runReport(rest)

	case "doctor":
		return runDoctor()

	case "rollback":
		return runRollback(rest)

	case "recover":
		return runRecover(rest)

	case "uninstall":
		return runUninstall(rest)

	case "update", "upgrade":
		return runUpdate(rest)

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

// themeFlag is the explicit install opt-in that applies the bundled cortex
// theme to the OpenCode configuration.
const themeFlag = "--theme"

// runInstallWithTheme consumes the --theme opt-in and then delegates to the
// shared install front end. The dispatcher only recognizes and removes the
// token: building the options, planning, and writing stay in the install
// service. Without the token the install surface is untouched.
func runInstallWithTheme(args []string) error {
	args, applyTheme := stripThemeFlag(args)
	if !applyTheme {
		return runInstall(args)
	}
	flags, err := parseRunFlags(args, "install", true)
	if err != nil {
		return err
	}
	if flags.Target != "" {
		targetList, err := targets.ParseTargets(flags.Target)
		if err != nil {
			return err
		}
		for _, target := range targetList {
			if target != targets.TargetOpenCode {
				return fmt.Errorf("--theme applies to the opencode target only; install the %s target without --theme", target)
			}
		}
	}
	service, err := newService()
	if err != nil {
		return err
	}
	opts := install.DefaultOptions()
	opts.Version = Version
	opts.ApplyTheme = true
	return previewAndApply("install", flags, opts, service.Install)
}

// stripThemeFlag removes every --theme token and reports whether the opt-in was
// requested. Tokens after a bare "--" separator are verbatim command data and
// are never scanned, so a `--theme` that is not a CLI flag stays representable.
func stripThemeFlag(args []string) ([]string, bool) {
	found := false
	stripped := make([]string, 0, len(args))
	for i, arg := range args {
		if arg == "--" {
			stripped = append(stripped, args[i:]...)
			break
		}
		if strings.EqualFold(arg, themeFlag) {
			found = true
			continue
		}
		stripped = append(stripped, arg)
	}
	return stripped, found
}

// stripDebugFlag removes every exact `--debug` token that appears before the
// first bare "--" separator and reports whether any was found. Tokens after
// the separator are verbatim command data (the local MCP command vector) and
// are never scanned or rewritten, so a server argument that happens to be
// `--debug` stays representable.
func stripDebugFlag(args []string) ([]string, bool) {
	found := false
	stripped := make([]string, 0, len(args))
	for i, arg := range args {
		if arg == "--" {
			stripped = append(stripped, args[i:]...)
			break
		}
		if arg == "--debug" {
			found = true
			continue
		}
		stripped = append(stripped, arg)
	}
	return stripped, found
}

// debugEnvEnabled reports whether CORTEX_IA_DEBUG requests debug tracing.
// Only the documented truthy spellings activate it.
func debugEnvEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("CORTEX_IA_DEBUG"))) {
	case "1", "true", "yes":
		return true
	default:
		return false
	}
}

// enableDebugLogging activates the stderr + file debug sinks and records the
// invocation header. Debug output never touches stdout; stdout stays reserved
// for machine-readable command receipts.
func enableDebugLogging(args []string) {
	stateHome, err := cortexStateHome()
	if err != nil {
		logging.Enable("")
		logging.Debugf("cortex-ia invocation args=%q version=%s state_home=<unresolved: %v>", args, Version, err)
		return
	}
	logging.Enable(stateHome)
	logging.Debugf("cortex-ia invocation args=%q version=%s state_home=%s", args, Version, stateHome)
}

// retiredCommands are removed legacy surfaces. They fail clearly instead of
// silently changing behavior.
var retiredCommands = map[string]bool{
	"detect":         true,
	"verify":         true,
	"repair":         true,
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
	"delegate":       true,
	"herdr":          true,
	"hook":           true,
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
		"%q was removed from the OpenCode CLI; available commands: install, sync, snapshot, work, worktree, board, ledger, ui, openspec, web, doc, diagram, mcp, model, report, doctor, rollback, recover, uninstall, update, version, help",
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
  cortex-ia install [--target <list>] [--dry-run] [--overwrite] [--theme]
                                      Install the assets and plugins for
                                      the specified targets (opencode, claude, all)
  cortex-ia sync [--target <list>] [--dry-run] [--overwrite]
                                      Reconcile an installed home with the
                                      current asset set for targets
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
  cortex-ia model list [--json]       List agents with their effective models,
                                      variants, sources, and ownership
  cortex-ia model get <agent> [--json]
                                      Report one agent's effective model,
                                      variant, and source
  cortex-ia model set <agent> <provider/model[#variant]> [--effort <level>]
                    [--json] [--dry-run]
                                      Assign an agent model in the global config
  cortex-ia model unset <agent> [--json] [--dry-run]
                                      Remove an agent's managed model entry
  cortex-ia model doctor [--json]     Diagnose agent-model config, drift,
                                      template safety, and markdown pins
  cortex-ia model catalog [--json] [--provider <id>]
                                      List selectable provider/model entries with
                                      variants and the acquisition source
  cortex-ia snapshot read --project <project> --id <id> [--expected-sha256 <digest>]
                                      Read and verify one bounded local Cortex snapshot
  cortex-ia work create|revise|archive|list|status|approvals|fingerprint
                                      Define, revise, or inspect the local task DAG
  cortex-ia work claim|renew|controller-renew|transition|approve|retry
                                      Acquire or advance bounded task authority
  cortex-ia work lease|reserve|lease-renew|release|release-all|verify-lease
                                      Reserve, renew, or verify workspace file scopes
  cortex-ia work review-refresh|decompose|recover|reconcile
                                      Refresh review bindings, decompose, or sweep state
  cortex-ia work reconcile <task-id> --reason <text> --session <id> --revision <n>
                                      Force-release an orphaned live claim
  cortex-ia worktree list|validate   Inspect authoritative Git worktrees (read-only)
  cortex-ia board create|list|status|archive|unarchive|delete
                                      Group task DAGs into local task boards
  cortex-ia board serve [--addr 127.0.0.1:7331]
                                      Serve the embedded Cortex-IA operations console
  cortex-ia ledger fact|progress|status
                                      Inspect or update the Dual Ledger (facts + progress)
  cortex-ia ui snapshot              Print a bounded read-only TUI snapshot
  cortex-ia openspec validate|list|status|archive|new
                                      Manage the OpenSpec SDD workspace
  cortex-ia web [--addr 127.0.0.1:7331] [--board <id>] [--task <id>] [--open] [--daemon]
                                      Launch local Cortex-IA web dashboard in browser
  cortex-ia doc convert|inspect      Convert office/PDF docs to Markdown or inspect metadata
  cortex-ia diagram validate|render|compare|reach
                                     Validate, render, compare, or trace system diagrams
  cortex-ia report error|send|config|flush|status
                                      Report errors or manage reporting configuration
  cortex-ia doctor                   Assess installation health (read-only)
  cortex-ia rollback [backup-id]|list  Restore a backup or list available backups
  cortex-ia recover [list]           List pending recovery journals (read-only)
  cortex-ia recover <journal-id>     Restore one pending journal; typing its
                                      exact ID confirms the recovery
  cortex-ia uninstall [--dry-run] [--target <list>]
                                      Remove the accredited installation
  cortex-ia update [--check]         Check for and install latest release
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
  --theme                            Apply the bundled cortex theme to the
                                      OpenCode configuration (install only,
                                      opencode target only). Opt-in: without
                                      it the theme key is never touched, and
                                      an explicit light/dark mode is kept
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

The CLI configures OpenCode and owns local task/lease control under Cortex-IA Work Authority.
Former platform adapters, persona, profile, and model-routing flags remain removed.
`, Version, presetNames())
}
