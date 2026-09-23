package delegation

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRetryWorkCircuitBreakerRefusesConsecutiveFails(t *testing.T) {
	ctx := context.Background()
	store, workspace := cbTestStore(t)
	item := cbCreateTask(t, ctx, store, workspace, "cb-fails", []string{"internal/delegation/src.go"})

	cbReviewOutcome(t, ctx, store, item.ID, "FAIL")
	if _, err := store.RetryWork(ctx, item.ID, 0); err != nil {
		t.Fatalf("retry after one FAIL refused: %v", err)
	}
	cbReviewOutcome(t, ctx, store, item.ID, "BLOCKED")
	if _, err := store.RetryWork(ctx, item.ID, 0); err != nil {
		t.Fatalf("retry with ignored BLOCKED refused: %v", err)
	}
	cbReviewOutcome(t, ctx, store, item.ID, "INCONCLUSIVE")
	if _, err := store.RetryWork(ctx, item.ID, 0); err != nil {
		t.Fatalf("retry with ignored INCONCLUSIVE refused: %v", err)
	}
	cbReviewOutcome(t, ctx, store, item.ID, "FAIL")

	before, err := store.GetWork(ctx, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if before.Status != WorkBlocked {
		t.Fatalf("expected blocked task, got %s", before.Status)
	}
	cbInsertSentinelLease(t, store, item.ID)

	_, err = store.RetryWork(ctx, item.ID, before.Revision)
	if !errors.Is(err, ErrWorkReviewFailStreak) {
		t.Fatalf("expected review-fail streak refusal, got %v", err)
	}
	if !strings.Contains(err.Error(), "cortex_ia_work_decompose") {
		t.Fatalf("breaker error must route to decomposition, got %v", err)
	}

	after, err := store.GetWork(ctx, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.Status != WorkBlocked || after.Revision != before.Revision {
		t.Fatalf("refused retry mutated task: %+v", after)
	}
	var leases, approvals int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM work_leases WHERE item_id=?`, item.ID).Scan(&leases); err != nil {
		t.Fatal(err)
	}
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM work_approvals WHERE item_id=?`, item.ID).Scan(&approvals); err != nil {
		t.Fatal(err)
	}
	if leases != 1 || approvals != 4 {
		t.Fatalf("refused retry deleted state: leases=%d approvals=%d", leases, approvals)
	}
}

func TestRetryWorkCircuitBreakerPassResetsStreak(t *testing.T) {
	ctx := context.Background()
	store, workspace := cbTestStore(t)
	item := cbCreateTask(t, ctx, store, workspace, "cb-pass", []string{"internal/delegation/src.go"})

	cbInsertApproval(t, store, item.ID, "FAIL", "2026-01-01T00:00:01Z")
	cbInsertApproval(t, store, item.ID, "FAIL", "2026-01-01T00:00:02Z")
	cbInsertApproval(t, store, item.ID, "PASS", "2026-01-01T00:00:03Z")
	cbBlockTask(t, store, item.ID)

	item, err := store.RetryWork(ctx, item.ID, 0)
	if err != nil {
		t.Fatalf("trailing PASS must not trip the breaker: %v", err)
	}
	if item.Status != WorkReady {
		t.Fatalf("expected ready after PASS reset, got %s", item.Status)
	}
}

func TestRetryWorkCircuitBreakerPureTestExemption(t *testing.T) {
	ctx := context.Background()
	store, workspace := cbTestStore(t)
	item := cbCreateTask(t, ctx, store, workspace, "cb-puretest", []string{
		"internal/delegation/work_retry_circuitbreaker_test.go",
		"scripts/tests/run.sh",
	})

	cbInsertApproval(t, store, item.ID, "FAIL", "2026-01-01T00:00:01Z")
	cbInsertApproval(t, store, item.ID, "FAIL", "2026-01-01T00:00:02Z")
	cbBlockTask(t, store, item.ID)

	item, err := store.RetryWork(ctx, item.ID, 0)
	if err != nil {
		t.Fatalf("pure-test task must stay retryable: %v", err)
	}
	if item.Status != WorkReady {
		t.Fatalf("expected ready pure-test retry, got %s", item.Status)
	}
	var exemptions int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM work_events WHERE item_id=? AND kind='pure_test_exemption'`, item.ID).Scan(&exemptions); err != nil {
		t.Fatal(err)
	}
	if exemptions != 1 {
		t.Fatalf("expected one pure-test exemption event, got %d", exemptions)
	}
}

func TestRetryWorkCircuitBreakerEmptyAllowedFilesNotExempt(t *testing.T) {
	ctx := context.Background()
	store, workspace := cbTestStore(t)
	item := cbCreateTask(t, ctx, store, workspace, "cb-empty", nil)

	cbInsertApproval(t, store, item.ID, "FAIL", "2026-01-01T00:00:01Z")
	cbInsertApproval(t, store, item.ID, "FAIL", "2026-01-01T00:00:02Z")
	cbBlockTask(t, store, item.ID)

	if _, err := store.RetryWork(ctx, item.ID, 0); !errors.Is(err, ErrWorkReviewFailStreak) {
		t.Fatalf("undeclared scope must not be exempt, got %v", err)
	}
}

func cbTestStore(t *testing.T) (*Store, string) {
	t.Helper()
	t.Setenv("CORTEX_IA_HOME", t.TempDir())
	workspace := t.TempDir()
	store, err := OpenStore(filepath.Join(t.TempDir(), "delegation.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if _, err := store.CreateBoard(context.Background(), "cb-board", "Circuit Breaker Board", ""); err != nil {
		t.Fatal(err)
	}
	return store, workspace
}

func cbCreateTask(t *testing.T, ctx context.Context, store *Store, workspace, id string, allowed []string) WorkItem {
	t.Helper()
	item, err := store.CreateWorkInBoardWithDefinition(ctx, "cb-board", id, id+" title", nil, WorkDefinition{
		Project: workspace, Objective: "objective", Acceptance: "acceptance", Verification: "verification", AllowedFiles: allowed,
	})
	if err != nil {
		t.Fatalf("create task %s: %v", id, err)
	}
	return item
}

func cbReviewOutcome(t *testing.T, ctx context.Context, store *Store, id, verdict string) {
	t.Helper()
	claimed, err := store.ClaimWork(ctx, id, "impl", time.Minute)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if _, err := store.TransitionWork(ctx, id, claimed.Token, claimed.Revision, WorkInReview); err != nil {
		t.Fatalf("transition in_review: %v", err)
	}
	if _, err := store.ApproveWork(ctx, id, "reviewer", verdict, "evidence", 0); err != nil {
		t.Fatalf("approve %s: %v", verdict, err)
	}
}

func cbInsertApproval(t *testing.T, store *Store, id, verdict, createdAt string) {
	t.Helper()
	if _, err := store.db.Exec(`INSERT INTO work_approvals(item_id,reviewer,verdict,evidence,created_at) VALUES(?,?,?,?,?)`, id, "reviewer", verdict, "evidence", createdAt); err != nil {
		t.Fatal(err)
	}
}

func cbBlockTask(t *testing.T, store *Store, id string) {
	t.Helper()
	if _, err := store.db.Exec(`UPDATE work_items SET status='blocked',revision=revision+1 WHERE id=?`, id); err != nil {
		t.Fatal(err)
	}
}

func cbInsertSentinelLease(t *testing.T, store *Store, id string) {
	t.Helper()
	if _, err := store.db.Exec(`INSERT INTO work_leases(path,item_id,token_hash,expires_at,created_at,updated_at) VALUES(?,?,?,?,?,?)`,
		"internal/delegation/sentinel.go", id, "sentinel", "2999-01-01T00:00:00Z", "2026-01-01T00:00:00Z", "2026-01-01T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
}
