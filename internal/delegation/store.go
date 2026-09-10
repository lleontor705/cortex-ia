package delegation

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type Status string

const (
	StatusAccepted  Status = "accepted"
	StatusStarting  Status = "starting"
	StatusRunning   Status = "running"
	StatusBlocked   Status = "blocked"
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
	StatusTimedOut  Status = "timed_out"
	StatusCancelled Status = "cancelled"
	StatusLost      Status = "lost"
)

var ErrJobNotFound = errors.New("delegation job not found")
var ErrInvalidTransition = errors.New("invalid delegation state transition")

type Job struct {
	ConversationOwnership
	ID              string  `json:"job_id"`
	Role            string  `json:"role"`
	TaskID          string  `json:"task_id,omitempty"`
	ObjectiveDigest string  `json:"objective_digest"`
	Status          Status  `json:"status"`
	Transport       string  `json:"transport"`
	Workspace       string  `json:"workspace"`
	Worktree        string  `json:"worktree,omitempty"`
	PID             int     `json:"pid,omitempty"`
	PaneID          string  `json:"pane_id,omitempty"`
	LeaseOwner      string  `json:"lease_owner,omitempty"`
	LeaseExpiresAt  *string `json:"lease_expires_at,omitempty"`
	Attempt         int     `json:"attempt"`
	ErrorCode       string  `json:"error_code,omitempty"`
	ErrorMessage    string  `json:"error_message,omitempty"`
	CreatedAt       string  `json:"created_at"`
	UpdatedAt       string  `json:"updated_at"`
	StartedAt       string  `json:"started_at,omitempty"`
	FinishedAt      string  `json:"finished_at,omitempty"`
}

type NewJob struct {
	ConversationOwnership
	Role            string
	TaskID          string
	ObjectiveDigest string
	Transport       string
	Workspace       string
	Worktree        string
}

type Receipt struct {
	JobID      string          `json:"job_id"`
	Status     Status          `json:"status"`
	Output     json.RawMessage `json:"output,omitempty"`
	OutputHash string          `json:"output_hash,omitempty"`
	ExitCode   int             `json:"exit_code"`
	CreatedAt  string          `json:"created_at"`
}

// ConversationOwnership is host-proven provenance, never claim or board identity.
type ConversationOwnership struct {
	OpenCodeSessionID       string `json:"opencode_session_id,omitempty"`
	OpenCodeRootSessionID   string `json:"opencode_root_session_id,omitempty"`
	OpenCodeParentSessionID string `json:"opencode_parent_session_id,omitempty"`
}

func (o ConversationOwnership) Validate() error {
	s, r, p := o.OpenCodeSessionID, o.OpenCodeRootSessionID, o.OpenCodeParentSessionID
	for _, id := range []string{s, r, p} {
		if len(id) > 256 || strings.ContainsAny(id, "\x00") {
			return errors.New("invalid OpenCode session ID")
		}
		for _, c := range id {
			if c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '_' || c == '-' {
				continue
			}
			return errors.New("invalid OpenCode session ID")
		}
	}
	if s == "" && r == "" && p == "" || s != "" && r != "" && (s == r && p == "" || s != r && p != "" && p != s) {
		return nil
	}
	return errors.New("invalid OpenCode ownership tuple")
}

type Store struct {
	db  *sql.DB
	now func() time.Time
}

func DefaultDBPath(home string) string { return filepath.Join(home, ".cortex-ia", "delegation.db") }

