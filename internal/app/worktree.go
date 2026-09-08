package app

import (
	"errors"
	"fmt"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/delegation"
)

func runWorktree(args []string) error {
	if len(args) == 0 || isHelp(args[0]) {
		fmt.Println("Usage: cortex-ia worktree <subcommand> [options]")
		fmt.Println("\nSubcommands:")
		fmt.Println("  list [--repo <repo-path>]                                    List authoritative git worktrees")
		fmt.Println("  validate <worktree-path> [--repo <repo>] [--head <commit>]   Validate worktree contract against git porcelain")
		fmt.Println("\nRetired: create, clean, drop, delete, remove, prune; use current_workspace. Existing worktrees are preserved.")
		return nil
	}

	sub := strings.ToLower(args[0])
	switch sub {
	case "list":
		repo := "."
		for i := 1; i < len(args); i++ {
			if args[i] == "--repo" && i+1 < len(args) {
				repo = args[i+1]
				i++
			}
		}
		records, err := delegation.ListWorktrees(repo)
		if err != nil {
			return err
		}
		return printJSON(records)

	case "validate":
		if len(args) < 2 {
			return errors.New("usage: cortex-ia worktree validate <worktree-path> [--repo <repo-path>] [--head <commit>]")
		}
		dest := args[1]
		repo := "."
		var expectedHEAD string
		for i := 2; i < len(args); i++ {
			if args[i] == "--repo" && i+1 < len(args) {
				repo = args[i+1]
				i++
			} else if args[i] == "--head" && i+1 < len(args) {
				expectedHEAD = args[i+1]
				i++
			}
		}
		record, err := delegation.ValidateWorktreeContract(repo, dest, expectedHEAD)
		if err != nil {
			return err
		}
		return printJSON(map[string]any{
			"valid":    true,
			"worktree": record.Path,
			"head":     record.HEAD,
			"branch":   record.Branch,
			"detached": record.Detached,
		})

	case "create", "clean", "drop", "delete", "remove", "prune":
		return fmt.Errorf("worktree %s is retired; isolated_worktree strategy is retired; use current_workspace. Existing worktrees are preserved; use worktree list or validate for read-only inspection", sub)

	default:
		return fmt.Errorf("unknown worktree subcommand %q (see 'cortex-ia worktree --help')", args[0])
	}
}
