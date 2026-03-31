package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/agents"
	"github.com/lleontor705/cortex-ia/internal/config"
	"github.com/lleontor705/cortex-ia/internal/model"
	"github.com/lleontor705/cortex-ia/internal/pipeline"
	"github.com/lleontor705/cortex-ia/internal/state"
	"github.com/lleontor705/cortex-ia/internal/system"
	"github.com/lleontor705/cortex-ia/internal/tui"
	"github.com/lleontor705/cortex-ia/internal/update"
	"github.com/lleontor705/cortex-ia/internal/verify"
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

func runDetect() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}

	fmt.Println("Detecting system...")
	sysInfo := system.Detect()
	fmt.Printf("  OS: %s/%s\n", sysInfo.OS, sysInfo.Arch)
	fmt.Printf("  Package Manager: %s\n", sysInfo.Profile.PackageManager)
	fmt.Printf("  Shell: %s\n", sysInfo.Tools.Shell)

	fmt.Println("\nRuntime dependencies:")
	printToolStatus("  Node.js", sysInfo.Tools.NodeVersion)
	printToolStatus("  npx", boolToVersion(sysInfo.Tools.NpxAvailable))
	printToolStatus("  Git", sysInfo.Tools.GitVersion)
	printToolStatus("  Go", sysInfo.Tools.GoVersion)
	printToolStatus("  Cortex MCP", boolToVersion(sysInfo.Tools.CortexFound))
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
			if i+1 >= len(args) {
				return fmt.Errorf("flag --agent requires a value")
			}
			i++
			selection.Agents = append(selection.Agents, model.AgentID(args[i]))
		case "--preset":
			if i+1 >= len(args) {
				return fmt.Errorf("flag --preset requires a value")
			}
			i++
			selection.Preset = model.PresetID(args[i])
		case "--model-preset":
			if i+1 >= len(args) {
				return fmt.Errorf("flag --model-preset requires a value (balanced, performance, economy)")
			}
			i++
			selection.ModelAssignments = model.ModelsForPreset(model.ModelPreset(args[i]))
		case "--persona":
			if i+1 >= len(args) {
				return fmt.Errorf("flag --persona requires a value (professional, mentor, minimal)")
			}
			i++
			selection.Persona = model.PersonaID(args[i])
		case "--local":
			cwd, _ := os.Getwd()
			cfg, _, _ := config.FindProjectConfig(cwd)
			if cfg != nil {
				config.ApplyToSelection(cfg, &selection)
				fmt.Println("Loaded project config from .cortex-ia.yaml")
			} else {
				fmt.Println("No .cortex-ia.yaml found. Run 'cortex-ia init' first.")
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

	result, installErr := pipeline.Install(homeDir, registry, selection, Version, selection.DryRun)

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

	return installErr
}

func printHelp() {
	fmt.Printf(`cortex-ia %s — AI agent ecosystem configurator

Usage:
  cortex-ia                  Launch interactive TUI
  cortex-ia install          Install ecosystem (auto-detect agents)
  cortex-ia install --agent claude-code --preset full
  cortex-ia install --model-preset balanced|performance|economy
  cortex-ia install --persona professional|mentor|minimal
  cortex-ia install --local           Use project .cortex-ia.yaml config
  cortex-ia install --dry-run
  cortex-ia init                     Create .cortex-ia.yaml in current dir
  cortex-ia skill add <path>         Add community skill
  cortex-ia skill list               List community skills
  cortex-ia skill remove <name>      Remove community skill
  cortex-ia sync             Refresh managed files from current state
  cortex-ia sync --persona mentor
  cortex-ia detect           Detect installed agents and system info
  cortex-ia config           Show current configuration
  cortex-ia list agents      List detected agents
  cortex-ia list components  List installed components
  cortex-ia list backups     List available backups
  cortex-ia doctor           Verify installed files from lockfile
  cortex-ia verify           Alias of doctor
  cortex-ia repair           Re-apply managed files from lockfile/state
  cortex-ia rollback         Restore managed files from the last backup
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
  cli-orchestrator  Multi-CLI routing with circuit breaker (4 MCP tools)
  agent-mailbox    Inter-agent messaging system (9 MCP tools)
  forgespec        SDD contract validation + task board (15 MCP tools)
  context7         Live framework/library documentation
  sdd              Full 9-phase SDD workflow with orchestrator
  skills           Utility skills (non-SDD)
  conventions      Shared cortex conventions and memory protocol

`, Version)
}

