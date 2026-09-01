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
		fmt.Println("  create <destination-path> [--repo <repo-path>]    Create a clean isolated worktree")
		fmt.Println("  clean <worktree-path>                            Reset and clean a worktree")
		fmt.Println("  drop <worktree-path> [--repo <repo-path>]        Remove an ephemeral worktree")
		return nil
	}

	sub := strings.ToLower(args[0])
	switch sub {
	case "create":
		if len(args) < 2 {
			return errors.New("usage: cortex-ia worktree create <destination-path> [--repo <repo-path>]")
		}
		dest := args[1]
		repo := "."
		if len(args) >= 4 && args[2] == "--repo" {
			repo = args[3]
		}
		path, err := delegation.CreateEphemeralWorktree(repo, dest)
		if err != nil {
			return err
		}
		return printJSON(map[string]string{"worktree": path, "status": "ready"})

	case "clean":
		if len(args) < 2 {
			return errors.New("usage: cortex-ia worktree clean <worktree-path>")
		}
		if err := delegation.CleanWorktree(args[1]); err != nil {
			return err
		}
		return printJSON(map[string]string{"worktree": args[1], "status": "clean"})

	case "drop", "delete", "remove":
		if len(args) < 2 {
			return errors.New("usage: cortex-ia worktree drop <worktree-path> [--repo <repo-path>]")
		}
		dest := args[1]
		repo := ""
		if len(args) >= 4 && args[2] == "--repo" {
			repo = args[3]
		}
		if err := delegation.DropEphemeralWorktree(repo, dest); err != nil {
			return err
		}
		return printJSON(map[string]string{"worktree": dest, "status": "dropped"})

	default:
		return fmt.Errorf("unknown worktree subcommand %q (see 'cortex-ia worktree --help')", args[0])
	}
}
