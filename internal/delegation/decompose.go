package delegation

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const MaxDecompositionSteps = 8

type WorkStepDefinition struct {
	ID           string   `json:"task_id"`
	Title        string   `json:"title"`
	Objective    string   `json:"objective"`
	Acceptance   string   `json:"acceptance_criteria"`
	Verification string   `json:"verification"`
	AllowedFiles []string `json:"allowed_files,omitempty"`
}

type WorkDecomposition struct {
	Parent   WorkItem   `json:"parent"`
	Children []WorkItem `json:"children"`
}

type normalizedWorkStep struct {
	WorkStepDefinition
	allowedFilesJSON string
}

// DecomposeWork atomically replaces one blocked task with a sequential chain
// of smaller tasks. The original remains in the append-only history and is
// presented as superseded; downstream dependencies move to the final child.
func (s *Store) DecomposeWork(ctx context.Context, id string, expectedRevision int64, steps []WorkStepDefinition) (WorkDecomposition, error) {
	id = strings.TrimSpace(id)
	if id == "" || expectedRevision <= 0 {
		return WorkDecomposition{}, errors.New("task id and positive revision are required")
	}
	normalized, err := normalizeWorkSteps(id, steps)
	if err != nil {
		return WorkDecomposition{}, err
	}
	now := s.timestamp()
	err = s.immediate(ctx, func(conn *sql.Conn) error {
		var boardID, workspace string
		var status WorkStatus
		var revision int64
		if err := conn.QueryRowContext(ctx, `SELECT board_id,workspace,status,revision FROM work_items WHERE id=?`, id).Scan(&boardID, &workspace, &status, &revision); errors.Is(err, sql.ErrNoRows) {
			return ErrWorkNotFound
		} else if err != nil {
			return err
		}
		if status != WorkBlocked {
			return fmt.Errorf("%w: task %s is %s, not blocked", ErrWorkConflict, id, status)
		}
		if revision != expectedRevision {
			return fmt.Errorf("%w: task is blocked at revision %d, not %d", ErrWorkConflict, revision, expectedRevision)
		}
		var prior int
		if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM work_decomposition_steps WHERE parent_id=?`, id).Scan(&prior); err != nil {
			return err
		}
		if prior != 0 {
			return fmt.Errorf("%w: task %s is already decomposed", ErrWorkConflict, id)
		}

		dependencies, err := workDependencyIDs(ctx, conn, id)
		if err != nil {
			return err
		}
		var unresolved int
		if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM work_dependencies d JOIN work_items dep ON dep.id=d.depends_on WHERE d.item_id=? AND dep.status<>'done'`, id).Scan(&unresolved); err != nil {
			return err
		}

		for index, step := range normalized {
			childStatus := WorkBacklog
			if index == 0 && unresolved == 0 {
				childStatus = WorkReady
			}
			if _, err := conn.ExecContext(ctx, `INSERT INTO work_items(id,title,status,created_at,updated_at,board_id,workspace) VALUES(?,?,?,?,?,?,?)`, step.ID, step.Title, childStatus, now, now, boardID, workspace); err != nil {
				return fmt.Errorf("create decomposition task %q: %w", step.ID, err)
			}
			if _, err := conn.ExecContext(ctx, `INSERT INTO work_definitions(item_id,objective,acceptance_criteria,verification,allowed_files_json) VALUES(?,?,?,?,?)`, step.ID, step.Objective, step.Acceptance, step.Verification, step.allowedFilesJSON); err != nil {
				return fmt.Errorf("create decomposition definition %q: %w", step.ID, err)
			}
			childDependencies := dependencies
			if index > 0 {
				childDependencies = []string{normalized[index-1].ID}
			}
			for _, dependency := range childDependencies {
				if _, err := conn.ExecContext(ctx, `INSERT INTO work_dependencies(item_id,depends_on) VALUES(?,?)`, step.ID, dependency); err != nil {
					return fmt.Errorf("link decomposition dependency %q -> %q: %w", step.ID, dependency, err)
				}
			}
			if _, err := conn.ExecContext(ctx, `INSERT INTO work_decomposition_steps(parent_id,child_id,position) VALUES(?,?,?)`, id, step.ID, index+1); err != nil {
				return fmt.Errorf("link decomposition task %q: %w", step.ID, err)
			}
			if err := s.addWorkEvent(ctx, conn, step.ID, "created_by_decomposition", "", string(childStatus), id); err != nil {
				return err
			}
		}

		lastChild := normalized[len(normalized)-1].ID
		if _, err := conn.ExecContext(ctx, `INSERT OR IGNORE INTO work_dependencies(item_id,depends_on)
			SELECT item_id,? FROM work_dependencies WHERE depends_on=?`, lastChild, id); err != nil {
			return fmt.Errorf("redirect downstream dependencies: %w", err)
		}
		if _, err := conn.ExecContext(ctx, `DELETE FROM work_dependencies WHERE depends_on=?`, id); err != nil {
			return fmt.Errorf("remove superseded dependencies: %w", err)
		}
		_, _ = conn.ExecContext(ctx, `DELETE FROM work_leases WHERE item_id=?`, id)
		_, _ = conn.ExecContext(ctx, `DELETE FROM work_claims WHERE item_id=?`, id)
		result, err := conn.ExecContext(ctx, `UPDATE work_items SET revision=revision+1,updated_at=? WHERE id=? AND status='blocked' AND revision=?`, now, id, revision)
		if err != nil {
			return err
		}
		changed, _ := result.RowsAffected()
		if changed != 1 {
			return fmt.Errorf("%w: stale decomposition revision", ErrWorkConflict)
		}
		childIDs := make([]string, 0, len(normalized))
		for _, step := range normalized {
			childIDs = append(childIDs, step.ID)
		}
		detail, _ := json.Marshal(childIDs)
		return s.addWorkEvent(ctx, conn, id, "decomposed", string(WorkBlocked), string(WorkSuperseded), string(detail))
	})
	if err != nil {
		return WorkDecomposition{}, err
	}
	parent, err := s.GetWork(ctx, id)
	if err != nil {
		return WorkDecomposition{}, err
	}
	result := WorkDecomposition{Parent: parent, Children: make([]WorkItem, 0, len(normalized))}
	for _, step := range normalized {
		child, err := s.GetWork(ctx, step.ID)
		if err != nil {
			return WorkDecomposition{}, err
		}
		result.Children = append(result.Children, child)
	}
	return result, nil
}

