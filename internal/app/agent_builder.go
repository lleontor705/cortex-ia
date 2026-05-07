package app

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/lleontor705/cortex-ia/internal/agentbuilder"
	"github.com/lleontor705/cortex-ia/internal/agents"
	"github.com/lleontor705/cortex-ia/internal/model"
	"github.com/lleontor705/cortex-ia/internal/state"
)

// runAgentBuilder dispatches `cortex-ia agent-builder <subcmd>`.
func runAgentBuilder(args []string) error {
	if len(args) == 0 {
		return printAgentBuilderUsage()
	}

	switch args[0] {
	case "list", "ls":
		return agentBuilderList()
	case "create", "new":
		return agentBuilderCreate(args[1:])
	case "remove", "rm", "delete":
		if len(args) < 2 {
			return fmt.Errorf("usage: cortex-ia agent-builder remove <name>")
		}
		return agentBuilderRemove(args[1])
	case "help", "--help", "-h":
		return printAgentBuilderUsage()
	default:
		return fmt.Errorf("agent-builder: unknown subcommand %q", args[0])
	}
}

func printAgentBuilderUsage() error {
	fmt.Println("Usage:")
	fmt.Println("  cortex-ia agent-builder list")
	fmt.Println("  cortex-ia agent-builder create --engine <id> --purpose <text> [flags]")
	fmt.Println("  cortex-ia agent-builder remove <name>")
	fmt.Println()
	fmt.Println("Flags for create:")
	fmt.Println("  --engine <id>            claude|opencode|gemini|codex (required)")
	fmt.Println("  --purpose <text>         natural-language description of the skill (required)")
	fmt.Println("  --sdd <mode>             standalone|phase-support|new-phase  (default: standalone)")
	fmt.Println("  --phase <id>             SDD phase id; required when --sdd is phase-support|new-phase")
	fmt.Println("  --target <agent-id>      may repeat; defaults to all installed adapters")
	fmt.Println("  --persona <id>           professional|mentor|minimal (default: professional)")
	fmt.Println("  --timeout <seconds>      engine subprocess timeout (default: 120)")
	fmt.Println("  --dry-run                print the prompt that would be sent and exit")
	return nil
}

func agentBuilderList() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	regPath, err := state.AgentBuilderRegistryPath(home)
	if err != nil {
		return err
	}
	reg, err := agentbuilder.LoadRegistry(regPath)
	if err != nil {
		return err
	}
	if len(reg.Agents) == 0 {
		fmt.Println("No custom skills built yet. Run `cortex-ia agent-builder create` to make one.")
		return nil
	}
	fmt.Printf("Custom skills (%d):\n", len(reg.Agents))
	for _, e := range reg.Agents {
		fmt.Printf("  %-30s engine=%s sdd=%s targets=%d\n",
			e.Name, e.Engine, e.SDDMode, len(e.Targets))
	}
	return nil
}

func agentBuilderRemove(name string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	regPath, err := state.AgentBuilderRegistryPath(home)
	if err != nil {
		return err
	}
	reg, err := agentbuilder.LoadRegistry(regPath)
	if err != nil {
		return err
	}
	if !reg.RemoveByName(name) {
		return fmt.Errorf("agent-builder: %q is not in the registry", name)
	}
	if err := agentbuilder.SaveRegistry(regPath, reg); err != nil {
		return err
	}
	fmt.Printf("Removed %q from registry. (Existing SKILL.md files in agent dirs left in place — delete them manually if needed.)\n", name)
	return nil
}

