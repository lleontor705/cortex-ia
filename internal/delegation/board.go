package delegation

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

const DefaultBoardID = "default"

var ErrBoardNotFound = errors.New("task board not found")

type WorkBoard struct {
	ID          string               `json:"board_id"`
	Title       string               `json:"title"`
	Description string               `json:"description,omitempty"`
	Status      string               `json:"status"`
	Revision    int64                `json:"revision"`
	Counts      map[WorkStatus]int64 `json:"counts"`
	CreatedAt   string               `json:"created_at"`
	UpdatedAt   string               `json:"updated_at"`
}

type BoardSnapshot struct {
	Board WorkBoard  `json:"board"`
	Items []WorkItem `json:"items"`
}

func (s *Store) CreateBoard(ctx context.Context, id, title, description string) (WorkBoard, error) {
	id, title, description = strings.TrimSpace(id), strings.TrimSpace(title), strings.TrimSpace(description)
	if id == "" || title == "" {
		return WorkBoard{}, errors.New("board id and title are required")
	}
	if len(id) > 128 || len(title) > 256 || len(description) > 2048 {
		return WorkBoard{}, errors.New("board id, title, or description exceeds size limit")
	}
	if strings.ContainsAny(id, "\\/\r\n\t") {
		return WorkBoard{}, errors.New("board id must not contain path separators or control whitespace")
	}
	now := s.timestamp()
	if _, err := s.db.ExecContext(ctx, `INSERT INTO work_boards(id,title,description,status,created_at,updated_at) VALUES(?,?,?,'active',?,?)`, id, title, description, now, now); err != nil {
		return WorkBoard{}, fmt.Errorf("create task board: %w", err)
	}
	return s.GetBoard(ctx, id)
}

