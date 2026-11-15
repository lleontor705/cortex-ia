package delegation

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// DefaultDegradeStaleWorkTTL is the R3 hygiene window: ready/blocked tasks whose
// last update predates it degrade to backlog unless a user decision is pending.
const DefaultDegradeStaleWorkTTL = 168 * time.Hour

// degradeClassReady labels degraded ready tasks in the receipt; blocked tasks are
// labeled by their durable blocked_reason class.
const degradeClassReady = "ready"

// DegradeStaleWorkOptions configures class-aware TTL degradation.
type DegradeStaleWorkOptions struct {
	BoardID string        `json:"board_id,omitempty"`
	TTL     time.Duration `json:"ttl,omitempty"`
	DryRun  bool          `json:"dry_run"`
}

// DegradeStaleWorkResult is the structured receipt: total degraded count, task IDs
// grouped by degradation class, and the needs_user IDs that were exempted.
type DegradeStaleWorkResult struct {
	BoardID  string              `json:"board_id,omitempty"`
	TTL      string              `json:"ttl"`
	DryRun   bool                `json:"dry_run"`
	Degraded int                 `json:"degraded"`
	ByReason map[string][]string `json:"by_reason"`
	Exempted []string            `json:"exempted_ids"`
}

type degradeCandidate struct {
	id    string
	class string
}

// DegradeStaleWork demotes stale ready/blocked tasks to backlog after TTL. It is
// class-aware: blocked tasks carrying needs_user are exempt because a user decision
// is pending; every other class degrades, with legacy NULL read as unclassified.
// Rows with live claims or leases, and every non ready/blocked status, are never
// touched. The candidate scan runs outside the write lock; the batch apply reruns
// the eligibility guard inside BEGIN IMMEDIATE so a concurrent claim cannot be
// clobbered.
func (s *Store) DegradeStaleWork(ctx context.Context, opts DegradeStaleWorkOptions) (DegradeStaleWorkResult, error) {
	ttl := opts.TTL
	if ttl <= 0 {
		ttl = DefaultDegradeStaleWorkTTL
	}
	now := s.timestamp()
	cutoff := s.now().Add(-ttl).UTC().Format(time.RFC3339Nano)
	boardID := strings.TrimSpace(opts.BoardID)

	result := DegradeStaleWorkResult{
		BoardID:  boardID,
		TTL:      ttl.String(),
		DryRun:   opts.DryRun,
		ByReason: map[string][]string{},
		Exempted: []string{},
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT wi.id, wi.status, COALESCE(wi.blocked_reason,'')
		FROM work_items wi
		WHERE wi.status IN ('ready','blocked')
		  AND (? = '' OR wi.board_id = ?)
		  AND wi.updated_at <= ?
		  AND NOT EXISTS (SELECT 1 FROM work_claims c WHERE c.item_id = wi.id AND c.expires_at > ?)
		  AND NOT EXISTS (SELECT 1 FROM work_leases l WHERE l.item_id = wi.id AND l.expires_at > ?)
		ORDER BY wi.updated_at ASC, wi.id ASC`, boardID, boardID, cutoff, now, now)
	if err != nil {
		return result, fmt.Errorf("query degrade candidates: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var candidates []degradeCandidate
	for rows.Next() {
		var id, reason string
		var status WorkStatus
		if err := rows.Scan(&id, &status, &reason); err != nil {
			return result, err
		}
		if status == WorkBlocked && WorkBlockedReasonClass(reason) == WorkBlockedReasonNeedsUser {
			result.Exempted = append(result.Exempted, id)
			continue
		}
		candidates = append(candidates, degradeCandidate{id: id, class: degradeClass(status, reason)})
	}
	if err := rows.Err(); err != nil {
		return result, err
	}

	if opts.DryRun {
		for _, candidate := range candidates {
			result.ByReason[candidate.class] = append(result.ByReason[candidate.class], candidate.id)
			result.Degraded++
		}
		return result, nil
	}
	if len(candidates) == 0 {
		return result, nil
	}

	err = s.immediate(ctx, func(conn *sql.Conn) error {
		for _, candidate := range candidates {
			var from WorkStatus
			var revision int64
			var reason string
			err := conn.QueryRowContext(ctx, `
				SELECT status, revision, COALESCE(blocked_reason,'')
				FROM work_items
				WHERE id = ?
				  AND status IN ('ready','blocked')
				  AND NOT EXISTS (SELECT 1 FROM work_claims c WHERE c.item_id = work_items.id AND c.expires_at > ?)
				  AND NOT EXISTS (SELECT 1 FROM work_leases l WHERE l.item_id = work_items.id AND l.expires_at > ?)`,
				candidate.id, now, now).Scan(&from, &revision, &reason)
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			if err != nil {
				return err
			}
			if from == WorkBlocked && WorkBlockedReasonClass(reason) == WorkBlockedReasonNeedsUser {
				continue
			}
			res, err := conn.ExecContext(ctx, `UPDATE work_items SET status='backlog',blocked_reason=NULL,revision=revision+1,updated_at=? WHERE id=? AND status=? AND revision=?`, now, candidate.id, from, revision)
			if err != nil {
				return fmt.Errorf("degrade task %s: %w", candidate.id, err)
			}
			if changed, _ := res.RowsAffected(); changed != 1 {
				continue
			}
			class := degradeClass(from, reason)
			if err := s.addWorkEvent(ctx, conn, candidate.id, "degraded", string(from), string(WorkBacklog), fmt.Sprintf("%d:class=%s", revision, class)); err != nil {
				return err
			}
			result.ByReason[class] = append(result.ByReason[class], candidate.id)
			result.Degraded++
		}
		return nil
	})
	if err != nil {
		return result, fmt.Errorf("execute degrade: %w", err)
	}
	return result, nil
}

func degradeClass(status WorkStatus, reason string) string {
	if status == WorkBlocked {
		return WorkBlockedReasonClass(reason)
	}
	return degradeClassReady
}