func normalizeWorkSteps(parentID string, steps []WorkStepDefinition) ([]normalizedWorkStep, error) {
	if len(steps) < 2 || len(steps) > MaxDecompositionSteps {
		return nil, fmt.Errorf("decomposition requires 2-%d atomic tasks", MaxDecompositionSteps)
	}
	seen := make(map[string]struct{}, len(steps))
	result := make([]normalizedWorkStep, 0, len(steps))
	for _, input := range steps {
		input.ID = strings.TrimSpace(input.ID)
		input.Title = strings.TrimSpace(input.Title)
		input.Objective = strings.TrimSpace(input.Objective)
		input.Acceptance = strings.TrimSpace(input.Acceptance)
		input.Verification = strings.TrimSpace(input.Verification)
		if input.ID == "" || input.ID == parentID || input.Title == "" {
			return nil, fmt.Errorf("each decomposition task requires a unique id and title")
		}
		if _, duplicate := seen[input.ID]; duplicate {
			return nil, fmt.Errorf("duplicate decomposition task id %q", input.ID)
		}
		seen[input.ID] = struct{}{}
		if len(input.ID) > 128 || len(input.Title) > 512 {
			return nil, fmt.Errorf("decomposition task %q id or title exceeds size limit", input.ID)
		}
		if input.Objective == "" || input.Acceptance == "" || input.Verification == "" {
			return nil, fmt.Errorf("decomposition task %q requires objective, acceptance criteria, and verification", input.ID)
		}
		if len(input.Objective) > 4096 || len(input.Acceptance) > 4096 || len(input.Verification) > 2048 {
			return nil, fmt.Errorf("decomposition task %q definition exceeds size limit", input.ID)
		}
		if len(input.AllowedFiles) > 128 {
			return nil, fmt.Errorf("decomposition task %q allowed file count exceeds limit", input.ID)
		}
		files := make([]string, 0, len(input.AllowedFiles))
		seenFiles := make(map[string]struct{}, len(input.AllowedFiles))
		for _, value := range input.AllowedFiles {
			file, err := canonicalLeasePath(value)
			if err != nil {
				return nil, fmt.Errorf("decomposition task %q invalid allowed file %q: %w", input.ID, value, err)
			}
			if _, duplicate := seenFiles[file]; duplicate {
				continue
			}
			seenFiles[file] = struct{}{}
			files = append(files, file)
		}
		input.AllowedFiles = files
		encoded, err := json.Marshal(files)
		if err != nil {
			return nil, err
		}
		result = append(result, normalizedWorkStep{WorkStepDefinition: input, allowedFilesJSON: string(encoded)})
	}
	return result, nil
}

func workDependencyIDs(ctx context.Context, conn *sql.Conn, id string) ([]string, error) {
	rows, err := conn.QueryContext(ctx, `SELECT depends_on FROM work_dependencies WHERE item_id=? ORDER BY depends_on`, id)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var dependencies []string
	for rows.Next() {
		var dependency string
		if err := rows.Scan(&dependency); err != nil {
			return nil, err
		}
		dependencies = append(dependencies, dependency)
	}
	return dependencies, rows.Err()
}
