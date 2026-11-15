package delegation

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// TaskFact represents a verified environmental or codebase truth stored in the Task Ledger.
type TaskFact struct {
	ID        int64  `json:"id"`
	BoardID   string `json:"board_id"`
	Fact      string `json:"fact"`
	Source    string `json:"source"`
	CreatedAt string `json:"created_at"`
}

// ProgressEvaluation represents an orchestrator reflection cycle in the Progress Ledger.
type ProgressEvaluation struct {
	ID            int64  `json:"id"`
	BoardID       string `json:"board_id"`
	Cycle         int    `json:"cycle"`
	Summary       string `json:"summary"`
	DriftDetected bool   `json:"drift_detected"`
	Action        string `json:"action"`
	CreatedAt     string `json:"created_at"`
}

// LedgerReport bundles facts and progress evaluations for a given board.
type LedgerReport struct {
	BoardID  string               `json:"board_id"`
	Facts    []TaskFact           `json:"facts"`
	Progress []ProgressEvaluation `json:"progress"`
}

// AddFact appends an authoritative environmental or technical fact to the Task Ledger.
func (s *Store) AddFact(ctx context.Context, boardID, fact, source string) (TaskFact, error) {
	boardID = strings.TrimSpace(boardID)
	if boardID == "" {
		boardID = DefaultBoardID
	}
	fact = strings.TrimSpace(fact)
	if fact == "" {
		return TaskFact{}, errors.New("fact description is required")
	}
	source = strings.TrimSpace(source)
	if source == "" {
		source = "orchestrator"
	}
	if len(fact) > 4096 {
		return TaskFact{}, errors.New("fact text exceeds maximum allowed size (4096 bytes)")
	}

	var created TaskFact
	now := s.timestamp()

	err := s.immediate(ctx, func(conn *sql.Conn) error {
		res, err := conn.ExecContext(ctx, `INSERT INTO task_facts(board_id, fact, source, created_at)
			VALUES(?, ?, ?, ?)`, boardID, fact, source, now)
		if err != nil {
			return fmt.Errorf("insert task fact: %w", err)
		}
		id, err := res.LastInsertId()
		if err != nil {
			return fmt.Errorf("get fact last insert id: %w", err)
		}
		created = TaskFact{
			ID:        id,
			BoardID:   boardID,
			Fact:      fact,
			Source:    source,
			CreatedAt: now,
		}
		return nil
	})
	if err != nil {
		return TaskFact{}, err
	}
	return created, nil
}

// ListFacts retrieves all authoritative facts recorded for a board in chronological order.
func (s *Store) ListFacts(ctx context.Context, boardID string) ([]TaskFact, error) {
	boardID = strings.TrimSpace(boardID)
	if boardID == "" {
		boardID = DefaultBoardID
	}

	rows, err := s.db.QueryContext(ctx, `SELECT id, board_id, fact, source, created_at
		FROM task_facts
		WHERE board_id = ?
		ORDER BY id ASC`, boardID)
	if err != nil {
		return nil, fmt.Errorf("query task facts: %w", err)
	}
	defer func() { _ = rows.Close() }()

	facts := make([]TaskFact, 0)
	for rows.Next() {
		var f TaskFact
		if err := rows.Scan(&f.ID, &f.BoardID, &f.Fact, &f.Source, &f.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan task fact: %w", err)
		}
		facts = append(facts, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate task facts: %w", err)
	}
	return facts, nil
}

// RecordProgress appends a cycle evaluation to the Progress Ledger, recording drift detection and intended next action.
func (s *Store) RecordProgress(ctx context.Context, boardID string, cycle int, summary string, drift bool, action string) (ProgressEvaluation, error) {
	boardID = strings.TrimSpace(boardID)
	if boardID == "" {
		boardID = DefaultBoardID
	}
	summary = strings.TrimSpace(summary)
	if summary == "" {
		return ProgressEvaluation{}, errors.New("progress summary is required")
	}
	action = strings.TrimSpace(action)
	if action == "" {
		action = "continue"
	}
	if len(summary) > 4096 {
		return ProgressEvaluation{}, errors.New("progress summary exceeds maximum allowed size (4096 bytes)")
	}

	var created ProgressEvaluation
	now := s.timestamp()

	err := s.immediate(ctx, func(conn *sql.Conn) error {
		if cycle <= 0 {
			var maxCycle int
			_ = conn.QueryRowContext(ctx, `SELECT COALESCE(MAX(cycle), 0) FROM progress_evaluations WHERE board_id = ?`, boardID).Scan(&maxCycle)
			cycle = maxCycle + 1
		}

		driftInt := 0
		if drift {
			driftInt = 1
		}

		res, err := conn.ExecContext(ctx, `INSERT INTO progress_evaluations(board_id, cycle, summary, drift_detected, action, created_at)
			VALUES(?, ?, ?, ?, ?, ?)`, boardID, cycle, summary, driftInt, action, now)
		if err != nil {
			return fmt.Errorf("insert progress evaluation: %w", err)
		}
		id, err := res.LastInsertId()
		if err != nil {
			return fmt.Errorf("get progress last insert id: %w", err)
		}
		created = ProgressEvaluation{
			ID:            id,
			BoardID:       boardID,
			Cycle:         cycle,
			Summary:       summary,
			DriftDetected: drift,
			Action:        action,
			CreatedAt:     now,
		}
		return nil
	})
	if err != nil {
		return ProgressEvaluation{}, err
	}
	return created, nil
}

// ListProgress retrieves all progress evaluations recorded for a board in chronological cycle order.
func (s *Store) ListProgress(ctx context.Context, boardID string) ([]ProgressEvaluation, error) {
	boardID = strings.TrimSpace(boardID)
	if boardID == "" {
		boardID = DefaultBoardID
	}

	rows, err := s.db.QueryContext(ctx, `SELECT id, board_id, cycle, summary, drift_detected, action, created_at
		FROM progress_evaluations
		WHERE board_id = ?
		ORDER BY cycle ASC`, boardID)
	if err != nil {
		return nil, fmt.Errorf("query progress evaluations: %w", err)
	}
	defer func() { _ = rows.Close() }()

	evals := make([]ProgressEvaluation, 0)
	for rows.Next() {
		var p ProgressEvaluation
		var driftInt int
		if err := rows.Scan(&p.ID, &p.BoardID, &p.Cycle, &p.Summary, &driftInt, &p.Action, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan progress evaluation: %w", err)
		}
		p.DriftDetected = driftInt == 1
		evals = append(evals, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate progress evaluations: %w", err)
	}
	return evals, nil
}

// GetLedger retrieves both the Task Ledger (facts) and Progress Ledger (evaluations) for a board.
func (s *Store) GetLedger(ctx context.Context, boardID string) (LedgerReport, error) {
	boardID = strings.TrimSpace(boardID)
	if boardID == "" {
		boardID = DefaultBoardID
	}

	facts, err := s.ListFacts(ctx, boardID)
	if err != nil {
		return LedgerReport{}, err
	}
	progress, err := s.ListProgress(ctx, boardID)
	if err != nil {
		return LedgerReport{}, err
	}
	return LedgerReport{
		BoardID:  boardID,
		Facts:    facts,
		Progress: progress,
	}, nil
}