func (s *Store) ListBoards(ctx context.Context) ([]WorkBoard, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM work_boards ORDER BY CASE status WHEN 'active' THEN 0 ELSE 1 END, CASE id WHEN 'default' THEN 0 ELSE 1 END, updated_at DESC, id`)
	if err != nil {
		return nil, err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
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
	boards := make([]WorkBoard, 0, len(ids))
	for _, id := range ids {
		board, err := s.GetBoard(ctx, id)
		if err != nil {
			return nil, err
		}
		boards = append(boards, board)
	}
	return boards, nil
}

func (s *Store) GetBoard(ctx context.Context, id string) (WorkBoard, error) {
	var board WorkBoard
	err := s.db.QueryRowContext(ctx, `SELECT id,title,description,COALESCE(status, 'active'),revision,created_at,updated_at FROM work_boards WHERE id=?`, strings.TrimSpace(id)).Scan(
		&board.ID, &board.Title, &board.Description, &board.Status, &board.Revision, &board.CreatedAt, &board.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return WorkBoard{}, ErrBoardNotFound
	}
	if err != nil {
		return WorkBoard{}, err
	}
	board.Counts = map[WorkStatus]int64{}
	rows, err := s.db.QueryContext(ctx, `SELECT CASE WHEN EXISTS (
		SELECT 1 FROM work_decomposition_steps d WHERE d.parent_id=work_items.id
	) THEN 'superseded' ELSE status END AS effective_status,COUNT(*)
		FROM work_items WHERE board_id=? GROUP BY effective_status`, board.ID)
	if err != nil {
		return WorkBoard{}, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var status WorkStatus
		var count int64
		if err := rows.Scan(&status, &count); err != nil {
			return WorkBoard{}, err
		}
		board.Counts[status] = count
	}
	return board, rows.Err()
}

func (s *Store) ArchiveBoard(ctx context.Context, id string) (WorkBoard, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return WorkBoard{}, errors.New("board id is required")
	}
	if id == DefaultBoardID {
		return WorkBoard{}, errors.New("cannot archive the default task board")
	}
	now := s.timestamp()
	err := s.immediate(ctx, func(conn *sql.Conn) error {
		var status string
		var rev int64
		err := conn.QueryRowContext(ctx, `SELECT COALESCE(status, 'active'), revision FROM work_boards WHERE id=?`, id).Scan(&status, &rev)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrBoardNotFound
		}
		if err != nil {
			return err
		}
		if status == "archived" {
			return nil
		}
		res, err := conn.ExecContext(ctx, `UPDATE work_boards SET status='archived', revision=revision+1, updated_at=? WHERE id=?`, now, id)
		if err != nil {
			return fmt.Errorf("archive task board: %w", err)
		}
		affected, _ := res.RowsAffected()
		if affected == 0 {
			return ErrBoardNotFound
		}
		return nil
	})
	if err != nil {
		return WorkBoard{}, err
	}
	return s.GetBoard(ctx, id)
}

func (s *Store) UnarchiveBoard(ctx context.Context, id string) (WorkBoard, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return WorkBoard{}, errors.New("board id is required")
	}
	now := s.timestamp()
	err := s.immediate(ctx, func(conn *sql.Conn) error {
		var status string
		err := conn.QueryRowContext(ctx, `SELECT COALESCE(status, 'active') FROM work_boards WHERE id=?`, id).Scan(&status)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrBoardNotFound
		}
		if err != nil {
			return err
		}
		if status == "active" {
			return nil
		}
		res, err := conn.ExecContext(ctx, `UPDATE work_boards SET status='active', revision=revision+1, updated_at=? WHERE id=?`, now, id)
		if err != nil {
			return fmt.Errorf("unarchive task board: %w", err)
		}
		affected, _ := res.RowsAffected()
		if affected == 0 {
			return ErrBoardNotFound
		}
		return nil
	})
	if err != nil {
		return WorkBoard{}, err
	}
	return s.GetBoard(ctx, id)
}

func (s *Store) DeleteBoard(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("board id is required")
	}
	if id == DefaultBoardID {
		return errors.New("cannot delete the default task board")
	}
	return s.immediate(ctx, func(conn *sql.Conn) error {
		var status string
		err := conn.QueryRowContext(ctx, `SELECT COALESCE(status, 'active') FROM work_boards WHERE id=?`, id).Scan(&status)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrBoardNotFound
		}
		if err != nil {
			return err
		}
		if status != "archived" {
			return errors.New("only archived boards can be deleted; archive the board first")
		}
		// 1. Delete dependencies referencing items in this board
		if _, err := conn.ExecContext(ctx, `DELETE FROM work_dependencies WHERE item_id IN (SELECT id FROM work_items WHERE board_id=?) OR depends_on IN (SELECT id FROM work_items WHERE board_id=?)`, id, id); err != nil {
			return fmt.Errorf("delete board dependencies: %w", err)
		}
		// 2. Delete decomposition steps
		if _, err := conn.ExecContext(ctx, `DELETE FROM work_decomposition_steps WHERE parent_id IN (SELECT id FROM work_items WHERE board_id=?) OR child_id IN (SELECT id FROM work_items WHERE board_id=?)`, id, id); err != nil {
			return fmt.Errorf("delete board decomposition steps: %w", err)
		}
		// 3. Delete work items
		if _, err := conn.ExecContext(ctx, `DELETE FROM work_items WHERE board_id=?`, id); err != nil {
			return fmt.Errorf("delete board work items: %w", err)
		}
		// 4. Delete the board
		res, err := conn.ExecContext(ctx, `DELETE FROM work_boards WHERE id=?`, id)
		if err != nil {
			return fmt.Errorf("delete board: %w", err)
		}
		affected, _ := res.RowsAffected()
		if affected == 0 {
			return ErrBoardNotFound
		}
		return nil
	})
}

func (s *Store) BoardSnapshot(ctx context.Context, id string) (BoardSnapshot, error) {
	board, err := s.GetBoard(ctx, id)
	if err != nil {
		return BoardSnapshot{}, err
	}
	items, err := s.ListWorkByBoard(ctx, board.ID)
	if err != nil {
		return BoardSnapshot{}, err
	}
	return BoardSnapshot{Board: board, Items: items}, nil
}
