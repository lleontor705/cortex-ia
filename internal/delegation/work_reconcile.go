package delegation

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	reconcileConditionTaskNotFound        = "task_not_found"
	reconcileConditionStatusNotInProgress = "status_not_in_progress"
	reconcileConditionClaimNotLive        = "claim_not_live"
	reconcileConditionReasonMissing       = "reason_missing"
	reconcileConditionReasonTooLong       = "reason_too_long"
	reconcileConditionCurrentSessionOwner = "current_session_owner"
	reconcileConditionFreshNoEvidence     = "claim_fresh_no_inactivity_evidence"
	reconcileConditionRevisionMismatch    = "revision_mismatch"
	reconcileConditionAttemptLimit        = "attempt_limit"
)

const maxReconcileReasonBytes = 512

// workReconcileStaleWindow mirrors the cortex-work bridge maintenancePolicy
// stale_progress_ms (15m). A claim whose heartbeat (work_claims.updated_at) is
// older than this window is treated as orphaned, so it may be force-released
// without explicit host-attested owner-inactivity evidence. Keeping the two
// windows aligned prevents the bridge from renewing authority the CLI considers
// reclaimable, and vice versa.
const workReconcileStaleWindow = 15 * time.Minute

// ErrWorkReconcileRefused marks a fail-closed reconcile refusal. Nothing is
// mutated when it is returned; the concrete error enumerates every failed
// condition.
var ErrWorkReconcileRefused = errors.New("work reconcile refused")

// WorkReconcileRefusalError carries the full set of failed release conditions so
// a single invocation surfaces every reason at once instead of one at a time.
type WorkReconcileRefusalError struct {
	TaskID string
	Codes  []string
}

func (e *WorkReconcileRefusalError) Error() string {
	return fmt.Sprintf("%v: task %s failed release conditions: %s", ErrWorkReconcileRefused, e.TaskID, strings.Join(e.Codes, ", "))
}

func (e *WorkReconcileRefusalError) Unwrap() error { return ErrWorkReconcileRefused }

// FailedConditionCodes returns a copy of the ordered failed-condition codes.
func (e *WorkReconcileRefusalError) FailedConditionCodes() []string {
	return append([]string(nil), e.Codes...)
}

type ReconcileInput struct {
	TaskID               string
	Reason               string
	HostSessionID        string
	ExpectedRevision     int64
	To                   WorkStatus
	OwnerSessionInactive bool
}

// ReconcileDecisionSnapshot is the immutable audit record of the inputs that
// justified one release. It is persisted verbatim as the reconciled event detail
// and echoed in the receipt; it never carries claim or lease token material.
type ReconcileDecisionSnapshot struct {
	ActorSession          string   `json:"actor_session"`
	Reason                string   `json:"reason"`
	ClaimOwner            string   `json:"claim_owner"`
	ClaimAttempt          int64    `json:"claim_attempt"`
	ClaimExpiresAt        string   `json:"claim_expires_at"`
	LastRenewedAt         string   `json:"last_renewed_at"`
	StalenessWindowMillis int64    `json:"staleness_window_ms"`
	OwnerSessionInactive  bool     `json:"owner_session_inactive"`
	RevisionBefore        int64    `json:"revision_before"`
	RevisionAfter         int64    `json:"revision_after"`
	ReleasedLeases        []string `json:"released_leases"`
}

type ReconcileResult struct {
	Decision       string                    `json:"decision"`
	TaskID         string                    `json:"task_id"`
	From           WorkStatus                `json:"from_status"`
	To             WorkStatus                `json:"to_status"`
	RevisionBefore int64                     `json:"revision_before"`
	RevisionAfter  int64                     `json:"revision_after"`
	ReleasedLeases []string                  `json:"released_leases"`
	Inputs         ReconcileDecisionSnapshot `json:"decision_inputs"`
}

