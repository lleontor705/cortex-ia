package delegation

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os/user"
	"strings"
	"time"
	"unicode/utf8"
)

// Every projection and admission decision binds the proof to the same execution.
// A status change or a changed execution identity cannot inherit an old proof.
const reconciledJobSQL = `EXISTS (SELECT 1 FROM delegation_reconciliations p WHERE
	p.job_id=j.id AND p.attempt=j.attempt AND p.workspace=j.workspace AND p.pid=j.pid
	AND p.started_at=j.started_at AND p.transport=j.transport AND p.pane_id=j.pane_id
	AND p.job_created_at=j.created_at AND p.job_updated_at=j.updated_at)`

func (s *Store) migrateReconciliation(ctx context.Context, conn *sql.Conn) error {
	for _, statement := range []string{
		`CREATE TABLE delegation_reconciliations (
			job_id TEXT NOT NULL REFERENCES delegation_jobs(id), attempt INTEGER NOT NULL CHECK(attempt>0),
			workspace TEXT NOT NULL, pid INTEGER NOT NULL CHECK(pid>0), started_at TEXT NOT NULL,
			transport TEXT NOT NULL, pane_id TEXT NOT NULL, job_created_at TEXT NOT NULL, job_updated_at TEXT NOT NULL,
			boot_at TEXT NOT NULL CHECK(length(boot_at)<=64), source TEXT NOT NULL CHECK(length(source)<=128),
			reason TEXT NOT NULL CHECK(length(CAST(reason AS BLOB)) BETWEEN 1 AND 1024),
			actor TEXT NOT NULL CHECK(length(CAST(actor AS BLOB)) BETWEEN 1 AND 512),
			recorded_at TEXT NOT NULL, PRIMARY KEY(job_id,attempt)
		) STRICT`,
		`CREATE TRIGGER delegation_reconciliations_immutable_update BEFORE UPDATE ON delegation_reconciliations BEGIN SELECT RAISE(ABORT,'immutable reconciliation proof'); END`,
		`CREATE TRIGGER delegation_reconciliations_immutable_delete BEFORE DELETE ON delegation_reconciliations BEGIN SELECT RAISE(ABORT,'immutable reconciliation proof'); END`,
	} {
		if _, err := conn.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("reconciliation migration: %w", err)
		}
	}
	_, err := conn.ExecContext(ctx, `INSERT INTO schema_migrations(version,applied_at) VALUES(13,?)`, s.timestamp())
	return err
}

type bootObservation struct {
	Time   time.Time
	Source string
}

type reconciliationIdentity struct {
	Status                                                        Status
	Attempt, PID                                                  int
	Workspace, StartedAt, Transport, PaneID, CreatedAt, UpdatedAt string
}

type Reconciliation struct {
	JobID                  string `json:"job_id"`
	Attempt                int    `json:"attempt"`
	Status                 Status `json:"status"`
	Reconciled             bool   `json:"reconciled"`
	ReconciliationRequired bool   `json:"reconciliation_required"`
	BootAt                 string `json:"boot_at"`
	Source                 string `json:"source"`
	Reason                 string `json:"reason"`
	Actor                  string `json:"actor"`
	RecordedAt             string `json:"recorded_at"`
}

type reconciliationReader interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func readReconciliation(ctx context.Context, db reconciliationReader, id string) (reconciliationIdentity, Reconciliation, error) {
	var identity reconciliationIdentity
	err := db.QueryRowContext(ctx, `SELECT status,attempt,pid,workspace,started_at,transport,pane_id,created_at,updated_at FROM delegation_jobs WHERE id=?`, id).Scan(
		&identity.Status, &identity.Attempt, &identity.PID, &identity.Workspace, &identity.StartedAt, &identity.Transport, &identity.PaneID, &identity.CreatedAt, &identity.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return identity, Reconciliation{}, ErrJobNotFound
	}
	if err != nil {
		return identity, Reconciliation{}, err
	}
	proof := Reconciliation{JobID: id, Attempt: identity.Attempt, Status: identity.Status}
	err = db.QueryRowContext(ctx, `SELECT p.boot_at,p.source,p.reason,p.actor,p.recorded_at FROM delegation_reconciliations p JOIN delegation_jobs j ON p.job_id=j.id AND p.attempt=j.attempt WHERE j.id=? AND j.status='lost' AND `+reconciledJobSQL, id).Scan(&proof.BootAt, &proof.Source, &proof.Reason, &proof.Actor, &proof.RecordedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return identity, proof, nil
	}
	proof.Reconciled = err == nil
	return identity, proof, err
}