func runDoctor() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}

	s, err := state.Load(homeDir)
	if err != nil {
		return err
	}
	lock, err := state.LoadLock(homeDir)
	if err != nil {
		return err
	}

	if len(s.InstalledAgents) == 0 && len(lock.InstalledAgents) == 0 {
		return fmt.Errorf("no cortex-ia installation metadata found")
	}

	registry := agents.NewDefaultRegistry()
	ctx := &verify.Context{HomeDir: homeDir, State: s, Lock: lock, Registry: registry}
	report := verify.Run(ctx, verify.DefaultChecks())

	fmt.Println("cortex-ia doctor")
	if len(lock.InstalledAgents) > 0 {
		fmt.Printf("  Agents: %v\n", lock.InstalledAgents)
		fmt.Printf("  Components: %v\n", lock.Components)
	}
	fmt.Println()

	for _, r := range report.Results {
		if r.Passed {
			fmt.Printf("  [+] %s\n", r.Name)
		} else if r.Severity == verify.SeverityWarning {
			fmt.Printf("  [~] %s: %s\n", r.Name, r.Message)
		} else {
			fmt.Printf("  [!] %s: %s\n", r.Name, r.Message)
		}
	}

	fmt.Printf("\n  %d passed, %d failed, %d warnings\n", report.Passed, report.Failed, report.Warned)

	if report.HasErrors() {
		return fmt.Errorf("doctor detected %d error(s)", report.Failed)
	}
	return nil
}

func runRepair(args []string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}

	dryRun := false
	for _, arg := range args {
		if arg == "--dry-run" {
			dryRun = true
		}
	}

	registry := agents.NewDefaultRegistry()
	fmt.Println("Reapplying cortex-ia installation from lockfile/state...")

	result, repairErr := pipeline.Repair(homeDir, registry, Version, dryRun)

	fmt.Printf("\nRepair complete.\n")
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

	return repairErr
}

func runSync(args []string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}

	s, err := state.Load(homeDir)
	if err != nil {
		return err
	}
	lock, err := state.LoadLock(homeDir)
	if err != nil {
		return err
	}

	sel, err := pipeline.SelectionFromState(s, lock)
	if err != nil {
		return err
	}

	// Parse optional flags.
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--dry-run":
			sel.DryRun = true
		case "--persona":
			if i+1 >= len(args) {
				return fmt.Errorf("flag --persona requires a value")
			}
			i++
			sel.Persona = model.PersonaID(args[i])
		}
	}

	registry := agents.NewDefaultRegistry()
	fmt.Println("Syncing cortex-ia installation...")

	result, err := pipeline.Install(homeDir, registry, sel, Version, sel.DryRun)

	fmt.Printf("\nSync complete.\n")
	fmt.Printf("  Components: %v\n", result.ComponentsDone)
	fmt.Printf("  Files changed: %d\n", len(result.FilesChanged))
	if len(result.Errors) > 0 {
		fmt.Println("\nWarnings:")
		for _, e := range result.Errors {
			fmt.Printf("  %s\n", e)
		}
	}
	return err
}