// ReconcileWork force-releases a LIVE-but-orphaned claim that the normal
// expiry-driven RecoverWork sweep can never touch: the orchestrator lost the
// claim token, so the task would otherwise idle until the claim TTL elapses.
// Expired claims stay recover's exclusive domain. Every release condition is
// evaluated before any mutation and the whole operation runs in one
// BEGIN IMMEDIATE transaction, so a refusal leaves durable state byte-identical.
func (s *Store) ReconcileWork(ctx context.Context, input ReconcileInput) (ReconcileResult, error) {
	taskID := strings.TrimSpace(input.TaskID)
	if taskID == "" {
		return ReconcileResult{}, errors.New("reconcile requires a task id")
	}
	hostSession := strings.TrimSpace(input.HostSessionID)
	if hostSession == "" {
		return ReconcileResult{}, errors.New("reconcile requires a host session id")
	}
	to := input.To
	if to == "" {
		to = WorkBlocked
	}
	if to != WorkBlocked && to != WorkReady {
		return ReconcileResult{}, errors.New("reconcile target must be blocked or ready")
	}

	now := s.now().UTC()
	nowStamp := now.Format(time.RFC3339Nano)
	var result ReconcileResult
	err := s.immediate(ctx, func(conn *sql.Conn) error {
		reason := strings.TrimSpace(input.Reason)
		var codes []string
		switch {
		case reason == "":
			codes = append(codes, reconcileConditionReasonMissing)
		case len(reason) > maxReconcileReasonBytes:
			codes = append(codes, reconcileConditionReasonTooLong)
		}

		var status WorkStatus
		var revision int64
		lookupErr := conn.QueryRowContext(ctx, `SELECT status,revision FROM work_items WHERE id=?`, taskID).Scan(&status, &revision)
		if errors.Is(lookupErr, sql.ErrNoRows) {
			codes = append(codes, reconcileConditionTaskNotFound)
			return newReconcileRefusal(taskID, codes)
		}
		if lookupErr != nil {
			return lookupErr
		}

		var owner, expiresAt, updatedAt string
		var attempt int64
		claimErr := conn.QueryRowContext(ctx, `SELECT owner,attempt,expires_at,updated_at FROM work_claims WHERE item_id=?`, taskID).Scan(&owner, &attempt, &expiresAt, &updatedAt)
		if claimErr != nil && !errors.Is(claimErr, sql.ErrNoRows) {
			return claimErr
		}
		claimLive := claimErr == nil
		if claimLive {
			expiry, parseErr := time.Parse(time.RFC3339Nano, expiresAt)
			if parseErr != nil {
				return fmt.Errorf("reconcile: parse claim expiry: %w", parseErr)
			}
			// An expired claim belongs to work recover; reconcile must not become a
			// second expiry path.
			claimLive = expiry.After(now)
		}

		if status != WorkInProgress {
			codes = append(codes, reconcileConditionStatusNotInProgress)
		}
		if !claimLive {
			codes = append(codes, reconcileConditionClaimNotLive)
		}
		if input.ExpectedRevision <= 0 || revision != input.ExpectedRevision {
			codes = append(codes, reconcileConditionRevisionMismatch)
		}
		if claimLive {
			if owner == "opencode-session:"+hostSession {
				codes = append(codes, reconcileConditionCurrentSessionOwner)
			} else if !input.OwnerSessionInactive {
				renewed, parseErr := time.Parse(time.RFC3339Nano, updatedAt)
				if parseErr != nil {
					return fmt.Errorf("reconcile: parse claim heartbeat: %w", parseErr)
				}
				if now.Sub(renewed) <= workReconcileStaleWindow {
					codes = append(codes, reconcileConditionFreshNoEvidence)
				}
			}
		}
		if to == WorkReady {
			attempts, countErr := workAttemptCount(ctx, conn, taskID)
			if countErr != nil {
				return countErr
			}
			// Direct-to-ready is attempt-capped so reconcile cannot become an
			// infinite-retry loophole around the anti-revision circuit breaker.
			if attempts >= MaxWorkAttempts {
				codes = append(codes, reconcileConditionAttemptLimit)
			}
		}
		if len(codes) > 0 {
			return newReconcileRefusal(taskID, codes)
		}

		released, err := readReconcileLeasePaths(ctx, conn, taskID)
		if err != nil {
			return err
		}
		if _, err := conn.ExecContext(ctx, `DELETE FROM work_leases WHERE item_id=?`, taskID); err != nil {
			return fmt.Errorf("reconcile: release file leases: %w", err)
		}
		if _, err := conn.ExecContext(ctx, `DELETE FROM work_claims WHERE item_id=?`, taskID); err != nil {
			return fmt.Errorf("reconcile: release claim: %w", err)
		}
		update, err := conn.ExecContext(ctx, `UPDATE work_items SET status=?,revision=revision+1,updated_at=? WHERE id=? AND status='in_progress' AND revision=?`, to, nowStamp, taskID, revision)
		if err != nil {
			return fmt.Errorf("reconcile: transition task: %w", err)
		}
		if changed, _ := update.RowsAffected(); changed != 1 {
			return fmt.Errorf("%w: stale reconcile revision", ErrWorkConflict)
		}

		snapshot := ReconcileDecisionSnapshot{
			ActorSession:          hostSession,
			Reason:                reason,
			ClaimOwner:            owner,
			ClaimAttempt:          attempt,
			ClaimExpiresAt:        expiresAt,
			LastRenewedAt:         updatedAt,
			StalenessWindowMillis: workReconcileStaleWindow.Milliseconds(),
			OwnerSessionInactive:  input.OwnerSessionInactive,
			RevisionBefore:        revision,
			RevisionAfter:         revision + 1,
			ReleasedLeases:        released,
		}
		detail, err := json.Marshal(snapshot)
		if err != nil {
			return fmt.Errorf("reconcile: encode audit snapshot: %w", err)
		}
		if _, err := conn.ExecContext(ctx, `INSERT INTO work_events(item_id,kind,from_status,to_status,detail,created_at) VALUES(?,?,?,?,?,?)`, taskID, "reconciled", string(WorkInProgress), string(to), string(detail), nowStamp); err != nil {
			return fmt.Errorf("reconcile: append audit event: %w", err)
		}

		result = ReconcileResult{
			Decision:       "released",
			TaskID:         taskID,
			From:           WorkInProgress,
			To:             to,
			RevisionBefore: revision,
			RevisionAfter:  revision + 1,
			ReleasedLeases: released,
			Inputs:         snapshot,
		}
		return nil
	})
	if err != nil {
		return ReconcileResult{}, err
	}
	return result, nil
}

func newReconcileRefusal(taskID string, codes []string) error {
	return &WorkReconcileRefusalError{TaskID: taskID, Codes: codes}
}

func readReconcileLeasePaths(ctx context.Context, conn *sql.Conn, taskID string) ([]string, error) {
	rows, err := conn.QueryContext(ctx, `SELECT path FROM work_leases WHERE item_id=? ORDER BY path`, taskID)
	if err != nil {
		return nil, fmt.Errorf("reconcile: read leases: %w", err)
	}
	defer func() { _ = rows.Close() }()
	paths := []string{}
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return nil, err
		}
		paths = append(paths, path)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return paths, nil
}