// Reconcile proves only termination, never successful execution or write authority.
// Boot evidence is collected internally; callers cannot supply a timestamp or force it.
func (s *Store) Reconcile(ctx context.Context, id, reason, sessionID string) (Reconciliation, error) {
	if sessionID != "" {
		if err := (ConversationOwnership{OpenCodeSessionID: sessionID, OpenCodeRootSessionID: sessionID}).Validate(); err != nil {
			return Reconciliation{}, err
		}
	}
	operator, err := user.Current()
	if err != nil {
		return Reconciliation{}, fmt.Errorf("identify local reconciliation operator: %w", err)
	}
	actor := "local:" + operator.Uid
	if sessionID != "" {
		actor += ";opencode:" + sessionID
	}
	return s.reconcile(ctx, id, reason, actor, observeSystemBoot)
}

func (s *Store) reconcile(ctx context.Context, id, reason, actor string, observe func(context.Context) (bootObservation, error)) (Reconciliation, error) {
	reason = strings.TrimSpace(reason)
	if len(reason) == 0 || len(reason) > 1024 || !utf8.ValidString(reason) || strings.ContainsRune(reason, 0) || len(actor) == 0 || len(actor) > 512 || !utf8.ValidString(actor) || strings.ContainsRune(actor, 0) {
		return Reconciliation{}, errors.New("reconciliation requires a reason of 1..1024 UTF-8 bytes and a valid local actor")
	}
	identity, proof, err := readReconciliation(ctx, s.db, id)
	if err != nil || proof.Reconciled {
		return proof, err
	}
	if identity.Status != StatusLost {
		return proof, fmt.Errorf("%w: only lost jobs may be reconciled; active workers must acknowledge cancellation", ErrInvalidTransition)
	}
	started, err := time.Parse(time.RFC3339Nano, identity.StartedAt)
	created, createdErr := time.Parse(time.RFC3339Nano, identity.CreatedAt)
	workspace, workspaceErr := CanonicalWorkspace(identity.Workspace)
	if err != nil || createdErr != nil || started.Before(created) || workspaceErr != nil || workspace != identity.Workspace || identity.Attempt < 1 || identity.PID < 1 {
		return proof, errors.New("reconciliation requires a valid recorded execution identity and start time")
	}
	boot, err := observe(ctx)
	if err != nil {
		return proof, fmt.Errorf("cannot prove prior-boot termination: %w", err)
	}
	// Both persisted starts and Windows CIM boot time are UTC wall-clock observations.
	// They assume a trustworthy local clock; reject missing/future times and a one-second
	// uncertainty window instead of treating timestamp equality or PID absence as proof.
	if boot.Time.IsZero() || boot.Time.After(s.now()) || !boot.Time.After(started.Add(time.Second)) || boot.Source == "" || len(boot.Source) > 128 {
		return proof, errors.New("termination unproven: execution is not demonstrably before the current boot; workspace remains fenced")
	}
	err = s.immediate(ctx, func(conn *sql.Conn) error {
		current, existing, err := readReconciliation(ctx, conn, id)
		if err != nil {
			return err
		}
		if current != identity {
			return fmt.Errorf("%w: execution changed while collecting boot evidence", ErrWorkConflict)
		}
		if existing.Reconciled {
			proof = existing
			return nil
		}
		proof = Reconciliation{JobID: id, Attempt: identity.Attempt, Status: StatusLost, Reconciled: true, BootAt: boot.Time.UTC().Format(time.RFC3339Nano), Source: boot.Source, Reason: reason, Actor: actor, RecordedAt: s.timestamp()}
		_, err = conn.ExecContext(ctx, `INSERT INTO delegation_reconciliations(job_id,attempt,workspace,pid,started_at,transport,pane_id,job_created_at,job_updated_at,boot_at,source,reason,actor,recorded_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, id, identity.Attempt, identity.Workspace, identity.PID, identity.StartedAt, identity.Transport, identity.PaneID, identity.CreatedAt, identity.UpdatedAt, proof.BootAt, proof.Source, reason, actor, proof.RecordedAt)
		if err != nil {
			return err
		}
		return s.addEvent(ctx, conn, id, "termination_reconciled", StatusLost, StatusLost, "prior-boot process termination proved; execution outcome unchanged")
	})
	if err != nil {
		return Reconciliation{}, err
	}
	return proof, nil
}