func OpenStore(path string) (*Store, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("delegation database path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create delegation state directory: %w", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open delegation database: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	store := &Store{db: db, now: time.Now}
	if err := store.initialize(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) initialize(ctx context.Context) error {
	for _, pragma := range []string{
		"PRAGMA journal_mode=WAL", "PRAGMA foreign_keys=ON", "PRAGMA busy_timeout=5000",
	} {
		if _, err := s.db.ExecContext(ctx, pragma); err != nil {
			return fmt.Errorf("initialize delegation database: %w", err)
		}
	}
	return s.immediate(ctx, func(conn *sql.Conn) error {
		if _, err := conn.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
				version INTEGER PRIMARY KEY,
				applied_at TEXT NOT NULL
			) STRICT`); err != nil {
			return fmt.Errorf("create migration ledger: %w", err)
		}
		var version int
		if err := conn.QueryRowContext(ctx, `SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&version); err != nil {
			return fmt.Errorf("read migration ledger: %w", err)
		}
		if version > 12 {
			return fmt.Errorf("cortex database schema %d is newer than supported schema 12", version)
		}
		statements := []string{
			`CREATE TABLE IF NOT EXISTS delegation_jobs (
				id TEXT PRIMARY KEY,
				role TEXT NOT NULL,
				task_id TEXT NOT NULL DEFAULT '',
				objective_digest TEXT NOT NULL,
				status TEXT NOT NULL CHECK(status IN ('accepted','starting','running','blocked','succeeded','failed','timed_out','cancelled','lost')),
				transport TEXT NOT NULL CHECK(transport IN ('herdr','direct')),
				workspace TEXT NOT NULL,
				worktree TEXT NOT NULL DEFAULT '',
				pid INTEGER NOT NULL DEFAULT 0,
				lease_owner TEXT NOT NULL DEFAULT '',
				lease_expires_at TEXT,
				attempt INTEGER NOT NULL DEFAULT 0,
				error_code TEXT NOT NULL DEFAULT '',
				error_message TEXT NOT NULL DEFAULT '',
				created_at TEXT NOT NULL,
				updated_at TEXT NOT NULL,
				started_at TEXT NOT NULL DEFAULT '',
				finished_at TEXT NOT NULL DEFAULT ''
			) STRICT`,
			`CREATE TABLE IF NOT EXISTS delegation_events (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				job_id TEXT NOT NULL REFERENCES delegation_jobs(id) ON DELETE CASCADE,
				kind TEXT NOT NULL,
				from_status TEXT NOT NULL DEFAULT '',
				to_status TEXT NOT NULL DEFAULT '',
				detail TEXT NOT NULL DEFAULT '',
				created_at TEXT NOT NULL
			) STRICT`,
			`CREATE TABLE IF NOT EXISTS delegation_receipts (
				job_id TEXT PRIMARY KEY REFERENCES delegation_jobs(id) ON DELETE CASCADE,
				status TEXT NOT NULL,
				output_json TEXT NOT NULL DEFAULT '',
				output_hash TEXT NOT NULL DEFAULT '',
				exit_code INTEGER NOT NULL,
				created_at TEXT NOT NULL
			) STRICT`,
			`CREATE INDEX IF NOT EXISTS delegation_jobs_status_idx ON delegation_jobs(status, updated_at)`,
			`CREATE INDEX IF NOT EXISTS delegation_jobs_updated_idx ON delegation_jobs(updated_at DESC)`,
			`CREATE INDEX IF NOT EXISTS delegation_events_created_idx ON delegation_events(created_at DESC)`,
			`CREATE TABLE IF NOT EXISTS work_items (
				id TEXT PRIMARY KEY,
				title TEXT NOT NULL,
				status TEXT NOT NULL CHECK(status IN ('backlog','ready','in_progress','in_review','done','blocked')),
				revision INTEGER NOT NULL DEFAULT 1,
				created_at TEXT NOT NULL,
				updated_at TEXT NOT NULL
			) STRICT`,
			`CREATE TABLE IF NOT EXISTS work_dependencies (
				item_id TEXT NOT NULL REFERENCES work_items(id) ON DELETE CASCADE,
				depends_on TEXT NOT NULL REFERENCES work_items(id) ON DELETE RESTRICT,
				PRIMARY KEY(item_id, depends_on),
				CHECK(item_id <> depends_on)
			) STRICT`,
			`CREATE TABLE IF NOT EXISTS work_claims (
				item_id TEXT PRIMARY KEY REFERENCES work_items(id) ON DELETE CASCADE,
				owner TEXT NOT NULL,
				token_hash TEXT NOT NULL,
				attempt INTEGER NOT NULL,
				expires_at TEXT NOT NULL,
				created_at TEXT NOT NULL,
				updated_at TEXT NOT NULL
			) STRICT`,
			`CREATE TABLE IF NOT EXISTS work_leases (
				path TEXT PRIMARY KEY,
				item_id TEXT NOT NULL REFERENCES work_items(id) ON DELETE CASCADE,
				token_hash TEXT NOT NULL,
				expires_at TEXT NOT NULL,
				created_at TEXT NOT NULL,
				updated_at TEXT NOT NULL
			) STRICT`,
			`CREATE TABLE IF NOT EXISTS work_approvals (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				item_id TEXT NOT NULL REFERENCES work_items(id) ON DELETE CASCADE,
				reviewer TEXT NOT NULL,
				verdict TEXT NOT NULL CHECK(verdict IN ('PASS','FAIL','BLOCKED','INCONCLUSIVE')),
				evidence TEXT NOT NULL DEFAULT '',
				created_at TEXT NOT NULL
			) STRICT`,
			`CREATE TABLE IF NOT EXISTS work_events (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				item_id TEXT NOT NULL REFERENCES work_items(id) ON DELETE CASCADE,
				kind TEXT NOT NULL,
				from_status TEXT NOT NULL DEFAULT '',
				to_status TEXT NOT NULL DEFAULT '',
				detail TEXT NOT NULL DEFAULT '',
				created_at TEXT NOT NULL
			) STRICT`,
			`CREATE INDEX IF NOT EXISTS work_items_status_idx ON work_items(status, updated_at)`,
			`CREATE INDEX IF NOT EXISTS work_leases_item_idx ON work_leases(item_id, expires_at)`,
			`CREATE INDEX IF NOT EXISTS work_events_created_idx ON work_events(created_at DESC)`,
			`CREATE TABLE IF NOT EXISTS work_boards (
				id TEXT PRIMARY KEY,
				title TEXT NOT NULL,
				description TEXT NOT NULL DEFAULT '',
				status TEXT NOT NULL DEFAULT 'active',
				revision INTEGER NOT NULL DEFAULT 1,
				created_at TEXT NOT NULL,
				updated_at TEXT NOT NULL
			) STRICT`,
			`CREATE TABLE IF NOT EXISTS managed_worktrees (
				id TEXT PRIMARY KEY,
				repo_path TEXT NOT NULL,
				worktree_path TEXT NOT NULL UNIQUE,
				branch TEXT NOT NULL DEFAULT '',
				base_ref TEXT NOT NULL DEFAULT '',
				task_id TEXT NOT NULL DEFAULT '',
				job_id TEXT NOT NULL DEFAULT '',
				status TEXT NOT NULL CHECK(status IN ('active','released','pruned')),
				created_at TEXT NOT NULL,
				updated_at TEXT NOT NULL
			) STRICT`,
			`CREATE INDEX IF NOT EXISTS managed_worktrees_repo_idx ON managed_worktrees(repo_path, status)`,
			`CREATE INDEX IF NOT EXISTS managed_worktrees_task_idx ON managed_worktrees(task_id)`,
		}
		for _, statement := range statements {
			if _, err := conn.ExecContext(ctx, statement); err != nil {
				return fmt.Errorf("migrate delegation database: %w", err)
			}
		}
		if _, err := conn.ExecContext(ctx, `INSERT OR IGNORE INTO schema_migrations(version, applied_at) VALUES(1, ?)`, s.timestamp()); err != nil {
			return fmt.Errorf("record delegation migration: %w", err)
		}
		if _, err := conn.ExecContext(ctx, `INSERT OR IGNORE INTO schema_migrations(version, applied_at) VALUES(2, ?)`, s.timestamp()); err != nil {
			return fmt.Errorf("record work-control migration: %w", err)
		}
		if version < 3 {
			now := s.timestamp()
			if _, err := conn.ExecContext(ctx, `INSERT OR IGNORE INTO work_boards(id,title,description,created_at,updated_at) VALUES('default','Default','Default Cortex task board',?,?)`, now, now); err != nil {
				return fmt.Errorf("create default task board: %w", err)
			}
			if _, err := conn.ExecContext(ctx, `ALTER TABLE work_items ADD COLUMN board_id TEXT NOT NULL DEFAULT 'default'`); err != nil {
				return fmt.Errorf("add task board ownership: %w", err)
			}
			if _, err := conn.ExecContext(ctx, `CREATE INDEX work_items_board_status_idx ON work_items(board_id, status, updated_at)`); err != nil {
				return fmt.Errorf("index task board items: %w", err)
			}
			if _, err := conn.ExecContext(ctx, `INSERT INTO schema_migrations(version, applied_at) VALUES(3, ?)`, now); err != nil {
				return fmt.Errorf("record task-board migration: %w", err)
			}
		}
		if version < 4 {
			now := s.timestamp()
			if _, err := conn.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS work_definitions (
				item_id TEXT PRIMARY KEY REFERENCES work_items(id) ON DELETE CASCADE,
				objective TEXT NOT NULL DEFAULT '',
				acceptance_criteria TEXT NOT NULL DEFAULT '',
				verification TEXT NOT NULL DEFAULT '',
				allowed_files_json TEXT NOT NULL DEFAULT '[]'
			) STRICT`); err != nil {
				return fmt.Errorf("create work definitions: %w", err)
			}
			if _, err := conn.ExecContext(ctx, `INSERT INTO schema_migrations(version, applied_at) VALUES(4, ?)`, now); err != nil {
				return fmt.Errorf("record work-definition migration: %w", err)
			}
		}
		if version < 5 {
			now := s.timestamp()
			if _, err := conn.ExecContext(ctx, `ALTER TABLE work_items ADD COLUMN workspace TEXT NOT NULL DEFAULT ''`); err != nil {
				return fmt.Errorf("add work project ownership: %w", err)
			}
			if _, err := conn.ExecContext(ctx, `UPDATE work_items
				SET workspace = (
					SELECT MIN(j.workspace) FROM delegation_jobs j
					WHERE j.task_id = work_items.id AND TRIM(j.workspace) <> ''
				)
				WHERE workspace = '' AND 1 = (
					SELECT COUNT(DISTINCT LOWER(j.workspace)) FROM delegation_jobs j
					WHERE j.task_id = work_items.id AND TRIM(j.workspace) <> ''
				)`); err != nil {
				return fmt.Errorf("backfill work project ownership from delegations: %w", err)
			}
			if _, err := conn.ExecContext(ctx, `UPDATE work_items
				SET workspace = (
					SELECT MIN(seed.workspace) FROM work_items seed
					WHERE seed.board_id = work_items.board_id AND TRIM(seed.workspace) <> ''
				)
				WHERE workspace = '' AND 1 = (
					SELECT COUNT(DISTINCT LOWER(seed.workspace)) FROM work_items seed
					WHERE seed.board_id = work_items.board_id AND TRIM(seed.workspace) <> ''
				)`); err != nil {
				return fmt.Errorf("backfill board project ownership: %w", err)
			}
			if _, err := conn.ExecContext(ctx, `CREATE INDEX work_items_workspace_status_idx ON work_items(workspace, status, updated_at)`); err != nil {
				return fmt.Errorf("index work project ownership: %w", err)
			}
			if _, err := conn.ExecContext(ctx, `INSERT INTO schema_migrations(version, applied_at) VALUES(5, ?)`, now); err != nil {
				return fmt.Errorf("record work project migration: %w", err)
			}
		}
		if version < 6 {
			now := s.timestamp()
			if _, err := conn.ExecContext(ctx, `CREATE TABLE work_decomposition_steps (
				parent_id TEXT NOT NULL REFERENCES work_items(id) ON DELETE CASCADE,
				child_id TEXT NOT NULL UNIQUE REFERENCES work_items(id) ON DELETE RESTRICT,
				position INTEGER NOT NULL CHECK(position >= 1 AND position <= 8),
				PRIMARY KEY(parent_id, position),
				CHECK(parent_id <> child_id)
			) STRICT`); err != nil {
				return fmt.Errorf("create work decomposition steps: %w", err)
			}
			if _, err := conn.ExecContext(ctx, `CREATE INDEX work_decomposition_parent_idx ON work_decomposition_steps(parent_id, position)`); err != nil {
				return fmt.Errorf("index work decomposition steps: %w", err)
			}
			if _, err := conn.ExecContext(ctx, `INSERT INTO schema_migrations(version, applied_at) VALUES(6, ?)`, now); err != nil {
				return fmt.Errorf("record work decomposition migration: %w", err)
			}
		}
		if version < 7 {
			now := s.timestamp()
			if _, err := conn.ExecContext(ctx, `ALTER TABLE delegation_jobs ADD COLUMN pane_id TEXT NOT NULL DEFAULT ''`); err != nil {
				return fmt.Errorf("add pane_id to delegation_jobs: %w", err)
			}
			if _, err := conn.ExecContext(ctx, `INSERT INTO schema_migrations(version, applied_at) VALUES(7, ?)`, now); err != nil {
				return fmt.Errorf("record delegation pane migration: %w", err)
			}
		}
		if version < 8 {
			now := s.timestamp()
			// Try adding status column; ignore error if fresh DB table already has it
			_, _ = conn.ExecContext(ctx, `ALTER TABLE work_boards ADD COLUMN status TEXT NOT NULL DEFAULT 'active'`)
			if _, err := conn.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS work_boards_status_idx ON work_boards(status, updated_at)`); err != nil {
				return fmt.Errorf("index task board status: %w", err)
			}
			if _, err := conn.ExecContext(ctx, `INSERT INTO schema_migrations(version, applied_at) VALUES(8, ?)`, now); err != nil {
				return fmt.Errorf("record board status migration: %w", err)
			}
		}
		if version < 9 {
			now := s.timestamp()
			if _, err := conn.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS work_reviews (
				item_id TEXT NOT NULL REFERENCES work_items(id) ON DELETE CASCADE,
				review_id TEXT NOT NULL UNIQUE,
				attempt INTEGER NOT NULL,
				implementation_owner TEXT NOT NULL,
				review_revision INTEGER NOT NULL,
				created_at TEXT NOT NULL,
				PRIMARY KEY(item_id)
			) STRICT`); err != nil {
				return fmt.Errorf("create work reviews: %w", err)
			}
			_, _ = conn.ExecContext(ctx, `ALTER TABLE work_approvals ADD COLUMN review_id TEXT NOT NULL DEFAULT ''`)
			_, _ = conn.ExecContext(ctx, `ALTER TABLE work_approvals ADD COLUMN review_revision INTEGER NOT NULL DEFAULT 0`)
			_, _ = conn.ExecContext(ctx, `ALTER TABLE work_approvals ADD COLUMN attempt INTEGER NOT NULL DEFAULT 0`)
			if _, err := conn.ExecContext(ctx, `INSERT INTO schema_migrations(version, applied_at) VALUES(9, ?)`, now); err != nil {
				return fmt.Errorf("record work review lineage migration: %w", err)
			}
		}
		if version < 10 {
			now := s.timestamp()
			if _, err := conn.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS task_facts (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				board_id TEXT NOT NULL DEFAULT 'default',
				fact TEXT NOT NULL,
				source TEXT NOT NULL DEFAULT 'orchestrator',
				created_at TEXT NOT NULL
			) STRICT`); err != nil {
				return fmt.Errorf("create task facts: %w", err)
			}
			if _, err := conn.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS task_facts_board_idx ON task_facts(board_id, created_at)`); err != nil {
				return fmt.Errorf("index task facts: %w", err)
			}
			if _, err := conn.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS progress_evaluations (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				board_id TEXT NOT NULL DEFAULT 'default',
				cycle INTEGER NOT NULL,
				summary TEXT NOT NULL,
				drift_detected INTEGER NOT NULL DEFAULT 0,
				action TEXT NOT NULL DEFAULT 'continue',
				created_at TEXT NOT NULL
			) STRICT`); err != nil {
				return fmt.Errorf("create progress evaluations: %w", err)
			}
			if _, err := conn.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS progress_evaluations_board_idx ON progress_evaluations(board_id, cycle)`); err != nil {
				return fmt.Errorf("index progress evaluations: %w", err)
			}
			if _, err := conn.ExecContext(ctx, `INSERT INTO schema_migrations(version, applied_at) VALUES(10, ?)`, now); err != nil {
				return fmt.Errorf("record dual ledger migration: %w", err)
			}
		}
		if version < 11 {
			for _, table := range []string{"work_items", "delegation_jobs"} {
				for _, column := range []string{"opencode_session_id", "opencode_root_session_id", "opencode_parent_session_id"} {
					statement := fmt.Sprintf(`ALTER TABLE %s ADD COLUMN %s TEXT NOT NULL DEFAULT '' CHECK(length(CAST(%s AS BLOB)) <= 256 AND instr(%s,char(0))=0 AND %s NOT GLOB '*[^A-Za-z0-9_-]*')`, table, column, column, column, column)
					if _, err := conn.ExecContext(ctx, statement); err != nil {
						return fmt.Errorf("add conversation ownership: %w", err)
					}
				}
				for _, statement := range []string{
					fmt.Sprintf(`CREATE INDEX %s_conversation_idx ON %s(workspace,opencode_root_session_id,status,updated_at DESC)`, table, table),
					fmt.Sprintf(`CREATE TRIGGER %s_ownership_insert BEFORE INSERT ON %s WHEN NOT (
						(NEW.opencode_session_id='' AND NEW.opencode_root_session_id='' AND NEW.opencode_parent_session_id='') OR
						(NEW.opencode_session_id<>'' AND NEW.opencode_root_session_id<>'' AND (
						(NEW.opencode_session_id=NEW.opencode_root_session_id AND NEW.opencode_parent_session_id='') OR
						(NEW.opencode_session_id<>NEW.opencode_root_session_id AND NEW.opencode_parent_session_id<>'' AND NEW.opencode_parent_session_id<>NEW.opencode_session_id))))
						BEGIN SELECT RAISE(ABORT,'invalid OpenCode ownership tuple'); END`, table, table),
					fmt.Sprintf(`CREATE TRIGGER %s_ownership_immutable BEFORE UPDATE OF opencode_session_id,opencode_root_session_id,opencode_parent_session_id ON %s
						WHEN NEW.opencode_session_id IS NOT OLD.opencode_session_id OR NEW.opencode_root_session_id IS NOT OLD.opencode_root_session_id OR NEW.opencode_parent_session_id IS NOT OLD.opencode_parent_session_id
						BEGIN SELECT RAISE(ABORT,'immutable OpenCode ownership'); END`, table, table),
				} {
					if _, err := conn.ExecContext(ctx, statement); err != nil {
						return fmt.Errorf("guard conversation ownership: %w", err)
					}
				}
			}
			if _, err := conn.ExecContext(ctx, `INSERT INTO schema_migrations(version,applied_at) VALUES(11,?)`, s.timestamp()); err != nil {
				return fmt.Errorf("record conversation ownership migration: %w", err)
			}
		}
		if _, err := conn.ExecContext(ctx, `PRAGMA optimize`); err != nil {
			return fmt.Errorf("optimize cortex database: %w", err)
		}
		if version < 12 {
			for _, statement := range []string{
				`ALTER TABLE work_definitions ADD COLUMN contract_json TEXT NOT NULL DEFAULT '' CHECK(length(CAST(contract_json AS BLOB))<=32768)`,
				`ALTER TABLE work_reviews ADD COLUMN binding_json TEXT NOT NULL DEFAULT '' CHECK(length(CAST(binding_json AS BLOB))<=65536)`,
				`ALTER TABLE work_approvals ADD COLUMN binding_json TEXT NOT NULL DEFAULT '' CHECK(length(CAST(binding_json AS BLOB))<=65536)`,
				`ALTER TABLE work_approvals ADD COLUMN implementation_owner TEXT NOT NULL DEFAULT ''`,
				`CREATE TABLE change_archives (archive_key TEXT PRIMARY KEY, board_id TEXT NOT NULL, workspace TEXT NOT NULL, change_id TEXT NOT NULL, workflow TEXT NOT NULL, spec_plane TEXT NOT NULL, binding_sha256 TEXT NOT NULL, manifest_sha256 TEXT NOT NULL DEFAULT '', source_path TEXT NOT NULL DEFAULT '', destination_path TEXT NOT NULL DEFAULT '', status TEXT NOT NULL CHECK(status IN('prepared','complete')), receipt_json TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL, updated_at TEXT NOT NULL) STRICT`,
			} {
				if _, err := conn.ExecContext(ctx, statement); err != nil {
					return fmt.Errorf("SDD contract migration: %w", err)
				}
			}
			if _, err := conn.ExecContext(ctx, `INSERT INTO schema_migrations(version,applied_at) VALUES(12,?)`, s.timestamp()); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Store) timestamp() string { return s.now().UTC().Format(time.RFC3339Nano) }

func (s *Store) immediate(ctx context.Context, fn func(*sql.Conn) error) error {
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()
	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_, _ = conn.ExecContext(context.Background(), "ROLLBACK")
		}
	}()
	if err := fn(conn); err != nil {
		return err
	}
	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return err
	}
	committed = true
	return nil
}

func newID() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}

func (s *Store) SetPaneID(ctx context.Context, jobID, paneID string) error {
	now := s.timestamp()
	return s.immediate(ctx, func(conn *sql.Conn) error {
		_, err := conn.ExecContext(ctx, `UPDATE delegation_jobs SET pane_id=?, updated_at=? WHERE id=?`, paneID, now, jobID)
		return err
	})
}

func (s *Store) Create(ctx context.Context, input NewJob) (Job, error) {
	if err := input.Validate(); err != nil {
		return Job{}, err
	}
	if !supportedRoles[input.Role] {
		return Job{}, fmt.Errorf("unsupported role %q", input.Role)
	}
	if input.Transport != "herdr" && input.Transport != "direct" {
		return Job{}, fmt.Errorf("unsupported transport %q", input.Transport)
	}
	if strings.TrimSpace(input.Workspace) == "" || strings.TrimSpace(input.ObjectiveDigest) == "" {
		return Job{}, errors.New("workspace and objective digest are required")
	}
	workspace, err := CanonicalWorkspace(input.Workspace)
	if err != nil {
		return Job{}, fmt.Errorf("normalize job workspace: %w", err)
	}
	input.Workspace = workspace
	id, err := newID()
	if err != nil {
		return Job{}, err
	}
	now := s.timestamp()
	job := Job{ID: id, Role: input.Role, TaskID: input.TaskID, ObjectiveDigest: input.ObjectiveDigest, Status: StatusAccepted, Transport: input.Transport, Workspace: input.Workspace, Worktree: input.Worktree, CreatedAt: now, UpdatedAt: now}
	job.ConversationOwnership = input.ConversationOwnership
	err = s.immediate(ctx, func(conn *sql.Conn) error {
		if job.TaskID != "" {
			var taskWorkspace string
			err := conn.QueryRowContext(ctx, `SELECT workspace FROM work_items WHERE id=?`, job.TaskID).Scan(&taskWorkspace)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return err
			}
			if err == nil {
				if strings.TrimSpace(taskWorkspace) != "" && !WorkspacesCompatible(taskWorkspace, job.Workspace) {
					return fmt.Errorf("%w: task/job workspace mismatch", ErrWorkConflict)
				}
			}
		}
		_, err := conn.ExecContext(ctx, `INSERT INTO delegation_jobs(id, role, task_id, objective_digest, status, transport, workspace, worktree, created_at, updated_at,opencode_session_id,opencode_root_session_id,opencode_parent_session_id) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`, job.ID, job.Role, job.TaskID, job.ObjectiveDigest, job.Status, job.Transport, job.Workspace, job.Worktree, now, now, job.OpenCodeSessionID, job.OpenCodeRootSessionID, job.OpenCodeParentSessionID)
		if err != nil {
			return err
		}
		return s.addEvent(ctx, conn, job.ID, "created", "", StatusAccepted, "")
	})
	return job, err
}

func (s *Store) Claim(ctx context.Context, id, owner string, pid int, ttl time.Duration) error {
	if strings.TrimSpace(owner) == "" || ttl <= 0 {
		return errors.New("claim owner and positive ttl are required")
	}
	now := s.timestamp()
	expires := s.now().UTC().Add(ttl).Format(time.RFC3339Nano)
	return s.immediate(ctx, func(conn *sql.Conn) error {
		result, err := conn.ExecContext(ctx, `UPDATE delegation_jobs SET status='starting', pid=?, lease_owner=?, lease_expires_at=?, attempt=attempt+1, updated_at=?, started_at=? WHERE id=? AND status='accepted'`, pid, owner, expires, now, now, id)
		if err != nil {
			return err
		}
		return s.requireTransition(ctx, conn, result, id, StatusAccepted, StatusStarting)
	})
}

func (s *Store) MarkRunning(ctx context.Context, id string) error {
	return s.transition(ctx, id, []Status{StatusStarting}, StatusRunning, "", "")
}

func (s *Store) MarkBlocked(ctx context.Context, id, detail string) error {
	return s.transition(ctx, id, []Status{StatusRunning}, StatusBlocked, "blocked", detail)
}

func (s *Store) MarkResumed(ctx context.Context, id string) error {
	return s.transition(ctx, id, []Status{StatusBlocked}, StatusRunning, "resumed", "")
}

func (s *Store) ExtendJobLease(ctx context.Context, id, owner string, ttl time.Duration) error {
	if ttl <= 0 {
		return errors.New("positive ttl is required")
	}
	now := s.timestamp()
	expires := s.now().UTC().Add(ttl).Format(time.RFC3339Nano)
	return s.immediate(ctx, func(conn *sql.Conn) error {
		var status Status
		var currentOwner string
		if err := conn.QueryRowContext(ctx, `SELECT status, lease_owner FROM delegation_jobs WHERE id=?`, id).Scan(&status, &currentOwner); err != nil {
			return err
		}
		if status != StatusRunning && status != StatusStarting && status != StatusBlocked {
			return fmt.Errorf("%w: job %s is not active", ErrInvalidTransition, id)
		}
		if owner != "" && currentOwner != owner {
			return fmt.Errorf("%w: lease owner mismatch", ErrInvalidTransition)
		}
		_, err := conn.ExecContext(ctx, `UPDATE delegation_jobs SET lease_expires_at=?, updated_at=? WHERE id=?`, expires, now, id)
		return err
	})
}

func (s *Store) Complete(ctx context.Context, id string, status Status, receipt Receipt, code, message string) error {
	if status != StatusSucceeded && status != StatusFailed && status != StatusTimedOut && status != StatusCancelled {
		return fmt.Errorf("%w: terminal status %q", ErrInvalidTransition, status)
	}
	if len(receipt.Output) > 1024*1024 {
		return errors.New("delegation receipt exceeds 1 MiB")
	}
	now := s.timestamp()
	return s.immediate(ctx, func(conn *sql.Conn) error {
		var from Status
		if err := conn.QueryRowContext(ctx, `SELECT status FROM delegation_jobs WHERE id=?`, id).Scan(&from); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrJobNotFound
			}
			return err
		}
		if from != StatusStarting && from != StatusRunning && from != StatusBlocked {
			return fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, from, status)
		}
		_, err := conn.ExecContext(ctx, `UPDATE delegation_jobs SET status=?, error_code=?, error_message=?, lease_owner='', lease_expires_at=NULL, updated_at=?, finished_at=? WHERE id=? AND status=?`, status, bounded(code, 64), bounded(message, 512), now, now, id, from)
		if err != nil {
			return err
		}
		receipt.JobID, receipt.Status, receipt.CreatedAt = id, status, now
		_, err = conn.ExecContext(ctx, `INSERT INTO delegation_receipts(job_id,status,output_json,output_hash,exit_code,created_at) VALUES(?,?,?,?,?,?) ON CONFLICT(job_id) DO UPDATE SET status=excluded.status, output_json=excluded.output_json, output_hash=excluded.output_hash, exit_code=excluded.exit_code, created_at=excluded.created_at`, id, status, string(receipt.Output), receipt.OutputHash, receipt.ExitCode, now)
		if err != nil {
			return err
		}
		return s.addEvent(ctx, conn, id, "completed", from, status, bounded(message, 512))
	})
}

func (s *Store) Cancel(ctx context.Context, id string) error {
	now := s.timestamp()
	return s.immediate(ctx, func(conn *sql.Conn) error {
		var from Status
		if err := conn.QueryRowContext(ctx, `SELECT status FROM delegation_jobs WHERE id=?`, id).Scan(&from); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrJobNotFound
			}
			return err
		}
		if terminal(from) {
			return nil
		}
		_, err := conn.ExecContext(ctx, `UPDATE delegation_jobs SET status='cancelled', error_code='CANCELLED', error_message='cancellation requested', lease_owner='', lease_expires_at=NULL, updated_at=?, finished_at=? WHERE id=? AND status=?`, now, now, id, from)
		if err != nil {
			return err
		}
		_, err = conn.ExecContext(ctx, `INSERT INTO delegation_receipts(job_id,status,output_json,output_hash,exit_code,created_at) VALUES(?,'cancelled','{}','',-1,?) ON CONFLICT(job_id) DO UPDATE SET status='cancelled', output_json='{}', output_hash='', exit_code=-1, created_at=excluded.created_at`, id, now)
		if err != nil {
			return err
		}
		return s.addEvent(ctx, conn, id, "cancelled", from, StatusCancelled, "cancellation requested")
	})
}

func (s *Store) Get(ctx context.Context, id string) (Job, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id,role,task_id,objective_digest,status,transport,workspace,worktree,pid,pane_id,lease_owner,lease_expires_at,attempt,error_code,error_message,created_at,updated_at,started_at,finished_at,opencode_session_id,opencode_root_session_id,opencode_parent_session_id FROM delegation_jobs WHERE id=?`, id)
	var job Job
	var lease sql.NullString
	if err := row.Scan(&job.ID, &job.Role, &job.TaskID, &job.ObjectiveDigest, &job.Status, &job.Transport, &job.Workspace, &job.Worktree, &job.PID, &job.PaneID, &job.LeaseOwner, &lease, &job.Attempt, &job.ErrorCode, &job.ErrorMessage, &job.CreatedAt, &job.UpdatedAt, &job.StartedAt, &job.FinishedAt, &job.OpenCodeSessionID, &job.OpenCodeRootSessionID, &job.OpenCodeParentSessionID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Job{}, ErrJobNotFound
		}
		return Job{}, err
	}
	if lease.Valid {
		job.LeaseExpiresAt = &lease.String
	}
	return job, nil
}

