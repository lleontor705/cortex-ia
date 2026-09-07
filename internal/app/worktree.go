package app

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/delegation"
)

func getOptionalStore() (*delegation.Store, func()) {
	home, err := cortexStateHome()
	if err != nil {
		return nil, func() {}
	}
	store, err := delegation.OpenStore(delegation.DefaultDBPath(home))
	if err != nil {
		return nil, func() {}
	}
	return store, func() { _ = store.Close() }
}

func runWorktree(args []string) error {
	if len(args) == 0 || isHelp(args[0]) {
		fmt.Println("Usage: cortex-ia worktree <subcommand> [options]")
		fmt.Println("\nSubcommands:")
		fmt.Println("  list [--repo <repo-path>]                                    List authoritative git worktrees")
		fmt.Println("  validate <worktree-path> [--repo <repo>] [--head <commit>]   Validate worktree contract against git porcelain")
		fmt.Println("  create (retired)                                             isolated_worktree strategy is retired; use current_workspace")
		fmt.Println("  clean <worktree-path>                                       Reset and clean a worktree")
		fmt.Println("  drop <worktree-path> [--repo <repo-path>]                   Remove an ephemeral worktree")
		fmt.Println("  prune [--repo <repo-path>]                                  Clean unreferenced or stale worktrees")
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

	case "create":
		return errors.New("isolated_worktree strategy is retired; use current_workspace")

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
		for i := 2; i < len(args); i++ {
			if args[i] == "--repo" && i+1 < len(args) {
				repo = args[i+1]
				i++
			}
		}
		if err := delegation.DropEphemeralWorktree(repo, dest); err != nil {
			return err
		}
		store, cleanup := getOptionalStore()
		defer cleanup()
		if store != nil {
			_ = store.UpdateManagedWorktreeStatus(context.Background(), dest, "released")
		}
		return printJSON(map[string]string{"worktree": dest, "status": "dropped"})

	case "prune":
		repo := "."
		for i := 1; i < len(args); i++ {
			if args[i] == "--repo" && i+1 < len(args) {
				repo = args[i+1]
				i++
			}
		}
		pruned, err := delegation.PruneOrphanWorktrees(repo)
		if err != nil {
			return err
		}
		store, cleanup := getOptionalStore()
		defer cleanup()
		if store != nil {
			for _, p := range pruned {
				_ = store.UpdateManagedWorktreeStatus(context.Background(), p, "pruned")
			}
		}
		if pruned == nil {
			pruned = []string{}
		}
		return printJSON(map[string]any{
			"pruned": pruned,
			"status": "ok",
		})

	default:
		return fmt.Errorf("unknown worktree subcommand %q (see 'cortex-ia worktree --help')", args[0])
	}
}
