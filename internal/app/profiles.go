package app

import (
	"fmt"
	"os"

	"github.com/lleontor705/cortex-ia/internal/components/sdd"
	"github.com/lleontor705/cortex-ia/internal/opencode"
	"github.com/lleontor705/cortex-ia/internal/state"
)

// runProfiles implements `cortex-ia profiles <subcmd> [args]`.
//
// Subcommands:
//
//	list                                                     show every saved profile
//	create <name>:<provider>/<model>                         set ALL SDD phases to the given model
//	set    <name>:<phase>:<provider>/<model>                 override a single phase
//	delete <name>                                            remove a profile
func runProfiles(args []string) error {
	if len(args) == 0 {
		return printProfilesUsage()
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}

	switch args[0] {
	case "list", "ls":
		return profilesList(homeDir)

	case "create", "add":
		if len(args) < 2 {
			return fmt.Errorf("usage: cortex-ia profiles create <name>:<provider>/<model>")
		}
		return profilesCreate(homeDir, args[1])

	case "set":
		if len(args) < 2 {
			return fmt.Errorf("usage: cortex-ia profiles set <name>:<phase>:<provider>/<model>")
		}
		return profilesSet(homeDir, args[1])

	case "delete", "rm", "remove":
		if len(args) < 2 {
			return fmt.Errorf("usage: cortex-ia profiles delete <name>")
		}
		return profilesDelete(homeDir, args[1])

	case "apply":
		if len(args) < 2 {
			return fmt.Errorf("usage: cortex-ia profiles apply <name>")
		}
		return profilesApply(homeDir, args[1])

	case "help", "--help", "-h":
		return printProfilesUsage()

	default:
		return fmt.Errorf("profiles: unknown subcommand %q", args[0])
	}
}

func printProfilesUsage() error {
	fmt.Println("Usage:")
	fmt.Println("  cortex-ia profiles list")
	fmt.Println("  cortex-ia profiles create <name>:<provider>/<model>")
	fmt.Println("  cortex-ia profiles set    <name>:<phase>:<provider>/<model>")
	fmt.Println("  cortex-ia profiles delete <name>")
	fmt.Println("  cortex-ia profiles apply  <name>   Write the profile's per-phase models into opencode.json")
	return nil
}

func profilesList(homeDir string) error {
	profs, err := state.LoadProfiles(homeDir)
	if err != nil {
		return err
	}
	if len(profs) == 0 {
		fmt.Println("No profiles saved. Create one with `cortex-ia profiles create`.")
		return nil
	}
	fmt.Printf("Profiles (%d):\n", len(profs))
	for _, p := range profs {
		fmt.Printf("  %s\n", sdd.ProfileSummary(p))
	}
	return nil
}

func profilesCreate(homeDir, spec string) error {
	p, err := sdd.ParseProfileSpec(spec)
	if err != nil {
		return err
	}
	profs, err := state.LoadProfiles(homeDir)
	if err != nil {
		return err
	}
	profs = sdd.UpsertProfile(profs, p)
	if err := state.SaveProfiles(homeDir, profs); err != nil {
		return err
	}
	fmt.Printf("Profile %q created — every SDD phase mapped.\n", p.Name)
	return nil
}

func profilesSet(homeDir, spec string) error {
	name, phase, providerModel, err := sdd.ParseProfilePhaseSpec(spec)
	if err != nil {
		return err
	}
	profs, err := state.LoadProfiles(homeDir)
	if err != nil {
		return err
	}
	profs = sdd.SetProfilePhase(profs, name, phase, providerModel)
	if err := state.SaveProfiles(homeDir, profs); err != nil {
		return err
	}
	fmt.Printf("Profile %q: phase %q → %s.\n", name, phase, providerModel)
	return nil
}

func profilesApply(homeDir, name string) error {
	profs, err := state.LoadProfiles(homeDir)
	if err != nil {
		return err
	}
	p, ok := sdd.FindProfile(profs, name)
	if !ok {
		return fmt.Errorf("profile %q not found", name)
	}

	assignments := sdd.ProfileToOpenCodeAssignments(p)
	if len(assignments) == 0 {
		return fmt.Errorf("profile %q has no usable phase assignments — every value was empty or unparseable", name)
	}

	if err := opencode.ApplyToOpenCodeConfig(homeDir, assignments); err != nil {
		return fmt.Errorf("apply profile to opencode.json: %w", err)
	}

	fmt.Printf("Profile %q applied to opencode.json (%d phase assignment(s)).\n", name, len(assignments))
	for phase, a := range assignments {
		fmt.Printf("  sdd-%-9s → %s\n", phase, a.FormatOpenCodeModel())
	}
	return nil
}

func profilesDelete(homeDir, name string) error {
	profs, err := state.LoadProfiles(homeDir)
	if err != nil {
		return err
	}
	updated, removed := sdd.RemoveProfile(profs, name)
	if !removed {
		return fmt.Errorf("profile %q not found", name)
	}
	if err := state.SaveProfiles(homeDir, updated); err != nil {
		return err
	}
	fmt.Printf("Profile %q removed.\n", name)
	return nil
}
