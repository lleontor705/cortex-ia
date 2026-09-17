package delegation

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func reconciliationFixture(t *testing.T) (*Store, NewJob, Job, func(context.Context) (bootObservation, error)) {
	t.Helper()
	s, input := cancellationStore(t)
	now := time.Now().UTC().Truncate(time.Second)
	s.now = func() time.Time { return now.Add(-48 * time.Hour) }
	job, err := s.Create(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Claim(context.Background(), job.ID, "synthetic-worker", os.Getpid(), time.Minute); err != nil {
		t.Fatal(err)
	}
	s.now = func() time.Time { return now }
	if _, err = s.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
	return s, input, job, func(context.Context) (bootObservation, error) {
		return bootObservation{Time: now.Add(-24 * time.Hour), Source: "synthetic-boot"}, nil
	}
}

func TestReconciliationPriorBootPreservesHistory(t *testing.T) {
	s, input, job, observe := reconciliationFixture(t)
	ctx := context.Background()
	before, _ := s.Get(ctx, job.ID)
	receiptBefore, _ := s.Result(ctx, job.ID)
	_, err := s.db.Exec(`INSERT INTO work_items(id,title,status,created_at,updated_at) VALUES('fixture','fixture','in_progress','old','old'); INSERT INTO work_claims(item_id,owner,token_hash,attempt,expires_at,created_at,updated_at) VALUES('fixture','owner','synthetic-hash',1,'expired','old','old'); INSERT INTO work_leases(path,item_id,token_hash,expires_at,created_at,updated_at) VALUES('fixture.go','fixture','synthetic-hash','expired','old','old')`)
	if err != nil {
		t.Fatal(err)
	}
	proof, err := s.reconcile(ctx, job.ID, " inspected prior boot ", "local:synthetic", observe)
	if err != nil || !proof.Reconciled || proof.Reason != "inspected prior boot" {
		t.Fatalf("proof=%+v err=%v", proof, err)
	}
	duplicate, err := s.reconcile(ctx, job.ID, "again", "local:other", func(context.Context) (bootObservation, error) {
		t.Fatal("idempotency probed OS")
		return bootObservation{}, errors.New("unavailable")
	})
	if err != nil || duplicate != proof {
		t.Fatalf("duplicate=%+v err=%v", duplicate, err)
	}
	after, _ := s.Get(ctx, job.ID)
	if after.ReconciliationRequired || !after.TerminationReconciled {
		t.Fatal("status still fenced")
	}
	after.ReconciliationRequired = before.ReconciliationRequired
	after.TerminationReconciled = before.TerminationReconciled
	if !reflect.DeepEqual(before, after) {
		t.Fatal("historical job changed")
	}
	receiptAfter, _ := s.Result(ctx, job.ID)
	if receiptAfter.ReconciliationRequired || !receiptAfter.TerminationReconciled {
		t.Fatal("receipt projection inconsistent")
	}
	receiptAfter.ReconciliationRequired = receiptBefore.ReconciliationRequired
	receiptAfter.TerminationReconciled = receiptBefore.TerminationReconciled
	if !reflect.DeepEqual(receiptBefore, receiptAfter) {
		t.Fatal("historical receipt changed")
	}
	var claim, lease string
	_ = s.db.QueryRow(`SELECT token_hash||expires_at FROM work_claims WHERE item_id='fixture'`).Scan(&claim)
	_ = s.db.QueryRow(`SELECT token_hash||expires_at FROM work_leases WHERE item_id='fixture'`).Scan(&lease)
	if claim != "synthetic-hashexpired" || lease != claim {
		t.Fatal("old authority changed")
	}
	for _, query := range []string{`UPDATE delegation_reconciliations SET reason='changed'`, `DELETE FROM delegation_reconciliations`} {
		if _, err = s.db.Exec(query); err == nil {
			t.Fatal("mutable proof")
		}
	}
	next, err := s.Create(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Create(ctx, input); err == nil || !strings.Contains(err.Error(), next.ID) || !strings.Contains(err.Error(), "cancellation") {
		t.Fatalf("active blocker lost: %v", err)
	}
}

func TestReconciliationRefusesUnprovenAndActive(t *testing.T) {
	for _, kind := range []string{"same-boot-absent", "same-boot-reused", "missing-start", "missing-boot", "future-boot", "ambiguous", "pending-cancel", "unsupported"} {
		t.Run(kind, func(t *testing.T) {
			s, input, job, observe := reconciliationFixture(t)
			reason := "synthetic reason"
			switch kind {
			case "same-boot-absent", "same-boot-reused":
				pid := 2147483647
				if kind == "same-boot-reused" {
					pid = os.Getpid()
				}
				_, _ = s.db.Exec(`UPDATE delegation_jobs SET pid=?,started_at=? WHERE id=?`, pid, s.now().Add(-time.Hour).Format(time.RFC3339Nano), job.ID)
			case "missing-start":
				_, _ = s.db.Exec(`UPDATE delegation_jobs SET started_at='' WHERE id=?`, job.ID)
			case "missing-boot":
				observe = func(context.Context) (bootObservation, error) { return bootObservation{}, nil }
			case "future-boot":
				observe = func(context.Context) (bootObservation, error) {
					return bootObservation{Time: s.now().Add(time.Hour), Source: "synthetic"}, nil
				}
			case "ambiguous":
				observe = func(context.Context) (bootObservation, error) {
					return bootObservation{Time: s.now().Add(-48*time.Hour + time.Millisecond), Source: "synthetic"}, nil
				}
			case "pending-cancel":
				_, _ = s.db.Exec(`UPDATE delegation_jobs SET status='running',error_code='CANCEL_REQUESTED' WHERE id=?`, job.ID)
			case "unsupported":
				observe = func(context.Context) (bootObservation, error) { return bootObservation{}, errors.New("unsupported") }
			}
			if _, err := s.reconcile(context.Background(), job.ID, reason, "local:test", observe); err == nil {
				t.Fatal("unproven termination accepted")
			}
			if _, err := s.Create(context.Background(), input); err == nil || !strings.Contains(err.Error(), job.ID) {
				t.Fatalf("fence lost: %v", err)
			}
			var count int
			_ = s.db.QueryRow(`SELECT COUNT(*) FROM delegation_reconciliations`).Scan(&count)
			if count != 0 {
				t.Fatal("proof persisted")
			}
		})
	}
}

func TestReconciliationRevalidatesIdentityAndRollsBack(t *testing.T) {
	for _, field := range []string{"attempt=attempt+1", "pid=pid+1", "workspace=workspace||'/other'", "status='running'", "started_at='changed'", "pane_id='reused'", "rollback"} {
		t.Run(field, func(t *testing.T) {
			s, _, job, observe := reconciliationFixture(t)
			if field == "rollback" {
				_, err := s.db.Exec(`CREATE TRIGGER reject_reconciled_event BEFORE INSERT ON delegation_events WHEN NEW.kind='termination_reconciled' BEGIN SELECT RAISE(ABORT,'synthetic persistence failure'); END`)
				if err != nil {
					t.Fatal(err)
				}
			} else {
				original := observe
				observe = func(ctx context.Context) (bootObservation, error) {
					_, err := s.db.Exec(`UPDATE delegation_jobs SET `+field+` WHERE id=?`, job.ID)
					if err != nil {
						t.Fatal(err)
					}
					return original(ctx)
				}
			}
			if _, err := s.reconcile(context.Background(), job.ID, "test", "local:test", observe); err == nil {
				t.Fatal("changed execution or partial persistence accepted")
			}
			var count int
			_ = s.db.QueryRow(`SELECT COUNT(*) FROM delegation_reconciliations`).Scan(&count)
			if count != 0 {
				t.Fatal("partial proof survived")
			}
		})
	}
}

func TestReconciliationConcurrentAndOtherBlocker(t *testing.T) {
	s, input, job, observe := reconciliationFixture(t)
	ctx := context.Background()
	ready := make(chan struct{}, 2)
	release := make(chan struct{})
	probe := func(ctx context.Context) (bootObservation, error) {
		ready <- struct{}{}
		<-release
		return observe(ctx)
	}
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Go(func() {
			if _, err := s.reconcile(ctx, job.ID, "concurrent", "local:test", probe); err != nil {
				t.Error(err)
			}
		})
	}
	<-ready
	<-ready
	close(release)
	wg.Wait()
	for _, table := range []string{"delegation_reconciliations", "delegation_events WHERE kind='termination_reconciled'"} {
		var n int
		_ = s.db.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&n)
		if n != 1 {
			t.Fatalf("duplicate audit rows %s: %d", table, n)
		}
	}
	other, err := s.Create(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Claim(ctx, other.ID, "other", 123, time.Second); err != nil {
		t.Fatal(err)
	}
	_, _ = s.db.Exec(`UPDATE delegation_jobs SET lease_expires_at='2000-01-01T00:00:00Z' WHERE id=?`, other.ID)
	if _, err = s.Recover(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Create(ctx, input); err == nil || !strings.Contains(err.Error(), other.ID) || !strings.Contains(err.Error(), "delegate reconcile") {
		t.Fatalf("other lost blocker not actionable: %v", err)
	}
	_, _ = s.db.Exec(`UPDATE delegation_jobs SET attempt=attempt+1 WHERE id=?`, job.ID)
	changed, err := s.Get(ctx, job.ID)
	if err != nil || !changed.ReconciliationRequired {
		t.Fatal("proof authorized changed attempt")
	}
}

func TestReconciliationMigrationAndNoReceipt(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "state.db")
	s, err := OpenStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.db.Exec(`DROP TRIGGER delegation_reconciliations_immutable_delete; DROP TRIGGER delegation_reconciliations_immutable_update; DROP TABLE delegation_reconciliations; DELETE FROM schema_migrations WHERE version=13`)
	if err != nil {
		t.Fatal(err)
	}
	_ = s.Close()
	for i := 0; i < 2; i++ {
		s, err = OpenStore(dbPath)
		if err != nil {
			t.Fatal(err)
		}
		var n int
		_ = s.db.QueryRow(`SELECT COUNT(*) FROM delegation_reconciliations`).Scan(&n)
		if n != 0 {
			t.Fatal("migration fabricated proof")
		}
		_ = s.Close()
	}
	s, _, job, observe := reconciliationFixture(t)
	_, _ = s.db.Exec(`DELETE FROM delegation_receipts WHERE job_id=?`, job.ID)
	if _, err = s.reconcile(context.Background(), job.ID, "no receipt", "local:test", observe); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Result(context.Background(), job.ID); err == nil {
		t.Fatal("fabricated receipt")
	}
}
