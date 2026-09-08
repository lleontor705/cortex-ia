package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/delegation"
)

func runUI(args []string) error {
	if len(args) == 0 || isHelp(args[0]) {
		fmt.Println("Usage: cortex-ia ui snapshot [--project <path>] [--session-id <id>] [--root-session-id <id>]")
		return nil
	}
	if strings.ToLower(args[0]) != "snapshot" {
		return fmt.Errorf("unknown ui command (usage: cortex-ia ui snapshot [--project <path>])")
	}
	opts, positionals, err := workOptions(args[1:], map[string]bool{"--project": false, "--session-id": false, "--root-session-id": false})
	if err != nil || len(positionals) != 0 {
		return fmt.Errorf("invalid ui snapshot arguments (usage: cortex-ia ui snapshot [--project <path>])")
	}
	projectRoot, err := delegation.ResolveProjectRoot(oneOption(opts, "--project"))
	if err != nil {
		return fmt.Errorf("resolve TUI project: %w", err)
	}
	home, err := cortexStateHome()
	if err != nil {
		return err
	}
	store, err := delegation.OpenStore(delegation.DefaultDBPath(home))
	if err != nil {
		return err
	}
	defer func() { _ = store.Close() }()
	dashboard, err := store.DashboardForConversation(context.Background(), projectRoot, oneOption(opts, "--session-id"), oneOption(opts, "--root-session-id"))
	if err != nil {
		return err
	}

	return printJSON(dashboard)
}
