package delegation

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type WorkStatus string

const (
	WorkBacklog       WorkStatus = "backlog"
	WorkReady         WorkStatus = "ready"
	WorkInProgress    WorkStatus = "in_progress"
	WorkInReview      WorkStatus = "in_review"
	WorkDone          WorkStatus = "done"
	WorkBlocked       WorkStatus = "blocked"
	WorkSuperseded    WorkStatus = "superseded"
	maxLeasePathBytes            = 1024
	MaxWorkAttempts   int64      = 5
)

var ErrWorkNotFound = errors.New("work item not found")
var ErrWorkConflict = errors.New("work control conflict")
var ErrWorkAttemptLimit = errors.New("work attempt limit reached")

type WorkItem struct {
	ID           string      `json:"task_id"`
	BoardID      string      `json:"board_id"`
	Workspace    string      `json:"workspace,omitempty"`
	Title        string      `json:"title"`
	Objective    string      `json:"objective,omitempty"`
	Acceptance   string      `json:"acceptance_criteria,omitempty"`
	Verification string      `json:"verification,omitempty"`
	AllowedFiles []string    `json:"allowed_files,omitempty"`
	Replaces     string      `json:"replaces,omitempty"`
	ReplacedBy   []string    `json:"replaced_by,omitempty"`
	Status       WorkStatus  `json:"status"`
	Revision     int64       `json:"revision"`
	Dependencies []string    `json:"dependencies,omitempty"`
	Claim        *WorkClaim  `json:"claim,omitempty"`
	Leases       []WorkLease `json:"leases,omitempty"`
	CreatedAt    string      `json:"created_at"`
	UpdatedAt    string      `json:"updated_at"`
}

type WorkDefinition struct {
	Objective    string
	Acceptance   string
	Verification string
	AllowedFiles []string
}

type WorkClaim struct {
	Owner     string `json:"owner"`
	Attempt   int64  `json:"attempt"`
	Revision  int64  `json:"revision,omitempty"`
	ExpiresAt string `json:"expires_at"`
	Token     string `json:"claim_token,omitempty"`
}

type WorkLease struct {
	Path      string `json:"path"`
	ItemID    string `json:"task_id"`
	ExpiresAt string `json:"expires_at"`
	Token     string `json:"lease_token,omitempty"`
}

type WorkApproval struct {
	ItemID    string `json:"task_id"`
	Revision  int64  `json:"revision"`
	Reviewer  string `json:"reviewer"`
	Verdict   string `json:"verdict"`
	Evidence  string `json:"evidence,omitempty"`
	CreatedAt string `json:"created_at"`
}

