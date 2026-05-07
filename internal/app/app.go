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

	case "gga":
		return runGGA(args[1:])

	case "profiles":
		return runProfiles(args[1:])

	case "agent-builder":
		return runAgentBuilder(args[1:])

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

	case "auto-install":
		return runAutoInstall(args[1:])

	case "help", "--help", "-h":
		printHelp()
		return nil

	default:
		return fmt.Errorf("unknown command: %s (use --help for usage)", args[0])
	}
}

func printHelp() {
	fmt.Printf(`cortex-ia %s — AI agent ecosystem configurator

Usage:
  cortex-ia                  Launch interactive TUI
  cortex-ia install          Install ecosystem (auto-detect agents)
  cortex-ia install --agent claude-code --preset full
  cortex-ia install --model-preset balanced|performance|economy
  cortex-ia install --persona professional|mentor|minimal
  cortex-ia install --profile <name>  Use a saved model profile
  cortex-ia install --local           Use project .cortex-ia.yaml config
  cortex-ia install --dry-run
  cortex-ia init                     Create .cortex-ia.yaml in current dir
  cortex-ia skill add <path>         Add community skill
  cortex-ia skill list               List community skills
  cortex-ia skill remove <name>      Remove community skill
  cortex-ia sync             Refresh managed files from current state
  cortex-ia sync --persona mentor
  cortex-ia sync --profile <name>     Use a saved model profile
  cortex-ia detect           Detect installed agents and system info
  cortex-ia config           Show current configuration
  cortex-ia list agents      List detected agents
  cortex-ia list components  List installed components
  cortex-ia list backups     List available backups
  cortex-ia list profiles    List saved OpenCode SDD profiles
  cortex-ia list skills      List installed community skills
  cortex-ia doctor           Verify installed files from lockfile
  cortex-ia verify           Alias of doctor
  cortex-ia repair           Re-apply managed files from lockfile/state
  cortex-ia rollback         Restore managed files from the last backup
  cortex-ia gga --provider <id>  Switch GGA provider (anthropic|openai|google|ollama|claude|opencode|gemini|codex)
  cortex-ia gga --list           List supported GGA providers
  cortex-ia agent-builder list                              List custom skills built via the Agent Builder
  cortex-ia agent-builder create --engine claude --purpose "review go diffs" --target claude-code
  cortex-ia agent-builder remove <name>                     Remove a custom skill from the registry
  cortex-ia profiles list                                   List saved OpenCode SDD profiles
  cortex-ia profiles create <name>:<provider>/<model>       Create a profile that maps every SDD phase to one model
  cortex-ia profiles set <name>:<phase>:<provider>/<model>  Override a single SDD phase
  cortex-ia profiles apply <name>                           Write the profile's per-phase models into opencode.json
  cortex-ia profiles delete <name>                          Remove a profile
  cortex-ia uninstall        Reverse all (or selected) cortex-ia injections
  cortex-ia uninstall --component persona --component cortex
  cortex-ia uninstall --agent claude-code --dry-run
  cortex-ia uninstall --all  Wipe every managed change and clear state
  cortex-ia auto-install     Install missing agents via package managers
  cortex-ia auto-install --dry-run
  cortex-ia update           Check for available updates
  cortex-ia version          Show version
  cortex-ia help             Show this help

Presets:
  full      All 8 components (default)
  minimal   Cortex + ForgeSpec + Context7 + SDD
  custom    Select components manually (TUI)

Components:
  cortex           Persistent cross-session memory (19 MCP tools)
  agent-mailbox    Inter-agent messaging system (9 MCP tools)
  forgespec        SDD contract validation + task board (15 MCP tools)
  context7         Live framework/library documentation
  sdd              Full 9-phase SDD workflow with orchestrator
  skills           Utility skills (non-SDD)
  conventions      Shared cortex conventions and memory protocol
  gga              Guardian Angel — AI-powered pre-commit code review

`, Version)
}
