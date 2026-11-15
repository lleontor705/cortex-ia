package delegation

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type recoveryFixture struct {
	store *Store
	clock time.Time
}

func newRecoveryFixture(t *testing.T) *recoveryFixture {
	t.Helper()
	store, err := OpenStore(filepath.Join(t.TempDir(), "delegation.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	f := &recoveryFixture{store: store, clock: time.Now().UTC()}
	store.now = func() time.Time { return f.clock }
	return f
}

func (f *recoveryFixture) advance(d time.Duration) {
	f.clock = f.clock.Add(d)
}

func (f *recoveryFixture) create(t *testing.T, id string) {
	t.Helper()
	if _, err := f.store.CreateWorkInBoardWithDefinition(context.Background(), DefaultBoardID, id, id, nil, WorkDefinition{
		Project: t.TempDir(), Objective: "objective", Acceptance: "acceptance", Verification: "verification", AllowedFiles: []string{"src/a.go"},
	}); err != nil {
		t.Fatal(err)
	}
}

func (f *recoveryFixture) status(t *testing.T, id string) WorkStatus {
	t.Helper()
	item, err := f.store.GetWork(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return item.Status
}

func (f *recoveryFixture) count(t *testing.T, itemID string) int {
	return f.rows(t, "work_reviews", itemID)
}

func (f *recoveryFixture) rows(t *testing.T, table, itemID string) int {
	t.Helper()
	var n int
	if err := f.store.db.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM `+table+` WHERE item_id=?`, itemID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func assertScopedDenied(t *testing.T, err error, reason string) {
	t.Helper()
	if !errors.Is(err, ErrScopedRecoverDenied) {
		t.Fatalf("error %v is not ErrScopedRecoverDenied", err)
	}
	if !strings.Contains(err.Error(), reason) {
		t.Fatalf("denial %v does not name reason %q", err, reason)
	}
}

func TestRecoverWorkKeepsFreshReviewAfterClaimExpiry(t *testing.T) {
	ctx := context.Background()
	f := newRecoveryFixture(t)

	f.create(t, "live-review")
	claim, err := f.store.ClaimWorkWithLeases(ctx, "live-review", "implement-owner", []string{"src/a.go"}, 2*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	reviewed, err := f.store.TransitionWork(ctx, "live-review", claim.Token, claim.Revision, WorkInReview)
	if err != nil {
		t.Fatal(err)
	}

	f.create(t, "lost-claim")
	if _, err := f.store.ClaimWorkWithLeases(ctx, "lost-claim", "implement-owner-2", []string{"src/a.go"}, 2*time.Minute); err != nil {
		t.Fatal(err)
	}

	f.advance(3 * time.Minute)

	recovered, err := f.store.RecoverWork(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if recovered != 1 {
		t.Fatalf("expected only the expired in_progress claim to recover, got %d", recovered)
	}
	if got := f.status(t, "live-review"); got != WorkInReview {
		t.Fatalf("claim expiry flipped a live in_review task: %s", got)
	}
	if got := f.status(t, "lost-claim"); got != WorkBlocked {
		t.Fatalf("expired in_progress task was not recovered: %s", got)
	}
	if n := f.count(t, "live-review"); n != 1 {
		t.Fatalf("live review row was destroyed: %d", n)
	}

	approved, err := f.store.ApproveWork(ctx, "live-review", "independent-reviewer", "PASS", "independent verification", reviewed.Revision)
	if err != nil {
		t.Fatalf("approval after claim expiry: %v", err)
	}
	if approved.Verdict != "PASS" {
		t.Fatalf("unexpected verdict %s", approved.Verdict)
	}
	if got := f.status(t, "live-review"); got != WorkDone {
		t.Fatalf("approved task is not done: %s", got)
	}
}

func TestRecoverWorkBlocksAbandonedReview(t *testing.T) {
	ctx := context.Background()
	f := newRecoveryFixture(t)

	f.create(t, "abandoned-review")
	claim, err := f.store.ClaimWorkWithLeases(ctx, "abandoned-review", "implement-owner", []string{"src/a.go"}, 2*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.TransitionWork(ctx, "abandoned-review", claim.Token, claim.Revision, WorkInReview); err != nil {
		t.Fatal(err)
	}

	f.advance(40 * time.Minute)

	recovered, err := f.store.RecoverWork(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if recovered != 1 {
		t.Fatalf("expected abandoned review recovery, got %d", recovered)
	}
	if got := f.status(t, "abandoned-review"); got != WorkBlocked {
		t.Fatalf("stale review was not blocked: %s", got)
	}
	if n := f.count(t, "abandoned-review"); n != 0 {
		t.Fatalf("stale review row was not cleaned: %d", n)
	}
	var kind string
	if err := f.store.db.QueryRowContext(ctx, `SELECT kind FROM work_events WHERE item_id='abandoned-review' AND to_status='blocked' ORDER BY id DESC LIMIT 1`).Scan(&kind); err != nil {
		t.Fatal(err)
	}
	if kind != "review_abandoned" {
		t.Fatalf("unexpected recovery event kind: %q", kind)
	}
}

func TestREQ_WAUTH_003_ScopedRecoverOwnerMatch(t *testing.T) {
	ctx := context.Background()
	f := newRecoveryFixture(t)
	f.create(t, "owned-one")
	f.create(t, "owned-two")
	if _, err := f.store.ClaimWorkWithLeases(ctx, "owned-one", "implement-owner", []string{"src/a.go"}, 2*time.Minute); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.ClaimWorkWithLeases(ctx, "owned-two", "implement-owner", []string{"src/b.go"}, 2*time.Minute); err != nil {
		t.Fatal(err)
	}
	f.advance(3 * time.Minute)

	recovered, err := f.store.RecoverWorkScoped(ctx, "owned-one", "implement-owner")
	if err != nil {
		t.Fatal(err)
	}
	if recovered != 1 {
		t.Fatalf("scoped recover recovered %d tasks, want 1", recovered)
	}
	if got := f.status(t, "owned-one"); got != WorkBlocked {
		t.Fatalf("scoped task not blocked: %s", got)
	}
	if got := f.status(t, "owned-two"); got != WorkInProgress {
		t.Fatalf("scoped recover touched a foreign task: %s", got)
	}
	if n := f.rows(t, "work_claims", "owned-one"); n != 0 {
		t.Fatalf("scoped recover left %d claim rows", n)
	}
	if n := f.rows(t, "work_leases", "owned-one"); n != 0 {
		t.Fatalf("scoped recover left %d lease rows", n)
	}
}

func TestREQ_WAUTH_003_ScopedRecoverOwnerMismatchFailsClosed(t *testing.T) {
	ctx := context.Background()
	f := newRecoveryFixture(t)
	f.create(t, "owned")
	if _, err := f.store.ClaimWorkWithLeases(ctx, "owned", "implement-owner", []string{"src/a.go"}, 2*time.Minute); err != nil {
		t.Fatal(err)
	}
	f.advance(3 * time.Minute)

	_, err := f.store.RecoverWorkScoped(ctx, "owned", "implement-impostor")
	assertScopedDenied(t, err, "owner_mismatch")
	if got := f.status(t, "owned"); got != WorkInProgress {
		t.Fatalf("denied recover mutated status: %s", got)
	}
	if n := f.rows(t, "work_claims", "owned"); n != 1 {
		t.Fatalf("denied recover dropped the claim: %d", n)
	}
	if n := f.rows(t, "work_leases", "owned"); n != 1 {
		t.Fatalf("denied recover dropped the lease: %d", n)
	}
}

func TestREQ_WAUTH_003_ScopedRecoverNotExpiredFailsClosed(t *testing.T) {
	ctx := context.Background()
	f := newRecoveryFixture(t)
	f.create(t, "fresh")
	if _, err := f.store.ClaimWorkWithLeases(ctx, "fresh", "implement-owner", []string{"src/a.go"}, time.Hour); err != nil {
		t.Fatal(err)
	}

	_, err := f.store.RecoverWorkScoped(ctx, "fresh", "implement-owner")
	assertScopedDenied(t, err, "claim_not_expired")
	if got := f.status(t, "fresh"); got != WorkInProgress {
		t.Fatalf("denied recover mutated status: %s", got)
	}
}

func TestREQ_WAUTH_003_ScopedRecoverTaskNotFoundFailsClosed(t *testing.T) {
	f := newRecoveryFixture(t)
	_, err := f.store.RecoverWorkScoped(context.Background(), "ghost", "implement-owner")
	assertScopedDenied(t, err, "task_not_found")
}

func TestREQ_WAUTH_003_ScopedRecoverFlagValidation(t *testing.T) {
	cases := []struct {
		name    string
		scope   RecoverScope
		wantErr bool
	}{
		{name: "unscoped", scope: RecoverScope{}},
		{name: "task_only", scope: RecoverScope{TaskID: "t"}, wantErr: true},
		{name: "owner_only", scope: RecoverScope{Owner: "o"}, wantErr: true},
		{name: "both", scope: RecoverScope{TaskID: "t", Owner: "o"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.scope.Validate(); (err != nil) != tc.wantErr {
				t.Fatalf("Validate(%+v) err = %v, wantErr = %v", tc.scope, err, tc.wantErr)
			}
			if tc.scope.Scoped() != (tc.scope.TaskID != "") {
				t.Fatalf("Scoped() disagrees with TaskID for %+v", tc.scope)
			}
		})
	}
}