func token() (string, error) {
	raw := make([]byte, 32)
	if _, err := randRead(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}

var randRead = func(raw []byte) (int, error) { return rand.Read(raw) }

func tokenHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func (s *Store) CreateWork(ctx context.Context, id, title string, dependencies []string) (WorkItem, error) {
	return s.CreateWorkInBoard(ctx, DefaultBoardID, id, title, dependencies)
}

func (s *Store) CreateWorkInBoard(ctx context.Context, boardID, id, title string, dependencies []string) (WorkItem, error) {
	return s.CreateWorkInBoardWithDefinition(ctx, boardID, id, title, dependencies, WorkDefinition{})
}

func (s *Store) CreateWorkInBoardWithDefinition(ctx context.Context, boardID, id, title string, dependencies []string, definition WorkDefinition) (WorkItem, error) {
	workspace, err := ResolveProjectRoot("")
	if err != nil {
		return WorkItem{}, fmt.Errorf("resolve task project: %w", err)
	}
	return s.createWorkInBoardWithDefinition(ctx, workspace, boardID, id, title, dependencies, definition)
}

func (s *Store) createWorkInBoardWithDefinition(ctx context.Context, workspace, boardID, id, title string, dependencies []string, definition WorkDefinition) (WorkItem, error) {
	boardID = strings.TrimSpace(boardID)
	if boardID == "" {
		boardID = DefaultBoardID
	}
	id, title = strings.TrimSpace(id), strings.TrimSpace(title)
	if id == "" || title == "" {
		return WorkItem{}, errors.New("task id and title are required")
	}
	if len(id) > 128 || len(title) > 512 {
		return WorkItem{}, errors.New("task id or title exceeds size limit")
	}
	definition.Objective = strings.TrimSpace(definition.Objective)
	definition.Acceptance = strings.TrimSpace(definition.Acceptance)
	definition.Verification = strings.TrimSpace(definition.Verification)
	if len(definition.Objective) > 4096 || len(definition.Acceptance) > 4096 || len(definition.Verification) > 2048 {
		return WorkItem{}, errors.New("task objective, acceptance criteria, or verification exceeds size limit")
	}
	if len(definition.AllowedFiles) > 128 {
		return WorkItem{}, errors.New("task allowed file count exceeds limit")
	}
	allowedFiles := make([]string, 0, len(definition.AllowedFiles))
	seenFiles := make(map[string]struct{}, len(definition.AllowedFiles))
	for _, value := range definition.AllowedFiles {
		path, err := canonicalLeasePath(value)
		if err != nil {
			return WorkItem{}, fmt.Errorf("invalid allowed file %q: %w", value, err)
		}
		if _, duplicate := seenFiles[path]; duplicate {
			continue
		}
		seenFiles[path] = struct{}{}
		allowedFiles = append(allowedFiles, path)
	}
	workspace, err := CanonicalWorkspace(workspace)
	if err != nil || workspace == "" {
		return WorkItem{}, errors.New("task project root is required")
	}
	allowedFilesJSON, err := json.Marshal(allowedFiles)
	if err != nil {
		return WorkItem{}, fmt.Errorf("encode allowed files: %w", err)
	}
	now := s.timestamp()
	status := WorkReady
	if len(dependencies) > 0 {
		status = WorkBacklog
	}
	err = s.immediate(ctx, func(conn *sql.Conn) error {
		var boardExists int
		if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM work_boards WHERE id=?`, boardID).Scan(&boardExists); err != nil {
			return err
		}
		if boardExists != 1 {
			return ErrBoardNotFound
		}
		if _, err := conn.ExecContext(ctx, `INSERT INTO work_items(id,title,status,created_at,updated_at,board_id,workspace) VALUES(?,?,?,?,?,?,?)`, id, title, status, now, now, boardID, workspace); err != nil {
			return fmt.Errorf("create work item: %w", err)
		}
		if _, err := conn.ExecContext(ctx, `INSERT INTO work_definitions(item_id,objective,acceptance_criteria,verification,allowed_files_json) VALUES(?,?,?,?,?)`, id, definition.Objective, definition.Acceptance, definition.Verification, string(allowedFilesJSON)); err != nil {
			return fmt.Errorf("create work definition: %w", err)
		}
		seen := map[string]bool{}
		for _, dependency := range dependencies {
			dependency = strings.TrimSpace(dependency)
			if dependency == "" || dependency == id || seen[dependency] {
				return fmt.Errorf("invalid dependency %q", dependency)
			}
			seen[dependency] = true
			var dependencyBoard, dependencyWorkspace string
			if err := conn.QueryRowContext(ctx, `SELECT board_id,workspace FROM work_items WHERE id=?`, dependency).Scan(&dependencyBoard, &dependencyWorkspace); err != nil {
				return fmt.Errorf("read dependency %q: %w", dependency, err)
			}
			if dependencyBoard != boardID {
				return fmt.Errorf("dependency %q belongs to board %q, not %q", dependency, dependencyBoard, boardID)
			}
			if dependencyWorkspace != "" && !sameWorkspace(dependencyWorkspace, workspace) {
				return fmt.Errorf("dependency %q belongs to another project", dependency)
			}
			if _, err := conn.ExecContext(ctx, `INSERT INTO work_dependencies(item_id,depends_on) VALUES(?,?)`, id, dependency); err != nil {
				return fmt.Errorf("add dependency %q: %w", dependency, err)
			}
		}
		return s.addWorkEvent(ctx, conn, id, "created", "", string(status), "")
	})
	if err != nil {
		return WorkItem{}, err
	}
	return s.GetWork(ctx, id)
}

func (s *Store) ListWork(ctx context.Context) ([]WorkItem, error) {
	return s.listWork(ctx, "", false)
}

func (s *Store) ListWorkByBoard(ctx context.Context, boardID string) ([]WorkItem, error) {
	if _, err := s.GetBoard(ctx, boardID); err != nil {
		return nil, err
	}
	return s.listWork(ctx, strings.TrimSpace(boardID), true)
}

func (s *Store) listWork(ctx context.Context, boardID string, filtered bool) ([]WorkItem, error) {
	query := `SELECT id FROM work_items ORDER BY CASE WHEN EXISTS (SELECT 1 FROM work_decomposition_steps d WHERE d.parent_id=work_items.id) THEN 6 ELSE CASE status WHEN 'in_progress' THEN 0 WHEN 'ready' THEN 1 WHEN 'blocked' THEN 2 WHEN 'in_review' THEN 3 WHEN 'backlog' THEN 4 ELSE 5 END END, updated_at, id`
	var rows *sql.Rows
	var err error
	if filtered {
		query = `SELECT id FROM work_items WHERE board_id=? ORDER BY CASE WHEN EXISTS (SELECT 1 FROM work_decomposition_steps d WHERE d.parent_id=work_items.id) THEN 6 ELSE CASE status WHEN 'in_progress' THEN 0 WHEN 'ready' THEN 1 WHEN 'blocked' THEN 2 WHEN 'in_review' THEN 3 WHEN 'backlog' THEN 4 ELSE 5 END END, updated_at, id`
		rows, err = s.db.QueryContext(ctx, query, boardID)
	} else {
		rows, err = s.db.QueryContext(ctx, query)
	}
	if err != nil {
		return nil, err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	items := make([]WorkItem, 0, len(ids))
	for _, id := range ids {
		item, err := s.GetWork(ctx, id)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (s *Store) GetWork(ctx context.Context, id string) (WorkItem, error) {
	var item WorkItem
	err := s.db.QueryRowContext(ctx, `SELECT id,board_id,workspace,title,status,revision,created_at,updated_at FROM work_items WHERE id=?`, id).Scan(&item.ID, &item.BoardID, &item.Workspace, &item.Title, &item.Status, &item.Revision, &item.CreatedAt, &item.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return WorkItem{}, ErrWorkNotFound
	}
	if err != nil {
		return WorkItem{}, err
	}
	if canonical, canonicalErr := CanonicalWorkspace(item.Workspace); canonicalErr == nil {
		item.Workspace = canonical
	}
	var allowedFilesJSON string
	err = s.db.QueryRowContext(ctx, `SELECT objective,acceptance_criteria,verification,allowed_files_json FROM work_definitions WHERE item_id=?`, id).Scan(&item.Objective, &item.Acceptance, &item.Verification, &allowedFilesJSON)
	if err == nil {
		if err := json.Unmarshal([]byte(allowedFilesJSON), &item.AllowedFiles); err != nil {
			return WorkItem{}, fmt.Errorf("decode task allowed files: %w", err)
		}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return WorkItem{}, err
	}
	deps, err := s.db.QueryContext(ctx, `SELECT depends_on FROM work_dependencies WHERE item_id=? ORDER BY depends_on`, id)
	if err != nil {
		return WorkItem{}, err
	}
	for deps.Next() {
		var dependency string
		if err := deps.Scan(&dependency); err != nil {
			_ = deps.Close()
			return WorkItem{}, err
		}
		item.Dependencies = append(item.Dependencies, dependency)
	}
	if err := deps.Close(); err != nil {
		return WorkItem{}, err
	}
	if err := s.db.QueryRowContext(ctx, `SELECT parent_id FROM work_decomposition_steps WHERE child_id=?`, id).Scan(&item.Replaces); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return WorkItem{}, err
	}
	replacements, err := s.db.QueryContext(ctx, `SELECT child_id FROM work_decomposition_steps WHERE parent_id=? ORDER BY position`, id)
	if err != nil {
		return WorkItem{}, err
	}
	for replacements.Next() {
		var childID string
		if err := replacements.Scan(&childID); err != nil {
			_ = replacements.Close()
			return WorkItem{}, err
		}
		item.ReplacedBy = append(item.ReplacedBy, childID)
	}
	if err := replacements.Close(); err != nil {
		return WorkItem{}, err
	}
	if len(item.ReplacedBy) > 0 {
		item.Status = WorkSuperseded
	}
	var claim WorkClaim
	err = s.db.QueryRowContext(ctx, `SELECT owner,attempt,expires_at FROM work_claims WHERE item_id=?`, id).Scan(&claim.Owner, &claim.Attempt, &claim.ExpiresAt)
	if err == nil {
		item.Claim = &claim
	} else if !errors.Is(err, sql.ErrNoRows) {
		return WorkItem{}, err
	}
	leases, err := s.db.QueryContext(ctx, `SELECT path,item_id,expires_at FROM work_leases WHERE item_id=? ORDER BY path`, id)
	if err != nil {
		return WorkItem{}, err
	}
	for leases.Next() {
		var lease WorkLease
		if err := leases.Scan(&lease.Path, &lease.ItemID, &lease.ExpiresAt); err != nil {
			return WorkItem{}, err
		}
		item.Leases = append(item.Leases, lease)
	}
	if err := leases.Err(); err != nil {
		_ = leases.Close()
		return WorkItem{}, err
	}
	if err := leases.Close(); err != nil {
		return WorkItem{}, err
	}
	return item, nil
}

// ValidateDelegationAuthority verifies that an implementation delegation is
// backed by live durable authority. The transient handoff never carries claim
// or lease tokens: the worker only needs proof that the task is in progress and
// that every writable path is reserved for that task.
func (s *Store) ValidateDelegationAuthority(ctx context.Context, id string, allowedFiles []string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("implement delegation requires a task id")
	}
	if len(allowedFiles) == 0 {
		return errors.New("implement delegation requires at least one allowed file")
	}

	item, err := s.GetWork(ctx, id)
	if err != nil {
		return err
	}
	if item.Status != WorkInProgress || item.Claim == nil {
		return fmt.Errorf("%w: task %s has no active implementation claim", ErrWorkConflict, id)
	}
	now := s.now().UTC()
	claimExpiry, err := time.Parse(time.RFC3339Nano, item.Claim.ExpiresAt)
	if err != nil || !claimExpiry.After(now) {
		return fmt.Errorf("%w: task %s claim is expired", ErrWorkConflict, id)
	}

	activeLeases := make(map[string]struct{}, len(item.Leases))
	for _, lease := range item.Leases {
		expires, parseErr := time.Parse(time.RFC3339Nano, lease.ExpiresAt)
		if parseErr == nil && expires.After(now) {
			activeLeases[filepath.ToSlash(lease.Path)] = struct{}{}
		}
	}
	seen := make(map[string]struct{}, len(allowedFiles))
	for _, allowed := range allowedFiles {
		leasePath, pathErr := canonicalLeasePath(allowed)
		if pathErr != nil {
			return pathErr
		}
		if _, duplicate := seen[leasePath]; duplicate {
			return fmt.Errorf("duplicate allowed file %q", leasePath)
		}
		seen[leasePath] = struct{}{}
		if _, leased := activeLeases[leasePath]; !leased {
			return fmt.Errorf("%w: allowed file %s has no active lease for task %s", ErrWorkConflict, leasePath, id)
		}
	}
	return nil
}

func (s *Store) ClaimWork(ctx context.Context, id, owner string, ttl time.Duration) (WorkClaim, error) {
	owner = strings.TrimSpace(owner)
	if owner == "" || ttl <= 0 {
		return WorkClaim{}, errors.New("owner and positive ttl are required")
	}
	plain, err := token()
	if err != nil {
		return WorkClaim{}, err
	}
	now, expires := s.timestamp(), s.now().UTC().Add(ttl).Format(time.RFC3339Nano)
	claim := WorkClaim{Owner: owner, ExpiresAt: expires, Token: plain}
	err = s.immediate(ctx, func(conn *sql.Conn) error {
		var status WorkStatus
		var revision int64
		if err := conn.QueryRowContext(ctx, `SELECT status,revision FROM work_items WHERE id=?`, id).Scan(&status, &revision); errors.Is(err, sql.ErrNoRows) {
			return ErrWorkNotFound
		} else if err != nil {
			return err
		}
		if status != WorkReady {
			return fmt.Errorf("%w: task %s is %s, not ready", ErrWorkConflict, id, status)
		}
		var unresolved int
		if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM work_dependencies d JOIN work_items dep ON dep.id=d.depends_on WHERE d.item_id=? AND dep.status<>'done'`, id).Scan(&unresolved); err != nil {
			return err
		}
		if unresolved != 0 {
			return fmt.Errorf("%w: task has %d unresolved dependencies", ErrWorkConflict, unresolved)
		}
		var decomposition int
		if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM work_decomposition_steps WHERE parent_id=?`, id).Scan(&decomposition); err != nil {
			return err
		}
		if decomposition != 0 {
			return fmt.Errorf("%w: task %s was superseded by atomic decomposition", ErrWorkConflict, id)
		}
		attempts, err := workAttemptCount(ctx, conn, id)
		if err != nil {
			return err
		}
		if attempts >= MaxWorkAttempts {
			return workAttemptLimitError(id, attempts)
		}
		attempt := attempts + 1
		_, _ = conn.ExecContext(ctx, `DELETE FROM work_leases WHERE item_id=?`, id)
		_, _ = conn.ExecContext(ctx, `DELETE FROM work_claims WHERE item_id=?`, id)
		if _, err := conn.ExecContext(ctx, `INSERT INTO work_claims(item_id,owner,token_hash,attempt,expires_at,created_at,updated_at) VALUES(?,?,?,?,?,?,?)`, id, owner, tokenHash(plain), attempt, expires, now, now); err != nil {
			return fmt.Errorf("%w: task already claimed", ErrWorkConflict)
		}
		result, err := conn.ExecContext(ctx, `UPDATE work_items SET status='in_progress',revision=revision+1,updated_at=? WHERE id=? AND revision=? AND status='ready'`, now, id, revision)
		if err != nil {
			return err
		}
		changed, _ := result.RowsAffected()
		if changed != 1 {
			return fmt.Errorf("%w: stale task revision", ErrWorkConflict)
		}
		claim.Attempt = attempt
		claim.Revision = revision + 1
		return s.addWorkEvent(ctx, conn, id, "claimed", string(WorkReady), string(WorkInProgress), owner)
	})
	return claim, err
}

func (s *Store) RenewWorkClaim(ctx context.Context, id, claimToken string, ttl time.Duration) (WorkClaim, error) {
	if claimToken == "" || ttl <= 0 {
		return WorkClaim{}, errors.New("claim token and positive ttl are required")
	}
	now, expires := s.timestamp(), s.now().UTC().Add(ttl).Format(time.RFC3339Nano)
	result, err := s.db.ExecContext(ctx, `UPDATE work_claims SET expires_at=?,updated_at=? WHERE item_id=? AND token_hash=? AND expires_at>?`, expires, now, id, tokenHash(claimToken), now)
	if err != nil {
		return WorkClaim{}, err
	}
	changed, _ := result.RowsAffected()
	if changed != 1 {
		return WorkClaim{}, fmt.Errorf("%w: stale or expired claim", ErrWorkConflict)
	}
	var claim WorkClaim
	if err := s.db.QueryRowContext(ctx, `SELECT owner,attempt,expires_at FROM work_claims WHERE item_id=?`, id).Scan(&claim.Owner, &claim.Attempt, &claim.ExpiresAt); err != nil {
		return WorkClaim{}, err
	}
	var rev int64
	_ = s.db.QueryRowContext(ctx, `SELECT revision FROM work_items WHERE id=?`, id).Scan(&rev)
	claim.Revision = rev
	claim.Token = claimToken
	return claim, nil
}

func canonicalLeasePath(value string) (string, error) {
	value = filepath.ToSlash(filepath.Clean(strings.TrimSpace(value)))
	if value == "" || value == "." || value == ".." || strings.HasPrefix(value, "../") || filepath.IsAbs(value) || strings.Contains(value, ":") {
		return "", errors.New("lease path must be a safe workspace-relative path")
	}
	if len(value) > maxLeasePathBytes {
		return "", errors.New("lease path exceeds size limit")
	}
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		value = strings.ToLower(value)
	}
	return value, nil
}

func (s *Store) ReserveWorkLease(ctx context.Context, id, claimToken, path string, ttl time.Duration) (WorkLease, error) {
	leasePath, err := canonicalLeasePath(path)
	if err != nil {
		return WorkLease{}, err
	}
	if claimToken == "" || ttl <= 0 {
		return WorkLease{}, errors.New("claim token and positive ttl are required")
	}
	plain, err := token()
	if err != nil {
		return WorkLease{}, err
	}
	now, expires := s.timestamp(), s.now().UTC().Add(ttl).Format(time.RFC3339Nano)
	lease := WorkLease{Path: leasePath, ItemID: id, ExpiresAt: expires, Token: plain}
	err = s.immediate(ctx, func(conn *sql.Conn) error {
		var status WorkStatus
		if err := conn.QueryRowContext(ctx, `SELECT status FROM work_items WHERE id=?`, id).Scan(&status); errors.Is(err, sql.ErrNoRows) {
			return ErrWorkNotFound
		} else if err != nil {
			return err
		}
		if status != WorkInProgress {
			return fmt.Errorf("%w: task must be in_progress to lease files", ErrWorkConflict)
		}
		var claimCount int
		if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM work_claims WHERE item_id=? AND token_hash=? AND expires_at>?`, id, tokenHash(claimToken), now).Scan(&claimCount); err != nil {
			return err
		}
		if claimCount != 1 {
			return fmt.Errorf("%w: active claim required to lease files", ErrWorkConflict)
		}
		var activeLease int
		if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM work_leases WHERE path=? AND expires_at>?`, leasePath, now).Scan(&activeLease); err != nil {
			return err
		}
		if activeLease != 0 {
			return fmt.Errorf("%w: file %s is already leased", ErrWorkConflict, leasePath)
		}
		if _, err := conn.ExecContext(ctx, `DELETE FROM work_leases WHERE path=?`, leasePath); err != nil {
			return err
		}
		if _, err := conn.ExecContext(ctx, `INSERT INTO work_leases(path,item_id,token_hash,expires_at,created_at,updated_at) VALUES(?,?,?,?,?,?)`, leasePath, id, tokenHash(plain), expires, now, now); err != nil {
			return fmt.Errorf("create lease: %w", err)
		}
		return s.addWorkEvent(ctx, conn, id, "lease_acquired", "", "", leasePath)
	})
	return lease, err
}

func (s *Store) RenewWorkLease(ctx context.Context, path, leaseToken string, ttl time.Duration) (WorkLease, error) {
	leasePath, err := canonicalLeasePath(path)
	if err != nil {
		return WorkLease{}, err
	}
	if leaseToken == "" || ttl <= 0 {
		return WorkLease{}, errors.New("lease token and positive ttl are required")
	}
	now, expires := s.timestamp(), s.now().UTC().Add(ttl).Format(time.RFC3339Nano)
	var lease WorkLease
	err = s.immediate(ctx, func(conn *sql.Conn) error {
		result, err := conn.ExecContext(ctx, `UPDATE work_leases SET expires_at=?,updated_at=? WHERE path=? AND token_hash=? AND expires_at>?`, expires, now, leasePath, tokenHash(leaseToken), now)
		if err != nil {
			return err
		}
		changed, _ := result.RowsAffected()
		if changed != 1 {
			return fmt.Errorf("%w: stale or expired lease", ErrWorkConflict)
		}
		if err := conn.QueryRowContext(ctx, `SELECT path,item_id,expires_at FROM work_leases WHERE path=?`, leasePath).Scan(&lease.Path, &lease.ItemID, &lease.ExpiresAt); err != nil {
			return err
		}
		lease.Token = leaseToken
		return s.addWorkEvent(ctx, conn, lease.ItemID, "lease_renewed", "", "", leasePath)
	})
	return lease, err
}

func (s *Store) ReleaseWorkLease(ctx context.Context, path, leaseToken string) error {
	leasePath, err := canonicalLeasePath(path)
	if err != nil {
		return err
	}
	return s.immediate(ctx, func(conn *sql.Conn) error {
		var id string
		if err := conn.QueryRowContext(ctx, `SELECT item_id FROM work_leases WHERE path=? AND token_hash=?`, leasePath, tokenHash(leaseToken)).Scan(&id); errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("%w: lease token mismatch", ErrWorkConflict)
		} else if err != nil {
			return err
		}
		if _, err := conn.ExecContext(ctx, `DELETE FROM work_leases WHERE path=? AND token_hash=?`, leasePath, tokenHash(leaseToken)); err != nil {
			return err
		}
		return s.addWorkEvent(ctx, conn, id, "lease_released", "", "", leasePath)
	})
}

func validWorkTransition(from, to WorkStatus) bool {
	switch from {
	case WorkInProgress:
		return to == WorkInReview || to == WorkBlocked
	case WorkInReview:
		return to == WorkInProgress || to == WorkBlocked
	default:
		return false
	}
}

func (s *Store) TransitionWork(ctx context.Context, id, claimToken string, expectedRevision int64, to WorkStatus) (WorkItem, error) {
	if claimToken == "" {
		return WorkItem{}, errors.New("claim token is required")
	}
	now := s.timestamp()
	err := s.immediate(ctx, func(conn *sql.Conn) error {
		var from WorkStatus
		var revision int64
		if err := conn.QueryRowContext(ctx, `SELECT status,revision FROM work_items WHERE id=?`, id).Scan(&from, &revision); errors.Is(err, sql.ErrNoRows) {
			return ErrWorkNotFound
		} else if err != nil {
			return err
		}
		if !validWorkTransition(from, to) {
			return fmt.Errorf("%w: invalid transition %s@%d -> %s", ErrWorkConflict, from, revision, to)
		}
		if expectedRevision > 0 && revision != expectedRevision && revision != expectedRevision+1 {
			return fmt.Errorf("%w: invalid or stale transition %s@%d -> %s (expected rev %d)", ErrWorkConflict, from, revision, to, expectedRevision)
		}
		var count int
		if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM work_claims WHERE item_id=? AND token_hash=? AND expires_at>?`, id, tokenHash(claimToken), now).Scan(&count); err != nil {
			return err
		}
		if count != 1 {
			return fmt.Errorf("%w: stale or expired claim", ErrWorkConflict)
		}
		if to == WorkBlocked {
			_, _ = conn.ExecContext(ctx, `DELETE FROM work_leases WHERE item_id=?`, id)
			_, _ = conn.ExecContext(ctx, `DELETE FROM work_claims WHERE item_id=?`, id)
		}
		if _, err := conn.ExecContext(ctx, `UPDATE work_items SET status=?,revision=revision+1,updated_at=? WHERE id=? AND revision=?`, to, now, id, revision); err != nil {
			return err
		}
		return s.addWorkEvent(ctx, conn, id, "transition", string(from), string(to), strconv.FormatInt(revision, 10))
	})
	if err != nil {
		return WorkItem{}, err
	}
	return s.GetWork(ctx, id)
}

