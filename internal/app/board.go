package app

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/lleontor705/cortex-ia/internal/cortexiaweb"
	"github.com/lleontor705/cortex-ia/internal/delegation"
)

func isHelp(s string) bool {
	l := strings.ToLower(s)
	return l == "--help" || l == "-h" || l == "help" || l == "-help"
}

func runBoard(args []string) error {
	if len(args) == 0 || isHelp(args[0]) {
		fmt.Println("Usage: cortex-ia board <subcommand> [options]")
		fmt.Println("\nSubcommands:")
		fmt.Println("  create <id> <title> [desc]   Create a new task board")
		fmt.Println("  list                         List all task boards")
		fmt.Println("  status <board-id>            Show board status and task snapshot")
		fmt.Println("  archive <board-id>           Archive a completed task board")
		fmt.Println("  unarchive <board-id>         Restore an archived task board to active")
		fmt.Println("  delete <board-id>            Delete an archived task board and its tasks")
		fmt.Println("  serve [--addr <host:port>]   Serve the local web operations dashboard")
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
	switch strings.ToLower(args[0]) {
	case "create":
		if len(args) > 1 && isHelp(args[1]) {
			return boardUsage("create <id> <title> [description] OR create --id <id> --title <title> [--description <text>]", nil)
		}
		opts, positionals, err := workOptions(args[1:], map[string]bool{"--id": false, "--title": false, "--description": false})
		if err != nil {
			return boardUsage("create <id> <title> [description] OR create --id <id> --title <title> [--description <text>]", err)
		}
		id := oneOption(opts, "--id")
		title := oneOption(opts, "--title")
		desc := oneOption(opts, "--description")
		if id == "" && len(positionals) > 0 {
			id = positionals[0]
			if len(positionals) > 1 {
				title = positionals[1]
			}
			if len(positionals) > 2 {
				desc = strings.Join(positionals[2:], " ")
			}
		}
		if id == "" || title == "" {
			return errors.New("board id and title are required (usage: cortex-ia board create <id> <title> [description] OR --id <id> --title <title> [--description <text>])")
		}
		board, err := store.CreateBoard(ctx, id, title, desc)
		if err != nil {
			return err
		}
		return printJSON(board)
	case "list":
		if len(args) > 1 && isHelp(args[1]) {
			return boardUsage("list", nil)
		}
		boards, err := store.ListBoards(ctx)
		if err != nil {
			return err
		}
		return printJSON(boards)
	case "status", "show", "get":
		if len(args) > 1 && isHelp(args[1]) {
			return boardUsage("status <board-id>", nil)
		}
		id, err := oneWorkID(args[1:])
		if err != nil {
			return boardUsage("status <board-id>", err)
		}
		snapshot, err := store.BoardSnapshot(ctx, id)
		if err != nil {
			return err
		}
		return printJSON(snapshot)
	case "archive":
		if len(args) > 1 && isHelp(args[1]) {
			return boardUsage("archive <board-id>", nil)
		}
		id, err := oneWorkID(args[1:])
		if err != nil {
			return boardUsage("archive <board-id>", err)
		}
		board, err := store.ArchiveBoard(ctx, id)
		if err != nil {
			return err
		}
		return printJSON(board)
	case "unarchive":
		if len(args) > 1 && isHelp(args[1]) {
			return boardUsage("unarchive <board-id>", nil)
		}
		id, err := oneWorkID(args[1:])
		if err != nil {
			return boardUsage("unarchive <board-id>", err)
		}
		board, err := store.UnarchiveBoard(ctx, id)
		if err != nil {
			return err
		}
		return printJSON(board)
	case "delete":
		if len(args) > 1 && isHelp(args[1]) {
			return boardUsage("delete <board-id>", nil)
		}
		id, err := oneWorkID(args[1:])
		if err != nil {
			return boardUsage("delete <board-id>", err)
		}
		if err := store.DeleteBoard(ctx, id); err != nil {
			return err
		}
		return printJSON(map[string]any{"deleted": true, "board_id": id})
	case "serve":
		if len(args) > 1 && isHelp(args[1]) {
			return boardUsage("serve [--addr <loopback-host:port>]", nil)
		}
		opts, positionals, err := workOptions(args[1:], map[string]bool{"--addr": false})
		if err != nil || len(positionals) != 0 {
			return boardUsage("serve [--addr <loopback-host:port>]", err)
		}
		address := oneOption(opts, "--addr")
		if address == "" {
			address = "127.0.0.1:7331"
		}
		return serveCortexIAWeb(store, address)
	default:
		return fmt.Errorf("unknown board subcommand %q (see 'cortex-ia board --help')", args[0])
	}
}

func runWeb(args []string) error {
	opts, positionals, err := workOptions(args, map[string]bool{"--addr": false})
	if err != nil {
		return fmt.Errorf("usage: cortex-ia web [--addr <loopback-host:port>] [--open]")
	}
	shouldOpen := false
	for _, p := range positionals {
		if p == "--open" || p == "-o" {
			shouldOpen = true
		} else {
			return fmt.Errorf("unknown argument %q; usage: cortex-ia web [--addr <loopback-host:port>] [--open]", p)
		}
	}
	for _, a := range args {
		if a == "--open" || a == "-o" {
			shouldOpen = true
		}
	}
	address := oneOption(opts, "--addr")
	if address == "" {
		address = "127.0.0.1:7331"
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

	if shouldOpen {
		go func() {
			time.Sleep(500 * time.Millisecond)
			openBrowserURL("http://" + address)
		}()
	}
	return serveCortexIAWeb(store, address)
}

func serveCortexIAWeb(store *delegation.Store, address string) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	ready := make(chan string, 1)
	errCh := make(chan error, 1)
	go func() { errCh <- cortexiaweb.Serve(ctx, store, address, ready) }()
	select {
	case address := <-ready:
		fmt.Printf("Cortex-IA web: %s\nPress Ctrl+C to stop.\n", address)
		return <-errCh
	case err := <-errCh:
		return err
	}
}

func openBrowserURL(targetURL string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", targetURL)
	case "darwin":
		cmd = exec.Command("open", targetURL)
	default:
		cmd = exec.Command("xdg-open", targetURL)
	}
	_ = cmd.Start()
}

func boardUsage(usage string, cause error) error {
	if cause != nil {
		return fmt.Errorf("%v (usage: cortex-ia board %s)", cause, usage)
	}
	fmt.Printf("Usage: cortex-ia board %s\n", usage)
	return nil
}
