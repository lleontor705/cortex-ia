package delegation

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

type WorkRevisionPlan struct {
	Version          int                    `json:"version"`
	TaskID           string                 `json:"task_id"`
	BoardID          string                 `json:"board_id"`
	Project          string                 `json:"project"`
	ExpectedRevision int64                  `json:"expected_revision"`
	ExpectedStatus   WorkStatus             `json:"expected_status"`
	Definition       WorkRevisionDefinition `json:"definition"`
}

type WorkRevisionDefinition struct {
	Title        string   `json:"title"`
	Objective    string   `json:"objective"`
	Acceptance   string   `json:"acceptance_criteria"`
	Verification string   `json:"verification"`
	AllowedFiles []string `json:"allowed_files"`
}

func DecodeWorkRevisionPlan(r io.Reader) (WorkRevisionPlan, error) {
	var plan WorkRevisionPlan
	decoder := json.NewDecoder(io.LimitReader(r, 128*1024+1))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&plan); err != nil {
		return WorkRevisionPlan{}, err
	}
	if decoder.Decode(new(any)) != io.EOF {
		return WorkRevisionPlan{}, errors.New("trailing revise plan JSON")
	}
	return plan, nil
}

// ReviseWorkDefinition atomically replaces the bounded definition of an unclaimed
// ready/backlog task. Completed tasks are not revisable; a new task is the only path
// for a changed definition.
func (s *Store) ReviseWorkDefinition(ctx context.Context, plan WorkRevisionPlan) (WorkItem, error) {
	normalized, allowedJSON, workspace, err := normalizeWorkRevisionPlan(ctx, plan)
	if err != nil {
		return WorkItem{}, err
	}

	now := s.timestamp()
	err = s.immediate(ctx, func(conn *sql.Conn) error {
		var live struct {
			BoardID, Workspace, Title, Status string
			Revision                          int64
		}
		if err := conn.QueryRowContext(ctx, `SELECT board_id,workspace,title,status,revision FROM work_items WHERE id=?`, normalized.TaskID).Scan(&live.BoardID, &live.Workspace, &live.Title, &live.Status, &live.Revision); errors.Is(err, sql.ErrNoRows) {
			return ErrWorkNotFound
		} else if err != nil {
			return err
		}
		liveWorkspace, err := CanonicalWorkspace(live.Workspace)
		if err != nil {
			return err
		}
		if live.BoardID != normalized.BoardID || liveWorkspace != workspace {
			return fmt.Errorf("%w: task identity mismatch", ErrWorkConflict)
		}
		if live.Revision != normalized.ExpectedRevision {
			return fmt.Errorf("%w: task is at revision %d, not %d", ErrWorkConflict, live.Revision, normalized.ExpectedRevision)
		}
		liveStatus := WorkStatus(live.Status)
		if liveStatus != normalized.ExpectedStatus || (liveStatus != WorkReady && liveStatus != WorkBacklog) {
			return fmt.Errorf("%w: task %s is %s, not revisable", ErrWorkConflict, normalized.TaskID, live.Status)
		}

		var activeClaims, activeLeases, activeReviews, replacements int
		if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM work_claims WHERE item_id=? AND expires_at>?`, normalized.TaskID, now).Scan(&activeClaims); err != nil {
			return err
		}
		if activeClaims != 0 {
			return fmt.Errorf("%w: task has an active claim", ErrWorkConflict)
		}
		if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM work_leases WHERE item_id=? AND expires_at>?`, normalized.TaskID, now).Scan(&activeLeases); err != nil {
			return err
		}
		if activeLeases != 0 {
			return fmt.Errorf("%w: task has active file leases", ErrWorkConflict)
		}
		if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM work_reviews WHERE item_id=?`, normalized.TaskID).Scan(&activeReviews); err != nil {
			return err
		}
		if activeReviews != 0 {
			return fmt.Errorf("%w: task has an active review", ErrWorkConflict)
		}
		if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM work_decomposition_steps WHERE parent_id=?`, normalized.TaskID).Scan(&replacements); err != nil {
			return err
		}
		if replacements != 0 {
			return fmt.Errorf("%w: task was superseded by decomposition", ErrWorkConflict)
		}

		var objective, acceptance, verification, filesJSON string
		if err := conn.QueryRowContext(ctx, `SELECT objective,acceptance_criteria,verification,allowed_files_json FROM work_definitions WHERE item_id=?`, normalized.TaskID).Scan(&objective, &acceptance, &verification, &filesJSON); err != nil {
			return err
		}

		oldHash := hashJSON([]any{live.Title, objective, acceptance, verification, filesJSON})
		newHash := hashJSON([]any{normalized.Definition.Title, normalized.Definition.Objective, normalized.Definition.Acceptance, normalized.Definition.Verification, allowedJSON})
		if _, err := conn.ExecContext(ctx, `UPDATE work_definitions SET objective=?,acceptance_criteria=?,verification=?,allowed_files_json=? WHERE item_id=?`, normalized.Definition.Objective, normalized.Definition.Acceptance, normalized.Definition.Verification, allowedJSON, normalized.TaskID); err != nil {
			return err
		}
		result, err := conn.ExecContext(ctx, `UPDATE work_items SET title=?,revision=revision+1,updated_at=? WHERE id=? AND revision=? AND status=?`, normalized.Definition.Title, now, normalized.TaskID, normalized.ExpectedRevision, live.Status)
		if err != nil {
			return err
		}
		changed, _ := result.RowsAffected()
		if changed != 1 {
			return fmt.Errorf("%w: stale revision", ErrWorkConflict)
		}
		detail, _ := json.Marshal(map[string]string{"old": oldHash, "new": newHash})
		return s.addWorkEvent(ctx, conn, normalized.TaskID, "definition_revised", live.Status, live.Status, string(detail))
	})
	if err != nil {
		return WorkItem{}, err
	}
	return s.GetWork(ctx, normalized.TaskID)
}