func (s *Store) ApproveWork(ctx context.Context, id, reviewer, verdict, evidence string, expectedRevision int64) (WorkApproval, error) {
	reviewer, verdict, evidence = strings.TrimSpace(reviewer), strings.ToUpper(strings.TrimSpace(verdict)), strings.TrimSpace(evidence)
	if reviewer == "" || (verdict != "PASS" && verdict != "FAIL" && verdict != "BLOCKED" && verdict != "INCONCLUSIVE") {
		return WorkApproval{}, errors.New("reviewer and valid verdict are required")
	}
	now := s.timestamp()
	approval := WorkApproval{ItemID: id, Revision: expectedRevision, Reviewer: reviewer, Verdict: verdict, Evidence: evidence, CreatedAt: now}
	err := s.immediate(ctx, func(conn *sql.Conn) error {
		var status WorkStatus
		var revision int64
		if err := conn.QueryRowContext(ctx, `SELECT status,revision FROM work_items WHERE id=?`, id).Scan(&status, &revision); errors.Is(err, sql.ErrNoRows) {
			return ErrWorkNotFound
		} else if err != nil {
			return err
		}
		if status != WorkInReview {
			return fmt.Errorf("%w: task must be in_review, currently %s", ErrWorkConflict, status)
		}
		if expectedRevision > 0 && revision != expectedRevision {
			return fmt.Errorf("%w: task is in_review at revision %d, not %d", ErrWorkConflict, revision, expectedRevision)
		}
		approval.Revision = revision
		var claimOwner string
		claimErr := conn.QueryRowContext(ctx, `SELECT owner FROM work_claims WHERE item_id=?`, id).Scan(&claimOwner)
		if claimErr != nil && !errors.Is(claimErr, sql.ErrNoRows) {
			return claimErr
		}
		if claimOwner != "" && claimOwner == reviewer {
			return fmt.Errorf("%w: implement owner cannot approve its own task", ErrWorkConflict)
		}
		if verdict == "PASS" && evidence == "" {
			return errors.New("PASS approval requires evidence")
		}
		if _, err := conn.ExecContext(ctx, `INSERT INTO work_approvals(item_id,reviewer,verdict,evidence,created_at) VALUES(?,?,?,?,?)`, id, reviewer, verdict, bounded(evidence, 512), now); err != nil {
			return err
		}
		to := WorkBlocked
		if verdict == "PASS" {
			to = WorkDone
		}
		result, err := conn.ExecContext(ctx, `UPDATE work_items SET status=?,revision=revision+1,updated_at=? WHERE id=? AND status='in_review' AND revision=?`, to, now, id, revision)
		if err != nil {
			return err
		}
		changed, _ := result.RowsAffected()
		if changed != 1 {
			return fmt.Errorf("%w: stale approval revision", ErrWorkConflict)
		}
		_, _ = conn.ExecContext(ctx, `DELETE FROM work_leases WHERE item_id=?`, id)
		_, _ = conn.ExecContext(ctx, `DELETE FROM work_claims WHERE item_id=?`, id)
		if err := s.addWorkEvent(ctx, conn, id, "approval", string(WorkInReview), string(to), verdict+":"+bounded(evidence, 128)); err != nil {
			return err
		}
		if to == WorkDone {
			return s.unlockDependents(ctx, conn, id, now)
		}
		return nil
	})
	return approval, err
}

