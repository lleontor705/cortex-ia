package app

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/lleontor705/cortex-ia/internal/delegation"
)

const uiSnapshotLimit = 20

type uiTask struct {
	ID         string                `json:"task_id"`
	BoardID    string                `json:"board_id"`
	Title      string                `json:"title"`
	Status     delegation.WorkStatus `json:"status"`
	Revision   int64                 `json:"revision"`
	Owner      string                `json:"owner,omitempty"`
	ClaimUntil string                `json:"claim_expires_at,omitempty"`
	LeaseCount int                   `json:"lease_count"`
	UpdatedAt  string                `json:"updated_at"`
}

type uiDelegation struct {
	ID           string            `json:"job_id"`
	Role         string            `json:"role"`
	TaskID       string            `json:"task_id,omitempty"`
	Status       delegation.Status `json:"status"`
	Transport    string            `json:"transport"`
	Attempt      int               `json:"attempt"`
	ErrorCode    string            `json:"error_code,omitempty"`
	ErrorMessage string            `json:"error_message,omitempty"`
	UpdatedAt    string            `json:"updated_at"`
}

type uiSnapshot struct {
	SchemaVersion int                         `json:"schema_version"`
	GeneratedAt   string                      `json:"generated_at"`
	ProjectRoot   string                      `json:"project_root"`
	Summary       delegation.DashboardSummary `json:"summary"`
	Sessions      []delegation.WorkSession    `json:"sessions"`
	Tasks         []uiTask                    `json:"tasks"`
	Delegations   []uiDelegation              `json:"delegations"`
}

func runUI(args []string) error {
	if len(args) == 0 || isHelp(args[0]) {
		fmt.Println("Usage: cortex-ia ui snapshot [--project <path>]")
		return nil
	}
	if strings.ToLower(args[0]) != "snapshot" {
		return fmt.Errorf("unknown ui command (usage: cortex-ia ui snapshot [--project <path>])")
	}
	opts, positionals, err := workOptions(args[1:], map[string]bool{"--project": false})
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
	dashboard, err := store.DashboardForWorkspace(context.Background(), projectRoot)
	if err != nil {
		return err
	}

	activeWork := append([]delegation.WorkItem(nil), dashboard.ActiveWork...)
	sort.SliceStable(activeWork, func(i, j int) bool {
		left, right := uiWorkPriority(activeWork[i].Status), uiWorkPriority(activeWork[j].Status)
		if left != right {
			return left < right
		}
		if activeWork[i].UpdatedAt != activeWork[j].UpdatedAt {
			return activeWork[i].UpdatedAt > activeWork[j].UpdatedAt
		}
		return activeWork[i].ID < activeWork[j].ID
	})
	snapshot := uiSnapshot{
		SchemaVersion: 1,
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339Nano),
		ProjectRoot:   projectRoot,
		Summary:       dashboard.Summary,
		Sessions:      boundedSlice(dashboard.Sessions, uiSnapshotLimit),
		Tasks:         make([]uiTask, 0, min(len(activeWork), uiSnapshotLimit)),
		Delegations:   make([]uiDelegation, 0, min(len(dashboard.Delegations), uiSnapshotLimit)),
	}
	for _, item := range boundedSlice(activeWork, uiSnapshotLimit) {
		task := uiTask{
			ID: item.ID, BoardID: item.BoardID, Title: item.Title, Status: item.Status,
			Revision: item.Revision, LeaseCount: len(item.Leases), UpdatedAt: item.UpdatedAt,
		}
		if item.Claim != nil {
			task.Owner = item.Claim.Owner
			task.ClaimUntil = item.Claim.ExpiresAt
		}
		snapshot.Tasks = append(snapshot.Tasks, task)
	}
	for _, job := range boundedSlice(dashboard.Delegations, uiSnapshotLimit) {
		snapshot.Delegations = append(snapshot.Delegations, uiDelegation{
			ID: job.ID, Role: job.Role, TaskID: job.TaskID, Status: job.Status,
			Transport: job.Transport, Attempt: job.Attempt, ErrorCode: job.ErrorCode,
			ErrorMessage: job.ErrorMessage, UpdatedAt: job.UpdatedAt,
		})
	}
	return printJSON(snapshot)
}

func uiWorkPriority(status delegation.WorkStatus) int {
	switch status {
	case delegation.WorkInProgress:
		return 0
	case delegation.WorkInReview:
		return 1
	case delegation.WorkBlocked:
		return 2
	case delegation.WorkReady:
		return 3
	case delegation.WorkBacklog:
		return 4
	default:
		return 5
	}
}

func boundedSlice[T any](values []T, limit int) []T {
	if len(values) <= limit {
		return values
	}
	return values[:limit]
}
