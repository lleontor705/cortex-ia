package delegation

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
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
	Workspace      string `json:"-"`
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

// DashboardFeed is a bounded page of operational history. Counts cover the
// entire selected board (or all boards), not only the returned page.
type DashboardFeed struct {
	Delegations      []DelegationView `json:"delegations"`
	Activity         []ActivityEvent  `json:"activity"`
	Page             int              `json:"page"`
	PageSize         int              `json:"page_size"`
	TotalDelegations int              `json:"total_delegations"`
	TotalActivity    int              `json:"total_activity"`
}

func (s *Store) DashboardFeed(ctx context.Context, board string, page int) (DashboardFeed, error) {
	const pageSize = 20
	result := DashboardFeed{Delegations: []DelegationView{}, Activity: []ActivityEvent{}, Page: page, PageSize: pageSize}
	if page < 0 || page > 1000000 {
		return result, fmt.Errorf("invalid feed page")
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return result, err
	}
	defer func() { _ = tx.Rollback() }()
	if board != "" {
		var exists int
		if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM work_boards WHERE id=?`, board).Scan(&exists); err != nil {
			return result, err
		}
		if exists == 0 {
			return result, ErrBoardNotFound
		}
	}
	// The board restriction is applied before ordering and pagination for both
	// task events and delegation events, whose entity IDs identify jobs.
	const populations = `WITH jobs AS (
		SELECT j.* FROM delegation_jobs j WHERE ?='' OR EXISTS (SELECT 1 FROM work_items w WHERE w.id=j.task_id AND w.board_id=?)
	), events AS (
		SELECT 'work' AS source,e.id AS event_id,e.item_id AS entity_id,w.title,e.kind,e.from_status,e.to_status,e.detail,e.created_at
		FROM work_events e JOIN work_items w ON w.id=e.item_id WHERE ?='' OR w.board_id=?
		UNION ALL
		SELECT 'delegation',e.id,e.job_id,j.role,e.kind,e.from_status,e.to_status,e.detail,e.created_at
		FROM delegation_events e JOIN jobs j ON j.id=e.job_id
	) `
	args := []any{board, board, board, board}
	if err := tx.QueryRowContext(ctx, populations+`SELECT (SELECT count(*) FROM jobs),(SELECT count(*) FROM events)`, args...).Scan(&result.TotalDelegations, &result.TotalActivity); err != nil {
		return result, err
	}
	read := func(query string, target any) error {
		var raw string
		queryArgs := append(append([]any{}, args...), pageSize, page*pageSize)
		if err := tx.QueryRowContext(ctx, populations+query, queryArgs...).Scan(&raw); err != nil {
			return err
		}
		return json.Unmarshal([]byte(raw), target)
	}
	if err := read(`SELECT json_group_array(json_object('job_id',id,'role',role,'task_id',task_id,'status',status,'transport',transport,'attempt',attempt,
		'lease_owner',lease_owner,'lease_expires_at',coalesce(lease_expires_at,''),'error_code',error_code,'error_message',substr(error_message,1,160),'created_at',created_at,'updated_at',updated_at))
		FROM (SELECT * FROM jobs ORDER BY updated_at DESC,id DESC LIMIT ? OFFSET ?)`, &result.Delegations); err != nil {
		return result, err
	}
	if err := read(`SELECT json_group_array(json_object('source',source,'entity_id',entity_id,'title',title,'kind',kind,'from',from_status,'to',to_status,'detail',detail,'created_at',created_at))
		FROM (SELECT * FROM events ORDER BY created_at DESC,source,event_id DESC LIMIT ? OFFSET ?)`, &result.Activity); err != nil {
		return result, err
	}
	return result, tx.Commit()
}

type ConversationTask struct {
	ConversationOwnership
	ID         string     `json:"task_id"`
	BoardID    string     `json:"board_id"`
	Title      string     `json:"title"`
	Status     WorkStatus `json:"status"`
	Revision   int64      `json:"revision"`
	Owner      string     `json:"owner,omitempty"`
	ClaimUntil string     `json:"claim_expires_at,omitempty"`
	LeaseCount int        `json:"lease_count"`
	UpdatedAt  string     `json:"updated_at"`
}

type ConversationAttention struct {
	ID        string `json:"id"`
	EntityID  string `json:"entity_id"`
	Kind      string `json:"kind"`
	Title     string `json:"title"`
	UpdatedAt string `json:"updated_at"`
}

type ConversationDashboard struct {
	SchemaVersion      int                     `json:"schema_version"`
	ProjectRoot        string                  `json:"project_root"`
	RequestedSessionID string                  `json:"requested_session_id"`
	RootSessionID      string                  `json:"root_session_id"`
	GeneratedAt        string                  `json:"generated_at"`
	Tasks              []ConversationTask      `json:"tasks"`
	Delegations        []Job                   `json:"delegations"`
	Attention          []ConversationAttention `json:"attention"`
	Summary            map[string]int          `json:"summary"`
	Counts             map[string]int          `json:"counts"`
}

// conversationPredicate applies durable ownership before any ordering or limit.
// Stored workspace keys are canonicalized at creation; jobs never confer task ownership.
const conversationPredicate = `workspace = ? AND opencode_root_session_id = ?
	AND length(opencode_session_id) BETWEEN 1 AND 256
	AND length(opencode_root_session_id) BETWEEN 1 AND 256
	AND length(opencode_parent_session_id) <= 256
	AND opencode_session_id NOT GLOB '*[^A-Za-z0-9_-]*'
	AND opencode_root_session_id NOT GLOB '*[^A-Za-z0-9_-]*'
	AND opencode_parent_session_id NOT GLOB '*[^A-Za-z0-9_-]*'
	AND instr(opencode_session_id,char(0))=0 AND instr(opencode_root_session_id,char(0))=0
	AND instr(opencode_parent_session_id,char(0))=0
	AND ((opencode_session_id=opencode_root_session_id AND opencode_parent_session_id='')
	OR (opencode_session_id<>opencode_root_session_id AND opencode_parent_session_id<>'' AND opencode_parent_session_id<>opencode_session_id))`

// DashboardForConversation reads every population in one SQLite snapshot and at
// one instant. Empty identities are an empty view, never an administrative fallback.
func (s *Store) DashboardForConversation(ctx context.Context, workspace, requestedSessionID, rootSessionID string) (ConversationDashboard, error) {
	workspace, err := CanonicalWorkspace(workspace)
	if err != nil || workspace == "" {
		return ConversationDashboard{}, fmt.Errorf("valid dashboard project root is required")
	}
	for _, id := range []string{requestedSessionID, rootSessionID} {
		if err := (ConversationOwnership{OpenCodeSessionID: id, OpenCodeRootSessionID: id}).Validate(); err != nil {
			return ConversationDashboard{}, err
		}
	}
	d := ConversationDashboard{SchemaVersion: 2, ProjectRoot: workspace, RequestedSessionID: requestedSessionID, RootSessionID: rootSessionID,
		GeneratedAt: s.now().UTC().Format(time.RFC3339Nano), Tasks: []ConversationTask{}, Delegations: []Job{}, Attention: []ConversationAttention{},
		Summary: map[string]int{
			"backlog": 0, "ready": 0, "in_progress": 0, "in_review": 0, "blocked": 0, "done": 0, "superseded": 0,
			"active_tasks": 0, "total_tasks": 0, "total_delegations": 0, "active_delegations": 0,
			"completed_delegations": 0, "failed_delegations": 0, "active_executions": 0, "total_attention": 0,
		},
		Counts: map[string]int{"active": 0, "review": 0, "attention": 0}}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return ConversationDashboard{}, err
	}
	defer func() { _ = tx.Rollback() }()
	// Even empty scope must surface database read failures.
	var schema int
	if err = tx.QueryRowContext(ctx, `SELECT MAX(version) FROM schema_migrations`).Scan(&schema); err != nil {
		return ConversationDashboard{}, err
	}
	if requestedSessionID == "" || rootSessionID == "" {
		return d, tx.Commit()
	}
	// Older jobs stored the supplied path verbatim. Match their canonical keys
	// before pagination without mutating rows or assigning missing ownership.
	workspaceKeys := []string{workspace}
	rows, err := tx.QueryContext(ctx, `SELECT DISTINCT workspace FROM delegation_jobs WHERE opencode_root_session_id=?`, rootSessionID)
	if err != nil {
		return ConversationDashboard{}, err
	}
	for rows.Next() {
		var key string
		if err = rows.Scan(&key); err != nil {
			_ = rows.Close()
			return ConversationDashboard{}, err
		}
		if key != workspace && SameWorkspace(key, workspace) {
			workspaceKeys = append(workspaceKeys, key)
		}
	}
	if err = rows.Err(); err != nil {
		_ = rows.Close()
		return ConversationDashboard{}, err
	}
	if err = rows.Close(); err != nil {
		return ConversationDashboard{}, err
	}
	encodedKeys, err := json.Marshal(workspaceKeys)
	if err != nil {
		return ConversationDashboard{}, err
	}
	jobPredicate := strings.Replace(conversationPredicate, "workspace = ?", "workspace IN (SELECT value FROM json_each(?))", 1)
	taskPredicate := `workspace IN (SELECT value FROM json_each(?))
		AND (
			(opencode_root_session_id = ? AND length(opencode_session_id) BETWEEN 1 AND 256
				AND length(opencode_root_session_id) BETWEEN 1 AND 256
				AND length(opencode_parent_session_id) <= 256
				AND opencode_session_id NOT GLOB '*[^A-Za-z0-9_-]*'
				AND opencode_root_session_id NOT GLOB '*[^A-Za-z0-9_-]*'
				AND opencode_parent_session_id NOT GLOB '*[^A-Za-z0-9_-]*'
				AND instr(opencode_session_id,char(0))=0 AND instr(opencode_root_session_id,char(0))=0
				AND instr(opencode_parent_session_id,char(0))=0
				AND ((opencode_session_id=opencode_root_session_id AND opencode_parent_session_id='')
				OR (opencode_session_id<>opencode_root_session_id AND opencode_parent_session_id<>'' AND opencode_parent_session_id<>opencode_session_id)))
			OR (opencode_root_session_id = '')
			OR id IN (SELECT item_id FROM work_claims WHERE owner = 'opencode-session:' || ? OR owner = 'opencode-session:' || ?)
			OR id IN (SELECT task_id FROM delegation_jobs WHERE opencode_root_session_id = ? AND task_id <> '')
		)`
	cte := `WITH tasks AS (SELECT * FROM work_items WHERE ` + taskPredicate + `
		AND NOT EXISTS (SELECT 1 FROM work_decomposition_steps d WHERE d.parent_id=work_items.id)), jobs AS (SELECT * FROM delegation_jobs WHERE ` + jobPredicate + `) `
	read := func(query string, dest any, extra ...any) error {
		args := append([]any{string(encodedKeys), rootSessionID, requestedSessionID, rootSessionID, rootSessionID, string(encodedKeys), rootSessionID}, extra...)
		var raw string
		if err := tx.QueryRowContext(ctx, cte+query, args...).Scan(&raw); err != nil {
			return err
		}
		return json.Unmarshal([]byte(raw), dest)
	}
	if err = read(`SELECT json_group_object(status,n) FROM (SELECT status,count(*) n FROM tasks GROUP BY status)`, &d.Summary); err != nil {
		return ConversationDashboard{}, err
	}
	d.Summary["total_tasks"] = d.Summary["backlog"] + d.Summary["ready"] + d.Summary["in_progress"] + d.Summary["in_review"] + d.Summary["blocked"] + d.Summary["done"]
	d.Summary["active_tasks"] = d.Summary["total_tasks"] - d.Summary["done"]
	jobCounts := map[string]int{}
	if err = read(`SELECT json_object(
		'total',count(*),
		'active',COALESCE(sum(status IN ('accepted','starting','running')),0),
		'completed',COALESCE(sum(status='succeeded'),0),
		'failed',COALESCE(sum(status IN ('failed','timed_out','cancelled','lost','blocked')),0)
	) FROM jobs`, &jobCounts); err != nil {
		return ConversationDashboard{}, err
	}
	d.Summary["total_delegations"] = jobCounts["total"]
	d.Summary["active_delegations"] = jobCounts["active"]
	d.Summary["completed_delegations"] = jobCounts["completed"]
	d.Summary["failed_delegations"] = jobCounts["failed"]

	activeExec := d.Summary["in_progress"]
	if jobCounts["active"] > activeExec {
		activeExec = jobCounts["active"]
	}
	d.Summary["active_executions"] = activeExec
	d.Counts["active"] = activeExec
	d.Counts["review"] = d.Summary["in_review"]
	if err = read(`SELECT json_group_array(json_object('task_id',id,'board_id',board_id,'title',title,'status',status,'revision',revision,
		'owner',COALESCE((SELECT owner FROM work_claims WHERE item_id=t.id),''),'claim_expires_at',COALESCE((SELECT expires_at FROM work_claims WHERE item_id=t.id),''),
		'lease_count',(SELECT count(*) FROM work_leases WHERE item_id=t.id),'updated_at',updated_at,
		'opencode_session_id',opencode_session_id,'opencode_root_session_id',opencode_root_session_id,'opencode_parent_session_id',opencode_parent_session_id))
		FROM (SELECT * FROM tasks WHERE status NOT IN ('superseded')
			ORDER BY CASE status
				WHEN 'in_progress' THEN 1
				WHEN 'in_review' THEN 2
				WHEN 'blocked' THEN 3
				WHEN 'ready' THEN 4
				WHEN 'backlog' THEN 5
				WHEN 'done' THEN 6
				ELSE 7 END, updated_at DESC, id ASC LIMIT 20) t`, &d.Tasks); err != nil {
		return ConversationDashboard{}, err
	}
	if err = read(`SELECT json_group_array(json_object('job_id',id,'role',role,'task_id',task_id,'workspace',workspace,'status',status,'transport',transport,'attempt',attempt,
		'error_code',error_code,'error_message',substr(error_message,1,160),'created_at',created_at,'updated_at',updated_at,
		'opencode_session_id',opencode_session_id,'opencode_root_session_id',opencode_root_session_id,'opencode_parent_session_id',opencode_parent_session_id))
		FROM (SELECT * FROM jobs ORDER BY updated_at DESC,id ASC LIMIT 20)`, &d.Delegations); err != nil {
		return ConversationDashboard{}, err
	}
	// UTC RFC3339Nano storage: removing Z preserves strict fractional ordering,
	// unlike SQLite julianday, which rounds sub-millisecond expiry boundaries.
	cte += `, attention AS (
		SELECT 'task:blocked:'||id id,id entity_id,'blocked_task' kind,title,updated_at FROM tasks WHERE status='blocked'
		UNION ALL SELECT 'task:expired:'||t.id,t.id,'expired_claim',t.title,t.updated_at FROM tasks t JOIN work_claims c ON c.item_id=t.id
		WHERE t.status<>'superseded' AND c.owner<>'' AND rtrim(c.expires_at,'Z')<rtrim(?,'Z')
		UNION ALL SELECT 'job:'||id,id,'delegation',role,updated_at FROM jobs WHERE status IN ('blocked','failed','timed_out','lost') OR error_code<>'') `
	attentionCounts := map[string]int{}
	if err = read(`SELECT json_object('total',count(*)) FROM attention`, &attentionCounts, d.GeneratedAt); err != nil {
		return ConversationDashboard{}, err
	}
	d.Summary["total_attention"], d.Counts["attention"] = attentionCounts["total"], attentionCounts["total"]
	if err = read(`SELECT json_group_array(json_object('id',id,'entity_id',entity_id,'kind',kind,'title',title,'updated_at',updated_at))
		FROM (SELECT * FROM attention ORDER BY updated_at DESC,id ASC LIMIT 20)`, &d.Attention, d.GeneratedAt); err != nil {
		return ConversationDashboard{}, err
	}
	return d, tx.Commit()
}

func (s *Store) Dashboard(ctx context.Context) (Dashboard, error) {
	return s.dashboard(ctx, "")
}

// DashboardForWorkspace returns only durable work and delegations owned by
// one project root. The unscoped Dashboard remains available to the web
// operations console and administrative CLI surfaces.
func (s *Store) DashboardForWorkspace(ctx context.Context, workspace string) (Dashboard, error) {
	workspace, err := CanonicalWorkspace(workspace)
	if err != nil || workspace == "" {
		return Dashboard{}, fmt.Errorf("valid dashboard project root is required")
	}
	return s.dashboard(ctx, workspace)
}

func (s *Store) dashboard(ctx context.Context, workspace string) (Dashboard, error) {
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
	if workspace != "" {
		matchedTaskIDs := make(map[string]bool)
		filteredDelegations := make([]DelegationView, 0, len(delegations))
		for _, job := range delegations {
			if sameWorkspace(job.Workspace, workspace) {
				filteredDelegations = append(filteredDelegations, job)
				if job.TaskID != "" {
					matchedTaskIDs[job.TaskID] = true
				}
			}
		}
		delegations = filteredDelegations

		filteredItems := make([]WorkItem, 0, len(items))
		boardIDs := make(map[string]bool)
		for _, item := range items {
			if sameWorkspace(item.Workspace, workspace) || (item.Workspace == "" && matchedTaskIDs[item.ID]) {
				filteredItems = append(filteredItems, item)
				boardIDs[item.BoardID] = true
			}
		}
		items = filteredItems

		filteredBoards := make([]WorkBoard, 0, len(boards))
		for _, board := range boards {
			if boardIDs[board.ID] {
				filteredBoards = append(filteredBoards, board)
			}
		}
		boards = filteredBoards

		visibleEntities := make(map[string]bool, len(items)+len(delegations))
		for _, item := range items {
			visibleEntities[item.ID] = true
		}
		for _, job := range delegations {
			visibleEntities[job.ID] = true
		}
		filteredActivity := make([]ActivityEvent, 0, len(activity))
		for _, event := range activity {
			if visibleEntities[event.EntityID] {
				filteredActivity = append(filteredActivity, event)
			}
		}
		activity = filteredActivity
	}

	itemsByBoard := make(map[string][]WorkItem)
	activeWork := make([]WorkItem, 0)
	owners := make(map[string]bool)
	blocked := 0
	for _, item := range items {
		itemsByBoard[item.BoardID] = append(itemsByBoard[item.BoardID], item)
		if item.Status != WorkDone && item.Status != WorkSuperseded {
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
		counts := make(map[WorkStatus]int64)
		for _, item := range boardItems {
			counts[item.Status]++
		}
		session := WorkSession{
			BoardID:     board.ID,
			Title:       board.Title,
			Description: board.Description,
			Status:      "empty",
			TaskCount:   len(boardItems),
			Counts:      counts,
			UpdatedAt:   board.UpdatedAt,
		}
		done := 0
		sessionOwners := make(map[string]bool)
		for _, item := range boardItems {
			if item.Status == WorkDone || item.Status == WorkSuperseded {
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
	rows, err := s.db.QueryContext(ctx, `SELECT workspace,count(*) FROM delegation_jobs WHERE status IN ('accepted','starting','running','blocked') GROUP BY workspace`)
	if err != nil {
		return Dashboard{}, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var jobWorkspace string
		var count int
		if err := rows.Scan(&jobWorkspace, &count); err != nil {
			return Dashboard{}, err
		}
		if workspace == "" || sameWorkspace(jobWorkspace, workspace) {
			activeDelegations += count
		}
	}
	if err := rows.Err(); err != nil {
		return Dashboard{}, err
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
	rows, err := s.db.QueryContext(ctx, `SELECT id,role,task_id,workspace,status,transport,attempt,lease_owner,COALESCE(lease_expires_at,''),error_code,error_message,created_at,updated_at FROM delegation_jobs ORDER BY updated_at DESC,id LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	views := make([]DelegationView, 0)
	for rows.Next() {
		var view DelegationView
		if err := rows.Scan(&view.ID, &view.Role, &view.TaskID, &view.Workspace, &view.Status, &view.Transport, &view.Attempt, &view.LeaseOwner, &view.LeaseExpiresAt, &view.ErrorCode, &view.ErrorMessage, &view.CreatedAt, &view.UpdatedAt); err != nil {
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