func (s *Store) Result(ctx context.Context, id string) (Receipt, error) {
	var receipt Receipt
	var output string
	err := s.db.QueryRowContext(ctx, `SELECT job_id,status,output_json,output_hash,exit_code,created_at FROM delegation_receipts WHERE job_id=?`, id).Scan(&receipt.JobID, &receipt.Status, &output, &receipt.OutputHash, &receipt.ExitCode, &receipt.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		var jobStatus Status
		checkErr := s.db.QueryRowContext(ctx, `SELECT status FROM delegation_jobs WHERE id=?`, id).Scan(&jobStatus)
		if checkErr == nil {
			return Receipt{}, fmt.Errorf("delegation job %s has not completed (current status: %s)", id, jobStatus)
		}
		return Receipt{}, ErrJobNotFound
	}
	receipt.Output = json.RawMessage(output)
	return receipt, err
}

func (s *Store) Recover(ctx context.Context) (int64, error) {
	now := s.timestamp()
	var changed int64
	err := s.immediate(ctx, func(conn *sql.Conn) error {
		rows, err := conn.QueryContext(ctx, `SELECT id,status FROM delegation_jobs WHERE status IN ('starting','running','blocked') AND lease_expires_at IS NOT NULL AND lease_expires_at < ? ORDER BY id`, now)
		if err != nil {
			return err
		}
		type expiredJob struct {
			id     string
			status Status
		}
		var expired []expiredJob
		for rows.Next() {
			var job expiredJob
			if err := rows.Scan(&job.id, &job.status); err != nil {
				_ = rows.Close()
				return err
			}
			expired = append(expired, job)
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return err
		}
		if err := rows.Close(); err != nil {
			return err
		}
		for _, job := range expired {
			result, err := conn.ExecContext(ctx, `UPDATE delegation_jobs SET status='lost', error_code='LEASE_EXPIRED', error_message='worker lease expired', lease_owner='', lease_expires_at=NULL, updated_at=?, finished_at=? WHERE id=? AND status=?`, now, now, job.id, job.status)
			if err != nil {
				return err
			}
			rowsAffected, err := result.RowsAffected()
			if err != nil {
				return err
			}
			if rowsAffected != 1 {
				return fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, job.status, StatusLost)
			}
			if _, err := conn.ExecContext(ctx, `INSERT INTO delegation_receipts(job_id,status,output_json,output_hash,exit_code,created_at) VALUES(?,'lost','{}','',-1,?) ON CONFLICT(job_id) DO UPDATE SET status='lost', output_json='{}', output_hash='', exit_code=-1, created_at=excluded.created_at`, job.id, now); err != nil {
				return err
			}
			if err := s.addEvent(ctx, conn, job.id, "lease_expired", job.status, StatusLost, "worker lease expired"); err != nil {
				return err
			}
			changed++
		}
		return nil
	})
	return changed, err
}

