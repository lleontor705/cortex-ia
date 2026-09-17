package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/lleontor705/cortex-ia/internal/delegation"
)

func runDelegate(args []string) error {
	if len(args) == 0 || isHelp(args[0]) {
		fmt.Println("Usage: cortex-ia delegate <subcommand> [options]")
		fmt.Println("\nSubcommands:")
		fmt.Println("  models [--json]                                  List available AGY models")
		fmt.Println("  policy --role <role>                             Read validated delegation policy")
		fmt.Println("  create --request-file <path> [--transport <t>]   Create an external delegation job")
		fmt.Println("  status <job-id>                                  Get job execution status")
		fmt.Println("  query <job-id>                                   Get coherent job view with status and receipt")
		fmt.Println("  wait <job-id> [--timeout <sec>]                  Wait for job completion with bounded polling")
		fmt.Println("  result <job-id>                                  Get structured job receipt")
		fmt.Println("  cancel <job-id>                                  Request cancellation; worker confirms termination")
		fmt.Println("  recover                                          Recover lost/expired delegation jobs")
		fmt.Println("  reconcile <job-id> --reason <text>               Prove prior-boot termination of a lost job")
		fmt.Println("  worker --job <id> --request-file <path>          Run worker process for accepted job")
		return nil
	}
	home, err := cortexStateHome()
	if err != nil {
		return err
	}
	ctx := context.Background()
	if args[0] == "policy" {
		if len(args) != 3 || args[1] != "--role" {
			return errors.New("usage: cortex-ia delegate policy --role <implement|investigate|planner|reviewer>")
		}
		switch args[2] {
		case "implement", "investigate", "planner", "reviewer":
		default:
			return errors.New("unsupported delegation policy role")
		}
		cfg, err := delegation.Load(filepath.Join(home, ".config", "opencode"))
		if err != nil {
			return err
		}
		role := cfg.Roles[args[2]]
		enabled := cfg.DelegationEnabled && role.Delegate && role.CLI == "agy"
		reason := "external_enabled"
		if !cfg.DelegationEnabled {
			reason = "delegation_disabled"
		} else if !role.Delegate || role.CLI != "agy" {
			reason = "role_native"
		}
		return printJSON(map[string]any{"schema_version": 1, "role": args[2], "external_enabled": enabled, "reason": reason})
	}
	if args[0] == "models" {
		models, err := delegation.ListAvailableModels(ctx)
		if err != nil {
			return err
		}
		if len(args) > 1 && args[1] == "--json" {
			return printJSON(models)
		}
		for _, m := range models {
			fmt.Printf("%-26s %s\n", m.ID, m.Name)
		}
		return nil
	}
	if args[0] == "worker" {
		jobID, requestPath, err := delegateWorkerArgs(args[1:])
		if err != nil {
			return err
		}
		return delegation.RunWorker(ctx, home, jobID, requestPath)
	}
	store, err := delegation.OpenStore(delegation.DefaultDBPath(home))
	if err != nil {
		return err
	}
	defer func() { _ = store.Close() }()
	switch args[0] {
	case "create":
		requestPath, transport, err := delegateCreateArgs(args[1:])
		if err != nil {
			return err
		}
		request, err := delegation.ReadRequest(requestPath)
		if err != nil {
			return err
		}
		cfg, err := delegation.Load(filepath.Join(home, ".config", "opencode"))
		if err != nil {
			return err
		}
		role := cfg.Roles[request.Role]
		if !cfg.DelegationEnabled || !role.Delegate || role.CLI != "agy" {
			return fmt.Errorf("external delegation is not enabled for role %q", request.Role)
		}
		job, err := delegation.CreateFromRequest(ctx, store, request, transport)
		if err != nil {
			return err
		}
		return printJSON(job)
	case "status":
		id, err := oneDelegateID(args[1:])
		if err != nil {
			return err
		}
		job, err := store.Get(ctx, id)
		if err != nil {
			return err
		}
		return printJSON(job)
	case "query", "view":
		id, err := oneDelegateID(args[1:])
		if err != nil {
			return err
		}
		view, err := store.Query(ctx, id)
		if err != nil {
			return err
		}
		return printJSON(view)
	case "wait":
		if len(args) < 2 {
			return errors.New("usage: cortex-ia delegate wait <job-id> [--timeout <seconds>]")
		}
		id := args[1]
		timeout := 30 * time.Minute
		for i := 2; i < len(args); i++ {
			if args[i] == "--timeout" && i+1 < len(args) {
				sec, err := strconv.Atoi(args[i+1])
				if err != nil || sec <= 0 {
					return errors.New("invalid timeout seconds")
				}
				timeout = time.Duration(sec) * time.Second
				i++
			}
		}
		view, err := store.Wait(ctx, id, timeout)
		if err != nil {
			return err
		}
		return printJSON(view)
	case "result":
		id, err := oneDelegateID(args[1:])
		if err != nil {
			return err
		}
		receipt, err := store.Result(ctx, id)
		if err != nil {
			return err
		}
		return printJSON(receipt)
	case "cancel":
		id, err := oneDelegateID(args[1:])
		if err != nil {
			return err
		}
		if err := store.Cancel(ctx, id); err != nil {
			return err
		}
		job, err := store.Get(ctx, id)
		if err != nil {
			return err
		}
		return printJSON(job)
	case "reconcile":
		if (len(args) != 4 && len(args) != 6) || args[1] == "" || args[2] != "--reason" || (len(args) == 6 && args[4] != "--session-id") {
			return errors.New("usage: cortex-ia delegate reconcile <job-id> --reason <text> [--session-id <host-session>]")
		}
		sessionID := ""
		if len(args) == 6 {
			if args[5] == "" {
				return errors.New("host session ID must not be empty")
			}
			sessionID = args[5]
		}
		proof, err := store.Reconcile(ctx, args[1], args[3], sessionID)
		if err != nil {
			return err
		}
		return printJSON(proof)
	case "recover":
		if len(args) != 1 {
			return errors.New("usage: cortex-ia delegate recover")
		}
		count, err := store.Recover(ctx)
		if err != nil {
			return err
		}
		return printJSON(map[string]int64{"recovered": count})
	case "set-pane":
		if len(args) != 3 {
			return errors.New("usage: cortex-ia delegate set-pane <job-id> <pane-id>")
		}
		if err := store.SetPaneID(ctx, args[1], args[2]); err != nil {
			return err
		}
		return printJSON(map[string]string{"job_id": args[1], "pane_id": args[2], "status": "updated"})
	default:
		return fmt.Errorf("unknown delegate subcommand %q", args[0])
	}
}

func delegateCreateArgs(args []string) (string, string, error) {
	if len(args) != 4 || args[0] != "--request-file" || args[2] != "--transport" {
		return "", "", errors.New("usage: cortex-ia delegate create --request-file <path> --transport <herdr|direct>")
	}
	if args[3] != "herdr" && args[3] != "direct" {
		return "", "", errors.New("delegate transport must be herdr or direct")
	}
	return args[1], args[3], nil
}

func delegateWorkerArgs(args []string) (string, string, error) {
	if len(args) != 4 || args[0] != "--job" || args[2] != "--request-file" {
		return "", "", errors.New("invalid internal delegation worker invocation")
	}
	return args[1], args[3], nil
}

func oneDelegateID(args []string) (string, error) {
	if len(args) != 1 || args[0] == "" {
		return "", errors.New("exactly one delegation job ID is required")
	}
	return args[0], nil
}

func printJSON(value any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}
