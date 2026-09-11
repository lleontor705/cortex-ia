package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
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
		address := cortexiaweb.NormalizeAddress(oneOption(opts, "--addr"))
		return serveCortexIAWeb(store, address)
	default:
		return fmt.Errorf("unknown board subcommand %q (see 'cortex-ia board --help')", args[0])
	}
}

func runWeb(args []string) error {
	if len(args) > 0 && isHelp(args[0]) {
		fmt.Println("Usage: cortex-ia web [--addr <loopback-host:port>] [--board <board-id>] [--task <task-id>] [--open] [--daemon]")
		fmt.Println("\nOptions:")
		fmt.Println("  --addr <host:port>   Listen or connect address (default: 127.0.0.1:7331)")
		fmt.Println("  --board <board-id>   Open directly to a specific board")
		fmt.Println("  --task <task-id>     Open directly to a specific task modal")
		fmt.Println("  --open, -o           Open dashboard URL in default browser")
		fmt.Println("  --daemon, -d         Start server in background if not already running")
		return nil
	}
	opts, positionals, err := workOptions(args, map[string]bool{
		"--addr":  false,
		"--board": false,
		"--task":  false,
	})
	if err != nil {
		return fmt.Errorf("usage: cortex-ia web [--addr <loopback-host:port>] [--board <board-id>] [--task <task-id>] [--open] [--daemon]")
	}
	shouldOpen := false
	isDaemon := false
	for _, p := range positionals {
		switch p {
		case "--open", "-o":
			shouldOpen = true
		case "--daemon", "-d":
			isDaemon = true
		default:
			return fmt.Errorf("unknown argument %q; usage: cortex-ia web [--addr <loopback-host:port>] [--board <board-id>] [--task <task-id>] [--open] [--daemon]", p)
		}
	}
	for _, a := range args {
		switch a {
		case "--open", "-o":
			shouldOpen = true
		case "--daemon", "-d":
			isDaemon = true
		}
	}
	address := cortexiaweb.NormalizeAddress(oneOption(opts, "--addr"))
	boardID := oneOption(opts, "--board")
	taskID := oneOption(opts, "--task")
	targetURL := buildWebURL(address, boardID, taskID)

	if isServerHealthy(address) {
		fmt.Printf("Cortex-IA web is already running at http://%s\n", address)
		if shouldOpen {
			openBrowserURL(targetURL)
		} else {
			fmt.Println("Use --open to view in browser or specify a different address with --addr.")
		}
		return nil
	}

	if isDaemon {
		exe, err := os.Executable()
		if err != nil {
			return fmt.Errorf("failed to get executable path: %w", err)
		}
		cmd := exec.Command(exe, "board", "serve", "--addr", address)
		cmd.Stdin = nil
		cmd.Stdout = nil
		cmd.Stderr = nil
		if err := cmd.Start(); err != nil {
			return fmt.Errorf("failed to start background web server: %w", err)
		}
		fmt.Printf("Started background Cortex-IA web server at http://%s\n", address)
		if shouldOpen {
			for i := 0; i < 10; i++ {
				time.Sleep(100 * time.Millisecond)
				if isServerHealthy(address) {
					break
				}
			}
			openBrowserURL(targetURL)
		}
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

	if shouldOpen {
		go func() {
			time.Sleep(500 * time.Millisecond)
			openBrowserURL(targetURL)
		}()
	}
	return serveCortexIAWeb(store, address)
}

func buildWebURL(address, boardID, taskID string) string {
	baseURL := "http://" + address + "/"
	params := url.Values{}
	if strings.TrimSpace(boardID) != "" {
		params.Set("board", strings.TrimSpace(boardID))
	}
	if strings.TrimSpace(taskID) != "" {
		params.Set("task", strings.TrimSpace(taskID))
	}
	encoded := params.Encode()
	if encoded != "" {
		baseURL += "?" + encoded + "#board"
	}
	return baseURL
}

func isServerHealthy(address string) bool {
	client := http.Client{Timeout: 600 * time.Millisecond}
	resp, err := client.Get("http://" + address + "/api/overview")
	if err != nil {
		return false
	}
	_ = resp.Body.Close()
	return resp.StatusCode == http.StatusOK
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
