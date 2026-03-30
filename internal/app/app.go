package app

import (
	"fmt"
	"os"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/agents"
	"github.com/lleontor705/cortex-ia/internal/model"
	"github.com/lleontor705/cortex-ia/internal/pipeline"
	"github.com/lleontor705/cortex-ia/internal/system"
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

	case "help", "--help", "-h":
		printHelp()
		return nil

	default:
		return fmt.Errorf("unknown command: %s (use --help for usage)", args[0])
	}
}

func runDetect() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}

	fmt.Println("Detecting system...")
	sysInfo := system.Detect()
	fmt.Printf("  OS: %s/%s\n", sysInfo.OS, sysInfo.Arch)
	fmt.Printf("  Package Manager: %s\n", sysInfo.Profile.PackageManager)
	fmt.Println()

	fmt.Println("Detecting installed agents...")
	registry := agents.NewDefaultRegistry()
	for _, adapter := range registry.All() {
		installed, binaryPath, _, configFound, err := adapter.Detect(homeDir)
		if err != nil {
			fmt.Printf("  [!] %-18s error: %v\n", adapter.Agent(), err)
			continue
		}
		if installed {
			status := "installed"
			if configFound {
				status += " + configured"
			}
			fmt.Printf("  [+] %-18s %s (%s)\n", adapter.Agent(), status, binaryPath)
		} else {
			fmt.Printf("  [-] %-18s not found\n", adapter.Agent())
		}
	}

	return nil
}

func runInstall(args []string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}

	selection := model.Selection{
		Preset: model.PresetFull,
	}

	// Parse flags.
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--agent":
			if i+1 < len(args) {
				i++
				selection.Agents = append(selection.Agents, model.AgentID(args[i]))
			}
		case "--preset":
			if i+1 < len(args) {
				i++
				selection.Preset = model.PresetID(args[i])
			}
		case "--dry-run":
			selection.DryRun = true
		}
	}

	registry := agents.NewDefaultRegistry()

	// If no agents specified, auto-detect installed ones.
	if len(selection.Agents) == 0 {
		for _, adapter := range registry.All() {
			installed, _, _, _, _ := adapter.Detect(homeDir)
			if installed {
				selection.Agents = append(selection.Agents, adapter.Agent())
			}
		}
		if len(selection.Agents) == 0 {
			return fmt.Errorf("no agents detected. Install at least one AI agent first")
		}
		fmt.Printf("Auto-detected agents: %v\n", selection.Agents)
	}

	result, err := pipeline.Install(homeDir, registry, selection, Version, selection.DryRun)
	if err != nil {
		return err
	}

	fmt.Printf("\nInstallation complete.\n")
	fmt.Printf("  Components: %v\n", result.ComponentsDone)
	fmt.Printf("  Files changed: %d\n", len(result.FilesChanged))
	if result.BackupID != "" {
		fmt.Printf("  Backup: %s\n", result.BackupID)
	}
	if len(result.Errors) > 0 {
		fmt.Println("\nWarnings:")
		for _, e := range result.Errors {
			fmt.Printf("  %s\n", e)
		}
	}

	return nil
}

func printHelp() {
	fmt.Printf(`cortex-ia %s — AI agent ecosystem configurator

Usage:
  cortex-ia                  Launch interactive TUI
  cortex-ia install          Install ecosystem (auto-detect agents)
  cortex-ia install --agent claude-code --preset full
  cortex-ia install --dry-run
  cortex-ia detect           Detect installed agents and system info
  cortex-ia version          Show version
  cortex-ia help             Show this help

Presets:
  full      All 8 components (default)
  minimal   Cortex + ForgeSpec + Context7 + SDD
  custom    Select components manually (TUI)

Components:
  cortex           Persistent cross-session memory (19 MCP tools)
  cli-orchestrator  Multi-CLI routing with circuit breaker (4 MCP tools)
  agent-mailbox    Inter-agent messaging system (9 MCP tools)
  forgespec        SDD contract validation + task board (15 MCP tools)
  context7         Live framework/library documentation
  sdd              Full 9-phase SDD workflow with orchestrator
  skills           Utility skills (non-SDD)
  conventions      Shared cortex conventions and memory protocol

`, Version)
}
