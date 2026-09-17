package delegation

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

var ErrTerminationUnconfirmed = errors.New("external process termination or cleanup unconfirmed; reconciliation required")

// Cancel requests termination; only an unclaimed job can terminate immediately.
func (s *Store) Cancel(ctx context.Context, id string) error {
	return s.immediate(ctx, func(conn *sql.Conn) error {
		var status Status
		var code string
		if err := conn.QueryRowContext(ctx, `SELECT status,error_code FROM delegation_jobs WHERE id=?`, id).Scan(&status, &code); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrJobNotFound
			}
			return err
		}
		if terminal(status) || code == "CANCEL_REQUESTED" {
			return nil
		}
		now := s.timestamp()
		if status == StatusAccepted {
			if _, err := conn.ExecContext(ctx, `UPDATE delegation_jobs SET status='cancelled',error_code='CANCELLED',error_message='cancelled before worker claim',updated_at=?,finished_at=? WHERE id=?`, now, now, id); err != nil {
				return err
			}
			if _, err := conn.ExecContext(ctx, `INSERT INTO delegation_receipts(job_id,status,output_json,output_hash,exit_code,created_at) VALUES(?,'cancelled','{}','',-1,?)`, id, now); err != nil {
				return err
			}
			return s.addEvent(ctx, conn, id, "cancelled", status, StatusCancelled, "cancelled before worker claim")
		}
		if _, err := conn.ExecContext(ctx, `UPDATE delegation_jobs SET error_code='CANCEL_REQUESTED',error_message='cancellation requested; awaiting worker termination acknowledgement',updated_at=? WHERE id=?`, now, id); err != nil {
			return err
		}
		return s.addEvent(ctx, conn, id, "cancellation_requested", status, status, "worker acknowledgement required")
	})
}

// CompleteWorker is called only after runAGY has confirmed tree and home cleanup.
// Its transaction orders completion against a concurrent cancellation request.
func (s *Store) CompleteWorker(ctx context.Context, id, owner string, status Status, receipt Receipt, code, message string) error {
	if owner == "" {
		return fmt.Errorf("%w: worker owner required", ErrInvalidTransition)
	}
	return s.completeWorker(ctx, id, owner, status, receipt, code, message)
}

func (s *Store) MarkTerminationUnconfirmed(ctx context.Context, id, owner string) error {
	return s.immediate(ctx, func(conn *sql.Conn) error {
		result, err := conn.ExecContext(ctx, `UPDATE delegation_jobs SET error_code=CASE WHEN error_code='CANCEL_REQUESTED' THEN error_code ELSE 'TERMINATION_UNCONFIRMED' END,error_message='process termination or cleanup unconfirmed; reconciliation required',updated_at=? WHERE id=? AND lease_owner=? AND status IN ('starting','running','blocked','lost')`, s.timestamp(), id, owner)
		if err != nil {
			return err
		}
		changed, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if changed != 1 {
			return fmt.Errorf("%w: worker owner mismatch", ErrInvalidTransition)
		}
		return s.addEvent(ctx, conn, id, "termination_unconfirmed", "", "", "workspace remains fenced")
	})
}
