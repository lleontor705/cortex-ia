package delegation

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

type DashboardSummary struct {
	Sessions          int `json:"sessions"`
	ActiveTasks       int `json:"active_tasks"`
	ActiveAgents      int `json:"active_agents"`
	ActiveDelegations int `json:"active_delegations"`
	Blocked           int `json:"blocked"`
}

type WorkSession struct {
	BoardID     string               `json:"board_id"`
	Title       string               `json:"title"`
	Description string               `json:"description,omitempty"`
	Status      string               `json:"status"`
	Progress    int                  `json:"progress"`
	TaskCount   int                  `json:"task_count"`
	Counts      map[WorkStatus]int64 `json:"counts"`
	Owners      []string             `json:"owners,omitempty"`
	UpdatedAt   string               `json:"updated_at"`
}

type DelegationView struct {
	ID             string `json:"job_id"`
	Role           string `json:"role"`
	TaskID         string `json:"task_id,omitempty"`
	Status         Status `json:"status"`
	Transport      string `json:"transport"`
	Attempt        int    `json:"attempt"`
	LeaseOwner     string `json:"lease_owner,omitempty"`
	LeaseExpiresAt string `json:"lease_expires_at,omitempty"`
	ErrorCode      string `json:"error_code,omitempty"`
	ErrorMessage   string `json:"error_message,omitempty"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}

type ActivityEvent struct {
	Source    string `json:"source"`
	EntityID  string `json:"entity_id"`
	Title     string `json:"title"`
	Kind      string `json:"kind"`
	From      string `json:"from,omitempty"`
	To        string `json:"to,omitempty"`
	Detail    string `json:"detail,omitempty"`
	CreatedAt string `json:"created_at"`
}

type Dashboard struct {
	Summary     DashboardSummary `json:"summary"`
	Sessions    []WorkSession    `json:"sessions"`
	ActiveWork  []WorkItem       `json:"active_work"`
	Delegations []DelegationView `json:"delegations"`
	Activity    []ActivityEvent  `json:"activity"`
}

func (s *Store) Dashboard(ctx context.Context) (Dashboard, error) {
	boards, err := s.ListBoards(ctx)
	if err != nil {
		return Dashboard{}, err
	}
	items, err := s.ListWork(ctx)
	if err != nil {
		return Dashboard{}, err
	}
	delegations, err := s.ListDelegations(ctx, 50)
	if err != nil {
		return Dashboard{}, err
	}
	activity, err := s.ListActivity(ctx, 80)
	if err != nil {
		return Dashboard{}, err
	}

	itemsByBoard := make(map[string][]WorkItem)
	activeWork := make([]WorkItem, 0)
	owners := make(map[string]bool)
	blocked := 0
	for _, item := range items {
		itemsByBoard[item.BoardID] = append(itemsByBoard[item.BoardID], item)
		if item.Status != WorkDone {
			activeWork = append(activeWork, item)
		}
		if item.Status == WorkBlocked {
			blocked++
		}
		if item.Claim != nil {
			owners[item.Claim.Owner] = true
		}
	}

	sessions := make([]WorkSession, 0, len(boards))
	for _, board := range boards {
		boardItems := itemsByBoard[board.ID]
		session := WorkSession{
			BoardID:     board.ID,
			Title:       board.Title,
			Description: board.Description,
			Status:      "empty",
			TaskCount:   len(boardItems),
			Counts:      board.Counts,
			UpdatedAt:   board.UpdatedAt,
		}
		done := 0
		sessionOwners := make(map[string]bool)
		for _, item := range boardItems {
			if item.Status == WorkDone {
				done++
			}
			if item.UpdatedAt > session.UpdatedAt {
				session.UpdatedAt = item.UpdatedAt
			}
			if item.Claim != nil {
				sessionOwners[item.Claim.Owner] = true
			}
		}
		if len(boardItems) > 0 {
			session.Status = "active"
			session.Progress = done * 100 / len(boardItems)
			if done == len(boardItems) {
				session.Status = "complete"
			}
		}
		for owner := range sessionOwners {
			session.Owners = append(session.Owners, owner)
		}
		sort.Strings(session.Owners)
		sessions = append(sessions, session)
	}

	activeDelegations := 0
	for _, job := range delegations {
		if job.Status == StatusAccepted || job.Status == StatusStarting || job.Status == StatusRunning || job.Status == StatusBlocked {
			activeDelegations++
		}
	}
	return Dashboard{
		Summary: DashboardSummary{
			Sessions:          len(sessions),
			ActiveTasks:       len(activeWork),
			ActiveAgents:      len(owners),
			ActiveDelegations: activeDelegations,
			Blocked:           blocked,
		},
		Sessions:    sessions,
		ActiveWork:  activeWork,
		Delegations: delegations,
		Activity:    activity,
	}, nil
}

func (s *Store) ListDelegations(ctx context.Context, limit int) ([]DelegationView, error) {
	if limit < 1 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,role,task_id,status,transport,attempt,lease_owner,COALESCE(lease_expires_at,''),error_code,error_message,created_at,updated_at FROM delegation_jobs ORDER BY updated_at DESC,id LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	views := make([]DelegationView, 0)
	for rows.Next() {
		var view DelegationView
		if err := rows.Scan(&view.ID, &view.Role, &view.TaskID, &view.Status, &view.Transport, &view.Attempt, &view.LeaseOwner, &view.LeaseExpiresAt, &view.ErrorCode, &view.ErrorMessage, &view.CreatedAt, &view.UpdatedAt); err != nil {
			return nil, err
		}
		view.ErrorMessage = bounded(view.ErrorMessage, 160)
		views = append(views, view)
	}
	return views, rows.Err()
}

func (s *Store) ListActivity(ctx context.Context, limit int) ([]ActivityEvent, error) {
	if limit < 1 || limit > 500 {
		limit = 80
	}
	rows, err := s.db.QueryContext(ctx, `SELECT source,entity_id,title,kind,from_status,to_status,detail,created_at FROM (
		SELECT 'work' AS source,e.item_id AS entity_id,w.title AS title,e.kind,e.from_status,e.to_status,e.detail,e.created_at
		FROM work_events e JOIN work_items w ON w.id=e.item_id
		UNION ALL
		SELECT 'delegation' AS source,e.job_id AS entity_id,j.role AS title,e.kind,e.from_status,e.to_status,e.detail,e.created_at
		FROM delegation_events e JOIN delegation_jobs j ON j.id=e.job_id
	) ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	events := make([]ActivityEvent, 0)
	for rows.Next() {
		var event ActivityEvent
		if err := rows.Scan(&event.Source, &event.EntityID, &event.Title, &event.Kind, &event.From, &event.To, &event.Detail, &event.CreatedAt); err != nil {
			return nil, err
		}
		event.Detail = strings.TrimSpace(event.Detail)
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read dashboard activity: %w", err)
	}
	return events, nil
}
