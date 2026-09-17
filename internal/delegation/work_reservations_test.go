package delegation

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestReservationTransactionRollback(t *testing.T) {
	ctx := context.Background()
	s, err := OpenStore(filepath.Join(t.TempDir(), "delegation.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	for _, id := range []string{"holder", "claimant", "batch"} {
		if _, err := s.CreateWork(ctx, id, id, nil); err != nil {
			t.Fatal(err)
		}
	}
	holder, err := s.ClaimWorkWithLeases(ctx, "holder", "holder-owner", []string{"z.go"}, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if holder.ReservedFiles[0].Token == "" {
		t.Fatal("missing lease token")
	}
	var eventsBefore int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM work_events WHERE item_id='claimant'`).Scan(&eventsBefore); err != nil {
		t.Fatal(err)
	}
	failed, err := s.ClaimWorkWithLeases(ctx, "claimant", "claimant-owner", []string{"a.go", "z.go"}, time.Minute)
	if !errors.Is(err, ErrWorkConflict) || failed.Token != "" || len(failed.ReservedFiles) != 0 {
		t.Fatal("conflict must not return authority")
	}
	var claims, leases, eventsAfter int
	var status string
	if err := s.db.QueryRowContext(ctx, `SELECT status FROM work_items WHERE id='claimant'`).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM work_claims WHERE item_id='claimant'`).Scan(&claims); err != nil {
		t.Fatal(err)
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM work_leases WHERE item_id='claimant'`).Scan(&leases); err != nil {
		t.Fatal(err)
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM work_events WHERE item_id='claimant'`).Scan(&eventsAfter); err != nil {
		t.Fatal(err)
	}
	if status != "ready" || claims != 0 || leases != 0 || eventsBefore != eventsAfter {
		t.Fatal("failed acquisition changed durable authority or events")
	}
	claim, err := s.ClaimWork(ctx, "batch", "batch-owner", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ReserveWorkLeases(ctx, "batch", claim.Token, []string{"b.go", "z.go"}, time.Minute); !errors.Is(err, ErrWorkConflict) {
		t.Fatal("expected conflicting batch")
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM work_leases WHERE item_id='batch'`).Scan(&leases); err != nil {
		t.Fatal(err)
	}
	if leases != 0 {
		t.Fatal("batch leaked a partial lease")
	}
	paths := []string{"c.go", "b.go"}
	got, err := s.ReserveWorkLeases(ctx, "batch", claim.Token, paths, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Path != "b.go" || got[1].Path != "c.go" || !reflect.DeepEqual(paths, []string{"c.go", "b.go"}) {
		t.Fatal("reservations must be sorted without mutating caller input")
	}
	for _, bad := range [][]string{{"d.go", "../escape"}, {"d.go", "./d.go"}, {}} {
		if _, err := s.ReserveWorkLeases(ctx, "batch", claim.Token, bad, time.Minute); err == nil {
			t.Fatal("invalid batch accepted")
		}
	}
	if _, err := s.ReserveWorkLeases(ctx, "batch", "invalid-token", []string{"d.go"}, time.Minute); !errors.Is(err, ErrWorkConflict) {
		t.Fatal("invalid claim accepted")
	}
}