func (s *Store) RecoverWork(ctx context.Context) (int64, error) {
	now := s.timestamp()
	var recovered int64
	err := s.immediate(ctx, func(conn *sql.Conn) error {
		rows, err := conn.QueryContext(ctx, `SELECT item_id FROM work_claims WHERE expires_at<=? ORDER BY item_id`, now)
		if err != nil {
			return err
		}
		var ids []string
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				_ = rows.Close()
				return err
			}
			ids = append(ids, id)
		}
		if err := rows.Close(); err != nil {
			return err
		}
		for _, id := range ids {
			result, err := conn.ExecContext(ctx, `UPDATE work_items SET status='blocked',revision=revision+1,updated_at=? WHERE id=? AND status='in_progress'`, now, id)
			if err != nil {
				return err
			}
			changed, _ := result.RowsAffected()
			recovered += changed
			_, _ = conn.ExecContext(ctx, `DELETE FROM work_leases WHERE item_id=?`, id)
			_, _ = conn.ExecContext(ctx, `DELETE FROM work_claims WHERE item_id=?`, id)
			if changed == 1 {
				if err := s.addWorkEvent(ctx, conn, id, "claim_expired", string(WorkInProgress), string(WorkBlocked), ""); err != nil {
					return err
				}
			}
		}
		_, _ = conn.ExecContext(ctx, `DELETE FROM work_leases WHERE expires_at<=?`, now)
		return nil
	})
	return recovered, err
}

