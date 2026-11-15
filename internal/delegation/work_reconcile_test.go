package delegation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

type reconcileFixture struct {
	store *Store
	clock time.Time
}

func newReconcileFixture(t *testing.T) *reconcileFixture {
	t.Helper()
	t.Setenv("CORTEX_IA_HOME", t.TempDir())
	store, err := OpenStore(filepath.Join(t.TempDir(), "delegation.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	f := &reconcileFixture{store: store, clock: time.Now().UTC()}
	store.now = func() time.Time { return f.clock }
	return f
}

func (f *reconcileFixture) advance(d time.Duration) { f.clock = f.clock.Add(d) }

func (f *reconcileFixture) create(t *testing.T, id string) {
	t.Helper()
	if _, err := f.store.CreateWorkInBoardWithDefinition(context.Background(), DefaultBoardID, id, id, nil, WorkDefinition{
		Project: t.TempDir(), Objective: "objective", Acceptance: "acceptance", Verification: "verification", AllowedFiles: []string{"src/a.go"},
	}); err != nil {
		t.Fatalf("create %s: %v", id, err)
	}
}

func (f *reconcileFixture) claim(t *testing.T, id, owner string, ttl time.Duration, paths ...string) WorkClaimReservation {
	t.Helper()
	res, err := f.store.ClaimWorkWithLeases(context.Background(), id, owner, paths, ttl)
	if err != nil {
		t.Fatalf("claim %s: %v", id, err)
	}
	return res
}

func (f *reconcileFixture) staleForeignClaim(t *testing.T, id string, paths ...string) (string, int64) {
	t.Helper()
	f.create(t, id)
	res := f.claim(t, id, "opencode-session:dead-session", time.Hour, paths...)
	f.advance(workReconcileStaleWindow + 5*time.Minute)
	return id, res.Revision
}

func (f *reconcileFixture) fingerprint(t *testing.T, id string) string {
	var fp string
	if err := f.store.db.QueryRowContext(context.Background(), `SELECT COALESCE((SELECT status FROM work_items WHERE id=?),'-')||'/'||COALESCE((SELECT revision FROM work_items WHERE id=?),0)||'/'||(SELECT COUNT(*) FROM work_claims WHERE item_id=?)||'/'||(SELECT COUNT(*) FROM work_leases WHERE item_id=?)||'/'||(SELECT COUNT(*) FROM work_events WHERE item_id=?)`, id, id, id, id, id).Scan(&fp); err != nil {
		t.Fatalf("fingerprint %s: %v", id, err)
	}
	return fp
}

func assertReconcileRefusal(t *testing.T, err error, want []string) {
	t.Helper()
	var refusal *WorkReconcileRefusalError
	if !errors.As(err, &refusal) || !errors.Is(err, ErrWorkReconcileRefused) {
		t.Fatalf("expected typed *WorkReconcileRefusalError unwrapping ErrWorkReconcileRefused, got %v", err)
	}
	if got := refusal.FailedConditionCodes(); !slices.Equal(got, want) {
		t.Fatalf("failed conditions = %v, want %v", got, want)
	}
}

func assertNoTokenMaterial(t *testing.T, res WorkClaimReservation, outputs ...string) {
	blob := strings.Join(outputs, "\n")
	secrets := []string{res.Token, tokenHash(res.Token)}
	for _, lease := range res.ReservedFiles {
		secrets = append(secrets, lease.Token, tokenHash(lease.Token))
	}
	for _, secret := range secrets {
		if strings.Contains(blob, secret) {
			t.Fatal("token material leaked into audit output")
		}
	}
}

func (f *reconcileFixture) refusalScenario(t *testing.T, kind string) (string, ReconcileInput) {
	t.Helper()
	base := func(id string, rev int64) ReconcileInput {
		return ReconcileInput{TaskID: id, Reason: "release reason", HostSessionID: "orch", ExpectedRevision: rev}
	}
	claimed := func(id, owner string, ttl time.Duration) (string, ReconcileInput) {
		f.create(t, id)
		return id, base(id, f.claim(t, id, owner, ttl).Revision)
	}
	switch kind {
	case "ghost":
		return "ghost", base("ghost", 1)
	case "ready":
		f.create(t, "ready-task")
		return "ready-task", base("ready-task", 1)
	case "expired":
		id, in := claimed("expired", "opencode-session:dead-session", time.Minute)
		f.advance(5 * time.Minute)
		return id, in
	case "self":
		return claimed("self-owned", "opencode-session:orch", time.Hour)
	case "fresh":
		return claimed("fresh", "opencode-session:dead-session", time.Hour)
	}
	id, rev := f.staleForeignClaim(t, "orphan-"+kind)
	in := base(id, rev)
	switch kind {
	case "reason_blank":
		in.Reason = "   "
	case "reason_long":
		in.Reason = strings.Repeat("x", maxReconcileReasonBytes+1)
	case "rev_plus1":
		in.ExpectedRevision = rev + 1
	case "rev_zero":
		in.ExpectedRevision = 0
	}
	return id, in
}

func TestReconcileWorkRefusalsMutateNothing(t *testing.T) {
	tests := []struct {
		name string
		kind string
		want []string
	}{
		{"reason_missing", "reason_blank", []string{reconcileConditionReasonMissing}},
		{"reason_too_long", "reason_long", []string{reconcileConditionReasonTooLong}},
		{"task_not_found", "ghost", []string{reconcileConditionTaskNotFound}},
		{"status_not_in_progress", "ready", []string{reconcileConditionStatusNotInProgress, reconcileConditionClaimNotLive}},
		{"claim_not_live_expired", "expired", []string{reconcileConditionClaimNotLive}},
		{"revision_mismatch", "rev_plus1", []string{reconcileConditionRevisionMismatch}},
		{"revision_unset", "rev_zero", []string{reconcileConditionRevisionMismatch}},
		{"current_session_owner", "self", []string{reconcileConditionCurrentSessionOwner}},
		{"fresh_foreign_claim_without_evidence", "fresh", []string{reconcileConditionFreshNoEvidence}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newReconcileFixture(t)
			id, input := f.refusalScenario(t, tc.kind)
			before := f.fingerprint(t, id)
			_, err := f.store.ReconcileWork(context.Background(), input)
			assertReconcileRefusal(t, err, tc.want)
			if after := f.fingerprint(t, id); after != before {
				t.Fatalf("refusal mutated durable state: before=%s after=%s", before, after)
			}
		})
	}
}

func TestReconcileWorkReleasesStaleOrphanedClaim(t *testing.T) {
	f := newReconcileFixture(t)
	const reason = "controller died with lost token"
	f.create(t, "released")
	res := f.claim(t, "released", "opencode-session:dead-session", time.Hour, "src/b.go", "src/a.go")
	f.advance(workReconcileStaleWindow + 5*time.Minute)
	result, err := f.store.ReconcileWork(context.Background(), ReconcileInput{
		TaskID: "released", Reason: reason, HostSessionID: "orch", ExpectedRevision: res.Revision,
	})
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	wantLeases := []string{"src/a.go", "src/b.go"}
	if result.Decision != "released" || result.From != WorkInProgress || result.To != WorkBlocked ||
		result.RevisionBefore != res.Revision || result.RevisionAfter != res.Revision+1 || !slices.Equal(result.ReleasedLeases, wantLeases) {
		t.Fatalf("unexpected receipt: %+v", result)
	}
	if after := f.fingerprint(t, "released"); !strings.HasPrefix(after, fmt.Sprintf("blocked/%d/0/0/", res.Revision+1)) {
		t.Fatalf("release did not atomically clear claim/leases: %s", after)
	}
	count, detail := 0, ""
	if err := f.store.db.QueryRowContext(context.Background(), `SELECT COUNT(*),COALESCE(MAX(detail),'') FROM work_events WHERE item_id='released' AND kind='reconciled'`).Scan(&count, &detail); err != nil {
		t.Fatalf("read reconciled events: %v", err)
	}
	var snapshot ReconcileDecisionSnapshot
	if err := json.Unmarshal([]byte(detail), &snapshot); err != nil || count != 1 {
		t.Fatalf("want exactly one reconciled decision snapshot, got %d (%v)", count, err)
	}
	if snapshot.ActorSession != "orch" || snapshot.Reason != reason || snapshot.ClaimOwner != "opencode-session:dead-session" || snapshot.ClaimAttempt != 1 || snapshot.LastRenewedAt == "" ||
		snapshot.StalenessWindowMillis != workReconcileStaleWindow.Milliseconds() || snapshot.OwnerSessionInactive || snapshot.RevisionBefore != res.Revision || snapshot.RevisionAfter != res.Revision+1 || !slices.Equal(snapshot.ReleasedLeases, wantLeases) {
		t.Fatalf("decision snapshot incomplete: %+v", snapshot)
	}
	receipt, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	assertNoTokenMaterial(t, res, detail, string(receipt))
	readyID, readyRev := f.staleForeignClaim(t, "ready-target", "src/c.go")
	ready, err := f.store.ReconcileWork(context.Background(), ReconcileInput{TaskID: readyID, Reason: "orphaned", HostSessionID: "orch", ExpectedRevision: readyRev, To: WorkReady})
	if err != nil || ready.To != WorkReady || !strings.HasPrefix(f.fingerprint(t, readyID), "ready/") {
		t.Fatalf("ready release failed: %+v / %v", ready, err)
	}
}

func TestReconcileWorkAttemptLimitGuard(t *testing.T) {
	ctx := context.Background()
	f := newReconcileFixture(t)
	f.create(t, "limit")
	res := f.claim(t, "limit", "opencode-session:dead-session", time.Hour, "src/a.go")
	for i := 0; i < int(MaxWorkAttempts)-1; i++ {
		if _, err := f.store.db.ExecContext(ctx, `INSERT INTO work_events(item_id,kind,from_status,to_status,detail,created_at) VALUES(?,?,?,?,?,?)`,
			"limit", "claimed", string(WorkReady), string(WorkInProgress), "seed", f.store.timestamp()); err != nil {
			t.Fatalf("seed attempt event: %v", err)
		}
	}
	f.advance(workReconcileStaleWindow + 5*time.Minute)
	before := f.fingerprint(t, "limit")
	_, err := f.store.ReconcileWork(ctx, ReconcileInput{
		TaskID: "limit", Reason: "cap reached", HostSessionID: "orch", ExpectedRevision: res.Revision, To: WorkReady,
	})
	assertReconcileRefusal(t, err, []string{reconcileConditionAttemptLimit})
	if after := f.fingerprint(t, "limit"); after != before {
		t.Fatalf("attempt-limit refusal mutated state: before=%s after=%s", before, after)
	}
	if result, err := f.store.ReconcileWork(ctx, ReconcileInput{
		TaskID: "limit", Reason: "cap reached", HostSessionID: "orch", ExpectedRevision: res.Revision, To: WorkBlocked,
	}); err != nil || result.To != WorkBlocked || !strings.HasPrefix(f.fingerprint(t, "limit"), "blocked/") {
		t.Fatalf("blocked release failed at the cap: %+v / %v", result, err)
	}
}

func TestReconcileWorkAtomicRollbackOnAuditFailure(t *testing.T) {
	f := newReconcileFixture(t)
	id, rev := f.staleForeignClaim(t, "atomic", "src/a.go")
	// The audit event is written after the destructive deletes; aborting it must roll the whole transaction back.
	if _, err := f.store.db.ExecContext(context.Background(), `CREATE TRIGGER fail_reconcile_audit BEFORE INSERT ON work_events WHEN NEW.kind='reconciled' BEGIN SELECT RAISE(ABORT,'audit failure'); END`); err != nil {
		t.Fatalf("install audit trigger: %v", err)
	}
	before := f.fingerprint(t, id)
	if _, err := f.store.ReconcileWork(context.Background(), ReconcileInput{
		TaskID: id, Reason: "atomic", HostSessionID: "orch", ExpectedRevision: rev,
	}); err == nil {
		t.Fatal("expected the audit failure to abort the reconcile")
	}
	if after := f.fingerprint(t, id); after != before {
		t.Fatalf("aborted reconcile left partial mutations: before=%s after=%s", before, after)
	}
}
