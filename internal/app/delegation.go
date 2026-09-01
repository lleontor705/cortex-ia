package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/lleontor705/cortex-ia/internal/delegation"
)

func runDelegate(args []string) error {
	if len(args) == 0 || isHelp(args[0]) {
		fmt.Println("Usage: cortex-ia delegate <subcommand> [options]")
		fmt.Println("\nSubcommands:")
		fmt.Println("  create --request-file <path> [--transport <t>]   Create an external delegation job")
		fmt.Println("  status <job-id>                                  Get job execution status")
		fmt.Println("  result <job-id>                                  Get structured job receipt")
		fmt.Println("  cancel <job-id>                                  Cancel an active job")
		fmt.Println("  recover                                          Recover lost/expired delegation jobs")
		fmt.Println("  worker --job <id> --request-file <path>          Run worker process for accepted job")
		return nil
	}
	home, err := cortexStateHome()
	if err != nil {
		return err
	}
	ctx := context.Background()
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
