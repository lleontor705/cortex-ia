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
	if _, err := s.db.ExecContext(ctx, `INSERT INTO work_boards(id,title,description,created_at,updated_at) VALUES(?,?,?,?,?)`, id, title, description, now, now); err != nil {
		return WorkBoard{}, fmt.Errorf("create task board: %w", err)
	}
	return s.GetBoard(ctx, id)
}

func (s *Store) ListBoards(ctx context.Context) ([]WorkBoard, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM work_boards ORDER BY CASE id WHEN 'default' THEN 0 ELSE 1 END, updated_at DESC, id`)
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
	err := s.db.QueryRowContext(ctx, `SELECT id,title,description,revision,created_at,updated_at FROM work_boards WHERE id=?`, strings.TrimSpace(id)).Scan(
		&board.ID, &board.Title, &board.Description, &board.Revision, &board.CreatedAt, &board.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return WorkBoard{}, ErrBoardNotFound
	}
	if err != nil {
		return WorkBoard{}, err
	}
	board.Counts = map[WorkStatus]int64{}
	rows, err := s.db.QueryContext(ctx, `SELECT status,COUNT(*) FROM work_items WHERE board_id=? GROUP BY status`, board.ID)
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
