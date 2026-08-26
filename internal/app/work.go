package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/lleontor705/cortex-ia/internal/delegation"
)

func runWork(args []string) error {
	if len(args) == 0 || isHelp(args[0]) {
		fmt.Println("Usage: cortex-ia work <subcommand> [options]")
		fmt.Println("\nSubcommands:")
		fmt.Println("  create <id> <title> [--board <board>] [--depends <id>]...   Create a work task")
		fmt.Println("  list [--board <board-id>]                                   List work items")
		fmt.Println("  status <task-id>                                            Get task details")
		fmt.Println("  claim <task-id> --owner <owner> [--ttl <duration>]          Claim a task")
		fmt.Println("  renew <task-id> --claim-token <token> [--ttl <duration>]    Renew a claim")
		fmt.Println("  lease <task-id> --claim-token <token> --path <file>         Reserve a file lease")
		fmt.Println("  lease-renew --path <file> --lease-token <token>             Renew a file lease")
		fmt.Println("  release --path <file> --lease-token <token>                 Release a file lease")
		fmt.Println("  transition <task-id> --claim-token <token> --to <status>    Transition task state")
		fmt.Println("  approve <task-id> --reviewer <id> --verdict <PASS|FAIL>     Approve/review a task")
		fmt.Println("  retry <task-id> --revision <n>                              Retry a task")
		fmt.Println("  recover                                                     Recover expired claims/leases")
		return nil
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
	ctx := context.Background()
	sub := strings.ToLower(args[0])
	switch sub {
	case "create":
		if len(args) > 1 && isHelp(args[1]) {
			return workUsage("create <id> <title> [--board <board-id>] [--depends <task-id>]... OR create --id <id> --title <title> [--board <board-id>] [--depends <task-id>]...", nil)
		}
		opts, positionals, err := workOptions(args[1:], map[string]bool{"--id": false, "--title": false, "--depends": true, "--board": false})
		if err != nil {
			return workUsage("create <id> <title> [--board <board-id>] [--depends <task-id>]... OR create --id <id> --title <title> [--board <board-id>] [--depends <task-id>]...", err)
		}
		id := oneOption(opts, "--id")
		title := oneOption(opts, "--title")
		boardID := oneOption(opts, "--board")
		depends := opts["--depends"]
		if id == "" && len(positionals) > 0 {
			id = positionals[0]
			if len(positionals) > 1 {
				title = positionals[1]
			}
		}
		if id == "" || title == "" {
			return errors.New("usage: cortex-ia work create <id> <title> [--board <board-id>] [--depends <task-id>]... OR --id <id> --title <title> [--board <board-id>] [--depends <task-id>]...")
		}
		item, err := store.CreateWorkInBoard(ctx, boardID, id, title, depends)
		if err != nil {
			return err
		}
		return printJSON(item)
	case "list":
		if len(args) > 1 && isHelp(args[1]) {
			return workUsage("list [--board <board-id>]", nil)
		}
		opts, positionals, err := workOptions(args[1:], map[string]bool{"--board": false})
		if err != nil || len(positionals) != 0 {
			return workUsage("list [--board <board-id>]", err)
		}
		var items []delegation.WorkItem
		if boardID := oneOption(opts, "--board"); boardID != "" {
			items, err = store.ListWorkByBoard(ctx, boardID)
		} else {
			items, err = store.ListWork(ctx)
		}
		if err != nil {
			return err
		}
		return printJSON(items)
	case "status", "show", "get":
		if len(args) > 1 && isHelp(args[1]) {
			return workUsage("status <task-id>", nil)
		}
		id, err := oneWorkID(args[1:])
		if err != nil {
			return workUsage("status <task-id>", err)
		}
		item, err := store.GetWork(ctx, id)
		if err != nil {
			return err
		}
		return printJSON(item)
	case "claim":
		if len(args) > 1 && isHelp(args[1]) {
			return workUsage("claim <task-id> --owner <owner> [--ttl <duration>]", nil)
		}
		id, opts, err := workIDOptions(args[1:], map[string]bool{"--owner": false, "--ttl": false})
		if err != nil {
			return workUsage("claim <task-id> --owner <owner> [--ttl <duration>]", err)
		}
		ttl, err := workTTL(oneOption(opts, "--ttl"))
		if err != nil {
			return err
		}
		claim, err := store.ClaimWork(ctx, id, oneOption(opts, "--owner"), ttl)
		if err != nil {
			return err
		}
		return printJSON(claim)
	case "renew":
		if len(args) > 1 && isHelp(args[1]) {
			return workUsage("renew <task-id> --claim-token <token> [--ttl <duration>]", nil)
		}
		id, opts, err := workIDOptions(args[1:], map[string]bool{"--claim-token": false, "--ttl": false})
		if err != nil {
			return workUsage("renew <task-id> --claim-token <token> [--ttl <duration>]", err)
		}
		ttl, err := workTTL(oneOption(opts, "--ttl"))
		if err != nil {
			return err
		}
		claim, err := store.RenewWorkClaim(ctx, id, oneOption(opts, "--claim-token"), ttl)
		if err != nil {
			return err
		}
		return printJSON(claim)
	case "lease":
		if len(args) > 1 && isHelp(args[1]) {
			return workUsage("lease <task-id> --claim-token <token> --path <relative-path> [--ttl <duration>]", nil)
		}
		id, opts, err := workIDOptions(args[1:], map[string]bool{"--claim-token": false, "--path": false, "--ttl": false})
		if err != nil {
			return workUsage("lease <task-id> --claim-token <token> --path <relative-path> [--ttl <duration>]", err)
		}
		ttl, err := workTTL(oneOption(opts, "--ttl"))
		if err != nil {
			return err
		}
		lease, err := store.ReserveWorkLease(ctx, id, oneOption(opts, "--claim-token"), oneOption(opts, "--path"), ttl)
		if err != nil {
			return err
		}
		return printJSON(lease)
	case "lease-renew":
		if len(args) > 1 && isHelp(args[1]) {
			return workUsage("lease-renew --path <relative-path> --lease-token <token> [--ttl <duration>]", nil)
		}
		opts, positionals, err := workOptions(args[1:], map[string]bool{"--path": false, "--lease-token": false, "--ttl": false})
		if err != nil || len(positionals) != 0 {
			return workUsage("lease-renew --path <relative-path> --lease-token <token> [--ttl <duration>]", err)
		}
		ttl, err := workTTL(oneOption(opts, "--ttl"))
		if err != nil {
			return err
		}
		lease, err := store.RenewWorkLease(ctx, oneOption(opts, "--path"), oneOption(opts, "--lease-token"), ttl)
		if err != nil {
			return err
		}
		return printJSON(lease)
	case "release":
		if len(args) > 1 && isHelp(args[1]) {
			return workUsage("release --path <relative-path> --lease-token <token>", nil)
		}
		opts, positionals, err := workOptions(args[1:], map[string]bool{"--path": false, "--lease-token": false})
		if err != nil || len(positionals) != 0 {
			return workUsage("release --path <relative-path> --lease-token <token>", err)
		}
		if err := store.ReleaseWorkLease(ctx, oneOption(opts, "--path"), oneOption(opts, "--lease-token")); err != nil {
			return err
		}
		return printJSON(map[string]any{"released": true, "path": oneOption(opts, "--path")})
	case "transition":
		if len(args) > 1 && isHelp(args[1]) {
			return workUsage("transition <task-id> --claim-token <token> [--revision <n>] --to <in_review|in_progress|blocked>", nil)
		}
		id, opts, err := workIDOptions(args[1:], map[string]bool{"--claim-token": false, "--revision": false, "--to": false})
		if err != nil {
			return workUsage("transition <task-id> --claim-token <token> [--revision <n>] --to <in_review|in_progress|blocked>", err)
		}
		var revision int64
		if revStr := oneOption(opts, "--revision"); revStr != "" {
			revision, err = positiveRevision(revStr)
			if err != nil {
				return err
			}
		}
		item, err := store.TransitionWork(ctx, id, oneOption(opts, "--claim-token"), revision, delegation.WorkStatus(oneOption(opts, "--to")))
		if err != nil {
			return err
		}
		return printJSON(item)
	case "approve":
		if len(args) > 1 && isHelp(args[1]) {
			return workUsage("approve <task-id> --reviewer <id> [--revision <n>] --verdict <PASS|FAIL|BLOCKED|INCONCLUSIVE> [--evidence <ref>]", nil)
		}
		id, opts, err := workIDOptions(args[1:], map[string]bool{"--reviewer": false, "--verdict": false, "--evidence": false, "--revision": false})
		if err != nil {
			return workUsage("approve <task-id> --reviewer <id> [--revision <n>] --verdict <PASS|FAIL|BLOCKED|INCONCLUSIVE> [--evidence <ref>]", err)
		}
		var revision int64
		if revStr := oneOption(opts, "--revision"); revStr != "" {
			revision, err = positiveRevision(revStr)
			if err != nil {
				return err
			}
		}
		approval, err := store.ApproveWork(ctx, id, oneOption(opts, "--reviewer"), oneOption(opts, "--verdict"), oneOption(opts, "--evidence"), revision)
		if err != nil {
			return err
		}
		return printJSON(approval)
	case "retry":
		if len(args) > 1 && isHelp(args[1]) {
			return workUsage("retry <task-id> [--revision <n>]", nil)
		}
		id, opts, err := workIDOptions(args[1:], map[string]bool{"--revision": false})
		if err != nil {
			return workUsage("retry <task-id> [--revision <n>]", err)
		}
		var revision int64
		if revStr := oneOption(opts, "--revision"); revStr != "" {
			revision, err = positiveRevision(revStr)
			if err != nil {
				return err
			}
		}
		item, err := store.RetryWork(ctx, id, revision)
		if err != nil {
			return err
		}
		return printJSON(item)
	case "recover":
		if len(args) > 1 && isHelp(args[1]) {
			return workUsage("recover", nil)
		}
		if len(args) != 1 {
			return workUsage("recover", nil)
		}
		count, err := store.RecoverWork(ctx)
		if err != nil {
			return err
		}
		return printJSON(map[string]int64{"recovered": count})
	default:
		return fmt.Errorf("unknown work subcommand %q (see 'cortex-ia work --help')", args[0])
	}
}

func cortexStateHome() (string, error) {
	if value := strings.TrimSpace(os.Getenv("CORTEX_IA_HOME")); value != "" {
		absolute, err := filepath.Abs(value)
		if err != nil {
			return "", fmt.Errorf("resolve CORTEX_IA_HOME: %w", err)
		}
		return filepath.Clean(absolute), nil
	}
	return os.UserHomeDir()
}

func workOptions(args []string, allowed map[string]bool) (map[string][]string, []string, error) {
	values := map[string][]string{}
	positionals := []string{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		repeatable, ok := allowed[arg]
		if !ok {
			positionals = append(positionals, arg)
			continue
		}
		if i+1 >= len(args) || strings.HasPrefix(args[i+1], "--") {
			return nil, nil, fmt.Errorf("%s requires a value", arg)
		}
		if !repeatable && len(values[arg]) != 0 {
			return nil, nil, fmt.Errorf("%s may be provided only once", arg)
		}
		i++
		values[arg] = append(values[arg], args[i])
	}
	return values, positionals, nil
}

func workIDOptions(args []string, allowed map[string]bool) (string, map[string][]string, error) {
	opts, positionals, err := workOptions(args, allowed)
	if err != nil {
		return "", nil, err
	}
	if len(positionals) != 1 || strings.HasPrefix(positionals[0], "-") {
		return "", nil, errors.New("exactly one task id is required")
	}
	return positionals[0], opts, nil
}

func oneOption(values map[string][]string, name string) string {
	if len(values[name]) == 0 {
		return ""
	}
	return values[name][0]
}

func workTTL(value string) (time.Duration, error) {
	if value == "" {
		return 15 * time.Minute, nil
	}
	ttl, err := time.ParseDuration(value)
	if err != nil || ttl < time.Second || ttl > 24*time.Hour {
		return 0, errors.New("ttl must be a duration between 1s and 24h")
	}
	return ttl, nil
}

func positiveRevision(value string) (int64, error) {
	revision, err := strconv.ParseInt(value, 10, 64)
	if err != nil || revision <= 0 {
		return 0, errors.New("revision must be a positive integer")
	}
	return revision, nil
}

func oneWorkID(args []string) (string, error) {
	if len(args) != 1 || args[0] == "" || strings.HasPrefix(args[0], "-") {
		return "", errors.New("exactly one task id is required")
	}
	return args[0], nil
}

func workUsage(usage string, cause error) error {
	if cause != nil {
		return fmt.Errorf("%v (usage: cortex-ia work %s)", cause, usage)
	}
	fmt.Printf("Usage: cortex-ia work %s\n", usage)
	return nil
}
