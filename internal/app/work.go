package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
		fmt.Println("  create <id> <title> [--board <board>] [definition options]     Create a work task")
		fmt.Println("  list [--board <board-id>]                                   List work items")
		fmt.Println("  status <task-id>                                            Get task details")
		fmt.Println("  claim <task-id> --owner <owner> [--ttl <duration>]          Claim a task")
		fmt.Println("  renew <task-id> --claim-token <token> [--ttl <duration>]    Renew a claim")
		fmt.Println("  reserve <task-id> --claim-token <token> --path <file>       Reserve exactly one file")
		fmt.Println("  lease <task-id> --claim-token <token> --path <file>         Reserve a file lease")
		fmt.Println("  lease-renew --path <file> --lease-token <token>             Renew a file lease")
		fmt.Println("  release --path <file> --lease-token <token>                 Release a file lease")
		fmt.Println("  transition <task-id> --claim-token <token> --to <status>    Transition task state")
		fmt.Println("  approve <task-id> --reviewer <id> --verdict <PASS|FAIL>     Approve/review a task")
		fmt.Println("  retry <task-id> --revision <n>                              Retry a task")
		fmt.Println("  revise --plan <file|@stdin>                                 Safely revise an unclaimed task definition")
		fmt.Println("  decompose <task-id> --revision <n> --plan <file|@stdin>       Replace a blocked task with atomic tasks")
		fmt.Println("  recover                                                     Recover expired claims/leases")
		fmt.Println("  verify-lease --path <file> [--task <id>] [--owner <owner>]  Verify active file lease")
		return nil
	}
	home, err := cortexStateHome()
	if err != nil {
		return err
	}
	dbPath := delegation.DefaultDBPath(home)
	sub := strings.ToLower(args[0])
	var store *delegation.Store
	if sub == "verify-lease" || sub == "check-lease" || sub == "status" || sub == "list" {
		store, err = delegation.OpenStoreReadOnly(dbPath)
		if err != nil {
			store, err = delegation.OpenStore(dbPath)
		}
	} else {
		store, err = delegation.OpenStore(dbPath)
	}
	if err != nil {
		return err
	}
	defer func() { _ = store.Close() }()
	ctx := context.Background()
	switch sub {
	case "create":
		if len(args) > 1 && isHelp(args[1]) {
			return workUsage("create <id> <title> [--board <id>] [--depends <id>]... [--objective <text>] [--acceptance <text>] [--verify <command>] [--file <path>]...", nil)
		}
		opts, positionals, err := workOptions(args[1:], map[string]bool{"--id": false, "--title": false, "--depends": true, "--board": false, "--objective": false, "--acceptance": false, "--verify": false, "--file": true, "--project": false, "--opencode-session-id": false, "--opencode-root-session-id": false, "--opencode-parent-session-id": false, "--contract-file": false, "--workflow": false})
		if err != nil {
			return workUsage("create <id> <title> [--board <id>] [--depends <id>]... [--objective <text>] [--acceptance <text>] [--verify <command>] [--file <path>]...", err)
		}
		id := oneOption(opts, "--id")
		title := oneOption(opts, "--title")
		boardID := oneOption(opts, "--board")
		depends := opts["--depends"]
		for _, value := range positionals {
			if strings.HasPrefix(value, "--") {
				return fmt.Errorf("unknown work create option %q", value)
			}
		}
		if len(positionals) > 2 || (id != "" && len(positionals) != 0) {
			return errors.New("work create requires exactly an id and title")
		}
		if _, explicit := opts["--project"]; explicit && strings.TrimSpace(oneOption(opts, "--project")) == "" {
			return errors.New("--project must not be empty")
		}
		if id == "" && len(positionals) > 0 {
			id = positionals[0]
			if len(positionals) > 1 {
				title = positionals[1]
			}
		}
		if id == "" || title == "" {
			return errors.New("work create requires an id and title; see cortex-ia work create --help")
		}
		var contract *delegation.SDDContract
		if file := oneOption(opts, "--contract-file"); file != "" {
			contract, err = delegation.ReadSDDContract(file)
			if err != nil {
				return err
			}
		}
		if workflow, explicit := opts["--workflow"]; explicit {
			if len(workflow) != 1 {
				return errors.New("work create requires one workflow")
			}
			switch workflow[0] {
			case "sdd-lite", "sdd-full":
				if contract == nil || contract.Workflow != workflow[0] {
					return errors.New("SDD workflow requires a matching --contract-file")
				}
			case "direct-change", "fast-tdd", "hotfix":
				if contract != nil {
					return errors.New("direct workflow cannot carry an SDD contract")
				}
			default:
				return errors.New("unsupported work create workflow")
			}
		}
		item, err := store.CreateWorkInBoardWithDefinition(ctx, boardID, id, title, depends, delegation.WorkDefinition{
			Project: oneOption(opts, "--project"), Contract: contract,
			ConversationOwnership: delegation.ConversationOwnership{
				OpenCodeSessionID:       oneOption(opts, "--opencode-session-id"),
				OpenCodeRootSessionID:   oneOption(opts, "--opencode-root-session-id"),
				OpenCodeParentSessionID: oneOption(opts, "--opencode-parent-session-id"),
			},
			Objective: oneOption(opts, "--objective"), Acceptance: oneOption(opts, "--acceptance"), Verification: oneOption(opts, "--verify"), AllowedFiles: opts["--file"],
		})
		if err != nil {
			return err
		}
		return printJSON(item)
	case "revise":
		return runWorkRevise(store, args[1:])
	case "review-refresh":
		opts, positionals, err := workOptions(args[1:], map[string]bool{"--revision": false})
		if err != nil || len(positionals) != 1 {
			return workUsage("review-refresh <task-id> --revision <observed>", err)
		}
		revision, err := strconv.ParseInt(oneOption(opts, "--revision"), 10, 64)
		if err != nil || revision <= 0 {
			return errors.New("review-refresh requires a positive --revision")
		}
		item, err := store.RefreshWorkReview(ctx, positionals[0], revision)
		if err != nil {
			return err
		}
		return printJSON(item)
	case "archive":
		opts, positionals, err := workOptions(args[1:], map[string]bool{"--board": false, "--project": false, "--change": false, "--workflow": false, "--spec-plane": false})
		if err != nil || len(positionals) != 0 {
			return workUsage("archive --board <id> --project <workspace> --change <id> --workflow <sdd-lite|sdd-full> --spec-plane <openspec|cortex|hybrid>", err)
		}
		for _, key := range []string{"--board", "--change", "--workflow", "--spec-plane"} {
			if oneOption(opts, key) == "" {
				return fmt.Errorf("work archive requires %s", key)
			}
		}
		project := oneOption(opts, "--project")
		if project == "" {
			project, err = os.Getwd()
			if err != nil {
				return err
			}
		}
		receipt, err := store.ArchiveChange(ctx, delegation.ArchiveOptions{BoardID: oneOption(opts, "--board"), Workspace: project, ChangeID: oneOption(opts, "--change"), Workflow: oneOption(opts, "--workflow"), SpecPlane: oneOption(opts, "--spec-plane")})
		if err != nil {
			return err
		}
		return printJSON(receipt)
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
		claimToken, err := workAuthorityOption(opts, "--claim-token")
		if err != nil {
			return err
		}
		claim, err := store.RenewWorkClaim(ctx, id, claimToken, ttl)
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
		claimToken, err := workAuthorityOption(opts, "--claim-token")
		if err != nil {
			return err
		}
		lease, err := store.ReserveWorkLease(ctx, id, claimToken, oneOption(opts, "--path"), ttl)
		if err != nil {
			return err
		}
		return printJSON(lease)
	case "reserve", "file-reserve":
		if len(args) > 1 && isHelp(args[1]) {
			return workUsage("reserve <task-id> --claim-token <token> --path <relative-path> [--ttl <duration>]", nil)
		}
		id, opts, err := workIDOptions(args[1:], map[string]bool{"--claim-token": false, "--path": false, "--ttl": false})
		if err != nil {
			return workUsage("reserve <task-id> --claim-token <token> --path <relative-path> [--ttl <duration>]", err)
		}
		ttl, err := workTTL(oneOption(opts, "--ttl"))
		if err != nil {
			return err
		}
		claimToken, err := workAuthorityOption(opts, "--claim-token")
		if err != nil {
			return err
		}
		lease, err := store.ReserveWorkLease(ctx, id, claimToken, oneOption(opts, "--path"), ttl)
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
		leaseToken, err := workAuthorityOption(opts, "--lease-token")
		if err != nil {
			return err
		}
		lease, err := store.RenewWorkLease(ctx, oneOption(opts, "--path"), leaseToken, ttl)
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
		leaseToken, err := workAuthorityOption(opts, "--lease-token")
		if err != nil {
			return err
		}
		if err := store.ReleaseWorkLease(ctx, oneOption(opts, "--path"), leaseToken); err != nil {
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
		claimToken, err := workAuthorityOption(opts, "--claim-token")
		if err != nil {
			return err
		}
		item, err := store.TransitionWork(ctx, id, claimToken, revision, delegation.WorkStatus(oneOption(opts, "--to")))
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
	case "decompose":
		if len(args) > 1 && isHelp(args[1]) {
			return workUsage("decompose <task-id> --revision <n> --plan <file|@stdin> [--contract-file <file>]", nil)
		}
		id, opts, err := workIDOptions(args[1:], map[string]bool{"--revision": false, "--plan": false, "--contract-file": false})
		if err != nil {
			return workUsage("decompose <task-id> --revision <n> --plan <file|@stdin> [--contract-file <file>]", err)
		}
		revision, err := positiveRevision(oneOption(opts, "--revision"))
		if err != nil {
			return err
		}
		plan, err := readWorkDecompositionPlan(oneOption(opts, "--plan"))
		if err != nil {
			return err
		}
		contract := plan.Contract
		if file := oneOption(opts, "--contract-file"); file != "" {
			contractFromFile, err := delegation.ReadSDDContract(file)
			if err != nil {
				return err
			}
			contract = contractFromFile
		}
		result, err := store.DecomposeWork(ctx, id, revision, plan.Tasks, contract)
		if err != nil {
			return err
		}
		return printJSON(result)
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
	case "verify-lease", "check-lease":
		if len(args) > 1 && isHelp(args[1]) {
			return workUsage("verify-lease --path <file> [--path <file2>] [--task <task-id>] [--owner <owner>]", nil)
		}
		opts, _, err := workOptions(args[1:], map[string]bool{"--path": true, "--task": false, "--owner": false, "--session-id": false, "--project": false})
		if err != nil {
			return workUsage("verify-lease --path <file> [--task <task-id>] [--owner <owner>]", err)
		}
		rawPaths := opts["--path"]
		if len(rawPaths) == 0 {
			return errors.New("work verify-lease requires --path <file>")
		}
		var paths []string
		for _, p := range rawPaths {
			for _, part := range strings.Split(p, ",") {
				if clean := strings.TrimSpace(part); clean != "" {
					paths = append(paths, clean)
				}
			}
		}
		if len(paths) == 0 {
			return errors.New("work verify-lease requires --path <file>")
		}

		if len(paths) == 1 {
			pathVal := paths[0]
			var res delegation.LeaseVerification
			if session, explicit := opts["--session-id"]; explicit {
				if len(session) != 1 || session[0] == "" || oneOption(opts, "--project") == "" || oneOption(opts, "--owner") != "" || oneOption(opts, "--task") != "" {
					return errors.New("session lease verification requires --project and --session-id without --owner or --task")
				}
				res, err = store.VerifySessionWorkLease(ctx, pathVal, oneOption(opts, "--project"), session[0])
			} else {
				res, err = store.VerifyWorkLease(ctx, pathVal, oneOption(opts, "--task"), oneOption(opts, "--owner"))
			}
			if err != nil {
				return err
			}
			if err := printJSON(res); err != nil {
				return err
			}
			if !res.Valid {
				return fmt.Errorf("lease verification failed: %s", res.Reason)
			}
			return nil
		}

		// Multiple paths batch verification
		if session, explicit := opts["--session-id"]; explicit {
			if len(session) != 1 || session[0] == "" || oneOption(opts, "--project") == "" || oneOption(opts, "--owner") != "" || oneOption(opts, "--task") != "" {
				return errors.New("session lease verification requires --project and --session-id without --owner or --task")
			}
			results, err := store.VerifySessionWorkLeases(ctx, paths, oneOption(opts, "--project"), session[0])
			if err != nil {
				return err
			}
			allValid := true
			var firstFailure delegation.LeaseVerification
			for _, r := range results {
				if !r.Valid {
					allValid = false
					firstFailure = r
					break
				}
			}
			if allValid {
				taskID := ""
				owner := ""
				expiresAt := ""
				if len(results) > 0 {
					taskID = results[0].TaskID
					owner = results[0].Owner
					expiresAt = results[0].ExpiresAt
				}
				if err := printJSON(map[string]any{
					"valid":      true,
					"paths":      paths,
					"task_id":    taskID,
					"owner":      owner,
					"expires_at": expiresAt,
					"results":    results,
				}); err != nil {
					return err
				}
				return nil
			}
			_ = printJSON(firstFailure)
			return fmt.Errorf("lease verification failed: %s", firstFailure.Reason)
		}

		// General multiple-path check
		allValid := true
		var firstFailure delegation.LeaseVerification
		var results []delegation.LeaseVerification
		for _, p := range paths {
			r, err := store.VerifyWorkLease(ctx, p, oneOption(opts, "--task"), oneOption(opts, "--owner"))
			if err != nil {
				return err
			}
			results = append(results, r)
			if !r.Valid && allValid {
				allValid = false
				firstFailure = r
			}
		}
		if allValid {
			return printJSON(map[string]any{
				"valid":   true,
				"paths":   paths,
				"results": results,
			})
		}
		_ = printJSON(firstFailure)
		return fmt.Errorf("lease verification failed: %s", firstFailure.Reason)
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

func workAuthorityOption(values map[string][]string, name string) (string, error) {
	value := oneOption(values, name)
	if value != "@stdin" {
		return value, nil
	}
	data, err := io.ReadAll(io.LimitReader(os.Stdin, 129))
	if err != nil {
		return "", fmt.Errorf("read %s from stdin: %w", name, err)
	}
	value = strings.TrimSpace(string(data))
	if value == "" || len(value) > 128 {
		return "", fmt.Errorf("%s from stdin must contain 1-128 characters", name)
	}
	return value, nil
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

type workDecompositionPlan struct {
	Contract *delegation.SDDContract         `json:"contract,omitempty"`
	Tasks    []delegation.WorkStepDefinition `json:"tasks"`
}

func readWorkDecompositionPlan(source string) (workDecompositionPlan, error) {
	if strings.TrimSpace(source) == "" {
		return workDecompositionPlan{}, errors.New("decomposition plan is required")
	}
	var reader io.Reader
	var file *os.File
	if source == "@stdin" {
		reader = os.Stdin
	} else {
		var err error
		file, err = os.Open(source)
		if err != nil {
			return workDecompositionPlan{}, fmt.Errorf("open decomposition plan: %w", err)
		}
		defer func() { _ = file.Close() }()
		reader = file
	}
	data, err := io.ReadAll(io.LimitReader(reader, 64*1024+1))
	if err != nil {
		return workDecompositionPlan{}, fmt.Errorf("read decomposition plan: %w", err)
	}
	if len(data) > 64*1024 {
		return workDecompositionPlan{}, errors.New("decomposition plan exceeds 64 KiB")
	}
	var plan workDecompositionPlan
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&plan); err != nil {
		return workDecompositionPlan{}, fmt.Errorf("decode decomposition plan: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return workDecompositionPlan{}, errors.New("decomposition plan must contain exactly one JSON object")
	}
	if len(plan.Tasks) == 0 {
		return workDecompositionPlan{}, errors.New("decomposition plan requires at least one task")
	}
	return plan, nil
}