func (s *Store) RetryWork(ctx context.Context, id string, expectedRevision int64) (WorkItem, error) {
	now := s.timestamp()
	err := s.immediate(ctx, func(conn *sql.Conn) error {
		var status WorkStatus
		var revision int64
		if err := conn.QueryRowContext(ctx, `SELECT status,revision FROM work_items WHERE id=?`, id).Scan(&status, &revision); errors.Is(err, sql.ErrNoRows) {
			return ErrWorkNotFound
		} else if err != nil {
			return err
		}
		if status != WorkBlocked {
			return fmt.Errorf("%w: task is %s, not blocked", ErrWorkConflict, status)
		}
		if expectedRevision > 0 && revision != expectedRevision {
			return fmt.Errorf("%w: task is blocked at revision %d, not %d", ErrWorkConflict, revision, expectedRevision)
		}
		var unresolved int
		if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM work_dependencies d JOIN work_items dep ON dep.id=d.depends_on WHERE d.item_id=? AND dep.status<>'done'`, id).Scan(&unresolved); err != nil {
			return err
		}
		if unresolved != 0 {
			return fmt.Errorf("%w: task has %d unresolved dependencies", ErrWorkConflict, unresolved)
		}
		var decomposition int
		if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM work_decomposition_steps WHERE parent_id=?`, id).Scan(&decomposition); err != nil {
			return err
		}
		if decomposition != 0 {
			return fmt.Errorf("%w: task %s was superseded by atomic decomposition", ErrWorkConflict, id)
		}
		attempts, err := workAttemptCount(ctx, conn, id)
		if err != nil {
			return err
		}
		if attempts >= MaxWorkAttempts {
			return workAttemptLimitError(id, attempts)
		}
		_, _ = conn.ExecContext(ctx, `DELETE FROM work_leases WHERE item_id=?`, id)
		_, _ = conn.ExecContext(ctx, `DELETE FROM work_claims WHERE item_id=?`, id)
		result, err := conn.ExecContext(ctx, `UPDATE work_items SET status='ready',revision=revision+1,updated_at=? WHERE id=? AND status='blocked' AND revision=?`, now, id, revision)
		if err != nil {
			return err
		}
		changed, _ := result.RowsAffected()
		if changed != 1 {
			return fmt.Errorf("%w: stale retry revision", ErrWorkConflict)
		}
		return s.addWorkEvent(ctx, conn, id, "retried", string(WorkBlocked), string(WorkReady), strconv.FormatInt(revision, 10))
	})
	if err != nil {
		return WorkItem{}, err
	}
	return s.GetWork(ctx, id)
}