func normalizeWorkRevisionPlan(ctx context.Context, plan WorkRevisionPlan) (WorkRevisionPlan, string, string, error) {
	_ = ctx
	plan.TaskID = strings.TrimSpace(plan.TaskID)
	plan.BoardID = strings.TrimSpace(plan.BoardID)
	plan.Project = strings.TrimSpace(plan.Project)
	plan.Definition.Title = strings.TrimSpace(plan.Definition.Title)
	plan.Definition.Objective = strings.TrimSpace(plan.Definition.Objective)
	plan.Definition.Acceptance = strings.TrimSpace(plan.Definition.Acceptance)
	plan.Definition.Verification = strings.TrimSpace(plan.Definition.Verification)
	if plan.Version != 1 || plan.TaskID == "" || plan.BoardID == "" || plan.Project == "" || plan.ExpectedRevision <= 0 {
		return WorkRevisionPlan{}, "", "", errors.New("revision plan requires version, identity, project and positive revision")
	}
	if plan.ExpectedStatus != WorkReady && plan.ExpectedStatus != WorkBacklog {
		return WorkRevisionPlan{}, "", "", errors.New("revision plan expected status must be ready or backlog")
	}
	if plan.Definition.Title == "" || len(plan.Definition.Title) > 512 || plan.Definition.Objective == "" || plan.Definition.Acceptance == "" || plan.Definition.Verification == "" {
		return WorkRevisionPlan{}, "", "", errors.New("revision definition requires bounded title, objective, acceptance and verification")
	}
	if len(plan.Definition.Objective) > 4096 || len(plan.Definition.Acceptance) > 4096 || len(plan.Definition.Verification) > 2048 {
		return WorkRevisionPlan{}, "", "", errors.New("revision definition exceeds size limit")
	}
	if len(plan.Definition.AllowedFiles) > 128 {
		return WorkRevisionPlan{}, "", "", errors.New("revision allowed file count exceeds limit")
	}
	workspace, err := ResolveProjectRoot(plan.Project)
	if err != nil {
		return WorkRevisionPlan{}, "", "", fmt.Errorf("resolve revision project: %w", err)
	}
	allowed := make([]string, 0, len(plan.Definition.AllowedFiles))
	seen := map[string]struct{}{}
	for _, value := range plan.Definition.AllowedFiles {
		file, err := canonicalLeasePath(value)
		if err != nil {
			return WorkRevisionPlan{}, "", "", fmt.Errorf("invalid allowed file %q: %w", value, err)
		}
		if _, duplicate := seen[file]; duplicate {
			continue
		}
		seen[file] = struct{}{}
		allowed = append(allowed, file)
	}
	plan.Definition.AllowedFiles = allowed
	allowedJSON, err := json.Marshal(allowed)
	if err != nil {
		return WorkRevisionPlan{}, "", "", err
	}
	return plan, string(allowedJSON), workspace, nil
}
