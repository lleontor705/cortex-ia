package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/skillregistry"
)

// runSkillRegistry handles the `cortex-ia skill-registry` subcommand.
// Default action is "refresh" when no action is specified.
func runSkillRegistry(args []string) error {
	action := "refresh"
	if len(args) > 0 {
		action = strings.ToLower(args[0])
	}

	switch action {
	case "refresh":
		return runSkillRegistryRefresh()
	default:
		return fmt.Errorf("unknown skill-registry action: %s (use: refresh)", action)
	}
}

// runSkillRegistryRefresh scans all skill tiers and writes the registry
// to .sdd/skill-registry.md in the current working directory.
func runSkillRegistryRefresh() error {
	out, err := skillregistry.Scan()
	if err != nil {
		return fmt.Errorf("scan skills: %w", err)
	}

	md := skillregistry.FormatMarkdown(out)

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	registryDir := filepath.Join(cwd, ".sdd")
	registryPath := filepath.Join(registryDir, "skill-registry.md")

	if err := os.MkdirAll(registryDir, 0o755); err != nil {
		return fmt.Errorf("create .sdd directory: %w", err)
	}

	if err := os.WriteFile(registryPath, []byte(md), 0o644); err != nil {
		return fmt.Errorf("write skill registry: %w", err)
	}

	fmt.Printf("Skill registry refreshed: %d skills written to %s\n", len(out.Skills), registryPath)
	return nil
}