func workAttemptCount(ctx context.Context, conn *sql.Conn, id string) (int64, error) {
	var attempts int64
	if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM work_events WHERE item_id=? AND kind='claimed'`, id).Scan(&attempts); err != nil {
		return 0, err
	}
	return attempts, nil
}

func workAttemptLimitError(id string, attempts int64) error {
	return fmt.Errorf("%w: task %s used %d of %d attempts; create a replacement task or reconcile manually", ErrWorkAttemptLimit, id, attempts, MaxWorkAttempts)
}

func (s *Store) addWorkEvent(ctx context.Context, conn *sql.Conn, id, kind, from, to, detail string) error {
	_, err := conn.ExecContext(ctx, `INSERT INTO work_events(item_id,kind,from_status,to_status,detail,created_at) VALUES(?,?,?,?,?,?)`, id, kind, from, to, bounded(detail, 512), s.timestamp())
	return err
}

func (s *Store) unlockDependents(ctx context.Context, conn *sql.Conn, completedID, now string) error {
	rows, err := conn.QueryContext(ctx, `SELECT DISTINCT d.item_id FROM work_dependencies d WHERE d.depends_on=? AND NOT EXISTS (SELECT 1 FROM work_dependencies all_d JOIN work_items dep ON dep.id=all_d.depends_on WHERE all_d.item_id=d.item_id AND dep.status<>'done')`, completedID)
	if err != nil {
		return err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, id := range ids {
		result, err := conn.ExecContext(ctx, `UPDATE work_items SET status='ready',revision=revision+1,updated_at=? WHERE id=? AND status='backlog'`, now, id)
		if err != nil {
			return err
		}
		changed, _ := result.RowsAffected()
		if changed == 1 {
			if err := s.addWorkEvent(ctx, conn, id, "dependencies_resolved", string(WorkBacklog), string(WorkReady), completedID); err != nil {
				return err
			}
		}
	}
	return nil
}