func (s *Store) transition(ctx context.Context, id string, from []Status, to Status, kind, detail string) error {
	return s.immediate(ctx, func(conn *sql.Conn) error {
		var current Status
		if err := conn.QueryRowContext(ctx, `SELECT status FROM delegation_jobs WHERE id=?`, id).Scan(&current); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrJobNotFound
			}
			return err
		}
		allowed := false
		for _, candidate := range from {
			allowed = allowed || current == candidate
		}
		if !allowed {
			return fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, current, to)
		}
		now := s.timestamp()
		finished := ""
		if terminal(to) {
			finished = now
		}
		result, err := conn.ExecContext(ctx, `UPDATE delegation_jobs SET status=?, updated_at=?, finished_at=? WHERE id=? AND status=?`, to, now, finished, id, current)
		if err != nil {
			return err
		}
		if err := s.requireTransition(ctx, conn, result, id, current, to); err != nil {
			return err
		}
		if kind == "" {
			kind = "transition"
		}
		return s.addEvent(ctx, conn, id, kind, current, to, bounded(detail, 512))
	})
}

func (s *Store) requireTransition(ctx context.Context, conn *sql.Conn, result sql.Result, id string, from, to Status) error {
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed != 1 {
		var count int
		if err := conn.QueryRowContext(ctx, `SELECT count(*) FROM delegation_jobs WHERE id=?`, id).Scan(&count); err != nil {
			return err
		}
		if count == 0 {
			return ErrJobNotFound
		}
		return fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, from, to)
	}
	return s.addEvent(ctx, conn, id, "transition", from, to, "")
}