func runConfig() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}

	s, err := state.Load(homeDir)
	if err != nil {
		return err
	}
	lock, err := state.LoadLock(homeDir)
	if err != nil {
		return err
	}

	fmt.Println("cortex-ia configuration")
	fmt.Printf("  Version: %s\n", s.Version)
	fmt.Printf("  Preset: %s\n", s.Preset)
	fmt.Printf("  Agents: %v\n", s.InstalledAgents)
	fmt.Printf("  Components: %v\n", s.Components)
	fmt.Printf("  Last install: %s\n", s.LastInstall.Format("2006-01-02 15:04:05"))
	fmt.Printf("  Last backup: %s\n", s.LastBackupID)
	fmt.Printf("  Tracked files: %d\n", len(lock.Files))
	fmt.Printf("  State: %s\n", state.StatePath(homeDir))
	fmt.Printf("  Lock: %s\n", state.LockPath(homeDir))
	fmt.Printf("  Skills: %s\n", state.SharedSkillsDir(homeDir))
	fmt.Printf("  Prompts: %s\n", state.SharedPromptsDir(homeDir))
	return nil
}

func runList(what string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}

	switch what {
	case "agents":
		registry := agents.NewDefaultRegistry()
		fmt.Println("Detected agents:")
		for _, adapter := range registry.All() {
			installed, binaryPath, _, _, _ := adapter.Detect(homeDir)
			if installed {
				fmt.Printf("  [+] %-18s %s\n", adapter.Agent(), binaryPath)
			} else {
				fmt.Printf("  [-] %-18s not found\n", adapter.Agent())
			}
		}

	case "components":
		s, err := state.Load(homeDir)
		if err != nil {
			return err
		}
		if len(s.Components) == 0 {
			fmt.Println("No components installed. Run 'cortex-ia install' first.")
			return nil
		}
		fmt.Println("Installed components:")
		for _, c := range s.Components {
			fmt.Printf("  [+] %s\n", c)
		}

	case "backups":
		backupsDir := state.SharedSkillsDir(homeDir) // reuse .cortex-ia base
		backupsDir = strings.Replace(backupsDir, "skills", "backups", 1)
		entries, err := os.ReadDir(backupsDir)
		if err != nil {
			fmt.Println("No backups found.")
			return nil
		}
		fmt.Println("Available backups:")
		for _, e := range entries {
			if e.IsDir() {
				fmt.Printf("  %s\n", e.Name())
			}
		}

	case "all":
		fmt.Println("Usage: cortex-ia list <agents|components|backups>")

	default:
		return fmt.Errorf("unknown list target: %s (use: agents, components, backups)", what)
	}
	return nil
}

func runSkill(action string, args []string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	communityDir := filepath.Join(homeDir, ".cortex-ia", "skills-community")

	switch action {
	case "list":
		fmt.Println("Community skills:")
		entries, err := os.ReadDir(communityDir)
		if err != nil {
			fmt.Println("  (none)")
			return nil
		}
		for _, e := range entries {
			if e.IsDir() {
				skillFile := filepath.Join(communityDir, e.Name(), "SKILL.md")
				if _, err := os.Stat(skillFile); err == nil {
					fmt.Printf("  [+] %s\n", e.Name())
				}
			}
		}
		return nil

	case "add":
		if len(args) == 0 {
			return fmt.Errorf("usage: cortex-ia skill add <path-to-skill-dir>")
		}
		srcDir := args[0]
		srcSkill := filepath.Join(srcDir, "SKILL.md")
		if _, err := os.Stat(srcSkill); os.IsNotExist(err) {
			return fmt.Errorf("no SKILL.md found in %s", srcDir)
		}
		skillName := filepath.Base(srcDir)
		dstDir := filepath.Join(communityDir, skillName)
		if err := os.MkdirAll(dstDir, 0o755); err != nil {
			return err
		}

		data, err := os.ReadFile(srcSkill)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dstDir, "SKILL.md"), data, 0o644); err != nil {
			return err
		}
		fmt.Printf("Added community skill: %s\n", skillName)
		fmt.Println("Run 'cortex-ia sync' to deploy it.")
		return nil

	case "remove":
		if len(args) == 0 {
			return fmt.Errorf("usage: cortex-ia skill remove <skill-name>")
		}
		skillName := args[0]
		skillDir := filepath.Join(communityDir, skillName)
		if _, err := os.Stat(skillDir); os.IsNotExist(err) {
			return fmt.Errorf("skill %q not found in community directory", skillName)
		}
		if err := os.RemoveAll(skillDir); err != nil {
			return err
		}
		fmt.Printf("Removed community skill: %s\n", skillName)
		return nil

	default:
		return fmt.Errorf("unknown skill action: %s (use: add, list, remove)", action)
	}
}

