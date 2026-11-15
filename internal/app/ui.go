package app

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"time"

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
	requestedSessionID := oneOption(opts, "--session-id")
	rootSessionID := oneOption(opts, "--root-session-id")
	// The cockpit polls this command every few seconds. A read-only handle never
	// runs DDL or BEGIN IMMEDIATE, so serving a snapshot cannot contend with the
	// work-authority writers for the database write lock.
	store, err := delegation.OpenStoreReadOnly(delegation.DefaultDBPath(home))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return printEmptySnapshot(projectRoot, requestedSessionID, rootSessionID)
		}
		return err
	}
	defer func() { _ = store.Close() }()
	dashboard, err := store.DashboardForConversation(context.Background(), projectRoot, requestedSessionID, rootSessionID)
	if err != nil {
		if isUninitializedSchemaError(err) {
			return printEmptySnapshot(projectRoot, requestedSessionID, rootSessionID)
		}
		return err
	}

	return printJSON(dashboard)
}

// isUninitializedSchemaError recognizes a database file that exists but was never
// migrated: the dashboard then fails on an absent table, which is a fresh install
// rather than a read failure worth surfacing to the cockpit.
func isUninitializedSchemaError(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "no such table")
}

func printEmptySnapshot(projectRoot, requestedSessionID, rootSessionID string) error {
	workspace, err := delegation.CanonicalWorkspace(projectRoot)
	if err != nil || workspace == "" {
		return fmt.Errorf("valid dashboard project root is required")
	}
	if requestedSessionID == "" {
		requestedSessionID = "global"
	}
	if rootSessionID == "" {
		rootSessionID = "global"
	}
	return printJSON(delegation.ConversationDashboard{
		SchemaVersion:      2,
		ProjectRoot:        workspace,
		RequestedSessionID: requestedSessionID,
		RootSessionID:      rootSessionID,
		GeneratedAt:        time.Now().UTC().Format(time.RFC3339Nano),
		Tasks:              []delegation.ConversationTask{},
		Delegations:        []delegation.Job{},
		Attention:          []delegation.ConversationAttention{},
		Summary: map[string]int{
			"backlog": 0, "ready": 0, "in_progress": 0, "in_review": 0, "blocked": 0, "done": 0, "superseded": 0,
			"active_tasks": 0, "total_tasks": 0, "total_delegations": 0, "active_delegations": 0,
			"completed_delegations": 0, "failed_delegations": 0, "active_executions": 0, "total_attention": 0,
		},
		Counts: map[string]int{"active": 0, "review": 0, "attention": 0},
	})
}
