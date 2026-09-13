package delegation

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

func parseProjectionTime(s string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err == nil {
		return t, nil
	}
	return time.Parse(time.RFC3339, s)
}

func (s *Store) ReadWorkProjection(ctx context.Context, id string, presentationRole string, actor string, notes []WorkProjectionNote) (WorkProjection, error) {
	return s.ReadWorkProjectionAt(ctx, id, presentationRole, actor, notes, time.Now().UTC())
}

func (s *Store) ReadWorkProjectionAt(ctx context.Context, id string, presentationRole string, actor string, notes []WorkProjectionNote, now time.Time) (WorkProjection, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return WorkProjection{AuthorityAvailable: false}, ErrWorkNotFound
	}

	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return WorkProjection{AuthorityAvailable: false}, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	var itemID, statusStr, updatedAt string
	var revision int64
	err = tx.QueryRowContext(ctx, `SELECT id, revision, status, updated_at FROM work_items WHERE id=?`, id).Scan(&itemID, &revision, &statusStr, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return WorkProjection{AuthorityAvailable: false}, ErrWorkNotFound
	}
	if err != nil {
		return WorkProjection{AuthorityAvailable: false}, err
	}

	status := WorkStatus(statusStr)

	var decompCount int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM work_decomposition_steps WHERE parent_id=?`, id).Scan(&decompCount); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return WorkProjection{AuthorityAvailable: false}, err
	}
	if decompCount > 0 {
		status = WorkSuperseded
	}

	rows, err := tx.QueryContext(ctx, `SELECT d.depends_on, COALESCE(dep.status, 'missing') FROM work_dependencies d LEFT JOIN work_items dep ON dep.id=d.depends_on WHERE d.item_id=? ORDER BY d.depends_on`, id)
	if err != nil {
		return WorkProjection{AuthorityAvailable: false}, err
	}

	var allDeps, unsatisfiedDeps []string
	for rows.Next() {
		var depID, depStatus string
		if err := rows.Scan(&depID, &depStatus); err != nil {
			_ = rows.Close()
			return WorkProjection{AuthorityAvailable: false}, err
		}
		allDeps = append(allDeps, depID)
		if depStatus != string(WorkDone) {
			unsatisfiedDeps = append(unsatisfiedDeps, depID)
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return WorkProjection{AuthorityAvailable: false}, err
	}
	_ = rows.Close()

	var claimOwner, claimExpiresAt string
	var claimExpired bool
	err = tx.QueryRowContext(ctx, `SELECT owner, expires_at FROM work_claims WHERE item_id=?`, id).Scan(&claimOwner, &claimExpiresAt)
	if err == nil {
		if expTime, parseErr := parseProjectionTime(claimExpiresAt); parseErr == nil {
			claimExpired = !now.Before(expTime)
		} else {
			claimExpired = true
		}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return WorkProjection{AuthorityAvailable: false}, err
	}

	var reviewID, implOwner string
	var reviewRev int64
	var hasActiveReview bool
	err = tx.QueryRowContext(ctx, `SELECT review_id, implementation_owner, review_revision FROM work_reviews WHERE item_id=?`, id).Scan(&reviewID, &implOwner, &reviewRev)
	if err == nil {
		hasActiveReview = true
	} else if !errors.Is(err, sql.ErrNoRows) {
		return WorkProjection{AuthorityAvailable: false}, err
	}

	var verificationVerdict string
	_ = tx.QueryRowContext(ctx, `SELECT verdict FROM work_approvals WHERE item_id=? ORDER BY revision DESC, created_at DESC LIMIT 1`, id).Scan(&verificationVerdict)

	input := WorkProjectionInput{
		TaskID:              itemID,
		Revision:            revision,
		AsOf:                updatedAt,
		Status:              status,
		VerificationVerdict: verificationVerdict,
		Dependencies:        allDeps,
		UnsatisfiedDeps:     unsatisfiedDeps,
		ClaimOwner:          claimOwner,
		ClaimExpiresAt:      claimExpiresAt,
		ClaimExpired:        claimExpired,
		ReviewID:            reviewID,
		ImplementationOwner: implOwner,
		ReviewRevision:      reviewRev,
		HasActiveReview:     hasActiveReview,
		AuthorityAvailable:  true,
		PresentationRole:    presentationRole,
		Actor:               actor,
		Notes:               notes,
		Now:                 now,
	}

	return ProjectWork(input), nil
}