func runInit() error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("cannot determine working directory: %w", err)
	}

	path, err := config.WriteDefault(cwd)
	if err != nil {
		return err
	}
	fmt.Printf("Created %s\n", path)
	fmt.Println("Edit this file to customize cortex-ia for your project.")
	fmt.Println("Then run: cortex-ia install --local")
	return nil
}

func runAutoInstall(args []string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}

	sysInfo := system.Detect()
	registry := agents.NewDefaultRegistry()

	dryRun := false
	for _, arg := range args {
		if arg == "--dry-run" {
			dryRun = true
		}
	}

	fmt.Println("Auto-installing missing agents...")
	installed := 0

	for _, adapter := range registry.All() {
		isInstalled, _, _, _, _ := adapter.Detect(homeDir)
		if isInstalled {
			fmt.Printf("  [+] %-18s already installed\n", adapter.Agent())
			continue
		}

		if !adapter.SupportsAutoInstall() {
			fmt.Printf("  [-] %-18s no auto-install available (desktop app)\n", adapter.Agent())
			continue
		}

		commands := adapter.InstallCommands(sysInfo.Profile)
		if len(commands) == 0 {
			fmt.Printf("  [-] %-18s no install commands for %s\n", adapter.Agent(), sysInfo.Profile.PackageManager)
			continue
		}

		if dryRun {
			for _, cmd := range commands {
				fmt.Printf("  [~] %-18s would run: %s\n", adapter.Agent(), strings.Join(cmd, " "))
			}
			continue
		}

		fmt.Printf("  [*] %-18s installing...\n", adapter.Agent())
		for _, cmd := range commands {
			fmt.Printf("      $ %s\n", strings.Join(cmd, " "))
			proc := exec.Command(cmd[0], cmd[1:]...)
			proc.Stdout = os.Stdout
			proc.Stderr = os.Stderr
			if err := proc.Run(); err != nil {
				fmt.Printf("  [!] %-18s install failed: %v\n", adapter.Agent(), err)
				break
			}
		}
		installed++
	}

	if dryRun {
		fmt.Println("\n(dry-run — no changes made)")
	} else {
		fmt.Printf("\n%d agent(s) installed.\n", installed)
	}
	return nil
}

func runUpdate() error {
	fmt.Println("Checking for updates...")
	result := update.Check(Version)
	fmt.Println(update.FormatCheckResult(result))
	if result.Error != nil {
		return result.Error
	}
	return nil
}

func printToolStatus(label, version string) {
	if version != "" {
		fmt.Printf("  [+] %-14s %s\n", label, version)
	} else {
		fmt.Printf("  [-] %-14s not found\n", label)
	}
}

func boolToVersion(found bool) string {
	if found {
		return "available"
	}
	return ""
}

func runRollback(args []string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}

	var backupID string
	for i := 0; i < len(args); i++ {
		if args[i] == "--backup" && i+1 < len(args) {
			i++
			backupID = args[i]
		}
	}

	manifest, err := pipeline.Rollback(homeDir, backupID)
	if err != nil {
		return err
	}

	fmt.Println("Rollback complete.")
	fmt.Printf("  Backup: %s\n", manifest.ID)
	fmt.Printf("  Files restored: %d\n", manifest.FileCount)
	return nil
}
