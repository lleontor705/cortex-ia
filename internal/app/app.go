package app

import (
	"fmt"
	"os"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/tui"
)

// Run is the main entry point for the cortex-ia CLI.
func Run() error {
	args := os.Args[1:]

	if len(args) > 0 {
		return runCLI(args)
	}

	return tui.Run(Version)
}

func runCLI(args []string) error {
	if err := preflightCLI(args); err != nil {
		return err
	}

	switch strings.ToLower(args[0]) {
	case "version", "--version", "-v":
		fmt.Printf("cortex-ia %s\n", Version)
		return nil

	case "detect", "--detect":
		return runDetect()

	case "install":
		return runInstall(args[1:])

	case "doctor", "verify":
		return runDoctor()

	case "repair":
		return runRepair(args[1:])

	case "rollback":
		return runRollback(args[1:])

	case "uninstall":
		return runUninstall(args[1:])

	case "update", "upgrade":
		return runUpdate()

	case "sync":
		return runSync(args[1:])

	case "config":
		return runConfig()

	case "list":
		if len(args) > 1 {
			return runList(args[1])
		}
		return runList("all")

	case "init":
		return runInit()

	case "skill":
		if len(args) < 2 {
			return fmt.Errorf("usage: cortex-ia skill <add|list|remove> [args]")
		}
		return runSkill(args[1], args[2:])

	case "skill-registry":
		return runSkillRegistry(args[1:])

	case "memory":
		return runMemory(args[1:])

	case "help", "--help", "-h":
		printHelp()
		return nil

	default:
		return fmt.Errorf("unknown command: %s (use --help for usage)", args[0])
	}
}

// RetiredSurfaceError identifies a command or flag that was removed from the
// configuration installer. It is returned before dispatch performs any setup.
type RetiredSurfaceError struct {
	Surface string
}

func (e RetiredSurfaceError) Error() string {
	return fmt.Sprintf("retired surface %q is no longer supported; use install, sync, repair, doctor, rollback, or uninstall", e.Surface)
}

// UnsupportedClientError identifies a client outside the canonical installer
// inventory before dispatch can access user state.
type UnsupportedClientError struct {
	Client string
}

func (e UnsupportedClientError) Error() string {
	return fmt.Sprintf("unsupported client %q; supported client is opencode", e.Client)
}

// preflightCLI scans the complete invocation before dispatch so a retired
// trailing flag cannot cause a supported command to access user state first.
func preflightCLI(args []string) error {
	for i, arg := range args {
		if strings.EqualFold(arg, "--agent") && i+1 < len(args) && !isCanonicalClient(args[i+1]) {
			return UnsupportedClientError{Client: args[i+1]}
		}
		if client, found := strings.CutPrefix(strings.ToLower(arg), "--agent="); found && !isCanonicalClient(client) {
			return UnsupportedClientError{Client: client}
		}

		switch {
		case strings.EqualFold(arg, "agent-builder"),
			strings.EqualFold(arg, "auto-install"),
			strings.EqualFold(arg, "profiles"),
			strings.EqualFold(arg, "profile"),
			strings.EqualFold(arg, "--profile"),
			strings.EqualFold(arg, "--model-preset"),
			strings.HasPrefix(strings.ToLower(arg), "--profile="),
			strings.HasPrefix(strings.ToLower(arg), "--model-preset="):
			return RetiredSurfaceError{Surface: arg}
		}
	}
	return nil
}

func isCanonicalClient(client string) bool {
	switch strings.ToLower(client) {
	case "opencode":
		return true
	default:
		return false
	}
}

func printHelp() {
	fmt.Printf(`cortex-ia %s — AI agent ecosystem configurator

Usage:
  cortex-ia                  Launch interactive TUI
  cortex-ia install          Install ecosystem (auto-detect agents)
  cortex-ia install --agent opencode --preset full
  cortex-ia install --persona professional|mentor|minimal
  cortex-ia install --local           Use project .cortex-ia.yaml config
  cortex-ia install --dry-run
  cortex-ia init                     Create .cortex-ia.yaml in current dir
  cortex-ia skill add <path>         Add community skill
  cortex-ia skill list               List community skills
  cortex-ia skill remove <name>      Remove community skill
  cortex-ia skill-registry refresh   Rebuild .sdd/skill-registry.md from all tiers
  cortex-ia sync             Refresh managed files from current state
  cortex-ia sync --persona mentor
  cortex-ia detect           Detect installed agents and system info
  cortex-ia config           Show current configuration
  cortex-ia list agents      List detected agents
  cortex-ia list components  List installed components
  cortex-ia list backups     List available backups
  cortex-ia list skills      List installed community skills
  cortex-ia doctor           Verify installed files from lockfile
  cortex-ia verify           Alias of doctor
  cortex-ia repair           Re-apply managed files from lockfile/state
  cortex-ia rollback         Restore managed files from the last backup
  cortex-ia uninstall        Reverse all (or selected) cortex-ia injections
  cortex-ia uninstall --component persona --component cortex
  cortex-ia uninstall --agent opencode --dry-run
  cortex-ia uninstall --all  Wipe every managed change and clear state
  cortex-ia update           Check for available updates
  cortex-ia version          Show version
  cortex-ia help             Show this help

Presets:
  full      All 7 components (default)
  minimal   Cortex + ForgeSpec + Context7 + SDD
  custom    Select components manually (TUI)

Components:
  cortex           Persistent cross-session memory (19 MCP tools)
  forgespec        SDD contract validation + task board (15 MCP tools)
  context7         Live framework/library documentation
  sdd              Full 9-phase SDD workflow with orchestrator
  skills           Utility skills (non-SDD)
  conventions      Shared cortex conventions and memory protocol
`, Version)
}
