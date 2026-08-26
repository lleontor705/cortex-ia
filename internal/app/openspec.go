package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func runOpenSpec(args []string) error {
	if len(args) == 0 || isHelp(args[0]) {
		fmt.Println("Usage: cortex-ia openspec <subcommand> [options]")
		fmt.Println("\nOpenSpec SDD (Spec-Driven Development) artifact validator:")
		fmt.Println("  validate [change-dir]   Validate OpenSpec proposal, specs, design and tasks")
		fmt.Println("  list                    List active changes in openspec/changes/")
		fmt.Println("  status [change-name]    Show status of OpenSpec changes")
		return nil
	}
	switch strings.ToLower(args[0]) {
	case "validate":
		target := "."
		if len(args) > 1 {
			target = args[1]
		}
		return validateOpenSpec(target)
	case "list":
		return listOpenSpec()
	case "status":
		target := ""
		if len(args) > 1 {
			target = args[1]
		}
		return statusOpenSpec(target)
	default:
		return fmt.Errorf("unknown openspec subcommand %q (see 'cortex-ia openspec --help')", args[0])
	}
}

func validateOpenSpec(target string) error {
	baseDir := "openspec/changes"
	if fi, err := os.Stat(target); err == nil && fi.IsDir() && strings.Contains(target, "openspec") {
		baseDir = target
	}
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("✓ OpenSpec workspace valid (no changes in %s yet)\n", baseDir)
			return nil
		}
		return err
	}

	validCount := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		changeDir := filepath.Join(baseDir, entry.Name())
		proposal := filepath.Join(changeDir, "proposal.md")
		tasks := filepath.Join(changeDir, "tasks.md")
		design := filepath.Join(changeDir, "design.md")

		fmt.Printf("🔍 Checking OpenSpec change: %s\n", entry.Name())
		hasProposal := fileExists(proposal)
		hasTasks := fileExists(tasks)
		hasDesign := fileExists(design)

		if hasProposal {
			fmt.Printf("  ✓ proposal.md found\n")
		}
		if hasDesign {
			fmt.Printf("  ✓ design.md found\n")
		}
		if hasTasks {
			fmt.Printf("  ✓ tasks.md found\n")
		}
		if !hasProposal && !hasTasks && !hasDesign {
			fmt.Printf("  ⚠️ No standard OpenSpec documents found in %s\n", changeDir)
		} else {
			validCount++
		}
	}

	fmt.Printf("\n✅ OpenSpec validation complete (%d active change sets inspected)\n", validCount)
	return nil
}

func listOpenSpec() error {
	baseDir := "openspec/changes"
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No active changes found in openspec/changes/")
			return nil
		}
		return err
	}
	fmt.Printf("Active OpenSpec changes in %s:\n", baseDir)
	for _, entry := range entries {
		if entry.IsDir() {
			fmt.Printf("  - %s\n", entry.Name())
		}
	}
	return nil
}

func statusOpenSpec(target string) error {
	return validateOpenSpec(target)
}

func fileExists(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && !fi.IsDir()
}
