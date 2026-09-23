package delegation

import (
	"context"
	"path/filepath"
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
	t.Helper()
	var n int
	if err := f.store.db.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM work_reviews WHERE item_id=?`, itemID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
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