func (s *Store) addEvent(ctx context.Context, conn *sql.Conn, id, kind string, from, to Status, detail string) error {
	_, err := conn.ExecContext(ctx, `INSERT INTO delegation_events(job_id,kind,from_status,to_status,detail,created_at) VALUES(?,?,?,?,?,?)`, id, kind, from, to, detail, s.timestamp())
	return err
}

func terminal(status Status) bool {
	return status == StatusSucceeded || status == StatusFailed || status == StatusTimedOut || status == StatusCancelled || status == StatusLost
}

func bounded(value string, limit int) string {
	value = strings.ReplaceAll(strings.ReplaceAll(value, "\r", " "), "\n", " ")
	if len(value) > limit {
		return value[:limit]
	}
	return value
}

// ManagedWorktree represents a tracked worktree entry in SQLite.
type ManagedWorktree struct {
	ID           string `json:"id"`
	RepoPath     string `json:"repo_path"`
	WorktreePath string `json:"worktree_path"`
	Branch       string `json:"branch,omitempty"`
	BaseRef      string `json:"base_ref,omitempty"`
	TaskID       string `json:"task_id,omitempty"`
	JobID        string `json:"job_id,omitempty"`
	Status       string `json:"status"` // active, released, pruned
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

// RegisterManagedWorktree inserts or updates a tracked worktree entry.
func (s *Store) RegisterManagedWorktree(ctx context.Context, m ManagedWorktree) error {
	if strings.TrimSpace(m.ID) == "" {
		m.ID = fmt.Sprintf("wt-%d", time.Now().UnixNano())
	}
	canonicalRepo, err := CanonicalWorkspace(m.RepoPath)
	if err == nil && canonicalRepo != "" {
		m.RepoPath = canonicalRepo
	}
	canonicalWT, err := CanonicalWorkspace(m.WorktreePath)
	if err == nil && canonicalWT != "" {
		m.WorktreePath = canonicalWT
	}
	if m.Status == "" {
		m.Status = "active"
	}
	now := s.timestamp()
	if m.CreatedAt == "" {
		m.CreatedAt = now
	}
	m.UpdatedAt = now

	return s.immediate(ctx, func(conn *sql.Conn) error {
		_, err := conn.ExecContext(ctx, `
			INSERT INTO managed_worktrees(id, repo_path, worktree_path, branch, base_ref, task_id, job_id, status, created_at, updated_at)
			VALUES(?,?,?,?,?,?,?,?,?,?)
			ON CONFLICT(worktree_path) DO UPDATE SET
				branch=excluded.branch,
				base_ref=excluded.base_ref,
				task_id=excluded.task_id,
				job_id=excluded.job_id,
				status=excluded.status,
				updated_at=excluded.updated_at
		`, m.ID, m.RepoPath, m.WorktreePath, m.Branch, m.BaseRef, m.TaskID, m.JobID, m.Status, m.CreatedAt, m.UpdatedAt)
		return err
	})
}

// GetManagedWorktree retrieves a tracked worktree by its path.
func (s *Store) GetManagedWorktree(ctx context.Context, worktreePath string) (*ManagedWorktree, error) {
	canonicalWT, err := CanonicalWorkspace(worktreePath)
	if err != nil || canonicalWT == "" {
		canonicalWT = filepath.ToSlash(filepath.Clean(worktreePath))
	}
	var m ManagedWorktree
	err = s.db.QueryRowContext(ctx, `
		SELECT id, repo_path, worktree_path, branch, base_ref, task_id, job_id, status, created_at, updated_at
		FROM managed_worktrees WHERE worktree_path=?
	`, canonicalWT).Scan(&m.ID, &m.RepoPath, &m.WorktreePath, &m.Branch, &m.BaseRef, &m.TaskID, &m.JobID, &m.Status, &m.CreatedAt, &m.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// ListManagedWorktrees returns tracked worktrees for a repository, optionally filtered by status.
func (s *Store) ListManagedWorktrees(ctx context.Context, repoPath, status string) ([]ManagedWorktree, error) {
	canonicalRepo, err := CanonicalWorkspace(repoPath)
	if err != nil || canonicalRepo == "" {
		canonicalRepo = filepath.ToSlash(filepath.Clean(repoPath))
	}
	query := `SELECT id, repo_path, worktree_path, branch, base_ref, task_id, job_id, status, created_at, updated_at FROM managed_worktrees WHERE repo_path=?`
	args := []any{canonicalRepo}
	if status != "" {
		query += ` AND status=?`
		args = append(args, status)
	}
	query += ` ORDER BY created_at DESC`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var list []ManagedWorktree
	for rows.Next() {
		var m ManagedWorktree
		if err := rows.Scan(&m.ID, &m.RepoPath, &m.WorktreePath, &m.Branch, &m.BaseRef, &m.TaskID, &m.JobID, &m.Status, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, m)
	}
	return list, rows.Err()
}

// UpdateManagedWorktreeStatus updates the lifecycle status of a managed worktree.
func (s *Store) UpdateManagedWorktreeStatus(ctx context.Context, worktreePath, status string) error {
	canonicalWT, err := CanonicalWorkspace(worktreePath)
	if err != nil || canonicalWT == "" {
		canonicalWT = filepath.ToSlash(filepath.Clean(worktreePath))
	}
	return s.immediate(ctx, func(conn *sql.Conn) error {
		_, err := conn.ExecContext(ctx, `UPDATE managed_worktrees SET status=?, updated_at=? WHERE worktree_path=?`, status, s.timestamp(), canonicalWT)
		return err
	})
}