func agentBuilderCreate(args []string) error {
	cfg := struct {
		engineID  string
		purpose   string
		sddMode   string
		sddPhase  string
		targets   []string
		persona   string
		timeoutS  int
		dryRun    bool
	}{
		sddMode:  "standalone",
		persona:  string(model.PersonaProfessional),
		timeoutS: 120,
	}

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--engine":
			i = consumeFlag(args, i, "--engine", &cfg.engineID)
		case "--purpose":
			i = consumeFlag(args, i, "--purpose", &cfg.purpose)
		case "--sdd":
			i = consumeFlag(args, i, "--sdd", &cfg.sddMode)
		case "--phase":
			i = consumeFlag(args, i, "--phase", &cfg.sddPhase)
		case "--persona":
			i = consumeFlag(args, i, "--persona", &cfg.persona)
		case "--target":
			if i+1 >= len(args) {
				return fmt.Errorf("--target requires a value")
			}
			i++
			cfg.targets = append(cfg.targets, args[i])
		case "--timeout":
			if i+1 >= len(args) {
				return fmt.Errorf("--timeout requires a value")
			}
			i++
			var seconds int
			if _, err := fmt.Sscanf(args[i], "%d", &seconds); err != nil {
				return fmt.Errorf("invalid --timeout: %w", err)
			}
			cfg.timeoutS = seconds
		case "--dry-run":
			cfg.dryRun = true
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}

	if cfg.engineID == "" {
		return fmt.Errorf("--engine is required (claude|opencode|gemini|codex)")
	}
	if strings.TrimSpace(cfg.purpose) == "" {
		return fmt.Errorf("--purpose is required")
	}

	engine := agentbuilder.NewEngine(model.AgentID(cfg.engineID))
	if engine == nil {
		return fmt.Errorf("unknown engine %q", cfg.engineID)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	registry := agents.NewDefaultRegistry()

	// Resolve target adapters.
	targetIDs := make([]model.AgentID, 0, len(cfg.targets))
	if len(cfg.targets) > 0 {
		for _, t := range cfg.targets {
			id := model.AgentID(t)
			if _, err := registry.Get(id); err != nil {
				return fmt.Errorf("unknown target agent %q", t)
			}
			targetIDs = append(targetIDs, id)
		}
	} else {
		for _, ia := range agents.DiscoverInstalled(registry, home) {
			targetIDs = append(targetIDs, ia.ID)
		}
	}
	if len(targetIDs) == 0 {
		return fmt.Errorf("no target agents found (use --target or install at least one supported agent)")
	}

	sddIntegration := &agentbuilder.SDDIntegration{
		Mode:  agentbuilder.SDDIntegrationMode(cfg.sddMode),
		Phase: cfg.sddPhase,
	}
	if (sddIntegration.Mode == agentbuilder.SDDPhaseSupport || sddIntegration.Mode == agentbuilder.SDDNewPhase) && cfg.sddPhase == "" {
		return fmt.Errorf("--phase is required when --sdd is %s", cfg.sddMode)
	}

	prompt := agentbuilder.ComposePrompt(
		cfg.purpose,
		sddIntegration,
		targetIDs,
		model.PersonaID(cfg.persona),
		nil,
	)

	if cfg.dryRun {
		fmt.Println("--- Prompt that would be sent to engine ---")
		fmt.Println(prompt)
		fmt.Println("--- End prompt ---")
		return nil
	}

	if !engine.Available() {
		return fmt.Errorf("engine %q binary not found on PATH", cfg.engineID)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.timeoutS)*time.Second)
	defer cancel()

	fmt.Printf("Generating skill via %s (timeout %ds)…\n", cfg.engineID, cfg.timeoutS)
	raw, err := engine.Generate(ctx, prompt)
	if err != nil {
		return fmt.Errorf("engine generate: %w", err)
	}

	parsed, err := agentbuilder.Parse(raw)
	if err != nil {
		return fmt.Errorf("parse engine output: %w", err)
	}

	if agentbuilder.HasConflictWithBuiltin(parsed.Name) {
		return fmt.Errorf("generated skill name %q collides with a built-in cortex-ia skill — re-run with a different --purpose phrasing", parsed.Name)
	}

	parsed.SDDConfig = sddIntegration
	fmt.Printf("Parsed skill: %s — %q\n", parsed.Name, parsed.Title)

	// Build adapter info via a closure over the agents registry.
	infos := agentbuilder.AdaptersFromRegistry(home,
		func(id model.AgentID) (string, string, bool, bool) {
			adapter, err := registry.Get(id)
			if err != nil {
				return "", "", false, false
			}
			return adapter.SkillsDir(home), adapter.SystemPromptFile(home), adapter.SupportsSkills(), true
		},
		targetIDs,
	)

	results, err := agentbuilder.InstallToAdapters(parsed, infos)
	for _, r := range results {
		marker := "[+]"
		if !r.Success {
			marker = "[-]"
		}
		fmt.Printf("  %s %-15s %s\n", marker, r.AgentID, r.Path)
	}
	if err != nil {
		return err
	}

	regPath, err := state.AgentBuilderRegistryPath(home)
	if err != nil {
		return err
	}
	reg, err := agentbuilder.LoadRegistry(regPath)
	if err != nil {
		return err
	}
	reg.Add(agentbuilder.RegistryEntry{
		Name:        parsed.Name,
		Title:       parsed.Title,
		Description: parsed.Description,
		Engine:      model.AgentID(cfg.engineID),
		SDDMode:     sddIntegration.Mode,
		SDDPhase:    sddIntegration.Phase,
		Targets:     targetIDs,
		Persona:     model.PersonaID(cfg.persona),
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
	})
	if err := agentbuilder.SaveRegistry(regPath, reg); err != nil {
		return err
	}

	fmt.Printf("\n✓ Skill %q registered. List with `cortex-ia agent-builder list`.\n", parsed.Name)
	return nil
}

// consumeFlag is a tiny helper so the create flag-parsing loop stays readable.
func consumeFlag(args []string, i int, name string, dst *string) int {
	if i+1 >= len(args) {
		fmt.Fprintf(os.Stderr, "%s requires a value\n", name)
		return i
	}
	*dst = args[i+1]
	return i + 1
}
