package delegation

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func degradeFixture(t *testing.T) (*Store, time.Time) {
	t.Helper()
	s, err := OpenStore(filepath.Join(t.TempDir(), "delegation.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	base := time.Now().UTC()
	s.now = func() time.Time { return base }
	return s, base
}

func seedReady(t *testing.T, s *Store, id string) {
	t.Helper()
	if _, err := s.CreateWorkInBoardWithDefinition(context.Background(), DefaultBoardID, id, id, nil,
		WorkDefinition{Project: t.TempDir(), AllowedFiles: []string{"src/a.go"}}); err != nil {
		t.Fatal(err)
	}
}

func seedBlocked(t *testing.T, s *Store, id, reason string) {
	t.Helper()
	ctx := context.Background()
	seedReady(t, s, id)
	claim, err := s.ClaimWorkWithLeases(ctx, id, "impl", []string{"src/a.go"}, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.TransitionWork(ctx, id, claim.Token, claim.Revision, WorkBlocked,
		WorkSubmissionInput{Summary: "blocked", BlockedReason: reason}); err != nil {
		t.Fatal(err)
	}
}

func ageTask(t *testing.T, s *Store, id string, at time.Time) {
	t.Helper()
	if _, err := s.db.Exec(`UPDATE work_items SET updated_at=? WHERE id=?`, at.UTC().Format(time.RFC3339Nano), id); err != nil {
		t.Fatal(err)
	}
}

func itemState(t *testing.T, s *Store, id string) (WorkStatus, string, int64) {
	t.Helper()
	item, err := s.GetWork(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return item.Status, item.BlockedReason, item.Revision
}

func TestREQ_DEGRADE_001_ReadyStaleDegrades(t *testing.T) {
	s, base := degradeFixture(t)
	seedReady(t, s, "r1")
	ageTask(t, s, "r1", base.Add(-200*time.Hour))

	res, err := s.DegradeStaleWork(context.Background(), DegradeStaleWorkOptions{})
	if err != nil {
		t.Fatal(err)
	}
	status, _, revision := itemState(t, s, "r1")
	if status != WorkBacklog || revision != 2 {
		t.Fatalf("ready stale => status %s rev %d, want backlog rev 2 (created at rev 1)", status, revision)
	}
	if res.Degraded != 1 || len(res.ByReason["ready"]) != 1 || res.ByReason["ready"][0] != "r1" {
		t.Fatalf("receipt = %+v", res)
	}
}

func TestREQ_DEGRADE_002_NeedsUserExempt(t *testing.T) {
	s, base := degradeFixture(t)
	seedBlocked(t, s, "b-needs", WorkBlockedReasonNeedsUser)
	ageTask(t, s, "b-needs", base.Add(-200*time.Hour))

	res, err := s.DegradeStaleWork(context.Background(), DegradeStaleWorkOptions{})
	if err != nil {
		t.Fatal(err)
	}
	status, reason, _ := itemState(t, s, "b-needs")
	if status != WorkBlocked || reason != WorkBlockedReasonNeedsUser {
		t.Fatalf("needs_user => status %s reason %q, want blocked unchanged", status, reason)
	}
	if res.Degraded != 0 || len(res.Exempted) != 1 || res.Exempted[0] != "b-needs" {
		t.Fatalf("receipt = %+v, want needs_user exempt", res)
	}
}

func TestREQ_DEGRADE_003_BlockedOtherClassDegrades(t *testing.T) {
	s, base := degradeFixture(t)
	seedBlocked(t, s, "b-up", WorkBlockedReasonUpstream)
	ageTask(t, s, "b-up", base.Add(-200*time.Hour))

	res, err := s.DegradeStaleWork(context.Background(), DegradeStaleWorkOptions{})
	if err != nil {
		t.Fatal(err)
	}
	status, reason, _ := itemState(t, s, "b-up")
	if status != WorkBacklog || reason != "" {
		t.Fatalf("blocked upstream => status %s reason %q, want backlog cleared reason", status, reason)
	}
	if res.Degraded != 1 || len(res.ByReason[WorkBlockedReasonUpstream]) != 1 {
		t.Fatalf("receipt = %+v, want upstream degrade", res)
	}
}

func TestREQ_DEGRADE_004_LiveClaimUntouched(t *testing.T) {
	s, base := degradeFixture(t)
	seedReady(t, s, "r-live")
	stamp := base.UTC().Format(time.RFC3339Nano)
	future := base.Add(time.Hour).UTC().Format(time.RFC3339Nano)
	if _, err := s.db.Exec(`INSERT INTO work_claims(item_id,owner,token_hash,attempt,expires_at,created_at,updated_at) VALUES('r-live','owner','hash',1,?,?,?)`, future, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`INSERT INTO work_leases(path,item_id,token_hash,expires_at,created_at,updated_at) VALUES('src/a.go','r-live','hash',?,?,?)`, future, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	ageTask(t, s, "r-live", base.Add(-200*time.Hour))

	res, err := s.DegradeStaleWork(context.Background(), DegradeStaleWorkOptions{})
	if err != nil {
		t.Fatal(err)
	}
	status, _, revision := itemState(t, s, "r-live")
	if status != WorkReady || revision != 1 {
		t.Fatalf("live-claim task => status %s rev %d, want ready rev 1 (unchanged)", status, revision)
	}
	if res.Degraded != 0 {
		t.Fatalf("receipt = %+v, want zero degradations", res)
	}
}

func TestREQ_DEGRADE_005_FreshTaskUntouched(t *testing.T) {
	s, _ := degradeFixture(t)
	seedReady(t, s, "r-fresh")

	res, err := s.DegradeStaleWork(context.Background(), DegradeStaleWorkOptions{})
	if err != nil {
		t.Fatal(err)
	}
	status, _, revision := itemState(t, s, "r-fresh")
	if status != WorkReady || revision != 1 || res.Degraded != 0 {
		t.Fatalf("fresh task => status %s rev %d degraded %d, want untouched", status, revision, res.Degraded)
	}
}

func TestREQ_DEGRADE_006_ReceiptCountsAndDryRun(t *testing.T) {
	s, base := degradeFixture(t)
	seedReady(t, s, "r1")
	seedBlocked(t, s, "b-up", WorkBlockedReasonUpstream)
	seedBlocked(t, s, "b-env", WorkBlockedReasonEnv)
	seedBlocked(t, s, "b-legacy", "")
	seedBlocked(t, s, "b-user", WorkBlockedReasonNeedsUser)
	seedReady(t, s, "r-fresh")
	for _, id := range []string{"r1", "b-up", "b-env", "b-legacy", "b-user"} {
		ageTask(t, s, id, base.Add(-200*time.Hour))
	}
	ctx := context.Background()

	dry, err := s.DegradeStaleWork(ctx, DegradeStaleWorkOptions{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if !dry.DryRun || dry.Degraded != 4 {
		t.Fatalf("dry-run receipt = %+v, want 4 degradable", dry)
	}
	if status, _, _ := itemState(t, s, "r1"); status != WorkReady {
		t.Fatalf("dry-run mutated r1 => %s", status)
	}

	real, err := s.DegradeStaleWork(ctx, DegradeStaleWorkOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if real.Degraded != 4 {
		t.Fatalf("real degraded = %d, want 4", real.Degraded)
	}
	for class, want := range map[string]string{
		"ready":                       "r1",
		WorkBlockedReasonUpstream:     "b-up",
		WorkBlockedReasonEnv:          "b-env",
		WorkBlockedReasonUnclassified: "b-legacy",
	} {
		if got := real.ByReason[class]; len(got) != 1 || got[0] != want {
			t.Fatalf("by_reason[%s] = %v, want [%s]", class, got, want)
		}
	}
	if len(real.Exempted) != 1 || real.Exempted[0] != "b-user" {
		t.Fatalf("exempted = %v, want [b-user]", real.Exempted)
	}
	if status, _, _ := itemState(t, s, "r-fresh"); status != WorkReady {
		t.Fatalf("fresh task degraded => %s", status)
	}
}
